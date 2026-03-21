// cmd/installer/tasks.go
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Task: Clone repository
func cloneRepository(m *model) error {
	setLastCommand("git", "clone", "--depth", "1", "https://github.com/Nomadcxx/smolbot.git", m.projectDir)

	result := runCommand(m, "git", "clone", "--depth", "1",
		"https://github.com/Nomadcxx/smolbot.git", m.projectDir)

	if result.Err != nil {
		return CommandError{
			Command:  "git clone",
			ExitCode: result.ExitCode,
			Output:   result.Output,
			Duration: result.Duration,
			Err:      result.Err,
		}
	}
	return nil
}

// Task: Build nanobot binary
func buildNanobot(m *model) error {
	setLastCommand("go", "build", "-o", "nanobot", "./cmd/nanobot")

	result := runCommand(m, "go", "build", "-o", "nanobot", "./cmd/nanobot")

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
func buildNanobotTUI(m *model) error {
	setLastCommand("go", "build", "-o", "nanobot-tui", "./cmd/nanobot-tui")

	result := runCommand(m, "go", "build", "-o", "nanobot-tui", "./cmd/nanobot-tui")

	if result.Err != nil {
		return CommandError{
			Command:  "go build nanobot-tui",
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
	for _, binary := range []string{"nanobot", "nanobot-tui"} {
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

// Task: Create workspace directory
func createWorkspace(m *model) error {
	if err := os.MkdirAll(m.workspacePath, 0755); err != nil {
		return fmt.Errorf("create workspace: %w", err)
	}
	return nil
}

// Task: Write config file
func writeConfig(m *model) error {
	configContent := fmt.Sprintf(`{
  "agents": {
    "defaults": {
      "model": "%s",
      "provider": "ollama",
      "maxTokens": 4096
    }
  },
  "providers": {
    "ollama": {
      "apiBase": "%s"
    }
  }
}`, m.selectedModel, m.ollamaURL)

	if err := os.WriteFile(m.configPath, []byte(configContent), 0644); err != nil {
		return fmt.Errorf("write config: %w", err)
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
Description=nanobot-go - AI coding assistant
After=network.target

[Service]
Type=simple
ExecStart=%s/.local/bin/nanobot run --config %s --port %d
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`, os.Getenv("HOME"), m.configPath, m.port)

	servicePath := filepath.Join(serviceDir, "nanobot-go.service")
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("write service file: %w", err)
	}

	// Reload systemd
	result := runCommand(m, "systemctl", "--user", "daemon-reload")
	if result.Err != nil {
		return fmt.Errorf("daemon-reload: %w", result.Err)
	}

	// Enable service
	result = runCommand(m, "systemctl", "--user", "enable", "nanobot-go")
	if result.Err != nil {
		return fmt.Errorf("enable service: %w", result.Err)
	}

	return nil
}

// Task: Start service
func startService(m *model) error {
	result := runCommand(m, "systemctl", "--user", "start", "nanobot-go")
	if result.Err != nil {
		return fmt.Errorf("start service: %w", result.Err)
	}
	return nil
}

// Task: Stop service (for upgrade)
func stopService(m *model) error {
	result := runCommand(m, "systemctl", "--user", "stop", "nanobot-go")
	if result.Err != nil {
		// Service might not be running, that's ok
		return nil
	}
	return nil
}

// Task: Backup config (for upgrade)
func backupConfig(m *model) error {
	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return nil
	}

	backupPath := m.configPath + ".backup"
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("backup config: %w", err)
	}
	return nil
}

// Initialize tasks based on install mode
func (m *model) initTasks() {
	if m.updateMode {
		// Upgrade mode
		m.tasks = []installTask{
			{name: "Backup config", description: "Backing up existing config", execute: backupConfig},
			{name: "Stop service", description: "Stopping nanobot-go", execute: stopService},
			{name: "Clone repository", description: "Cloning smolbot", execute: cloneRepository},
			{name: "Build nanobot", description: "Building daemon binary", execute: buildNanobot},
			{name: "Build nanobot-tui", description: "Building TUI binary", execute: buildNanobotTUI},
			{name: "Install binaries", description: "Installing to ~/.local/bin", execute: installBinaries},
			{name: "Setup systemd", description: "Updating systemd service", execute: setupSystemd, optional: true},
		}

		if m.daemonWasRunning {
			m.tasks = append(m.tasks, installTask{
				name:        "Start service",
				description: "Starting nanobot-go",
				execute:     startService,
				optional:    true,
			})
		}
	} else {
		// Fresh install
		m.tasks = []installTask{
			{name: "Clone repository", description: "Cloning smolbot", execute: cloneRepository},
			{name: "Build nanobot", description: "Building daemon binary", execute: buildNanobot},
			{name: "Build nanobot-tui", description: "Building TUI binary", execute: buildNanobotTUI},
			{name: "Install binaries", description: "Installing to ~/.local/bin", execute: installBinaries},
			{name: "Create workspace", description: "Creating ~/.nanobot-go", execute: createWorkspace},
			{name: "Write config", description: "Writing config.json", execute: writeConfig},
			{name: "Setup systemd", description: "Installing user service", execute: setupSystemd},
		}

		if m.enableService {
			m.tasks = append(m.tasks, installTask{
				name:        "Start service",
				description: "Starting nanobot-go",
				execute:     startService,
				optional:    true,
			})
		}
	}
}
