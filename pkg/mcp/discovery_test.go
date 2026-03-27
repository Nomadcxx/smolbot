package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestStdioDiscoveryClientDiscover(t *testing.T) {
	client := NewStdioDiscoveryClient(nil)
	t.Cleanup(client.Close)

	spec := mockConnectionSpec(t, nil)
	tools, err := client.Discover(context.Background(), spec)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(tools) != 1 || tools[0].Name != "echo" {
		t.Fatalf("unexpected tools: %#v", tools)
	}
}

func TestStdioDiscoveryClientInvoke(t *testing.T) {
	client := NewStdioDiscoveryClient(nil)
	t.Cleanup(client.Close)

	spec := mockConnectionSpec(t, nil)
	tools, err := client.Discover(context.Background(), spec)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected one tool, got %#v", tools)
	}

	result, err := client.Invoke(context.Background(), spec, "echo", json.RawMessage(`{"text":"hello"}`))
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result.Output != "echoed: hello" {
		t.Fatalf("unexpected output %#v", result)
	}
}

func TestStdioDiscoveryClientErrorTool(t *testing.T) {
	client := NewStdioDiscoveryClient(nil)
	t.Cleanup(client.Close)

	spec := mockConnectionSpec(t, map[string]string{
		"MOCK_MCP_ERROR_ON_CALL": "1",
	})
	if _, err := client.Discover(context.Background(), spec); err != nil {
		t.Fatalf("discover: %v", err)
	}

	result, err := client.Invoke(context.Background(), spec, "echo", json.RawMessage(`{"text":"hello"}`))
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result.Error != "boom" {
		t.Fatalf("unexpected error result %#v", result)
	}
}

func TestStdioDiscoveryClientUnsupportedContent(t *testing.T) {
	client := NewStdioDiscoveryClient(nil)
	t.Cleanup(client.Close)

	spec := mockConnectionSpec(t, map[string]string{
		"MOCK_MCP_UNSUPPORTED_CONTENT": "1",
	})
	if _, err := client.Discover(context.Background(), spec); err != nil {
		t.Fatalf("discover: %v", err)
	}

	result, err := client.Invoke(context.Background(), spec, "echo", json.RawMessage(`{"text":"hello"}`))
	if err != nil {
		t.Fatalf("invoke: %v", err)
	}
	if result.Output != "[unsupported content type: image]" {
		t.Fatalf("unexpected output %#v", result)
	}
}

func TestStdioDiscoveryClientReuse(t *testing.T) {
	client := NewStdioDiscoveryClient(nil)
	t.Cleanup(client.Close)

	initLog := filepath.Join(t.TempDir(), "init.log")
	spec := mockConnectionSpec(t, map[string]string{
		"MOCK_MCP_INIT_LOG": initLog,
	})

	if _, err := client.Discover(context.Background(), spec); err != nil {
		t.Fatalf("discover 1: %v", err)
	}
	if _, err := client.Discover(context.Background(), spec); err != nil {
		t.Fatalf("discover 2: %v", err)
	}

	data, err := os.ReadFile(initLog)
	if err != nil {
		t.Fatalf("read init log: %v", err)
	}
	if got := strings.Count(string(data), "init"); got != 1 {
		t.Fatalf("expected single initialization, got %d entries in %q", got, string(data))
	}
}

func TestStdioDiscoveryClientConcurrentFirstUseSingleInit(t *testing.T) {
	client := NewStdioDiscoveryClient(nil)
	t.Cleanup(client.Close)

	initLog := filepath.Join(t.TempDir(), "init.log")
	spec := mockConnectionSpec(t, map[string]string{
		"MOCK_MCP_INIT_LOG": initLog,
	})

	var wg sync.WaitGroup
	errs := make([]error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = client.Discover(context.Background(), spec)
		}(i)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			t.Fatalf("concurrent discover error: %v", err)
		}
	}

	data, err := os.ReadFile(initLog)
	if err != nil {
		t.Fatalf("read init log: %v", err)
	}
	if got := strings.Count(string(data), "init"); got != 1 {
		t.Fatalf("expected single initialization, got %d entries in %q", got, string(data))
	}
}

