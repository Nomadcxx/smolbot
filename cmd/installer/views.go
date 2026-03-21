// cmd/installer/views.go
package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// View renders the UI
func (m model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Terminal size check
	if m.width < 80 || m.height < 24 {
		return lipgloss.NewStyle().
			Foreground(ErrorColor).
			Background(BgBase).
			Bold(true).
			Width(m.width).
			Height(m.height).
			Render(fmt.Sprintf(
				"Terminal too small!\n\nMinimum: 80x24\nCurrent: %dx%d\n\nPlease resize.",
				m.width, m.height,
			))
	}

	var content strings.Builder

	// ASCII Header with Beams Animation
	if m.beams != nil {
		headerRendered := m.beams.Render()
		content.WriteString(headerRendered)
		content.WriteString("\n")
	}

	// Ticker/Tagline with Typewriter Animation
	if m.ticker != nil {
		tickerText := m.ticker.Render(m.width)
		tickerStyled := lipgloss.NewStyle().
			Foreground(FgMuted).
			Background(BgBase).
			Italic(true).
			Render(tickerText)
		content.WriteString(tickerStyled)
		content.WriteString("\n\n")
	}

	// Main content based on step
	var mainContent string
	switch m.step {
	case stepWelcome:
		mainContent = m.renderWelcome()
	case stepPrerequisites:
		mainContent = m.renderPrerequisites()
	case stepProvider:
		mainContent = m.renderProvider()
	case stepConfiguration:
		mainContent = m.renderConfiguration()
	case stepChannels:
		mainContent = m.renderChannels()
	case stepService:
		mainContent = m.renderService()
	case stepInstalling:
		mainContent = m.renderInstalling()
	case stepComplete:
		mainContent = m.renderComplete()
	case stepUninstall:
		mainContent = m.renderUninstall()
	}

	// Boxed content with rounded border
	mainStyle := lipgloss.NewStyle().
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Secondary).
		Foreground(FgPrimary).
		Background(BgBase).
		Width(m.width - 4)

	mainRendered := mainStyle.Render(mainContent)
	mainHeight := lipgloss.Height(mainRendered)

	// Place content with filled background
	mainPlaced := lipgloss.Place(
		m.width-4, mainHeight,
		lipgloss.Left, lipgloss.Top,
		mainRendered,
		lipgloss.WithWhitespaceBackground(BgBase),
	)
	content.WriteString(mainPlaced)
	content.WriteString("\n")

	// Help text footer
	helpText := m.getHelpText()
	if helpText != "" {
		helpStyle := lipgloss.NewStyle().
			Foreground(FgMuted).
			Background(BgBase).
			Italic(true).
			Width(m.width).
			Align(lipgloss.Center)
		content.WriteString("\n" + helpStyle.Render(helpText))
	}

	// Full-screen wrapper
	bgStyle := lipgloss.NewStyle().
		Background(BgBase).
		Foreground(FgPrimary).
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Top)

	return bgStyle.Render(content.String())
}

// getHelpText returns context-sensitive help text
func (m model) getHelpText() string {
	switch m.step {
	case stepWelcome:
		return "↑/↓: Navigate  •  Enter: Continue  •  q: Quit"
	case stepPrerequisites:
		return "Enter: Continue  •  q: Quit"
	case stepProvider:
		return "↑/↓: Navigate  •  Enter: Select  •  Esc: Back"
	case stepConfiguration:
		return "Tab: Next field  •  ↑/↓: Navigate  •  Enter: Continue  •  Esc: Back"
	case stepChannels:
		return "←/→: Toggle  •  Enter: Continue  •  Esc: Back"
	case stepService:
		return "Tab: Next option  •  ←/→: Toggle  •  Enter: Install  •  Esc: Back"
	case stepInstalling:
		return "Please wait..."
	case stepComplete:
		return "Enter: Exit"
	case stepUninstall:
		return "Enter: Confirm  •  Esc: Back"
	}
	return ""
}

