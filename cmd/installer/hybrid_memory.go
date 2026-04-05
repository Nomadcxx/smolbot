package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

var (
	execLookPath       = exec.LookPath
	execCombinedOutput = func(name string, args ...string) ([]byte, error) {
		return exec.Command(name, args...).CombinedOutput()
	}
	checkNodeVersionFn = checkNodeVersion
	checkNpmAvailableFn = checkNpmAvailable
	runNpmInstallFn     = runNpmInstall
)

func checkNodeVersion(minMajor int) (string, error) {
	if _, err := execLookPath("node"); err != nil {
		return "", fmt.Errorf("node is not installed or not in PATH: %w", err)
	}
	output, err := execCombinedOutput("node", "--version")
	if err != nil {
		return "", fmt.Errorf("node --version failed: %w", err)
	}
	version := strings.TrimSpace(string(output))
	trimmed := strings.TrimPrefix(version, "v")
	majorPart, _, _ := strings.Cut(trimmed, ".")
	major, err := strconv.Atoi(majorPart)
	if err != nil {
		return "", fmt.Errorf("parse node version %q: %w", version, err)
	}
	if major < minMajor {
		return "", fmt.Errorf("node %s is too old; need >= %d", version, minMajor)
	}
	return version, nil
}

func checkNpmAvailable() error {
	if _, err := execLookPath("npm"); err != nil {
		return fmt.Errorf("npm is not installed or not in PATH: %w", err)
	}
	if _, err := execCombinedOutput("npm", "--version"); err != nil {
		return fmt.Errorf("npm --version failed: %w", err)
	}
	return nil
}

func runNpmInstall(m *model, dir string) error {
	result := runCommandInDir(m, dir, "npm", "install", "--omit=dev")
	if result.Err != nil {
		return CommandError{
			Command:  "npm install --omit=dev",
			ExitCode: result.ExitCode,
			Output:   result.Output,
			Duration: result.Duration,
			Err:      result.Err,
		}
	}
	return nil
}

func setupHybridMemory(m *model) error {
	serverDir := filepath.Join(m.projectDir, "mcp", "hybrid-memory", "mcp")
	serverPath := filepath.Join(serverDir, "mcp-server.js")
	m.hybridMemoryReady = false
	m.hybridMemorySkipped = ""

	if _, err := os.Stat(serverPath); err != nil {
		m.hybridMemorySkipped = "hybrid-memory submodule not present"
		return nil
	}
	if _, err := checkNodeVersionFn(18); err != nil {
		m.hybridMemorySkipped = err.Error()
		return nil
	}
	if err := checkNpmAvailableFn(); err != nil {
		m.hybridMemorySkipped = err.Error()
		return nil
	}
	if err := runNpmInstallFn(m, serverDir); err != nil {
		m.hybridMemorySkipped = err.Error()
		return nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		m.hybridMemorySkipped = fmt.Sprintf("determine home directory: %v", err)
		return nil
	}
	memoryDir := filepath.Join(home, ".smolbot", "memory")
	if err := os.MkdirAll(memoryDir, 0o755); err != nil {
		m.hybridMemorySkipped = fmt.Sprintf("create hybrid-memory data dir: %v", err)
		return nil
	}
	m.hybridMemoryReady = true
	return nil
}

func setupHybridMemoryTask(m *model) error {
	if err := setupHybridMemory(m); err != nil {
		return err
	}
	if m.hybridMemorySkipped != "" {
		return errors.New(m.hybridMemorySkipped)
	}
	return nil
}

func hybridMemoryServerConfig(repoRoot string) map[string]any {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = os.Getenv("HOME")
	}
	return map[string]any{
		"enabled": true,
		"type":    "stdio",
		"command": "node",
		"args": []string{
			filepath.Join(repoRoot, "mcp", "hybrid-memory", "mcp", "mcp-server.js"),
		},
		"env": map[string]string{
			"HYBRID_MEMORY_DIR":  filepath.Join(home, ".smolbot", "memory"),
			"OLLAMA_HOST":        "http://localhost:11434",
			"OLLAMA_EMBED_MODEL": "mxbai-embed-large",
		},
		"toolTimeout":  30,
		"enabledTools": []string{"*"},
	}
}

func loadExistingMCPServers(configPath string) map[string]any {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return map[string]any{}
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not parse existing config %s: %v\n", configPath, err)
		return map[string]any{}
	}
	tools, ok := raw["tools"].(map[string]any)
	if !ok {
		return map[string]any{}
	}
	mcpServers, ok := tools["mcpServers"].(map[string]any)
	if !ok {
		return map[string]any{}
	}
	out := make(map[string]any, len(mcpServers))
	for key, value := range mcpServers {
		out[key] = value
	}
	return out
}
