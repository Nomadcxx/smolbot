// cmd/installer/tasks.go
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Task: Clone repository
func cloneRepository(m *model) error {
	// Create a subdirectory for the clone
	cloneDir := filepath.Join(m.projectDir, "smolbot")
	
	setLastCommand("git", "clone", "--depth", "1", "https://github.com/Nomadcxx/smolbot.git", cloneDir)

	result := runCommand(m, "git", "clone", "--depth", "1",
		"https://github.com/Nomadcxx/smolbot.git", cloneDir)

	if result.Err != nil {
		return CommandError{
			Command:  "git clone",
			ExitCode: result.ExitCode,
			Output:   result.Output,
			Duration: result.Duration,
			Err:      result.Err,
		}
	}

	// Update projectDir to the cloned directory
	m.projectDir = cloneDir
	return nil
}

// Task: Build nanobot binary
func buildSmolbot(m *model) error {
	setLastCommand("go", "build", "-o", "smolbot", "./cmd/smolbot")

	result := runCommand(m, "go", "build", "-o", "smolbot", "./cmd/smolbot")

	if result.Err != nil {
		return CommandError{
			Command:  "go build nanobot",
			ExitCode: result.ExitCode,
			Output:   result.Output,
			Duration: result.Duration,
			Err:      result.Err,
		}
	}
	return nil
}

// Task: Build nanobot-tui binary
func buildSmolbotTUI(m *model) error {
	setLastCommand("go", "build", "-o", "smolbot-tui", "./cmd/smolbot-tui")

	result := runCommand(m, "go", "build", "-o", "smolbot-tui", "./cmd/smolbot-tui")

	if result.Err != nil {
		return CommandError{
			Command:  "go build smolbot-tui",
			ExitCode: result.ExitCode,
			Output:   result.Output,
			Duration: result.Duration,
			Err:      result.Err,
		}
	}
	return nil
}

// Task: Install binaries to ~/.local/bin
func installBinaries(m *model) error {
	binDir := filepath.Join(os.Getenv("HOME"), ".local", "bin")

	// Ensure directory exists
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("create bin directory: %w", err)
	}

	// Install binaries
	for _, binary := range []string{"smolbot", "smolbot-tui"} {
		src := filepath.Join(m.projectDir, binary)
		dst := filepath.Join(binDir, binary)

		// Read source
		data, err := os.ReadFile(src)
		if err != nil {
			return fmt.Errorf("read %s: %w", binary, err)
		}

		// Write destination
		if err := os.WriteFile(dst, data, 0755); err != nil {
			return fmt.Errorf("install %s: %w", binary, err)
		}
	}

	return nil
}

// Task: Remove binaries (for uninstall)
func removeBinaries(m *model) error {
	binDir := filepath.Join(os.Getenv("HOME"), ".local", "bin")

	for _, binary := range []string{"smolbot", "smolbot-tui"} {
		dst := filepath.Join(binDir, binary)
		if err := os.Remove(dst); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", binary, err)
		}
	}

	return nil
}

// Task: Create workspace directory
func createWorkspace(m *model) error {
	if err := os.MkdirAll(m.workspacePath, 0755); err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}

	// Create memory subdirectory
	memoryPath := filepath.Join(m.workspacePath, "memory")
	if err := os.MkdirAll(memoryPath, 0755); err != nil {
		return fmt.Errorf("create memory directory: %w", err)
	}

	// Create SOUL.md and HEARTBEAT.md
	soulPath := filepath.Join(m.workspacePath, "SOUL.md")
	if _, err := os.Stat(soulPath); os.IsNotExist(err) {
		soulContent := "# Agent Personality\n\nYou are smolbot, a practical coding assistant. You help users write code, debug issues, and understand complex systems.\n"
		if err := os.WriteFile(soulPath, []byte(soulContent), 0644); err != nil {
			return fmt.Errorf("create SOUL.md: %w", err)
		}
	}

	heartbeatPath := filepath.Join(m.workspacePath, "HEARTBEAT.md")
	if _, err := os.Stat(heartbeatPath); os.IsNotExist(err) {
		heartbeatContent := "# Heartbeat Instructions\n\nPeriodic status checks and system maintenance.\n"
		if err := os.WriteFile(heartbeatPath, []byte(heartbeatContent), 0644); err != nil {
			return fmt.Errorf("create HEARTBEAT.md: %w", err)
		}
	}

	return nil
}

