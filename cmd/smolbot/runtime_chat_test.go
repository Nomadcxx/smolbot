package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/agent"
	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/gateway"
	"github.com/Nomadcxx/smolbot/pkg/provider"
	"github.com/Nomadcxx/smolbot/pkg/usage"
	"github.com/gorilla/websocket"
)

func TestGatewayChatRoundTrip(t *testing.T) {
	port := freePort(t)
	cfgPath := writeTestConfig(t, port)

	fakeProvider := &fakeRuntimeProvider{
		deltas: []*provider.StreamDelta{
			{Content: "hello from runtime"},
			{FinishReason: stringPtr("stop")},
		},
	}

	app, err := buildRuntime(daemonLaunchOptions{
		ConfigPath: cfgPath,
		Port:       port,
	}, runtimeDeps{
		Provider: fakeProvider,
	})
	if err != nil {
		t.Fatalf("buildRuntime: %v", err)
	}
	defer app.Close()

	httpServer := httptest.NewServer(app.gateway.Handler())
	defer httpServer.Close()

	conn := dialRuntimeWebsocket(t, httpServer.URL+"/ws")
	defer conn.Close()

	params, err := json.Marshal(map[string]any{
		"session": "s1",
		"message": "hello",
	})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	writeRuntimeFrame(t, conn, gateway.RequestFrame{
		ID:     "chat-1",
		Method: "chat.send",
		Params: params,
	})

	response := readRuntimeFrame(t, conn)
	if response.Kind != gateway.FrameResponse || !strings.Contains(string(response.Response.Result), `"runId":"run-s1"`) {
		t.Fatalf("unexpected chat.send response %#v", response)
	}

	done := readRuntimeEvent(t, conn, "chat.done")
	if !strings.Contains(string(done.Event.Payload), `"content":"hello from runtime"`) {
		t.Fatalf("unexpected chat.done event %#v", done)
	}

	if len(fakeProvider.requests) == 0 {
		t.Fatal("expected provider request to be recorded")
	}
	if got := fakeProvider.requests[0].Messages[len(fakeProvider.requests[0].Messages)-1].StringContent(); !strings.Contains(got, "hello") {
		t.Fatalf("unexpected provider request content %q", got)
	}

	history, err := app.sessions.GetHistory("s1", 20)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) == 0 {
		t.Fatal("expected persisted session history")
	}
}

func TestRunChatMessageUsesInProcessRuntime(t *testing.T) {
	origHome := os.Getenv("HOME")
	origDeps := runChatRuntimeDeps
	defer func() {
		_ = os.Setenv("HOME", origHome)
		runChatRuntimeDeps = origDeps
	}()

	home := t.TempDir()
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("Setenv HOME: %v", err)
	}
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(home, ".nanobot", "workspace")
	if err := writeConfigFile(filepath.Join(home, ".nanobot", "config.json"), &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	runChatRuntimeDeps = func() runtimeDeps {
		return runtimeDeps{
			Provider: &fakeRuntimeProvider{
				deltas: []*provider.StreamDelta{
					{Content: "hello from cli"},
					{FinishReason: stringPtr("stop")},
				},
			},
		}
	}

	output, err := runChatMessage(context.Background(), chatRequest{
		Session: "cli-session",
		Message: "hello",
	})
	if err != nil {
		t.Fatalf("runChatMessage: %v", err)
	}
	if output != "hello from cli" {
		t.Fatalf("unexpected output %q", output)
	}
}

