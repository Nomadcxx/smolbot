// cmd/installer/views.go
package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Welcome view
func (m model) welcomeView() string {
	var b strings.Builder

	// ASCII art
	b.WriteString(getSmolbotArt())
	b.WriteString("\n")

	// Welcome message
	title := headerStyle.Render("Welcome to nanobot-go")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Description
	desc := "AI-powered coding assistant for your terminal\n"
	desc += "Version: 1.0.0\n\n"
	b.WriteString(descriptionStyle.Render(desc))

	// Mode indicator
	if m.updateMode {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(defaultTheme.WarningColor)).
			Render("⚡ Upgrade mode detected (v" + m.existingVersion + ")"))
		b.WriteString("\n\n")
	}

	// Instructions
	b.WriteString(mutedStyle.Render("Press Enter to continue, or 'q' to exit"))

	return b.String()
}

// Prerequisites view
func (m model) prerequisitesView() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Prerequisites Check"))
	b.WriteString("\n\n")

	// Check Go
	goStatus := "[PENDING]"
	if _, err := exec.LookPath("go"); err == nil {
		goStatus = checkMark.String() + " Go detected"
	} else {
		goStatus = failMark.String() + " Go not found (required)"
	}
	b.WriteString(goStatus + "\n")

	// Check Git
	gitStatus := "[PENDING]"
	if _, err := exec.LookPath("git"); err == nil {
		gitStatus = checkMark.String() + " Git detected"
	} else {
		gitStatus = failMark.String() + " Git not found (required)"
	}
	b.WriteString(gitStatus + "\n")

	// Check Ollama (optional)
	ollamaStatus := checkMark.String() + " Ollama detected"
	if !m.ollamaDetected {
		ollamaStatus = skipMark.String() + " Ollama not detected (optional)"
	}
	b.WriteString(ollamaStatus + "\n")

	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("Press Enter to continue"))

	return b.String()
}

// Provider selection view
func (m model) providerView() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Select AI Provider"))
	b.WriteString("\n\n")

	providers := []struct {
		name        string
		description string
	}{
		{"Ollama", "Local AI models (recommended)"},
		{"OpenAI", "GPT-4, GPT-3.5 (requires API key)"},
		{"Anthropic", "Claude models (requires API key)"},
		{"Azure OpenAI", "Enterprise OpenAI (requires endpoint + key)"},
		{"Custom", "OpenAI-compatible endpoint"},
	}

	for i, p := range providers {
		marker := "○"
		if i == m.providerIndex {
			marker = "●"
			b.WriteString(selectedStyle.Render(fmt.Sprintf("  %s %-15s %s", marker, p.name, mutedStyle.Render(p.description))))
		} else {
			b.WriteString(fmt.Sprintf("  %s %-15s %s", marker, p.name, mutedStyle.Render(p.description)))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("↑/↓ to select, Enter to continue, 'b' to go back"))

	return b.String()
}