func TestStdioDiscoveryClientTimeout(t *testing.T) {
	client := NewStdioDiscoveryClient(nil)
	t.Cleanup(client.Close)

	spec := mockConnectionSpec(t, map[string]string{
		"MOCK_MCP_DELAY_LIST": "1",
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	if _, err := client.Discover(ctx, spec); err == nil {
		t.Fatal("expected discover timeout")
	}
}

func TestStdioDiscoveryClientInitializeUsesToolTimeout(t *testing.T) {
	client := NewStdioDiscoveryClient(nil)
	t.Cleanup(client.Close)

	spec := mockConnectionSpec(t, map[string]string{
		"MOCK_MCP_DELAY_INIT": "1",
	})
	spec.ToolTimeout = 20 * time.Millisecond

	if _, err := client.Discover(context.Background(), spec); err == nil {
		t.Fatal("expected initialize timeout from configured tool timeout")
	} else if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestStdioDiscoveryClientDiscoverUsesDefaultTimeout(t *testing.T) {
	client := NewStdioDiscoveryClient(nil)
	t.Cleanup(client.Close)

	spec := mockConnectionSpec(t, map[string]string{
		"MOCK_MCP_DELAY_LIST": "1",
	})
	spec.ToolTimeout = 20 * time.Millisecond

	if _, err := client.Discover(context.Background(), spec); err == nil {
		t.Fatal("expected discover timeout from default request timeout")
	}
}

func TestStdioDiscoveryClientDiscoverPrefersShorterConfiguredTimeout(t *testing.T) {
	client := NewStdioDiscoveryClient(nil)
	t.Cleanup(client.Close)

	spec := mockConnectionSpec(t, map[string]string{
		"MOCK_MCP_DELAY_LIST": "1",
	})
	spec.ToolTimeout = 20 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if _, err := client.Discover(ctx, spec); err == nil {
		t.Fatal("expected discover timeout from shorter configured timeout")
	} else if !errors.Is(err, context.DeadlineExceeded) && !strings.Contains(err.Error(), "deadline exceeded") {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}

func TestStdioDiscoveryClientCloseCancelsInFlightConnect(t *testing.T) {
	client := NewStdioDiscoveryClient(nil)

	spec := mockConnectionSpec(t, map[string]string{
		"MOCK_MCP_DELAY_INIT": "1",
	})

	errCh := make(chan error, 1)
	go func() {
		_, err := client.Discover(context.Background(), spec)
		errCh <- err
	}()

	time.Sleep(20 * time.Millisecond)
	client.Close()

	if err := <-errCh; err == nil {
		t.Fatal("expected discover error after close")
	}
	if _, err := client.Discover(context.Background(), spec); !errors.Is(err, errDiscoveryClientClosed) {
		t.Fatalf("expected closed client error, got %v", err)
	}
}

func TestStdioDiscoveryClientRejectsUnsupportedTransport(t *testing.T) {
	client := NewStdioDiscoveryClient(nil)
	spec := ConnectionSpec{Name: "remote", Transport: TransportSSE}
	if _, err := client.Discover(context.Background(), spec); err == nil {
		t.Fatal("expected unsupported transport error")
	} else if !IsUnsupportedTransport(err) {
		t.Fatalf("expected unsupported transport error, got %v", err)
	}
}

func TestStdioDiscoveryClientInitializeSurfacedChildStderr(t *testing.T) {
	client := NewStdioDiscoveryClient(nil)
	t.Cleanup(client.Close)

	spec := ConnectionSpec{
		Name:        "broken",
		Transport:   TransportStdio,
		Command:     "sh",
		Args:        []string{"-c", "echo better-sqlite3 ABI mismatch >&2; exit 1"},
		ToolTimeout: 200 * time.Millisecond,
	}

	if _, err := client.Discover(context.Background(), spec); err == nil {
		t.Fatal("expected initialize failure")
	} else if !strings.Contains(err.Error(), "better-sqlite3 ABI mismatch") {
		t.Fatalf("expected child stderr in initialize error, got %v", err)
	}
}

func mockConnectionSpec(t *testing.T, env map[string]string) ConnectionSpec {
	t.Helper()

	specEnv := map[string]string{}
	for k, v := range env {
		specEnv[k] = v
	}

	return ConnectionSpec{
		Name:        "mock",
		Transport:   TransportStdio,
		Command:     mockServerBinary(t),
		Args:        nil,
		Env:         specEnv,
		ToolTimeout: 200 * time.Millisecond,
	}
}