// Welcome screen
func (m model) renderWelcome() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("SMOLBOT Installer"))
	b.WriteString("\n\n")

	options := []struct {
		label       string
		description string
	}{
		{"Install", "Fresh installation with full configuration"},
	}

	if m.existingInstall {
		options = append([]struct {
			label       string
			description string
		}{
			{"Update", "Update existing installation"},
		}, options...)
		options = append(options, struct {
			label       string
			description string
		}{"Uninstall", "Remove SMOLBOT from system"})
	}

	for i, opt := range options {
		marker := "○"
		if i == m.selectedOption {
			marker = "●"
		}
		if i == m.selectedOption {
			b.WriteString(lipgloss.NewStyle().Foreground(Primary).Bold(true).
				Render(fmt.Sprintf("  %s %s", marker, opt.label)))
		} else {
			b.WriteString(fmt.Sprintf("  %s %s", marker, opt.label))
		}
		b.WriteString("\n")
		if opt.description != "" {
			b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).
				Render(fmt.Sprintf("     %s", opt.description)))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// Prerequisites screen
func (m model) renderPrerequisites() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Prerequisites Check"))
	b.WriteString("\n\n")

	// Go
	if m.hasGo {
		b.WriteString(checkMark.String() + " Go detected\n")
	} else {
		b.WriteString(failMark.String() + " Go not found (required)\n")
	}

	// Git
	if m.hasGit {
		b.WriteString(checkMark.String() + " Git detected\n")
	} else {
		b.WriteString(failMark.String() + " Git not found (required)\n")
	}

	// Ollama (optional)
	if m.ollamaDetected {
		b.WriteString(checkMark.String() + " Ollama detected\n")
	} else {
		b.WriteString(skipMark.String() + " Ollama not detected (optional)\n")
	}

	return b.String()
}

// Provider selection
func (m model) renderProvider() string {
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
		}
		if i == m.providerIndex {
			b.WriteString(lipgloss.NewStyle().Foreground(Primary).Bold(true).
				Render(fmt.Sprintf("  %s %-15s %s", marker, p.name,
					lipgloss.NewStyle().Foreground(FgMuted).Render(p.description))))
		} else {
			b.WriteString(fmt.Sprintf("  %s %-15s %s", marker, p.name,
				lipgloss.NewStyle().Foreground(FgMuted).Render(p.description)))
		}
		b.WriteString("\n")
	}

	return b.String()
}

// Configuration screen
func (m model) renderConfiguration() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Configuration"))
	b.WriteString("\n\n")

	providerNames := []string{"Ollama", "OpenAI", "Anthropic", "Azure OpenAI", "Custom"}
	b.WriteString(fmt.Sprintf("Provider: %s\n\n", providerNames[m.providerIndex]))

	switch m.providerIndex {
	case 0: // Ollama
		b.WriteString("Select Default Model:\n\n")
		if m.ollamaDetecting {
			b.WriteString("  " + m.spinner.View() + " Detecting Ollama models...\n")
		} else if len(m.ollamaModels) > 0 {
			for i, modelName := range m.ollamaModels {
				marker := "○"
				if i == m.ollamaModelIndex {
					marker = "●"
				}
				if i == m.ollamaModelIndex {
					b.WriteString(lipgloss.NewStyle().Foreground(Primary).Bold(true).
						Render(fmt.Sprintf("  %s %s", marker, modelName)))
				} else {
					b.WriteString(fmt.Sprintf("  %s %s", marker, modelName))
				}
				b.WriteString("\n")
			}
		} else {
			b.WriteString("  No Ollama models detected\n")
		}
	}

	return b.String()
}

