package gateway

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/agent"
	"github.com/Nomadcxx/smolbot/pkg/channel"
	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/cron"
	"github.com/Nomadcxx/smolbot/pkg/provider"
	"github.com/Nomadcxx/smolbot/pkg/session"
	"github.com/Nomadcxx/smolbot/pkg/skill"
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
		{Role: "user", Content: "more context"},
		{Role: "assistant", Content: "more reply"},
	}); err != nil {
		t.Fatalf("SaveMessages: %v", err)
	}
	if err := store.SaveMessages("s3", []provider.Message{
		{Role: "user", Content: "short"},
		{Role: "assistant", Content: "short reply"},
	}); err != nil {
		t.Fatalf("SaveMessages s3: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Provider = "openai"
	cfg.Agents.Defaults.Compression.Enabled = true
	cfg.Tools.MCPServers = map[string]config.MCPServerConfig{
		"filesystem": {Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem"}},
	}
	channels := channel.NewManager()
	channels.Register(&fakeChannel{name: "slack"})
	agentStub := &fakeAgentProcessor{response: "done", compactOriginal: 12000, compactCompressed: 7000, compactPct: 42}
	skills, err := skill.NewBuiltinRegistry()
	if err != nil {
		t.Fatalf("NewBuiltinRegistry: %v", err)
	}

	server := NewServer(ServerDeps{
		Agent:     agentStub,
		Sessions:  store,
		Channels:  channels,
		Config:    cfg,
		Skills:    skills,
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
		if !strings.Contains(string(frame.Response.Result), `"cron.list"`) {
			t.Fatalf("expected cron.list in hello payload %s", frame.Response.Result)
		}
	})

	t.Run("status", func(t *testing.T) {
		writeFrame(t, conn, RequestFrame{ID: "2", Method: "status"})
		frame := readFrame(t, conn)
		if !strings.Contains(string(frame.Response.Result), `"model":"gpt-test"`) {
			t.Fatalf("expected model in status, got %s", frame.Response.Result)
		}
		if !strings.Contains(string(frame.Response.Result), `"uptime":`) {
			t.Fatalf("expected uptime in status, got %s", frame.Response.Result)
		}
		var payload struct {
			Usage struct {
				ContextWindow int `json:"contextWindow"`
			} `json:"usage"`
		}
		if err := json.Unmarshal(frame.Response.Result, &payload); err != nil {
			t.Fatalf("unmarshal status payload: %v", err)
		}
		if payload.Usage.ContextWindow != cfg.Agents.Defaults.ContextWindowTokens {
			t.Fatalf("expected fallback context window %d, got %d", cfg.Agents.Defaults.ContextWindowTokens, payload.Usage.ContextWindow)
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

	t.Run("cron list with no cron service returns empty jobs", func(t *testing.T) {
		writeFrame(t, conn, RequestFrame{ID: "9", Method: "cron.list"})
		frame := readResponseFrame(t, conn, "9")
		if frame.Response.Error != nil {
			t.Fatalf("unexpected cron.list error %#v", frame.Response.Error)
		}
		var payload struct {
			Jobs []any `json:"jobs"`
		}
		if err := json.Unmarshal(frame.Response.Result, &payload); err != nil {
			t.Fatalf("unmarshal cron payload: %v", err)
		}
		if len(payload.Jobs) != 0 {
			t.Fatalf("expected empty cron list, got %#v", payload.Jobs)
		}
	})

	t.Run("compact", func(t *testing.T) {
		if err := store.SaveMessages("s1", []provider.Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "world"},
			{Role: "user", Content: "more context"},
			{Role: "assistant", Content: "more reply"},
		}); err != nil {
			t.Fatalf("reseeding s1: %v", err)
		}
		writeFrame(t, conn, RequestFrame{
			ID:     "9",
			Method: "compact",
			Params: json.RawMessage(`{"session":"s1"}`),
		})
		frame := readResponseFrame(t, conn, "9")
		if !strings.Contains(string(frame.Response.Result), `"compacted":true`) {
			t.Fatalf("unexpected compact payload %s", frame.Response.Result)
		}
		if !strings.Contains(string(frame.Response.Result), `"session":"s1"`) {
			t.Fatalf("expected session in compact payload %s", frame.Response.Result)
		}
		if agentStub.compactedSession != "s1" {
			t.Fatalf("expected compact to target session s1, got %q", agentStub.compactedSession)
		}
	})

	t.Run("compact no-op is explicit and uses fallback session", func(t *testing.T) {
		callsBefore := agentStub.compactCalls
		resp, err := server.handleRequest(context.Background(), &clientState{sessionKey: "s3"}, RequestFrame{
			ID:     "9b",
			Method: "compact",
		})
		if err != nil {
			t.Fatalf("handleRequest compact: %v", err)
		}
		payload, ok := resp.(map[string]any)
		if !ok {
			t.Fatalf("unexpected payload type %T", resp)
		}
		if got := payload["session"]; got != "s3" {
			t.Fatalf("expected fallback session s3, got %#v", got)
		}
		if got := payload["compacted"]; got != false {
			t.Fatalf("expected no-op compaction to be explicit, got %#v", got)
		}
		if got := payload["reason"]; got != "not enough history" {
			t.Fatalf("expected no-op reason, got %#v", got)
		}
		if agentStub.compactCalls != callsBefore {
			t.Fatalf("expected compact agent not to be called for no-op, got %d -> %d", callsBefore, agentStub.compactCalls)
		}
	})

	t.Run("compact no-reduction still emits done payload", func(t *testing.T) {
		agentStub.compactOriginal = 0
		agentStub.compactCompressed = 0
		agentStub.compactPct = 0
		defer func() {
			agentStub.compactOriginal = 12000
			agentStub.compactCompressed = 7000
			agentStub.compactPct = 42
		}()
		resp, err := server.handleRequest(context.Background(), &clientState{sessionKey: "s1"}, RequestFrame{
			ID:     "9c",
			Method: "compact",
		})
		if err != nil {
			t.Fatalf("handleRequest compact no-reduction: %v", err)
		}
		payload, ok := resp.(map[string]any)
		if !ok {
			t.Fatalf("unexpected payload type %T", resp)
		}
		if got := payload["compacted"]; got != false {
			t.Fatalf("expected explicit no-op, got %#v", got)
		}
		if got := payload["reason"]; got != "no reduction achieved" {
			t.Fatalf("expected no-reduction reason, got %#v", got)
		}
	})

	t.Run("skills list", func(t *testing.T) {
		writeFrame(t, conn, RequestFrame{ID: "10", Method: "skills.list"})
		frame := readResponseFrame(t, conn, "10")
		if !strings.Contains(string(frame.Response.Result), `"skills"`) {
			t.Fatalf("unexpected skills payload %s", frame.Response.Result)
		}
	})

	t.Run("mcps list", func(t *testing.T) {
		writeFrame(t, conn, RequestFrame{ID: "11", Method: "mcps.list"})
		frame := readResponseFrame(t, conn, "11")
		if !strings.Contains(string(frame.Response.Result), `"name":"filesystem"`) {
			t.Fatalf("unexpected mcps payload %s", frame.Response.Result)
		}
	})
}

func TestServerOllamaContextWindow(t *testing.T) {
	var requestCount atomic.Int32
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		switch r.URL.Path {
		case "/api/ps":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{
					{
						"name":           "qwen3:8b",
						"model":          "qwen3:8b",
						"context_length": 131072,
					},
				},
			})
		case "/api/show":
			t.Fatalf("did not expect /api/show fallback when /api/ps matched")
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer ollama.Close()

	store, err := session.NewStore(filepath.Join(t.TempDir(), "sessions.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	cfg := &config.Config{}
	cfg.Agents.Defaults.Model = "ollama/qwen3:8b"
	cfg.Agents.Defaults.Provider = "ollama"
	cfg.Agents.Defaults.ContextWindowTokens = 200000
	cfg.Providers = map[string]config.ProviderConfig{
		"ollama": {APIBase: ollama.URL},
	}

	agentStub := &fakeAgentProcessor{
		response: "done",
		events: []agent.Event{
			{
				Type: agent.EventUsage,
				Data: map[string]any{
					"promptTokens":     12,
					"completionTokens": 34,
					"totalTokens":      56,
				},
			},
		},
	}

	server := NewServer(ServerDeps{
		Agent:     agentStub,
		Sessions:  store,
		Config:    cfg,
		StartedAt: time.Now().Add(-time.Minute),
	})

	statusResp, err := server.handleRequest(context.Background(), &clientState{}, RequestFrame{
		ID:     "1",
		Method: "status",
	})
	if err != nil {
		t.Fatalf("handleRequest status: %v", err)
	}
	status, ok := statusResp.(map[string]any)
	if !ok {
		t.Fatalf("unexpected status payload type %T", statusResp)
	}
	usage, ok := status["usage"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected usage payload type %T", status["usage"])
	}
	if got := usage["contextWindow"]; got != 131072 {
		t.Fatalf("expected detected ollama context window 131072, got %#v", got)
	}

	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	conn := dialWebsocket(t, httpServer.URL+"/ws")
	defer conn.Close()

	writeFrame(t, conn, RequestFrame{
		ID:     "2",
		Method: "chat.send",
		Params: json.RawMessage(`{"session":"s1","message":"hello"}`),
	})

	frame := readEventFrame(t, conn, "chat.usage")
	var usagePayload struct {
		PromptTokens     int `json:"promptTokens"`
		CompletionTokens int `json:"completionTokens"`
		TotalTokens      int `json:"totalTokens"`
		ContextWindow    int `json:"contextWindow"`
	}
	if err := json.Unmarshal(frame.Event.Payload, &usagePayload); err != nil {
		t.Fatalf("unmarshal chat.usage payload: %v", err)
	}
	if usagePayload.ContextWindow != 131072 {
		t.Fatalf("expected detected ollama context window in chat.usage, got %d", usagePayload.ContextWindow)
	}
	if usagePayload.TotalTokens != 56 {
		t.Fatalf("unexpected usage payload %#v", usagePayload)
	}
	if got := requestCount.Load(); got != 1 {
		t.Fatalf("expected cached Ollama lookup to hit server once, got %d", got)
	}
}

func TestServerOllamaContextWindowTimeoutFallsBackQuickly(t *testing.T) {
	var requestCount atomic.Int32
	ollama := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		<-r.Context().Done()
	}))
	defer ollama.Close()

	cfg := &config.Config{}
	cfg.Agents.Defaults.Model = "ollama/qwen3:8b"
	cfg.Agents.Defaults.Provider = "ollama"
	cfg.Agents.Defaults.ContextWindowTokens = 200000
	cfg.Providers = map[string]config.ProviderConfig{
		"ollama": {APIBase: ollama.URL},
	}

	server := NewServer(ServerDeps{Config: cfg, StartedAt: time.Now().Add(-time.Minute)})

	start := time.Now()
	resp, err := server.handleRequest(context.Background(), &clientState{}, RequestFrame{
		ID:     "1",
		Method: "status",
	})
	if err != nil {
		t.Fatalf("handleRequest status: %v", err)
	}
	payload, ok := resp.(map[string]any)
	if !ok {
		t.Fatalf("unexpected status payload type %T", resp)
	}
	usage, ok := payload["usage"].(map[string]any)
	if !ok {
		t.Fatalf("unexpected usage payload type %T", payload["usage"])
	}
	if got := usage["contextWindow"]; got != cfg.Agents.Defaults.ContextWindowTokens {
		t.Fatalf("expected fallback context window %d, got %#v", cfg.Agents.Defaults.ContextWindowTokens, got)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("expected bounded Ollama lookup, took %s", elapsed)
	}
	if got := requestCount.Load(); got != 1 {
		t.Fatalf("expected a single timed-out lookup, got %d", got)
	}
}

func TestCronListMapsJobs(t *testing.T) {
	server := NewServer(ServerDeps{
		Cron: &fakeCronLister{
			jobs: []cron.Job{
				{
					ID:       "job-1",
					Name:     "Daily cleanup",
					Schedule: "every 5m",
					Enabled:  true,
					NextRun:  time.Date(2026, 3, 27, 10, 0, 0, 0, time.UTC),
				},
				{
					ID:       "job-2",
					Name:     "Paused sync",
					Schedule: "daily 02:00",
					Enabled:  false,
				},
			},
		},
	})

	httpServer := httptest.NewServer(server.Handler())
	defer httpServer.Close()
	conn := dialWebsocket(t, httpServer.URL+"/ws")
	defer conn.Close()

	writeFrame(t, conn, RequestFrame{ID: "1", Method: "cron.list"})
	frame := readResponseFrame(t, conn, "1")
	if frame.Response.Error != nil {
		t.Fatalf("unexpected cron.list error %#v", frame.Response.Error)
	}

	var payload struct {
		Jobs []struct {
			ID       string `json:"id"`
			Name     string `json:"name"`
			Schedule string `json:"schedule"`
			Status   string `json:"status"`
			NextRun  string `json:"nextRun"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(frame.Response.Result, &payload); err != nil {
		t.Fatalf("unmarshal cron payload: %v", err)
	}
	if len(payload.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %#v", payload.Jobs)
	}
	if payload.Jobs[0].Status != "active" || payload.Jobs[0].NextRun == "" {
		t.Fatalf("expected active job with next run, got %#v", payload.Jobs[0])
	}
	if payload.Jobs[1].Status != "paused" {
		t.Fatalf("expected paused job, got %#v", payload.Jobs[1])
	}
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
	lastReq           agent.Request
	response          string
	events            []agent.Event
	cancelled         []string
	compactedSession  string
	compactOriginal   int
	compactCompressed int
	compactPct        float64
	compactCalls      int
}

func (f *fakeAgentProcessor) ProcessDirect(_ context.Context, req agent.Request, cb agent.EventCallback) (string, error) {
	f.lastReq = req
	for _, event := range f.events {
		if cb != nil {
			cb(event)
		}
	}
	return f.response, nil
}

func (f *fakeAgentProcessor) CancelSession(sessionKey string) {
	f.cancelled = append(f.cancelled, sessionKey)
}

func (f *fakeAgentProcessor) CompactNow(_ context.Context, sessionKey string) (int, int, float64, error) {
	f.compactedSession = sessionKey
	f.compactCalls++
	return f.compactOriginal, f.compactCompressed, f.compactPct, nil
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

type fakeCronLister struct {
	jobs []cron.Job
}

func (f *fakeCronLister) ListJobs() []cron.Job {
	return append([]cron.Job(nil), f.jobs...)
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

func readEventFrame(t *testing.T, conn *websocket.Conn, name string) *DecodedFrame {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	_ = conn.SetReadDeadline(deadline)
	for {
		frame := readFrame(t, conn)
		if frame.Kind == FrameEvent && frame.Event.EventName == name {
			return frame
		}
	}
}
