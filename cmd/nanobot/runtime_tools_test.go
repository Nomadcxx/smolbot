package main

import (
	"context"
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/mcp"
	"github.com/Nomadcxx/smolbot/pkg/tool"
)

func TestBuildRuntimeMCPToolsRegistered(t *testing.T) {
	origNewMCPMgr := newMCPMgr
	t.Cleanup(func() { newMCPMgr = origNewMCPMgr })

	fakeClient := &fakeMCPDisonveryClient{
		tools: []mcp.RemoteTool{
			{Name: "memory_store", Description: "Store memory", InputSchema: map[string]any{"type": "object"}},
			{Name: "memory_get", Description: "Get memory", InputSchema: map[string]any{"type": "object"}},
		},
	}
	newMCPMgr = func() mcpDiscoveryManager {
		return &fakeMCPDisonveryClientManager{client: fakeClient}
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Tools.MCPServers = map[string]config.MCPServerConfig{
		"hybrid-memory": {
			URL:         "https://example.com/mcp",
			ToolTimeout: 30,
			EnabledTools: []string{"*"},
		},
	}

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	app, err := buildRuntime(daemonLaunchOptions{
		ConfigPath: cfgPath,
		Port:       port,
	}, runtimeDeps{})
	if err != nil {
		t.Fatalf("buildRuntime: %v", err)
	}
	defer app.Close()

	names := make([]string, 0)
	for _, def := range app.tools.Definitions() {
		names = append(names, def.Name)
	}

	if !slices.Contains(names, "mcp_hybrid-memory_memory_store") {
		t.Fatalf("expected MCP tool mcp_hybrid-memory_memory_store in runtime definitions, got %#v", names)
	}
	if !slices.Contains(names, "mcp_hybrid-memory_memory_get") {
		t.Fatalf("expected MCP tool mcp_hybrid-memory_memory_get in runtime definitions, got %#v", names)
	}
}

func TestBuildRuntimeMCPCoexistsWithBuiltinAndCronTools(t *testing.T) {
	origNewMCPMgr := newMCPMgr
	t.Cleanup(func() { newMCPMgr = origNewMCPMgr })

	fakeClient := &fakeMCPDisonveryClient{
		tools: []mcp.RemoteTool{
			{Name: "remote_tool", Description: "Remote", InputSchema: map[string]any{"type": "object"}},
		},
	}
	newMCPMgr = func() mcpDiscoveryManager {
		return &fakeMCPDisonveryClientManager{client: fakeClient}
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Tools.MCPServers = map[string]config.MCPServerConfig{
		"remote-server": {
			URL:         "https://example.com/mcp",
			ToolTimeout: 30,
			EnabledTools: []string{"*"},
		},
	}

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	app, err := buildRuntime(daemonLaunchOptions{
		ConfigPath: cfgPath,
		Port:       port,
	}, runtimeDeps{})
	if err != nil {
		t.Fatalf("buildRuntime: %v", err)
	}
	defer app.Close()

	names := make([]string, 0)
	for _, def := range app.tools.Definitions() {
		names = append(names, def.Name)
	}

	expectedTools := []string{
		"cron",
		"exec",
		"read_file",
		"write_file",
		"edit_file",
		"list_dir",
		"web_search",
		"web_fetch",
		"message",
		"spawn",
		"mcp_remote-server_remote_tool",
	}

	for _, expected := range expectedTools {
		if !slices.Contains(names, expected) {
			t.Fatalf("expected tool %q in runtime definitions, got %#v", expected, names)
		}
	}
}

func TestBuildRuntimeMCPEnabledToolsWarning(t *testing.T) {
	origNewMCPMgr := newMCPMgr
	t.Cleanup(func() { newMCPMgr = origNewMCPMgr })

	fakeClient := &fakeMCPDisonveryClient{
		tools: []mcp.RemoteTool{
			{Name: "actual_tool", Description: "Actual tool", InputSchema: map[string]any{"type": "object"}},
		},
	}
	var capturedWarnings []string
	newMCPMgr = func() mcpDiscoveryManager {
		return &fakeMCPDisonveryClientManager{client: fakeClient, onDiscover: func(warnings []string) {
			capturedWarnings = warnings
		}}
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Tools.MCPServers = map[string]config.MCPServerConfig{
		"test-server": {
			URL:         "https://example.com/mcp",
			ToolTimeout: 30,
			EnabledTools: []string{"actual_tool", "nonexistent_tool"},
		},
	}

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	app, err := buildRuntime(daemonLaunchOptions{
		ConfigPath: cfgPath,
		Port:       port,
	}, runtimeDeps{})
	if err != nil {
		t.Fatalf("buildRuntime: %v", err)
	}
	defer app.Close()

	if len(capturedWarnings) == 0 {
		t.Fatal("expected MCP warnings for unmatched enabledTools, got none")
	}
	found := false
	for _, w := range capturedWarnings {
		if strings.Contains(w, "nonexistent_tool") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected warning about unmatched enabledTools entry 'nonexistent_tool', got %#v", capturedWarnings)
	}
}

func TestMCPToolExecution(t *testing.T) {
	origNewMCPMgr := newMCPMgr
	t.Cleanup(func() { newMCPMgr = origNewMCPMgr })

	fakeClient := &fakeMCPDisonveryClient{
		tools: []mcp.RemoteTool{
			{Name: "echo", Description: "Echo args", InputSchema: map[string]any{"type": "object"}},
		},
	}
	newMCPMgr = func() mcpDiscoveryManager {
		return &fakeMCPDisonveryClientManager{client: fakeClient}
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Tools.MCPServers = map[string]config.MCPServerConfig{
		"echo-server": {
			URL:         "https://example.com/mcp",
			ToolTimeout: 30,
			EnabledTools: []string{"*"},
		},
	}

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	app, err := buildRuntime(daemonLaunchOptions{
		ConfigPath: cfgPath,
		Port:       port,
	}, runtimeDeps{})
	if err != nil {
		t.Fatalf("buildRuntime: %v", err)
	}
	defer app.Close()

	result, err := app.tools.Execute(context.Background(), "mcp_echo-server_echo", json.RawMessage(`{"msg":"hello"}`), tool.ToolContext{})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if result.Output != `{"echoed":"hello"}` {
		t.Fatalf("unexpected tool output %#v", result)
	}
	if fakeClient.lastInvokedTool != "echo" {
		t.Fatalf("expected echo to be invoked, got %q", fakeClient.lastInvokedTool)
	}
}

type fakeMCPDisonveryClient struct {
	tools           []mcp.RemoteTool
	lastInvokedTool string
}

func (f *fakeMCPDisonveryClient) Discover(_ context.Context, _ mcp.ConnectionSpec) ([]mcp.RemoteTool, error) {
	return f.tools, nil
}

func (f *fakeMCPDisonveryClient) Invoke(_ context.Context, _ mcp.ConnectionSpec, toolName string, _ json.RawMessage) (*tool.Result, error) {
	f.lastInvokedTool = toolName
	return &tool.Result{Output: `{"echoed":"hello"}`}, nil
}

type fakeMCPDisonveryClientManager struct {
	client      *fakeMCPDisonveryClient
	onDiscover  func([]string)
}

func (m *fakeMCPDisonveryClientManager) DiscoverAndRegister(ctx context.Context, registry *tool.Registry, servers map[string]config.MCPServerConfig) ([]string, error) {
	mgr := mcp.NewManager(m.client)
	warnings, err := mgr.DiscoverAndRegister(ctx, registry, servers)
	if m.onDiscover != nil {
		m.onDiscover(warnings)
	}
	return warnings, err
}
