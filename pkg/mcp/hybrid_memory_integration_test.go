//go:build integration

package mcp

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/tool"
)

func TestHybridMemoryDiscovery(t *testing.T) {
	client, registry := newHybridMemoryHarness(t)

	manager := NewManager(client)
	warnings, err := manager.DiscoverAndRegister(context.Background(), registry, map[string]config.MCPServerConfig{
		"hybrid-memory": hybridMemoryMCPConfig(t),
	})
	if err != nil {
		t.Fatalf("DiscoverAndRegister: %v", err)
	}
	if len(warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", warnings)
	}

	assertHybridMemoryToolSet(t, registry)
}

func TestHybridMemoryStoreSearchAfterStore(t *testing.T) {
	client, registry := newHybridMemoryHarness(t)

	manager := NewManager(client)
	if _, err := manager.DiscoverAndRegister(context.Background(), registry, map[string]config.MCPServerConfig{
		"hybrid-memory": hybridMemoryMCPConfig(t),
	}); err != nil {
		t.Fatalf("DiscoverAndRegister: %v", err)
	}

	storeRaw := json.RawMessage(`{"text":"bundled memory test","category":"decision","importance":"stable","entity":"smolbot","key":"hybrid-memory-integration"}`)
	storeResult, err := registry.Execute(context.Background(), "mcp_hybrid-memory_memory_store", storeRaw, tool.ToolContext{})
	if err != nil {
		t.Fatalf("memory_store: %v", err)
	}
	var stored map[string]any
	if err := json.Unmarshal([]byte(storeResult.Output), &stored); err != nil {
		t.Fatalf("unmarshal store result: %v", err)
	}
	if stored["id"] == "" {
		t.Fatalf("store result missing id: %#v", stored)
	}

	searchRaw := json.RawMessage(`{"query":"bundled memory test","limit":5}`)
	searchResult, err := registry.Execute(context.Background(), "mcp_hybrid-memory_memory_search", searchRaw, tool.ToolContext{})
	if err != nil {
		t.Fatalf("memory_search: %v", err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(searchResult.Output), &rows); err != nil {
		t.Fatalf("unmarshal search result: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("search returned no rows")
	}
}

func TestHybridMemoryStatsCleanupGetDelete(t *testing.T) {
	client, registry := newHybridMemoryHarness(t)

	manager := NewManager(client)
	if _, err := manager.DiscoverAndRegister(context.Background(), registry, map[string]config.MCPServerConfig{
		"hybrid-memory": hybridMemoryMCPConfig(t),
	}); err != nil {
		t.Fatalf("DiscoverAndRegister: %v", err)
	}

	storeRaw := json.RawMessage(`{"text":"stats memory","category":"pattern","importance":"session","entity":"smolbot","key":"hybrid-memory-stats"}`)
	storeResult, err := registry.Execute(context.Background(), "mcp_hybrid-memory_memory_store", storeRaw, tool.ToolContext{})
	if err != nil {
		t.Fatalf("memory_store: %v", err)
	}
	var stored map[string]any
	if err := json.Unmarshal([]byte(storeResult.Output), &stored); err != nil {
		t.Fatalf("unmarshal store result: %v", err)
	}
	id, _ := stored["id"].(string)
	if id == "" {
		t.Fatalf("store result missing id: %#v", stored)
	}

	statsResult, err := registry.Execute(context.Background(), "mcp_hybrid-memory_memory_stats", json.RawMessage(`{}`), tool.ToolContext{})
	if err != nil {
		t.Fatalf("memory_stats: %v", err)
	}
	if !strings.Contains(statsResult.Output, `"total"`) {
		t.Fatalf("stats output = %q, want total", statsResult.Output)
	}

	getResult, err := registry.Execute(context.Background(), "mcp_hybrid-memory_memory_get", json.RawMessage(`{"id":"`+id+`"}`), tool.ToolContext{})
	if err != nil {
		t.Fatalf("memory_get: %v", err)
	}
	if !strings.Contains(getResult.Output, `"id"`) {
		t.Fatalf("get output = %q, want id", getResult.Output)
	}

	cleanupResult, err := registry.Execute(context.Background(), "mcp_hybrid-memory_memory_cleanup", json.RawMessage(`{"dry_run":true}`), tool.ToolContext{})
	if err != nil {
		t.Fatalf("memory_cleanup: %v", err)
	}
	if !strings.Contains(cleanupResult.Output, `"dryRun": true`) && !strings.Contains(cleanupResult.Output, `"dryRun":true`) {
		t.Fatalf("cleanup output = %q, want dryRun", cleanupResult.Output)
	}

	deleteResult, err := registry.Execute(context.Background(), "mcp_hybrid-memory_memory_delete", json.RawMessage(`{"id":"`+id+`"}`), tool.ToolContext{})
	if err != nil {
		t.Fatalf("memory_delete: %v", err)
	}
	if !strings.Contains(deleteResult.Output, `"deleted": true`) && !strings.Contains(deleteResult.Output, `"deleted":true`) {
		t.Fatalf("delete output = %q, want deleted=true", deleteResult.Output)
	}
}

func TestHybridMemoryDisabledSkipsDiscovery(t *testing.T) {
	requireHybridMemoryServerAssets(t)

	client := NewStdioDiscoveryClient(nil)
	t.Cleanup(client.Close)
	registry := tool.NewRegistry()
	manager := NewManager(client)

	warnings, err := manager.DiscoverAndRegister(context.Background(), registry, map[string]config.MCPServerConfig{
		"hybrid-memory": func() config.MCPServerConfig {
			cfg := hybridMemoryMCPConfig(t)
			disabled := false
			cfg.Enabled = &disabled
			return cfg
		}(),
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
	if len(client.transports) != 0 {
		t.Fatalf("expected no transport started, got %d", len(client.transports))
	}
}

func TestHybridMemoryShutdown(t *testing.T) {
	requireHybridMemoryServerAssets(t)

	client := NewStdioDiscoveryClient(nil)
	registry := tool.NewRegistry()
	manager := NewManager(client)

	if _, err := manager.DiscoverAndRegister(context.Background(), registry, map[string]config.MCPServerConfig{
		"hybrid-memory": hybridMemoryMCPConfig(t),
	}); err != nil {
		t.Fatalf("DiscoverAndRegister: %v", err)
	}

	transport := client.transports["hybrid-memory"]
	if transport == nil || transport.cmd == nil || transport.cmd.Process == nil {
		t.Fatal("expected live transport process")
	}

	client.Close()
	time.Sleep(100 * time.Millisecond)
	if transport.cmd.ProcessState == nil || !transport.cmd.ProcessState.Exited() {
		t.Fatal("expected hybrid-memory process to be exited after Close")
	}
}

func TestHybridMemoryBadPath(t *testing.T) {
	client := NewStdioDiscoveryClient(nil)
	t.Cleanup(client.Close)
	registry := tool.NewRegistry()
	manager := NewManager(client)

	cfg := hybridMemoryMCPConfig(t)
	cfg.Args = []string{filepath.Join(t.TempDir(), "missing-server.js")}

	if _, err := manager.DiscoverAndRegister(context.Background(), registry, map[string]config.MCPServerConfig{
		"hybrid-memory": cfg,
	}); err == nil {
		t.Fatal("expected discovery error for bad path")
	}
}

func newHybridMemoryHarness(t *testing.T) (*StdioDiscoveryClient, *tool.Registry) {
	t.Helper()
	requireHybridMemoryServerAssets(t)

	client := NewStdioDiscoveryClient(nil)
	t.Cleanup(client.Close)
	return client, tool.NewRegistry()
}

func hybridMemoryMCPConfig(t *testing.T) config.MCPServerConfig {
	t.Helper()
	return config.MCPServerConfig{
		Type:    "stdio",
		Command: "node",
		Args: []string{
			hybridMemoryServerPath(t),
		},
		Env: map[string]string{
			"HYBRID_MEMORY_DIR":  t.TempDir(),
			"OLLAMA_HOST":        "http://127.0.0.1:9",
			"OLLAMA_EMBED_MODEL": "mxbai-embed-large",
		},
		ToolTimeout:  30,
		EnabledTools: []string{"*"},
	}
}

func hybridMemoryServerPath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "mcp", "hybrid-memory", "mcp", "mcp-server.js")
}

func requireHybridMemoryServerAssets(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node is required for hybrid-memory integration tests")
	}
	if _, err := exec.LookPath("npm"); err != nil {
		t.Skip("npm is required for hybrid-memory integration tests")
	}
	serverPath := hybridMemoryServerPath(t)
	if _, err := os.Stat(serverPath); err != nil {
		t.Skipf("hybrid-memory server missing: %v", err)
	}
	nodeModules := filepath.Join(filepath.Dir(serverPath), "node_modules")
	if _, err := os.Stat(nodeModules); err != nil {
		t.Skip("hybrid-memory dependencies not installed; run npm install --production in mcp/hybrid-memory/mcp")
	}
	preflight := exec.Command("node", "--input-type=module", "-e", "await import('@modelcontextprotocol/sdk/server/mcp.js'); await import('better-sqlite3'); await import('@lancedb/lancedb');")
	preflight.Dir = filepath.Dir(serverPath)
	if output, err := preflight.CombinedOutput(); err != nil {
		t.Skipf("hybrid-memory node dependencies are not runnable in this environment: %v (%s)", err, strings.TrimSpace(string(output)))
	}
}

func assertHybridMemoryToolSet(t *testing.T, registry *tool.Registry) {
	t.Helper()
	expected := []string{
		"mcp_hybrid-memory_memory_cleanup",
		"mcp_hybrid-memory_memory_delete",
		"mcp_hybrid-memory_memory_get",
		"mcp_hybrid-memory_memory_search",
		"mcp_hybrid-memory_memory_semantic",
		"mcp_hybrid-memory_memory_stats",
		"mcp_hybrid-memory_memory_store",
	}
	var got []string
	for _, def := range registry.Definitions() {
		got = append(got, def.Name)
	}
	for _, name := range expected {
		if !slices.Contains(got, name) {
			t.Fatalf("missing registered tool %q in %#v", name, got)
		}
	}
}
