package gateway

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nomadcxx/nanobot-go/pkg/agent"
	"github.com/Nomadcxx/nanobot-go/pkg/channel"
	"github.com/Nomadcxx/nanobot-go/pkg/config"
	"github.com/Nomadcxx/nanobot-go/pkg/provider"
	"github.com/Nomadcxx/nanobot-go/pkg/session"
	"github.com/gorilla/websocket"
)

func TestChatAbortVerifiesSessionAndRunId(t *testing.T) {
	store, err := session.NewStore(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	cfg := &config.Config{}
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Provider = "openai"
	channels := channel.NewManager()
	channels.Register(&fakeChannel{name: "slack"})
	agentStub := &fakeAgentProcessor{response: "done"}

	server := NewServer(ServerDeps{
		Agent:    agentStub,
		Sessions: store,
		Channels: channels,
		Config:   cfg,
		Version:  "test",
	})

	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	conn := dialWebsocketServer(t, httpServer.URL+"/ws")
	defer conn.Close()

	helloServer(t, conn)

	writeFrameServer(t, conn, RequestFrame{ID: "1", Method: "chat.send", Params: json.RawMessage(`{"session":"s1","message":"hello"}`)})
	readResponseFrameServer(t, conn, "1")

	if len(agentStub.cancelled) != 0 {
		t.Fatalf("agent cancelled before abort")
	}

	writeFrameServer(t, conn, RequestFrame{
		ID:     "2",
		Method: "chat.abort",
		Params: json.RawMessage(`{"runId":"nonexistent-run"}`),
	})
	frame := readResponseFrameServer(t, conn, "2")
	if frame.Kind != FrameResponse {
		t.Fatalf("expected response frame")
	}
	if frame.Response.Error == nil {
		t.Fatalf("chat.abort with bad runId should return error")
	}

	if len(agentStub.cancelled) != 0 {
		t.Fatalf("agent cancelled for nonexistent runId")
	}
}

func TestChatHistoryResponseShape(t *testing.T) {
	store, err := session.NewStore(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	if err := store.SaveMessages("s1", []provider.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "world"},
		{Role: "user", Content: "follow up"},
	}); err != nil {
		t.Fatalf("SaveMessages: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Provider = "openai"
	channels := channel.NewManager()
	agentStub := &fakeAgentProcessor{response: "done"}

	server := NewServer(ServerDeps{
		Agent:    agentStub,
		Sessions: store,
		Channels: channels,
		Config:   cfg,
		Version:  "test",
	})

	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	conn := dialWebsocketServer(t, httpServer.URL+"/ws")
	defer conn.Close()

	helloServer(t, conn)

	writeFrameServer(t, conn, RequestFrame{
		ID:     "1",
		Method: "chat.history",
		Params: json.RawMessage(`{"session":"s1","limit":2}`),
	})
	frame := readResponseFrameServer(t, conn, "1")
	if frame.Kind != FrameResponse || frame.Response.Error != nil {
		t.Fatalf("unexpected response: %#v", frame)
	}

	var items []map[string]any
	if err := json.Unmarshal(frame.Response.Result, &items); err != nil {
		t.Fatalf("Unmarshal result: %v", err)
	}

	if len(items) > 2 {
		t.Fatalf("chat.history with limit=2 returned %d items, want at most 2", len(items))
	}

	for _, item := range items {
		if item["role"] == nil || item["content"] == nil {
			t.Fatalf("history item missing role or content: %#v", item)
		}
	}
}

func TestToolEventPayloadsAreRich(t *testing.T) {
	store, err := session.NewStore(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	cfg := &config.Config{}
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Provider = "openai"
	channels := channel.NewManager()
	channels.Register(&fakeChannel{name: "slack"})

	eventSpy := &eventSpyAgent{}
	server := NewServer(ServerDeps{
		Agent:    eventSpy,
		Sessions: store,
		Channels: channels,
		Config:   cfg,
		Version:  "test",
	})

	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	conn := dialWebsocketServer(t, httpServer.URL+"/ws")
	defer conn.Close()

	helloServer(t, conn)

	writeFrameServer(t, conn, RequestFrame{
		ID:     "1",
		Method: "chat.send",
		Params: json.RawMessage(`{"session":"s1","message":"use a tool"}`),
	})

	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		var wf map[string]any
		if err := json.Unmarshal(data, &wf); err != nil {
			continue
		}
		if wf["type"] == "response" && wf["id"] == "1" {
			break
		}
	}

	if len(eventSpy.toolStartCalls) == 0 {
		t.Fatalf("chat.tool.start was never emitted")
	}
	if len(eventSpy.toolDoneCalls) == 0 {
		t.Fatalf("chat.tool.done was never emitted")
	}
	if eventSpy.toolStartCalls[0].input == "" {
		t.Fatalf("chat.tool.start payload missing input field")
	}
	if eventSpy.toolDoneCalls[0].output == "" {
		t.Fatalf("chat.tool.done payload missing output field")
	}
}

func TestStatusChannelStatesReturnsDetail(t *testing.T) {
	store, err := session.NewStore(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	cfg := &config.Config{}
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Provider = "openai"
	channels := channel.NewManager()
	channels.Register(&fakeChannel{name: "signal"})
	agentStub := &fakeAgentProcessor{response: "done"}

	server := NewServer(ServerDeps{
		Agent:    agentStub,
		Sessions: store,
		Channels: channels,
		Config:   cfg,
		Version:  "test",
	})

	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	conn := dialWebsocketServer(t, httpServer.URL+"/ws")
	defer conn.Close()

	helloServer(t, conn)

	writeFrameServer(t, conn, RequestFrame{ID: "1", Method: "status"})
	frame := readResponseFrameServer(t, conn, "1")
	if frame.Kind != FrameResponse || frame.Response.Error != nil {
		t.Fatalf("unexpected response: %#v", frame)
	}

	var result map[string]any
	if err := json.Unmarshal(frame.Response.Result, &result); err != nil {
		t.Fatalf("Unmarshal result: %v", err)
	}

	channelStates, ok := result["channelStates"].(map[string]any)
	if !ok {
		t.Fatalf("status response missing channelStates: %#v", result)
	}

	signalState, ok := channelStates["signal"].(map[string]any)
	if !ok {
		t.Fatalf("channelStates[signal] is not a map (flat string instead of structured): %#v", channelStates["signal"])
	}
	_ = signalState
}

func TestEventFrameHasCorrectStructure(t *testing.T) {
	frame, err := EncodeEvent(EventFrame{
		Name:  "chat.progress",
		Seq:   1,
		Event: json.RawMessage(`{"content":"working..."}`),
	})
	if err != nil {
		t.Fatalf("EncodeEvent: %v", err)
	}

	var wf wireFrame
	if err := json.Unmarshal(frame, &wf); err != nil {
		t.Fatalf("Unmarshal wire frame: %v", err)
	}

	if wf.Type != FrameEvent {
		t.Fatalf("event frame type = %q, want %q", wf.Type, FrameEvent)
	}
	if wf.Name != "chat.progress" {
		t.Fatalf("event frame name = %q, want %q", wf.Name, "chat.progress")
	}
	if wf.Seq == 0 {
		t.Fatalf("event frame seq = 0, want nonzero")
	}
}

func TestRequestFrameHasCorrectStructure(t *testing.T) {
	frame, err := EncodeRequest(RequestFrame{
		ID:     "1",
		Method: "chat.send",
		Params: json.RawMessage(`{"session":"s1","message":"hello"}`),
	})
	if err != nil {
		t.Fatalf("EncodeRequest: %v", err)
	}

	var wf wireFrame
	if err := json.Unmarshal(frame, &wf); err != nil {
		t.Fatalf("Unmarshal wire frame: %v", err)
	}

	if wf.Type != FrameRequest {
		t.Fatalf("request frame type = %q, want %q", wf.Type, FrameRequest)
	}
	if wf.ID != "1" {
		t.Fatalf("request frame id = %q, want %q", wf.ID, "1")
	}
	if wf.Method != "chat.send" {
		t.Fatalf("request frame method = %q, want %q", wf.Method, "chat.send")
	}
}

func TestResponseFrameHasCorrectStructure(t *testing.T) {
	frame, err := EncodeResponse(ResponseFrame{
		ID:     "1",
		Result: json.RawMessage(`{"runId":"run-1"}`),
	})
	if err != nil {
		t.Fatalf("EncodeResponse: %v", err)
	}

	var wf wireFrame
	if err := json.Unmarshal(frame, &wf); err != nil {
		t.Fatalf("Unmarshal wire frame: %v", err)
	}

	if wf.Type != FrameResponse {
		t.Fatalf("response frame type = %q, want %q", wf.Type, FrameResponse)
	}
	if wf.ID != "1" {
		t.Fatalf("response frame id = %q, want %q", wf.ID, "1")
	}
}

type toolStartInfo struct {
	name  string
	input string
}

type toolDoneInfo struct {
	name   string
	output string
}

type eventSpyAgent struct {
	toolStartCalls []toolStartInfo
	toolDoneCalls  []toolDoneInfo
}

func (a *eventSpyAgent) ProcessDirect(_ context.Context, req agent.Request, cb agent.EventCallback) (string, error) {
	if cb != nil {
		startInput := `{"arg":"value"}`
		startEvent := agent.Event{Type: agent.EventToolStart, Content: "test_tool", Data: map[string]any{"input": startInput}}
		doneEvent := agent.Event{Type: agent.EventToolDone, Content: "test_tool", Data: map[string]any{"output": "result"}}
		a.toolStartCalls = append(a.toolStartCalls, toolStartInfo{name: startEvent.Content, input: startInput})
		a.toolDoneCalls = append(a.toolDoneCalls, toolDoneInfo{name: doneEvent.Content, output: "result"})
		cb(startEvent)
		cb(doneEvent)
	}
	return "done", nil
}

func (a *eventSpyAgent) CancelSession(string) {}

func dialWebsocketServer(t *testing.T, rawURL string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + rawURL[strings.Index(rawURL, ":"):]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	return conn
}

func helloServer(t *testing.T, conn *websocket.Conn) {
	writeFrameServer(t, conn, RequestFrame{ID: "h", Method: "hello"})
	readResponseFrameServer(t, conn, "h")
}

func writeFrameServer(t *testing.T, conn *websocket.Conn, req RequestFrame) {
	t.Helper()
	data, err := EncodeRequest(req)
	if err != nil {
		t.Fatalf("EncodeRequest: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
}

func readResponseFrameServer(t *testing.T, conn *websocket.Conn, id string) *DecodedFrame {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	_ = conn.SetReadDeadline(deadline)
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage: %v", err)
		}
		frame, err := DecodeFrame(data)
		if err != nil {
			t.Fatalf("DecodeFrame: %v", err)
		}
		if frame.Kind == FrameResponse && frame.Response.ID == id {
			return frame
		}
	}
}