// Task: Remove workspace (for uninstall)
func removeWorkspace(m *model) error {
	// Remove current smolbot directory
	baseDir := filepath.Join(os.Getenv("HOME"), ".smolbot")
	if err := os.RemoveAll(baseDir); err != nil {
		return fmt.Errorf("remove workspace: %w", err)
	}
	
	// Also remove legacy nanobot directory if it exists
	legacyDir := filepath.Join(os.Getenv("HOME"), ".nanobot")
	if err := os.RemoveAll(legacyDir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove legacy workspace: %w", err)
	}
	
	return nil
}

// Task: Write config file with full structure
func writeConfig(m *model) error {
	config := map[string]interface{}{
		"agents": map[string]interface{}{
			"defaults": map[string]interface{}{
				"model":               m.selectedModel,
				"provider":            m.provider,
				"workspace":           m.workspacePath,
				"maxTokens":           8192,
				"contextWindowTokens": 128000,
				"temperature":         0.7,
				"maxToolIterations":   40,
				"compression": map[string]interface{}{
					"enabled":          true,
					"mode":             "default",
					"thresholdPercent": 60,
				},
			},
		},
		"providers":    map[string]interface{}{},
		"channels": map[string]interface{}{
			"sendProgress": true,
			"sendToolHints": true,
			"signal": map[string]interface{}{
				"enabled": false,
				"account": "",
				"cliPath": "signal-cli",
				"dataDir": filepath.Join(os.Getenv("HOME"), ".smolbot", "signal"),
			},
			"whatsapp": map[string]interface{}{
				"enabled":    false,
				"deviceName": "smolbot",
				"storePath":  filepath.Join(os.Getenv("HOME"), ".smolbot", "whatsapp.db"),
			},
		},
		"gateway": map[string]interface{}{
			"host": "127.0.0.1",
			"port": m.port,
			"heartbeat": map[string]interface{}{
				"enabled":  true,
				"interval": 60,
				"channel":  "",
			},
		},
		"tools": map[string]interface{}{
			"web": map[string]interface{}{
				"searchBackend": "duckduckgo",
				"maxResults":    5,
			},
			"exec": map[string]interface{}{
				"defaultTimeout":       60,
				"maxTimeout":           600,
				"denyPatterns":         []string{"rm -rf /", "dd if="},
				"restrictToWorkspace":  true,
			},
			"mcpServers": map[string]interface{}{},
		},
	}

	// Add provider-specific configuration
	providers := config["providers"].(map[string]interface{})
	switch m.provider {
	case providerOllama:
		providers["ollama"] = map[string]interface{}{
			"apiBase": m.ollamaURL,
		}
	case providerOpenAI:
		providers["openai"] = map[string]interface{}{
			"apiKey": m.apiKey,
		}
		if m.apiEndpoint != "" {
			providers["openai"].(map[string]interface{})["apiBase"] = m.apiEndpoint
		}
	case providerAnthropic:
		providers["anthropic"] = map[string]interface{}{
			"apiKey": m.apiKey,
		}
	case providerAzure:
		providers["azure"] = map[string]interface{}{
			"apiKey":   m.apiKey,
			"endpoint": m.apiEndpoint,
		}
	case providerCustom:
		providers["custom"] = map[string]interface{}{
			"apiBase": m.apiEndpoint,
			"apiKey":  m.apiKey,
		}
	}

	// Update channel settings if enabled
	channels := config["channels"].(map[string]interface{})
	if signalEnabled {
		signalConfig := channels["signal"].(map[string]interface{})
		signalConfig["enabled"] = true
		if m.signalCLIPath != "" {
			signalConfig["cliPath"] = m.signalCLIPath
		}
	}
	if whatsappEnabled {
		whatsappConfig := channels["whatsapp"].(map[string]interface{})
		whatsappConfig["enabled"] = true
		if m.whatsappDBPath != "" {
			whatsappConfig["storePath"] = m.whatsappDBPath
		}
	}

	// Marshal to JSON
	configData, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	// Ensure config directory exists
	configDir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	if err := os.WriteFile(m.configPath, configData, 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

// Task: Remove config (for uninstall)
func removeConfig(m *model) error {
	// Remove current config
	if err := os.Remove(m.configPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove config: %w", err)
	}
	
	// Also remove legacy config if it exists
	legacyConfigPath := filepath.Join(os.Getenv("HOME"), ".nanobot", "config.json")
	if err := os.Remove(legacyConfigPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove legacy config: %w", err)
	}
	
	return nil
}

// Task: Setup systemd user service
func setupSystemd(m *model) error {
	serviceDir := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user")
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return fmt.Errorf("create systemd dir: %w", err)
	}

	serviceContent := fmt.Sprintf(`[Unit]
Description=smolbot - AI coding assistant
After=network.target

[Service]
Type=simple
ExecStart=%s/.local/bin/smolbot run --config %s --workspace %s --port %d
Restart=on-failure
RestartSec=5
Environment=HOME=%s

[Install]
WantedBy=default.target
`, os.Getenv("HOME"), m.configPath, m.workspacePath, m.port, os.Getenv("HOME"))

	servicePath := filepath.Join(serviceDir, "smolbot.service")
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("write service file: %w", err)
	}

	// Reload systemd
	result := runCommand(m, "systemctl", "--user", "daemon-reload")
	if result.Err != nil {
		return fmt.Errorf("daemon-reload: %w", result.Err)
	}

	// Enable service
	result = runCommand(m, "systemctl", "--user", "enable", "smolbot")
	if result.Err != nil {
		return fmt.Errorf("enable service: %w", result.Err)
	}

	return nil
}

