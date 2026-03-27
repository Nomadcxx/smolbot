package gateway

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/pkg/agent"
	"github.com/Nomadcxx/smolbot/pkg/channel"
	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/provider"
	"github.com/Nomadcxx/smolbot/pkg/session"
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

	var envelope struct {
		Messages []map[string]any `json:"messages"`
	}
	if err := json.Unmarshal(frame.Response.Result, &envelope); err != nil {
		t.Fatalf("Unmarshal result: %v", err)
	}

	if len(envelope.Messages) > 2 {
		t.Fatalf("chat.history with limit=2 returned %d items, want at most 2", len(envelope.Messages))
	}

	for _, item := range envelope.Messages {
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

	channelsRaw, ok := result["channels"].([]any)
	if !ok {
		t.Fatalf("status response missing channels array: %#v", result)
	}
	if len(channelsRaw) == 0 {
		t.Fatalf("expected at least one channel, got 0")
	}
	ch, ok := channelsRaw[0].(map[string]any)
	if !ok {
		t.Fatalf("channels[0] is not an object: %#v", channelsRaw[0])
	}
	if ch["name"] != "signal" {
		t.Fatalf("expected channel name 'signal', got %v", ch["name"])
	}
}

func TestEventFrameHasCorrectStructure(t *testing.T) {
	frame, err := EncodeEvent(EventFrame{
		EventName: "chat.progress",
		Seq:       1,
		Payload:   json.RawMessage(`{"content":"working..."}`),
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
	if wf.Event != "chat.progress" {
		t.Fatalf("event frame name = %q, want %q", wf.Event, "chat.progress")
	}
	if wf.Seq == 0 {
		t.Fatalf("event frame seq = 0, want nonzero")
	}
}

func TestDelegatedAgentPayloadsRoundTrip(t *testing.T) {
	t.Run("spawned", func(t *testing.T) {
		want := client.AgentSpawnedPayload{
			ID:              "child-1",
			Name:            "Bernoulli",
			AgentType:       "explorer",
			Model:           "gpt-5.4 high",
			ReasoningEffort: "high",
			Description:     "Spec review Gate 6",
			PromptPreview:   "Review ONLY the Gate 6 changes.",
		}
		raw, err := json.Marshal(want)
		if err != nil {
			t.Fatalf("marshal spawned payload: %v", err)
		}
		var got client.AgentSpawnedPayload
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("unmarshal spawned payload: %v", err)
		}
		if got != want {
			t.Fatalf("spawned payload mismatch: got %#v want %#v", got, want)
		}
	})

	t.Run("completed", func(t *testing.T) {
		want := client.AgentCompletedPayload{
			ID:            "child-1",
			Name:          "Bernoulli",
			AgentType:     "explorer",
			Status:        "completed",
			Description:   "Spec review Gate 6",
			PromptPreview: "Review ONLY the Gate 6 changes.",
			Summary:       "✅ Spec compliant",
		}
		raw, err := json.Marshal(want)
		if err != nil {
			t.Fatalf("marshal completed payload: %v", err)
		}
		var got client.AgentCompletedPayload
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("unmarshal completed payload: %v", err)
		}
		if got != want {
			t.Fatalf("completed payload mismatch: got %#v want %#v", got, want)
		}
	})

	t.Run("wait_started", func(t *testing.T) {
		want := client.AgentWaitStartedPayload{
			Count: 3,
			Agents: []client.AgentWaitAgent{
				{ID: "child-1", Name: "Bernoulli", AgentType: "explorer"},
				{ID: "child-2", Name: "Averroes", AgentType: "explorer"},
				{ID: "child-3", Name: "Curie", AgentType: "explorer"},
			},
		}
		raw, err := json.Marshal(want)
		if err != nil {
			t.Fatalf("marshal wait-started payload: %v", err)
		}
		var got client.AgentWaitStartedPayload
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("unmarshal wait-started payload: %v", err)
		}
		if got.Count != want.Count {
			t.Fatalf("wait-started count mismatch: got %#v want %#v", got, want)
		}
		if len(got.Agents) != len(want.Agents) {
			t.Fatalf("wait-started payload mismatch: got %#v want %#v", got, want)
		}
		for i := range want.Agents {
			if got.Agents[i] != want.Agents[i] {
				t.Fatalf("wait-started agent[%d] mismatch: got %#v want %#v", i, got.Agents[i], want.Agents[i])
			}
		}
	})

	t.Run("wait_completed", func(t *testing.T) {
		want := client.AgentWaitCompletedPayload{
			Count: 2,
			Results: []client.AgentWaitResult{
				{
					ID:            "child-1",
					Name:          "Bernoulli",
					AgentType:     "explorer",
					Status:        "completed",
					Description:   "Spec review Gate 6",
					PromptPreview: "Review ONLY the Gate 6 changes.",
					Summary:       "✅ Spec compliant",
				},
				{
					ID:            "child-2",
					Name:          "Averroes",
					AgentType:     "explorer",
					Status:        "completed",
					Description:   "Code-quality review Gate 6",
					PromptPreview: "Review ONLY script changes.",
					Summary:       "✅ Approved",
				},
			},
		}
		raw, err := json.Marshal(want)
		if err != nil {
			t.Fatalf("marshal wait-completed payload: %v", err)
		}
		var got client.AgentWaitCompletedPayload
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("unmarshal wait-completed payload: %v", err)
		}
		if got.Count != want.Count {
			t.Fatalf("wait-completed count mismatch: got %#v want %#v", got, want)
		}
		if len(got.Results) != len(want.Results) {
			t.Fatalf("wait-completed payload mismatch: got %#v want %#v", got, want)
		}
		for i := range want.Results {
			if got.Results[i] != want.Results[i] {
				t.Fatalf("wait-completed result[%d] mismatch: got %#v want %#v", i, got.Results[i], want.Results[i])
			}
		}
	})
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
