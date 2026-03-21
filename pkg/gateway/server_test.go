package gateway

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/agent"
	"github.com/Nomadcxx/smolbot/pkg/channel"
	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/provider"
	"github.com/Nomadcxx/smolbot/pkg/session"
	"github.com/gorilla/websocket"
)

func TestServerMethods(t *testing.T) {
	store, err := session.NewStore(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()
	if err := store.SaveMessages("s1", []provider.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "world"},
	}); err != nil {
		t.Fatalf("SaveMessages: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Provider = "openai"
	channels := channel.NewManager()
	channels.Register(&fakeChannel{name: "slack"})
	agentStub := &fakeAgentProcessor{response: "done"}

	server := NewServer(ServerDeps{
		Agent:     agentStub,
		Sessions:  store,
		Channels:  channels,
		Config:    cfg,
		Version:   "test",
		StartedAt: time.Now().Add(-2 * time.Minute),
	})

	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	conn := dialWebsocket(t, httpServer.URL+"/ws")
	defer conn.Close()

	t.Run("hello", func(t *testing.T) {
		writeFrame(t, conn, RequestFrame{ID: "1", Method: "hello"})
		frame := readFrame(t, conn)
		if frame.Kind != FrameResponse || frame.Response.Error != nil {
			t.Fatalf("unexpected hello response %#v", frame)
		}
		if !strings.Contains(string(frame.Response.Result), `"server":"smolbot"`) {
			t.Fatalf("unexpected hello payload %s", frame.Response.Result)
		}
	})

	t.Run("status", func(t *testing.T) {
		writeFrame(t, conn, RequestFrame{ID: "2", Method: "status"})
		frame := readFrame(t, conn)
		if !strings.Contains(string(frame.Response.Result), `"connectedClients":1`) {
			t.Fatalf("expected connected client count, got %s", frame.Response.Result)
		}
		if !strings.Contains(string(frame.Response.Result), `"model":"gpt-test"`) {
			t.Fatalf("expected model in status, got %s", frame.Response.Result)
		}
		if !strings.Contains(string(frame.Response.Result), `"channelStates":{"slack":{"detail":"","state":"connected"}}`) {
			t.Fatalf("expected channel states in status, got %s", frame.Response.Result)
		}
	})

	t.Run("chat history", func(t *testing.T) {
		writeFrame(t, conn, RequestFrame{
			ID:     "3",
			Method: "chat.history",
			Params: json.RawMessage(`{"session":"s1"}`),
		})
		frame := readResponseFrame(t, conn, "3")
		if !strings.Contains(string(frame.Response.Result), `"role":"user"`) || !strings.Contains(string(frame.Response.Result), `"content":"world"`) {
			t.Fatalf("unexpected history payload %s", frame.Response.Result)
		}
	})

	t.Run("chat send decodes media", func(t *testing.T) {
		payload := map[string]any{
			"session": "s2",
			"message": "describe this",
			"channel": "slack",
			"chatID":  "C1",
			"media": []map[string]any{
				{
					"mimeType": "text/plain",
					"data":     base64.StdEncoding.EncodeToString([]byte("asset")),
				},
			},
		}
		raw, _ := json.Marshal(payload)
		writeFrame(t, conn, RequestFrame{ID: "4", Method: "chat.send", Params: raw})
		frame := readResponseFrame(t, conn, "4")
		if !strings.Contains(string(frame.Response.Result), `"runId":"run-s2"`) {
			t.Fatalf("unexpected chat.send payload %s", frame.Response.Result)
		}
		if agentStub.lastReq.Content != "describe this" || len(agentStub.lastReq.Media) != 1 || string(agentStub.lastReq.Media[0].Data) != "asset" {
			t.Fatalf("unexpected decoded agent request %#v", agentStub.lastReq)
		}
	})

	t.Run("sessions list", func(t *testing.T) {
		writeFrame(t, conn, RequestFrame{ID: "5", Method: "sessions.list"})
		frame := readResponseFrame(t, conn, "5")
		if !strings.Contains(string(frame.Response.Result), `"key":"s1"`) {
			t.Fatalf("unexpected sessions payload %s", frame.Response.Result)
		}
	})

	t.Run("sessions reset", func(t *testing.T) {
		writeFrame(t, conn, RequestFrame{
			ID:     "6",
			Method: "sessions.reset",
			Params: json.RawMessage(`{"session":"s1"}`),
		})
		frame := readResponseFrame(t, conn, "6")
		if frame.Response.Error != nil {
			t.Fatalf("unexpected reset error %#v", frame.Response.Error)
		}
		history, err := store.GetHistory("s1", 50)
		if err != nil {
			t.Fatalf("GetHistory: %v", err)
		}
		if len(history) != 0 {
			t.Fatalf("expected cleared history, got %#v", history)
		}
	})

	t.Run("models list and set", func(t *testing.T) {
		writeFrame(t, conn, RequestFrame{ID: "7", Method: "models.list"})
		frame := readResponseFrame(t, conn, "7")
		if !strings.Contains(string(frame.Response.Result), `"id":"gpt-test"`) {
			t.Fatalf("unexpected models payload %s", frame.Response.Result)
		}

		writeFrame(t, conn, RequestFrame{
			ID:     "8",
			Method: "models.set",
			Params: json.RawMessage(`{"model":"claude-test"}`),
		})
		frame = readResponseFrame(t, conn, "8")
		if frame.Response.Error != nil {
			t.Fatalf("unexpected set error %#v", frame.Response.Error)
		}
		if cfg.Agents.Defaults.Model != "claude-test" {
			t.Fatalf("expected model update, got %q", cfg.Agents.Defaults.Model)
		}
	})
}

func TestHealthEndpoint(t *testing.T) {
	server := NewServer(ServerDeps{})
	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()

	resp, err := http.Get(httpServer.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}
}

type fakeAgentProcessor struct {
	lastReq   agent.Request
	response  string
	cancelled []string
}

func (f *fakeAgentProcessor) ProcessDirect(_ context.Context, req agent.Request, _ agent.EventCallback) (string, error) {
	f.lastReq = req
	return f.response, nil
}

func (f *fakeAgentProcessor) CancelSession(sessionKey string) {
	f.cancelled = append(f.cancelled, sessionKey)
}

type fakeChannel struct{ name string }

func (f *fakeChannel) Name() string                                 { return f.name }
func (f *fakeChannel) Start(context.Context, channel.Handler) error { return nil }
func (f *fakeChannel) Stop(context.Context) error                   { return nil }
func (f *fakeChannel) Send(context.Context, channel.OutboundMessage) error {
	return nil
}
func (f *fakeChannel) Status(context.Context) (channel.Status, error) {
	return channel.Status{State: "connected"}, nil
}

func dialWebsocket(t *testing.T, rawURL string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(rawURL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	return conn
}

func writeFrame(t *testing.T, conn *websocket.Conn, req RequestFrame) {
	t.Helper()
	data, err := EncodeRequest(req)
	if err != nil {
		t.Fatalf("EncodeRequest: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
}

func readFrame(t *testing.T, conn *websocket.Conn) *DecodedFrame {
	t.Helper()
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	frame, err := DecodeFrame(data)
	if err != nil {
		t.Fatalf("DecodeFrame: %v", err)
	}
	return frame
}

func readResponseFrame(t *testing.T, conn *websocket.Conn, id string) *DecodedFrame {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	_ = conn.SetReadDeadline(deadline)
	for {
		frame := readFrame(t, conn)
		if frame.Kind == FrameResponse && frame.Response.ID == id {
			return frame
		}
	}
}