// Channels screen
func (m model) renderChannels() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Channel Setup (Optional)"))
	b.WriteString("\n\n")

	// Signal
	signalStatus := "[ ] Disabled"
	if m.signalEnabled {
		signalStatus = "[✓] Enabled"
	}
	b.WriteString(fmt.Sprintf("Signal Integration  %s\n", signalStatus))
	b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("  Requires signal-cli"))
	b.WriteString("\n\n")

	// WhatsApp
	whatsappStatus := "[ ] Disabled"
	if m.whatsappEnabled {
		whatsappStatus = "[✓] Enabled"
	}
	b.WriteString(fmt.Sprintf("WhatsApp Integration  %s\n", whatsappStatus))
	b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("  Requires QR code scan"))
	b.WriteString("\n\n")

	b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("Note: Can be configured later"))

	return b.String()
}

// Service screen
func (m model) renderService() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Service Setup"))
	b.WriteString("\n\n")

	// Enable service
	enableStatus := "[ ] No"
	if m.enableService {
		enableStatus = "[✓] Yes"
	}
	b.WriteString(fmt.Sprintf("Enable systemd service    %s\n", enableStatus))

	// Start now
	startStatus := "[ ] No"
	if m.startNow {
		startStatus = "[✓] Yes"
	}
	b.WriteString(fmt.Sprintf("Start service now         %s\n", startStatus))

	// Port
	b.WriteString(fmt.Sprintf("\nGateway Port: %d\n", m.port))

	return b.String()
}

// Installing screen
func (m model) renderInstalling() string {
	var b strings.Builder

	if m.updateMode {
		b.WriteString(headerStyle.Render("Upgrading SMOLBOT"))
	} else {
		b.WriteString(headerStyle.Render("Installing SMOLBOT"))
	}
	b.WriteString("\n\n")

	for i, task := range m.tasks {
		status := "[PENDING]"
		switch task.status {
		case statusRunning:
			status = m.spinner.View() + " " + task.description
		case statusComplete:
			status = checkMark.String() + " " + task.description
		case statusFailed:
			status = failMark.String() + " " + task.description
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

	return b.String()
}

// Complete screen
func (m model) renderComplete() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("✓ Installation Complete!"))
	b.WriteString("\n\n")

	if m.updateMode {
		b.WriteString("Successfully upgraded SMOLBOT\n")
	} else {
		b.WriteString("SMOLBOT is installed and ready to use!\n")
	}

	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  Config: %s\n", m.configPath))
	b.WriteString(fmt.Sprintf("  Workspace: %s\n", m.workspacePath))
	if m.enableService {
		b.WriteString("  Service: systemd user service enabled\n")
	}

	// Post-installation quick start guide
	b.WriteString("\n")
	b.WriteString(headerStyle.Render("Quick Start"))
	b.WriteString("\n\n")

	b.WriteString("Launch TUI (interactive mode):\n")
	b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("  nanobot-tui"))
	b.WriteString("\n\n")

	b.WriteString("Launch CLI (single command):\n")
	b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("  nanobot run \"your prompt here\""))
	b.WriteString("\n\n")

	b.WriteString("Service management:\n")
	b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("  systemctl --user start nanobot-go"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("  systemctl --user stop nanobot-go"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("  systemctl --user status nanobot-go"))
	b.WriteString("\n\n")

	b.WriteString("Edit configuration:\n")
	b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("  nano ~/.nanobot/config.json"))
	b.WriteString("\n")

	return b.String()
}

// Uninstall screen
func (m model) renderUninstall() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("⚠ Uninstall SMOLBOT"))
	b.WriteString("\n\n")

	b.WriteString("The following will be removed:\n\n")
	b.WriteString("  Binaries:\n")
	b.WriteString("    ~/.local/bin/nanobot\n")
	b.WriteString("    ~/.local/bin/nanobot-tui\n\n")
	b.WriteString("  Config:\n")
	b.WriteString("    ~/.nanobot/config.json\n\n")
	b.WriteString("  Workspace:\n")
	b.WriteString("    ~/.nanobot/workspace/\n\n")

	b.WriteString(lipgloss.NewStyle().Foreground(ErrorColor).Render("Warning: This action cannot be undone!"))

	return b.String()
}
