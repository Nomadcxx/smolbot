package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/tool"
)

func boolPtr(v bool) *bool { return &v }

func TestDetectTransport(t *testing.T) {
	if got := DetectTransport(config.MCPServerConfig{Type: "stdio"}); got != TransportStdio {
		t.Fatalf("expected stdio transport, got %q", got)
	}
	if got := DetectTransport(config.MCPServerConfig{URL: "https://example.com/mcp/sse"}); got != TransportSSE {
		t.Fatalf("expected sse transport, got %q", got)
	}
	if got := DetectTransport(config.MCPServerConfig{URL: "https://example.com/mcp"}); got != TransportStreamableHTTP {
		t.Fatalf("expected streamable http transport, got %q", got)
	}
}

func TestDiscoverAndRegister(t *testing.T) {
	client := &fakeDiscoveryClient{
		tools: []RemoteTool{
			{Name: "memory_store", Description: "Store memory", InputSchema: map[string]any{"type": "object"}},
			{Name: "memory_get", Description: "Get memory", InputSchema: map[string]any{"type": "object"}},
		},
	}
	manager := NewManager(client)
	registry := tool.NewRegistry()

	warnings, err := manager.DiscoverAndRegister(context.Background(), registry, map[string]config.MCPServerConfig{
		"hybrid-memory": {
			URL:          "https://example.com/mcp/sse",
			Headers:      map[string]string{"Authorization": "Bearer secret"},
			ToolTimeout:  12,
			EnabledTools: []string{"memory_store", "mcp_hybrid-memory_memory_get", "missing_tool"},
		},
	})
	if err != nil {
		t.Fatalf("DiscoverAndRegister: %v", err)
	}
	if client.lastSpec.Transport != TransportSSE {
		t.Fatalf("expected sse transport, got %#v", client.lastSpec)
	}
	if client.lastSpec.Headers["Authorization"] != "Bearer secret" {
		t.Fatalf("expected auth headers to propagate, got %#v", client.lastSpec.Headers)
	}
	if len(warnings) != 1 || warnings[0] == "" {
		t.Fatalf("expected unmatched enabledTools warning, got %#v", warnings)
	}

	defs := registry.Definitions()
	if len(defs) != 2 {
		t.Fatalf("expected two wrapped tools, got %#v", defs)
	}
	if defs[0].Name != "mcp_hybrid-memory_memory_get" || defs[1].Name != "mcp_hybrid-memory_memory_store" {
		t.Fatalf("unexpected wrapped names %#v", defs)
	}

	result, err := registry.Execute(context.Background(), "mcp_hybrid-memory_memory_store", json.RawMessage(`{"key":"value"}`), tool.ToolContext{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Output != "remote ok" {
		t.Fatalf("unexpected wrapper output %#v", result)
	}
	if client.invokeCalls != 1 || client.lastInvokedTool != "memory_store" {
		t.Fatalf("expected invocation through raw tool name, got %#v", client)
	}
}

func TestEnabledToolsWildcard(t *testing.T) {
	client := &fakeDiscoveryClient{
		tools: []RemoteTool{
			{Name: "tool_a", Description: "A", InputSchema: map[string]any{"type": "object"}},
			{Name: "tool_b", Description: "B", InputSchema: map[string]any{"type": "object"}},
		},
	}
	manager := NewManager(client)
	registry := tool.NewRegistry()

	warnings, err := manager.DiscoverAndRegister(context.Background(), registry, map[string]config.MCPServerConfig{
		"server": {
			Type:         "stdio",
			ToolTimeout:  5,
			EnabledTools: []string{"*"},
		},
	})
	if err != nil {
		t.Fatalf("DiscoverAndRegister: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("expected no warnings, got %#v", warnings)
	}
	if len(registry.Definitions()) != 2 {
		t.Fatalf("expected all tools registered, got %#v", registry.Definitions())
	}
}

func TestDiscoverAndRegisterNilClientLogsWarning(t *testing.T) {
	orig := slog.Default()
	t.Cleanup(func() {
		slog.SetDefault(orig)
	})

	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})))

	manager := NewManager(nil)
	registry := tool.NewRegistry()
	warnings, err := manager.DiscoverAndRegister(context.Background(), registry, map[string]config.MCPServerConfig{
		"test": {Command: "echo"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if warnings != nil {
		t.Fatalf("expected nil warnings, got %v", warnings)
	}
	if len(registry.Definitions()) != 0 {
		t.Fatalf("expected no tools registered")
	}
	if got := buf.String(); got == "" || !bytes.Contains(buf.Bytes(), []byte("mcp manager has no discovery client")) {
		t.Fatal("expected nil-client warning to be logged")
	}
}

func TestDiscoverAndRegisterSkipsUnsupportedTransport(t *testing.T) {
	client := NewStdioDiscoveryClient(nil)
	manager := NewManager(client)
	registry := tool.NewRegistry()

	warnings, err := manager.DiscoverAndRegister(context.Background(), registry, map[string]config.MCPServerConfig{
		"remote": {
			URL: "https://example.com/mcp",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(registry.Definitions()) != 0 {
		t.Fatalf("expected no tools registered, got %#v", registry.Definitions())
	}
	if len(warnings) != 1 || !bytes.Contains([]byte(warnings[0]), []byte("unsupported transport")) {
		t.Fatalf("expected unsupported transport warning, got %#v", warnings)
	}
}

type fakeDiscoveryClient struct {
	tools           []RemoteTool
	lastSpec        ConnectionSpec
	lastInvokedTool string
	invokeCalls     int
}

func (f *fakeDiscoveryClient) Discover(_ context.Context, spec ConnectionSpec) ([]RemoteTool, error) {
	f.lastSpec = spec
	return f.tools, nil
}

func (f *fakeDiscoveryClient) Invoke(_ context.Context, spec ConnectionSpec, toolName string, _ json.RawMessage, _ tool.ToolContext) (*tool.Result, error) {
	f.lastSpec = spec
	f.lastInvokedTool = toolName
	f.invokeCalls++
	return &tool.Result{Output: "remote ok"}, nil
}

func TestManagerToolCountsTrackedAfterDiscovery(t *testing.T) {
	client := &fakeDiscoveryClient{
		tools: []RemoteTool{
			{Name: "tool_a", Description: "A", InputSchema: map[string]any{"type": "object"}},
			{Name: "tool_b", Description: "B", InputSchema: map[string]any{"type": "object"}},
		},
	}
	manager := NewManager(client)
	registry := tool.NewRegistry()

	_, err := manager.DiscoverAndRegister(context.Background(), registry, map[string]config.MCPServerConfig{
		"my-server": {Type: "stdio", ToolTimeout: 5, EnabledTools: []string{"*"}},
	})
	if err != nil {
		t.Fatalf("DiscoverAndRegister: %v", err)
	}

	counts := manager.ToolCounts()
	if counts["my-server"] != 2 {
		t.Fatalf("expected 2 tools for my-server, got %d", counts["my-server"])
	}
}

func TestDiscoverSkipsDisabled(t *testing.T) {
	client := &fakeDiscoveryClient{
		tools: []RemoteTool{
			{Name: "tool_a", Description: "A", InputSchema: map[string]any{"type": "object"}},
		},
	}
	manager := NewManager(client)
	registry := tool.NewRegistry()

	warnings, err := manager.DiscoverAndRegister(context.Background(), registry, map[string]config.MCPServerConfig{
		"disabled-server": {
			Type:        "stdio",
			Command:     "echo",
			Enabled:     boolPtr(false),
			ToolTimeout: 5,
		},
	})
	if err != nil {
		t.Fatalf("DiscoverAndRegister: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
	if len(registry.Definitions()) != 0 {
		t.Fatalf("expected no tools registered, got %#v", registry.Definitions())
	}
	if client.lastSpec.Name != "" {
		t.Fatalf("expected disabled server not to be discovered, got spec %#v", client.lastSpec)
	}
}

func TestDiscoverIncludesEnabled(t *testing.T) {
	client := &fakeDiscoveryClient{
		tools: []RemoteTool{
			{Name: "tool_a", Description: "A", InputSchema: map[string]any{"type": "object"}},
		},
	}
	manager := NewManager(client)
	registry := tool.NewRegistry()

	warnings, err := manager.DiscoverAndRegister(context.Background(), registry, map[string]config.MCPServerConfig{
		"enabled-server": {
			Type:        "stdio",
			Command:     "echo",
			Enabled:     boolPtr(true),
			ToolTimeout: 5,
		},
	})
	if err != nil {
		t.Fatalf("DiscoverAndRegister: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
	if len(registry.Definitions()) != 1 {
		t.Fatalf("expected one tool registered, got %#v", registry.Definitions())
	}
	if client.lastSpec.Name != "enabled-server" {
		t.Fatalf("expected enabled server discovery, got spec %#v", client.lastSpec)
	}
}

func TestDiscoverIncludesEnabledByDefault(t *testing.T) {
	client := &fakeDiscoveryClient{
		tools: []RemoteTool{
			{Name: "tool_a", Description: "A", InputSchema: map[string]any{"type": "object"}},
		},
	}
	manager := NewManager(client)
	registry := tool.NewRegistry()

	warnings, err := manager.DiscoverAndRegister(context.Background(), registry, map[string]config.MCPServerConfig{
		"default-server": {
			Type:        "stdio",
			Command:     "echo",
			ToolTimeout: 5,
		},
	})
	if err != nil {
		t.Fatalf("DiscoverAndRegister: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}
	if len(registry.Definitions()) != 1 {
		t.Fatalf("expected one tool registered, got %#v", registry.Definitions())
	}
	if client.lastSpec.Name != "default-server" {
		t.Fatalf("expected default-enabled server discovery, got spec %#v", client.lastSpec)
	}
}
