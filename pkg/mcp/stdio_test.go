package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

var (
	mockServerOnce sync.Once
	mockServerPath string
	mockServerErr  error
)

func TestJSONRPCNotificationOmitsID(t *testing.T) {
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	if strings.Contains(string(data), `"id"`) {
		t.Fatalf("notification should omit id field, got %s", data)
	}
}

func TestJSONRPCIDSupportsNumberAndString(t *testing.T) {
	var numeric struct {
		ID jsonRPCID `json:"id"`
	}
	if err := json.Unmarshal([]byte(`{"id":1}`), &numeric); err != nil {
		t.Fatalf("unmarshal numeric id: %v", err)
	}
	if string(numeric.ID) != "1" {
		t.Fatalf("unexpected numeric id %q", numeric.ID)
	}

	var str struct {
		ID jsonRPCID `json:"id"`
	}
	if err := json.Unmarshal([]byte(`{"id":"req-1"}`), &str); err != nil {
		t.Fatalf("unmarshal string id: %v", err)
	}
	if string(str.ID) != `"req-1"` {
		t.Fatalf("unexpected string id %q", str.ID)
	}
}

func TestStdioTransportRoundTrip(t *testing.T) {
	transport := newMockTransport(t)
	t.Cleanup(func() {
		_ = transport.Close()
	})

	result, err := transport.Send(context.Background(), "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "smolbot", "version": "test"},
	})
	if err != nil {
		t.Fatalf("initialize: %v", err)
	}
	var initResult mcpInitResult
	if err := json.Unmarshal(result, &initResult); err != nil {
		t.Fatalf("unmarshal initialize result: %v", err)
	}
	if initResult.ServerInfo.Name != "mock-mcp" {
		t.Fatalf("server info = %#v", initResult.ServerInfo)
	}

	if err := transport.Notify(context.Background(), "notifications/initialized", nil); err != nil {
		t.Fatalf("notify: %v", err)
	}

	listResult, err := transport.Send(context.Background(), "tools/list", map[string]any{})
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	var tools mcpToolsListResult
	if err := json.Unmarshal(listResult, &tools); err != nil {
		t.Fatalf("unmarshal tools/list result: %v", err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "echo" {
		t.Fatalf("unexpected tools result %#v", tools)
	}

	callResult, err := transport.Send(context.Background(), "tools/call", map[string]any{
		"name":      "echo",
		"arguments": json.RawMessage(`{"text":"hello"}`),
	})
	if err != nil {
		t.Fatalf("tools/call: %v", err)
	}
	var call mcpCallResult
	if err := json.Unmarshal(callResult, &call); err != nil {
		t.Fatalf("unmarshal tools/call result: %v", err)
	}
	if call.IsError {
		t.Fatalf("expected success call result, got %#v", call)
	}
	if len(call.Content) != 1 || call.Content[0].Text != "echoed: hello" {
		t.Fatalf("unexpected call result %#v", call)
	}
}

func TestStdioTransportContextCancel(t *testing.T) {
	transport := newMockTransport(t, mockServerEnv("MOCK_MCP_DELAY_LIST", "1")...)
	t.Cleanup(func() {
		_ = transport.Close()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	_, err := transport.Send(ctx, "tools/list", map[string]any{})
	if err == nil {
		t.Fatal("expected context cancellation error")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestStdioTransportLargePayload(t *testing.T) {
	transport := newMockTransport(t, mockServerEnv("MOCK_MCP_LARGE", "1")...)
	t.Cleanup(func() {
		_ = transport.Close()
	})

	result, err := transport.Send(context.Background(), "tools/list", map[string]any{})
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	var list mcpToolsListResult
	if err := json.Unmarshal(result, &list); err != nil {
		t.Fatalf("unmarshal large result: %v", err)
	}
	if len(list.Tools) != 1 {
		t.Fatalf("expected one tool, got %#v", list)
	}
	if got := len(list.Tools[0].Description); got < 256*1024 {
		t.Fatalf("expected large payload, got %d bytes", got)
	}
}

func TestStdioTransportIgnoresUnsolicitedNotifications(t *testing.T) {
	transport := newMockTransport(t, mockServerEnv("MOCK_MCP_NOTIFY_BURST", "1")...)
	t.Cleanup(func() {
		_ = transport.Close()
	})

	result, err := transport.Send(context.Background(), "tools/list", map[string]any{})
	if err != nil {
		t.Fatalf("tools/list: %v", err)
	}
	var list mcpToolsListResult
	if err := json.Unmarshal(result, &list); err != nil {
		t.Fatalf("unmarshal tools/list result: %v", err)
	}
	if len(list.Tools) != 1 || list.Tools[0].Name != "echo" {
		t.Fatalf("unexpected tools result %#v", list)
	}
}

func TestStdioTransportSubprocessExit(t *testing.T) {
	transport := newMockTransport(t, mockServerEnv("MOCK_MCP_EXIT_ON_CALL", "1")...)
	t.Cleanup(func() {
		_ = transport.Close()
	})

	if _, err := transport.Send(context.Background(), "tools/call", map[string]any{
		"name":      "exit_now",
		"arguments": json.RawMessage(`{}`),
	}); err == nil {
		t.Fatal("expected subprocess exit error")
	}
}

func TestStdioTransportClose(t *testing.T) {
	transport := newMockTransport(t)
	if err := transport.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestStdioTransportConcurrentCalls(t *testing.T) {
	transport := newMockTransport(t)
	t.Cleanup(func() {
		_ = transport.Close()
	})

	var wg sync.WaitGroup
	results := make([]string, 2)
	errs := make([]error, 2)
	requests := []json.RawMessage{
		json.RawMessage(`{"text":"hello"}`),
		json.RawMessage(`{"text":"world"}`),
	}
	for i := range requests {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			result, err := transport.Send(context.Background(), "tools/call", map[string]any{
				"name":      "echo",
				"arguments": requests[i],
			})
			if err != nil {
				errs[i] = err
				return
			}
			var call mcpCallResult
			if err := json.Unmarshal(result, &call); err != nil {
				errs[i] = err
				return
			}
			if len(call.Content) > 0 {
				results[i] = call.Content[0].Text
			}
		}(i)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			t.Fatalf("concurrent call error: %v", err)
		}
	}
	if results[0] != "echoed: hello" || results[1] != `echoed: {"text":"world"}` {
		t.Fatalf("unexpected concurrent results %#v", results)
	}
}

func newMockTransport(t *testing.T, extraEnv ...string) *StdioTransport {
	t.Helper()

	env := map[string]string{}
	for i := 0; i+1 < len(extraEnv); i += 2 {
		env[extraEnv[i]] = extraEnv[i+1]
	}

	transport, err := NewStdioTransport(context.Background(), mockServerBinary(t), nil, env)
	if err != nil {
		t.Fatalf("new stdio transport: %v", err)
	}
	return transport
}

func mockServerBinary(t *testing.T) string {
	t.Helper()

	mockServerOnce.Do(func() {
		dir, err := os.MkdirTemp("", "mcp-mock-server-*")
		if err != nil {
			mockServerErr = err
			return
		}
		mockServerPath = filepath.Join(dir, "mock_mcp_server")
		cmd := exec.Command("go", "build", "-o", mockServerPath, "./testdata/mock_mcp_server")
		cmd.Dir = "."
		out, err := cmd.CombinedOutput()
		if err != nil {
			mockServerErr = fmt.Errorf("build mock server: %w: %s", err, string(out))
		}
	})
	if mockServerErr != nil {
		t.Fatal(mockServerErr)
	}
	return mockServerPath
}

func mockServerEnv(key, value string) []string {
	return []string{key, value}
}
