package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckNodeVersion_Present(t *testing.T) {
	origLookPath := execLookPath
	origCombinedOutput := execCombinedOutput
	t.Cleanup(func() {
		execLookPath = origLookPath
		execCombinedOutput = origCombinedOutput
	})

	execLookPath = func(file string) (string, error) {
		if file != "node" {
			t.Fatalf("lookPath file = %q, want node", file)
		}
		return "/usr/bin/node", nil
	}
	execCombinedOutput = func(name string, args ...string) ([]byte, error) {
		if name != "node" || len(args) != 1 || args[0] != "--version" {
			t.Fatalf("combinedOutput = %s %v, want node --version", name, args)
		}
		return []byte("v20.11.1\n"), nil
	}

	version, err := checkNodeVersion(18)
	if err != nil {
		t.Fatalf("checkNodeVersion: %v", err)
	}
	if version != "v20.11.1" {
		t.Fatalf("version = %q, want v20.11.1", version)
	}
}

func TestCheckNodeVersion_Missing(t *testing.T) {
	origLookPath := execLookPath
	t.Cleanup(func() { execLookPath = origLookPath })

	execLookPath = func(string) (string, error) {
		return "", errors.New("not found")
	}

	if _, err := checkNodeVersion(18); err == nil {
		t.Fatal("checkNodeVersion() error = nil, want missing-node error")
	}
}

func TestCheckNodeVersion_TooOld(t *testing.T) {
	origLookPath := execLookPath
	origCombinedOutput := execCombinedOutput
	t.Cleanup(func() {
		execLookPath = origLookPath
		execCombinedOutput = origCombinedOutput
	})

	execLookPath = func(string) (string, error) { return "/usr/bin/node", nil }
	execCombinedOutput = func(string, ...string) ([]byte, error) {
		return []byte("v16.20.0\n"), nil
	}

	if _, err := checkNodeVersion(18); err == nil {
		t.Fatal("checkNodeVersion() error = nil, want too-old error")
	}
}

