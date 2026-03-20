package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Nomadcxx/nanobot-go/pkg/config"
	"github.com/Nomadcxx/nanobot-go/pkg/tool"
)

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

func (f *fakeDiscoveryClient) Invoke(_ context.Context, spec ConnectionSpec, toolName string, _ json.RawMessage) (*tool.Result, error) {
	f.lastSpec = spec
	f.lastInvokedTool = toolName
	f.invokeCalls++
	return &tool.Result{Output: "remote ok"}, nil
}
