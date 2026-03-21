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

	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/gateway"
	"github.com/Nomadcxx/smolbot/pkg/provider"
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