// Task: Disable systemd service (for uninstall)
func disableSystemd(m *model) error {
	// Disable service
	result := runCommand(m, "systemctl", "--user", "disable", "smolbot")
	if result.Err != nil {
		// Service might not exist, that's ok
		return nil
	}

	// Remove service file
	servicePath := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user", "smolbot.service")
	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove service file: %w", err)
	}

	// Reload systemd
	runCommand(m, "systemctl", "--user", "daemon-reload")

	return nil
}

// Task: Start service
func startService(m *model) error {
	result := runCommand(m, "systemctl", "--user", "start", "smolbot")
	if result.Err != nil {
		return fmt.Errorf("start service: %w", result.Err)
	}
	return nil
}

// Task: Stop service
func stopService(m *model) error {
	result := runCommand(m, "systemctl", "--user", "stop", "smolbot")
	if result.Err != nil {
		// Service might not be running, that's ok
		return nil
	}
	return nil
}

// Task: Backup config with timestamp
func backupConfig(m *model) error {
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("read config for backup: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	backupPath := m.configPath + ".backup." + timestamp
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("backup config: %w", err)
	}

	return nil
}

// Compare versions (simple string comparison)
func compareVersions(v1, v2 string) int {
	// Remove 'v' prefix if present
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")

	// Simple comparison (assumes semantic versioning)
	if v1 == v2 {
		return 0
	}
	if v1 < v2 {
		return -1
	}
	return 1
}

// Initialize tasks based on install mode
func (m *model) initTasks() {
	if m.updateMode {
		// Upgrade mode
		m.tasks = []installTask{
			{name: "Backup config", description: "Backing up existing config", execute: backupConfig},
			{name: "Stop service", description: "Stopping smolbot", execute: stopService},
			{name: "Build smolbot", description: "Building daemon binary", execute: buildSmolbot},
			{name: "Build smolbot-tui", description: "Building TUI binary", execute: buildSmolbotTUI},
			{name: "Install binaries", description: "Installing to ~/.local/bin", execute: installBinaries},
			{name: "Setup systemd", description: "Updating systemd service", execute: setupSystemd, optional: true},
		}

		if m.daemonWasRunning {
			m.tasks = append(m.tasks, installTask{
				name:        "Start service",
				description: "Starting smolbot",
				execute:     startService,
				optional:    true,
			})
		}
	} else {
		// Fresh install
		m.tasks = []installTask{
			{name: "Build smolbot", description: "Building daemon binary", execute: buildSmolbot},
			{name: "Build smolbot-tui", description: "Building TUI binary", execute: buildSmolbotTUI},
			{name: "Install binaries", description: "Installing to ~/.local/bin", execute: installBinaries},
			{name: "Create workspace", description: "Creating ~/.smolbot/workspace", execute: createWorkspace},
			{name: "Write config", description: "Writing config.json", execute: writeConfig},
			{name: "Setup systemd", description: "Installing user service", execute: setupSystemd},
		}

		if m.enableService && m.startNow {
			m.tasks = append(m.tasks, installTask{
				name:        "Start service",
				description: "Starting smolbot",
				execute:     startService,
				optional:    true,
			})
		}
	}
}

// Initialize uninstall tasks
func (m *model) initUninstallTasks() {
	m.tasks = []installTask{
		{name: "Stop service", description: "Stopping smolbot", execute: stopService, optional: true},
		{name: "Disable systemd", description: "Removing systemd service", execute: disableSystemd, optional: true},
		{name: "Remove binaries", description: "Removing from ~/.local/bin", execute: removeBinaries},
		{name: "Remove config", description: "Removing config.json", execute: removeConfig, optional: true},
		{name: "Remove workspace", description: "Removing ~/.smolbot", execute: removeWorkspace, optional: true},
	}
}