func TestSetupHybridMemory_Happy(t *testing.T) {
	repo := t.TempDir()
	serverPath := filepath.Join(repo, "mcp", "hybrid-memory", "mcp")
	if err := os.MkdirAll(serverPath, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(serverPath, "mcp-server.js"), []byte("#!/usr/bin/env node"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	origCheckNodeVersion := checkNodeVersionFn
	origCheckNpmAvailable := checkNpmAvailableFn
	origRunNpmInstall := runNpmInstallFn
	t.Cleanup(func() {
		checkNodeVersionFn = origCheckNodeVersion
		checkNpmAvailableFn = origCheckNpmAvailable
		runNpmInstallFn = origRunNpmInstall
	})

	checkNodeVersionFn = func(int) (string, error) { return "v20.0.0", nil }
	checkNpmAvailableFn = func() error { return nil }
	ranInstall := false
	runNpmInstallFn = func(m *model, dir string) error {
		ranInstall = true
		if dir != serverPath {
			t.Fatalf("npm dir = %q, want %q", dir, serverPath)
		}
		return nil
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	m := &model{projectDir: repo}

	if err := setupHybridMemory(m); err != nil {
		t.Fatalf("setupHybridMemory: %v", err)
	}
	if !m.hybridMemoryReady {
		t.Fatal("hybridMemoryReady = false, want true")
	}
	if !ranInstall {
		t.Fatal("expected npm install to run")
	}
	if _, err := os.Stat(filepath.Join(home, ".smolbot", "memory")); err != nil {
		t.Fatalf("memory dir not created: %v", err)
	}
}

func TestSetupHybridMemory_NoNode(t *testing.T) {
	origCheckNodeVersion := checkNodeVersionFn
	t.Cleanup(func() { checkNodeVersionFn = origCheckNodeVersion })

	checkNodeVersionFn = func(int) (string, error) { return "", errors.New("node missing") }

	repo := t.TempDir()
	serverDir := filepath.Join(repo, "mcp", "hybrid-memory", "mcp")
	if err := os.MkdirAll(serverDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(serverDir, "mcp-server.js"), []byte("#!/usr/bin/env node"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	m := &model{projectDir: repo}
	if err := setupHybridMemory(m); err != nil {
		t.Fatalf("setupHybridMemory: %v", err)
	}
	if m.hybridMemoryReady {
		t.Fatal("hybridMemoryReady = true, want false")
	}
	if !strings.Contains(strings.ToLower(m.hybridMemorySkipped), "node") {
		t.Fatalf("hybridMemorySkipped = %q, want node-related skip reason", m.hybridMemorySkipped)
	}
}

func TestWriteConfig_IncludesHybridMemory(t *testing.T) {
	projectDir := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "config.json")
	m := &model{
		projectDir:         projectDir,
		configPath:         configPath,
		workspacePath:      filepath.Join(t.TempDir(), "workspace"),
		provider:           providerOllama,
		selectedModel:      "llama3.2",
		ollamaURL:          "http://localhost:11434",
		port:               18790,
		hybridMemoryReady:  true,
	}

	if err := writeConfig(m); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}

	var cfg map[string]any
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	tools := cfg["tools"].(map[string]any)
	mcpServers := tools["mcpServers"].(map[string]any)
	hm := mcpServers["hybrid-memory"].(map[string]any)
	if hm["command"] != "node" {
		t.Fatalf("command = %#v, want node", hm["command"])
	}
	args := hm["args"].([]any)
	if len(args) != 1 || !strings.HasSuffix(args[0].(string), "/mcp/hybrid-memory/mcp/mcp-server.js") {
		t.Fatalf("args = %#v, want mcp-server.js path", args)
	}
	env := hm["env"].(map[string]any)
	if !strings.HasSuffix(env["HYBRID_MEMORY_DIR"].(string), "/.smolbot/memory") {
		t.Fatalf("HYBRID_MEMORY_DIR = %#v, want ~/.smolbot/memory path", env["HYBRID_MEMORY_DIR"])
	}
}

func TestWriteConfig_PreservesExistingHybridMemoryConfigOnUpgrade(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.json")
	existing := map[string]any{
		"tools": map[string]any{
			"mcpServers": map[string]any{
				"custom-server": map[string]any{
					"type":    "stdio",
					"command": "custom",
				},
				"hybrid-memory": map[string]any{
					"type":    "stdio",
					"command": "custom-node",
					"args":    []any{"/custom/server.js"},
					"env": map[string]any{
						"HYBRID_MEMORY_DIR": "/custom/memory",
					},
				},
			},
		},
	}
	data, err := json.Marshal(existing)
	if err != nil {
		t.Fatalf("marshal seed config: %v", err)
	}
	if err := os.WriteFile(configPath, data, 0o600); err != nil {
		t.Fatalf("write seed config: %v", err)
	}

	m := &model{
		projectDir:        t.TempDir(),
		configPath:        configPath,
		workspacePath:     filepath.Join(t.TempDir(), "workspace"),
		provider:          providerOllama,
		selectedModel:     "llama3.2",
		ollamaURL:         "http://localhost:11434",
		port:              18790,
		updateMode:        true,
		hybridMemoryReady: true,
	}

	if err := writeConfig(m); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}

	var cfg map[string]any
	out, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if err := json.Unmarshal(out, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	mcpServers := cfg["tools"].(map[string]any)["mcpServers"].(map[string]any)
	if _, ok := mcpServers["custom-server"]; !ok {
		t.Fatal("custom-server missing after upgrade merge")
	}
	hm := mcpServers["hybrid-memory"].(map[string]any)
	if hm["command"] != "custom-node" {
		t.Fatalf("hybrid-memory command = %#v, want preserved custom-node", hm["command"])
	}
}

func TestInitTasksIncludesSetupHybridMemory(t *testing.T) {
	m := newModel()
	m.updateMode = false
	m.enableService = false
	m.startNow = false
	m.initTasks()

	foundFresh := false
	for _, task := range m.tasks {
		if task.name == "Setup hybrid-memory" {
			foundFresh = true
			if !task.optional {
				t.Fatal("setupHybridMemory should be optional in fresh install pipeline")
			}
		}
	}
	if !foundFresh {
		t.Fatal("fresh install pipeline missing setupHybridMemory task")
	}

	m.updateMode = true
	m.daemonWasRunning = false
	m.initTasks()

	foundUpgrade := false
	for _, task := range m.tasks {
		if task.name == "Setup hybrid-memory" {
			foundUpgrade = true
			if !task.optional {
				t.Fatal("setupHybridMemory should be optional in upgrade pipeline")
			}
		}
	}
	if !foundUpgrade {
		t.Fatal("upgrade pipeline missing setupHybridMemory task")
	}
}