// Configuration view - provider-specific
func (m model) configurationView() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Configuration"))
	b.WriteString("\n\n")

	// Show provider
	providerNames := []string{"Ollama", "OpenAI", "Anthropic", "Azure OpenAI", "Custom"}
	b.WriteString(fmt.Sprintf("Provider: %s\n\n", providerNames[m.providerIndex]))

	switch m.providerIndex {
	case 0: // Ollama
		b.WriteString("Select Default Model:\n\n")

		if m.ollamaDetecting {
			b.WriteString("  " + m.spinner.View() + " Detecting Ollama models...\n")
		} else if len(m.ollamaModels) > 0 {
			for i, model := range m.ollamaModels {
				marker := "○"
				if i == m.ollamaModelIndex {
					marker = "●"
				}
				b.WriteString(fmt.Sprintf("  %s %s\n", marker, model))
			}
		} else {
			b.WriteString("  No Ollama models detected\n")
			b.WriteString("  Run 'ollama pull llama2' to download a model\n")
		}

		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("↑/↓ to select, Enter to continue"))

	case 1: // OpenAI
		b.WriteString("OpenAI Configuration:\n\n")
		b.WriteString("API Key:\n")
		b.WriteString(m.inputs[3].View())
		b.WriteString("\n\n")
		b.WriteString("Model (default: gpt-4):\n")
		if m.selectedModel == "" {
			m.selectedModel = "gpt-4"
		}
		b.WriteString(fmt.Sprintf("  %s\n", m.selectedModel))
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("Tab to switch fields, Enter to continue"))

	case 2: // Anthropic
		b.WriteString("Anthropic Configuration:\n\n")
		b.WriteString("API Key:\n")
		b.WriteString(m.inputs[3].View())
		b.WriteString("\n\n")
		b.WriteString("Model (default: claude-3-sonnet):\n")
		if m.selectedModel == "" {
			m.selectedModel = "claude-3-sonnet-20240229"
		}
		b.WriteString(fmt.Sprintf("  %s\n", m.selectedModel))
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("Tab to switch fields, Enter to continue"))

	case 3: // Azure
		b.WriteString("Azure OpenAI Configuration:\n\n")
		b.WriteString("API Key:\n")
		b.WriteString(m.inputs[3].View())
		b.WriteString("\n\n")
		b.WriteString("Endpoint URL:\n")
		b.WriteString(m.inputs[1].View())
		b.WriteString("\n\n")
		b.WriteString("Model (default: gpt-4):\n")
		if m.selectedModel == "" {
			m.selectedModel = "gpt-4"
		}
		b.WriteString(fmt.Sprintf("  %s\n", m.selectedModel))
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("Tab to switch fields, Enter to continue"))

	case 4: // Custom
		b.WriteString("Custom Provider Configuration:\n\n")
		b.WriteString("API Endpoint:\n")
		b.WriteString(m.inputs[1].View())
		b.WriteString("\n\n")
		b.WriteString("API Key (optional):\n")
		b.WriteString(m.inputs[3].View())
		b.WriteString("\n\n")
		b.WriteString("Model name:\n")
		if m.selectedModel == "" {
			m.selectedModel = "default"
		}
		b.WriteString(fmt.Sprintf("  %s\n", m.selectedModel))
		b.WriteString("\n")
		b.WriteString(mutedStyle.Render("Tab to switch fields, Enter to continue"))
	}

	return b.String()
}

// Channel setup view
func (m model) channelsView() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Channel Setup (Optional)"))
	b.WriteString("\n\n")

	// Signal
	signalStatus := "[ ] Disabled"
	if signalEnabled {
		signalStatus = "[✓] Enabled"
	}
	b.WriteString(fmt.Sprintf("Signal Integration  %s\n", signalStatus))
	if signalEnabled {
		b.WriteString(mutedStyle.Render("  signal-cli path: " + m.inputs[1].View()))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// WhatsApp
	whatsappStatus := "[ ] Disabled"
	if whatsappEnabled {
		whatsappStatus = "[✓] Enabled"
	}
	b.WriteString(fmt.Sprintf("WhatsApp Integration  %s\n", whatsappStatus))

	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("←/→ to toggle, Enter to continue, 'b' to go back"))
	b.WriteString("\n\n")
	b.WriteString(mutedStyle.Render("Note: Channel setup can be configured later in config.json"))

	return b.String()
}