func TestBuildRuntimeWiresUsageStoreIntoAgentAndGateway(t *testing.T) {
	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Provider = "openai"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	app, err := buildRuntime(daemonLaunchOptions{
		ConfigPath: cfgPath,
		Port:       port,
	}, runtimeDeps{
		Provider: &fakeRuntimeProvider{
			deltas: []*provider.StreamDelta{
				{Content: "usage integrated"},
				{Usage: &provider.Usage{PromptTokens: 12, CompletionTokens: 8, TotalTokens: 20}},
				{FinishReason: stringPtr("stop")},
			},
		},
	})
	if err != nil {
		t.Fatalf("buildRuntime: %v", err)
	}
	defer app.Close()

	if _, err := app.agent.ProcessDirect(context.Background(), agent.Request{
		SessionKey: "s1",
		Content:    "hello",
	}, nil); err != nil {
		t.Fatalf("ProcessDirect: %v", err)
	}
	sessionResetsAt := time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC)
	weeklyResetsAt := time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC)
	if err := app.usage.SaveQuotaSummary(context.Background(), usage.QuotaSummary{
		ProviderID:         "openai",
		PlanName:           "pro",
		SessionUsedPercent: 2,
		SessionResetsAt:    &sessionResetsAt,
		WeeklyUsedPercent:  26.5,
		WeeklyResetsAt:     &weeklyResetsAt,
		State:              usage.QuotaStateLive,
		Source:             usage.QuotaSourceOllamaSettingsHTML,
		FetchedAt:          time.Date(2026, 3, 27, 23, 0, 0, 0, time.UTC),
		ExpiresAt:          time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("SaveQuotaSummary: %v", err)
	}

	if _, err := os.Stat(app.paths.UsageDB()); err != nil {
		t.Fatalf("usage db stat: %v", err)
	}

	httpServer := httptest.NewServer(app.gateway.Handler())
	defer httpServer.Close()

	conn := dialWebsocketGateway(t, httpServer.URL+"/ws")
	defer conn.Close()

	params, err := json.Marshal(map[string]any{"session": "s1"})
	if err != nil {
		t.Fatalf("Marshal status params: %v", err)
	}
	writeFrameGateway(t, conn, gateway.RequestFrame{
		ID:     "status-1",
		Method: "status",
		Params: params,
	})
	frame := readResponseFrameGateway(t, conn, "status-1")
	if frame.Kind != gateway.FrameResponse || frame.Response.Error != nil {
		t.Fatalf("status failed: %#v", frame)
	}

	var payload struct {
		PersistedUsage struct {
			ProviderID    string `json:"providerId"`
			ModelName     string `json:"modelName"`
			SessionTokens int    `json:"sessionTokens"`
			TodayTokens   int    `json:"todayTokens"`
			WeeklyTokens  int    `json:"weeklyTokens"`
			Quota         struct {
				PlanName           string  `json:"planName"`
				SessionUsedPercent float64 `json:"sessionUsedPercent"`
				WeeklyUsedPercent  float64 `json:"weeklyUsedPercent"`
				State              string  `json:"state"`
				Source             string  `json:"source"`
			} `json:"quota"`
		} `json:"persistedUsage"`
	}
	if err := json.Unmarshal(frame.Response.Result, &payload); err != nil {
		t.Fatalf("Unmarshal status payload: %v", err)
	}
	if payload.PersistedUsage.ProviderID != "openai" {
		t.Fatalf("providerId = %q, want openai", payload.PersistedUsage.ProviderID)
	}
	if payload.PersistedUsage.ModelName != "gpt-test" {
		t.Fatalf("modelName = %q, want gpt-test", payload.PersistedUsage.ModelName)
	}
	if payload.PersistedUsage.SessionTokens != 20 || payload.PersistedUsage.TodayTokens != 20 || payload.PersistedUsage.WeeklyTokens != 20 {
		t.Fatalf("unexpected persisted usage summary: %#v", payload.PersistedUsage)
	}
	if payload.PersistedUsage.Quota.PlanName != "pro" || payload.PersistedUsage.Quota.SessionUsedPercent != 2 || payload.PersistedUsage.Quota.WeeklyUsedPercent != 26.5 {
		t.Fatalf("unexpected persisted quota summary: %#v", payload.PersistedUsage.Quota)
	}
	if payload.PersistedUsage.Quota.State != string(usage.QuotaStateLive) || payload.PersistedUsage.Quota.Source != string(usage.QuotaSourceOllamaSettingsHTML) {
		t.Fatalf("unexpected persisted quota state: %#v", payload.PersistedUsage.Quota)
	}
}

type fakeRuntimeProvider struct {
	mu            sync.Mutex
	deltas        []*provider.StreamDelta
	requests      []provider.ChatRequest
	chatResponses []*provider.Response
	chatRequests  []provider.ChatRequest
}

func (f *fakeRuntimeProvider) Chat(_ context.Context, req provider.ChatRequest) (*provider.Response, error) {
	f.mu.Lock()
	f.chatRequests = append(f.chatRequests, req)
	if len(f.chatResponses) == 0 {
		f.mu.Unlock()
		return nil, io.EOF
	}
	resp := f.chatResponses[0]
	f.chatResponses = f.chatResponses[1:]
	f.mu.Unlock()
	return resp, nil
}

func (f *fakeRuntimeProvider) ChatStream(_ context.Context, req provider.ChatRequest) (*provider.Stream, error) {
	f.mu.Lock()
	f.requests = append(f.requests, req)
	deltas := append([]*provider.StreamDelta(nil), f.deltas...)
	f.mu.Unlock()

	idx := 0
	return provider.NewStream(func() (*provider.StreamDelta, error) {
		if idx >= len(deltas) {
			return nil, io.EOF
		}
		delta := deltas[idx]
		idx++
		return delta, nil
	}, func() error { return nil }), nil
}

func (f *fakeRuntimeProvider) Name() string { return "openai" }

func writeRuntimeFrame(t *testing.T, conn *websocket.Conn, frame gateway.RequestFrame) {
	t.Helper()
	data, err := gateway.EncodeRequest(frame)
	if err != nil {
		t.Fatalf("EncodeRequest: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
}

func readRuntimeFrame(t *testing.T, conn *websocket.Conn) *gateway.DecodedFrame {
	t.Helper()
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("ReadMessage: %v", err)
	}
	frame, err := gateway.DecodeFrame(data)
	if err != nil {
		t.Fatalf("DecodeFrame: %v", err)
	}
	return frame
}

func readRuntimeEvent(t *testing.T, conn *websocket.Conn, name string) *gateway.DecodedFrame {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	_ = conn.SetReadDeadline(deadline)
	for {
		frame := readRuntimeFrame(t, conn)
		if frame.Kind == gateway.FrameEvent && frame.Event.EventName == name {
			return frame
		}
	}
}

func dialRuntimeWebsocket(t *testing.T, rawURL string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(rawURL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	return conn
}

func stringPtr(value string) *string {
	return &value
}