// Service setup view
func (m model) serviceView() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Service Setup"))
	b.WriteString("\n\n")

	// Service options
	enableStatus := "[ ] No"
	if m.enableService {
		enableStatus = "[✓] Yes"
	}
	b.WriteString(fmt.Sprintf("Enable systemd service    %s\n", enableStatus))

	startStatus := "[ ] No"
	if m.startNow {
		startStatus = "[✓] Yes"
	}
	b.WriteString(fmt.Sprintf("Start service now         %s\n", startStatus))

	b.WriteString("\n")

	// Port
	b.WriteString("Gateway Port:\n")
	if m.focusedInput == 0 {
		b.WriteString(inputFocusedStyle.Render(m.inputs[0].View()))
	} else {
		b.WriteString(inputStyle.Render(m.inputs[0].View()))
	}
	b.WriteString("\n\n")

	// Workspace path
	b.WriteString("Workspace Path:\n")
	if m.focusedInput == 1 {
		b.WriteString(inputFocusedStyle.Render(m.inputs[1].View()))
	} else {
		b.WriteString(inputStyle.Render(m.inputs[1].View()))
	}
	b.WriteString("\n\n")

	// Config path
	b.WriteString("Config File Path:\n")
	if m.focusedInput == 2 {
		b.WriteString(inputFocusedStyle.Render(m.inputs[2].View()))
	} else {
		b.WriteString(inputStyle.Render(m.inputs[2].View()))
	}
	b.WriteString("\n\n")

	// Show errors if any
	if len(m.errors) > 0 {
		for _, err := range m.errors {
			b.WriteString(errorStyle.Render("✗ " + err))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(mutedStyle.Render("←/→ to toggle options, Tab to switch fields, Enter to install, 'b' to go back"))

	return b.String()
}

// Installing view
func (m model) installingView() string {
	var b strings.Builder

	if m.updateMode {
		b.WriteString(headerStyle.Render("Upgrading nanobot-go"))
	} else {
		b.WriteString(headerStyle.Render("Installing nanobot-go"))
	}
	b.WriteString("\n\n")

	// Task list
	for i, task := range m.tasks {
		status := "[PENDING]"
		switch task.status {
		case statusRunning:
			status = m.spinner.View() + " " + task.description
		case statusComplete:
			status = checkMark.String() + " " + task.description
		case statusFailed:
			status = failMark.String() + " " + task.description
			if task.optional {
				status += " (optional)"
			}
		case statusSkipped:
			status = skipMark.String() + " " + task.description
		}

		if i == m.currentTaskIndex && task.status == statusRunning {
			b.WriteString(lipgloss.NewStyle().Bold(true).Render(status))
		} else {
			b.WriteString(status)
		}
		b.WriteString("\n")
	}

	// Show error if any
	if m.currentTaskIndex < len(m.tasks) && m.tasks[m.currentTaskIndex].errorDetails != nil {
		b.WriteString("\n")
		errInfo := m.tasks[m.currentTaskIndex].errorDetails
		b.WriteString(errorStyle.Render("Error: " + errInfo.message))
		b.WriteString("\n")
		if m.tasks[m.currentTaskIndex].optional {
			b.WriteString(mutedStyle.Render("Press 's' to skip this optional task"))
			b.WriteString("  ")
		}
		b.WriteString(mutedStyle.Render("Press 'r' to retry"))
	}

	return b.String()
}

// Complete view
func (m model) completeView() string {
	var b strings.Builder

	b.WriteString(successStyle.Render("✓ Installation Complete!"))
	b.WriteString("\n\n")

	if m.updateMode {
		b.WriteString(fmt.Sprintf("Successfully upgraded nanobot-go\n"))
		if m.existingVersion != "" {
			b.WriteString(fmt.Sprintf("From: %s\n", m.existingVersion))
		}
		b.WriteString("To:   v1.0.0\n\n")
	} else {
		b.WriteString("nanobot-go is installed and ready to use!\n\n")
	}

	b.WriteString(titleStyle.Render("Installation Summary:"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Binaries: ~/.local/bin/nanobot, ~/.local/bin/nanobot-tui\n"))
	b.WriteString(fmt.Sprintf("  Config: %s\n", m.configPath))
	b.WriteString(fmt.Sprintf("  Workspace: %s\n", m.workspacePath))
	if m.enableService {
		b.WriteString("  Service: systemd user service enabled\n")
	}
	b.WriteString("\n")

	b.WriteString(titleStyle.Render("Quick Start:"))
	b.WriteString("\n")
	b.WriteString("  nanobot-tui              # Launch TUI\n")
	b.WriteString("  nanobot chat \"hello\"     # Quick CLI chat\n")
	b.WriteString("\n")

	if m.enableService {
		b.WriteString(titleStyle.Render("Systemd Commands:"))
		b.WriteString("\n")
		b.WriteString("  systemctl --user status nanobot-go\n")
		b.WriteString("  systemctl --user stop nanobot-go\n")
		b.WriteString("  systemctl --user restart nanobot-go\n")
		b.WriteString("\n")
	}

	b.WriteString(mutedStyle.Render("Press Enter or 'q' to exit"))

	return b.String()
}

// Uninstall view
func (m model) uninstallView() string {
	var b strings.Builder

	b.WriteString(errorStyle.Render("⚠ Uninstall nanobot-go"))
	b.WriteString("\n\n")

	b.WriteString("The following will be removed:\n\n")
	b.WriteString("  Binaries:\n")
	b.WriteString("    ~/.local/bin/nanobot\n")
	b.WriteString("    ~/.local/bin/nanobot-tui\n\n")
	b.WriteString("  Config:\n")
	b.WriteString("    ~/.nanobot/config.json\n\n")
	b.WriteString("  Workspace:\n")
	b.WriteString("    ~/.nanobot/workspace/\n\n")

	b.WriteString(errorStyle.Render("Warning: This action cannot be undone!"))
	b.WriteString("\n\n")

	b.WriteString("Options:\n")
	b.WriteString("  [●] Remove binaries only\n")
	b.WriteString("  [ ] Remove binaries + config\n")
	b.WriteString("  [ ] Remove everything (including workspace)\n")
	b.WriteString("\n")

	b.WriteString(mutedStyle.Render("Press Enter to confirm uninstall, 'b' to go back"))

	return b.String()
}
