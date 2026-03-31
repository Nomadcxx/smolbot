// cmd/installer/views.go
package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/skip2/go-qrcode"
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
	case stepSignalSetup:
		mainContent = m.renderSignalSetup()
	case stepWhatsAppSetup:
		mainContent = m.renderWhatsAppSetup()
	case stepTelegramSetup:
		mainContent = m.renderTelegramSetup()
	case stepDiscordSetup:
		mainContent = m.renderDiscordSetup()
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
		return "↑/↓: Select model  •  Enter: Confirm  •  Esc: Back"
	case stepChannels:
		return "↑/↓: Navigate  •  Space: Toggle  •  Enter: Continue  •  Esc: Back"
	case stepSignalSetup:
		if m.signalDone {
			return "Enter: Continue  •  Esc: Skip"
		}
		if m.signalQRCode != "" {
			return "Scan QR with Signal  •  Enter: Done  •  Esc: Skip"
		}
		return "Enter: Start  •  Esc: Skip"
	case stepWhatsAppSetup:
		if m.whatsappDone {
			return "Enter: Continue  •  Esc: Back"
		}
		if m.whatsappQRCode != "" {
			return "Scan QR with WhatsApp  •  Enter: Done  •  Esc: Skip"
		}
		return "Enter: Start  •  Esc: Skip"
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
			b.WriteString("\n")
			b.WriteString(lipgloss.NewStyle().Foreground(SuccessColor).
				Render(fmt.Sprintf("  ✓ Selected: %s", m.ollamaModels[m.ollamaModelIndex])))
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
	signalMarker := "○"
	signalStyle := lipgloss.NewStyle()
	if m.channelIndex == 0 {
		signalMarker = "●"
		signalStyle = lipgloss.NewStyle().Foreground(Primary).Bold(true)
	}
	signalStatus := "[ ] Disabled"
	if m.signalEnabled {
		signalStatus = "[✓] Enabled"
	}
	b.WriteString(signalStyle.Render(fmt.Sprintf("  %s Signal Integration  %s", signalMarker, signalStatus)))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("    Requires signal-cli"))
	b.WriteString("\n\n")

	// WhatsApp
	waStyle := lipgloss.NewStyle()
	waMarker := "○"
	if m.channelIndex == 1 {
		waMarker = "●"
		waStyle = lipgloss.NewStyle().Foreground(Primary).Bold(true)
	}
	whatsappStatus := "[ ] Disabled"
	if m.whatsappEnabled {
		whatsappStatus = "[✓] Enabled"
	}
	b.WriteString(waStyle.Render(fmt.Sprintf("  %s WhatsApp Integration  %s", waMarker, whatsappStatus)))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("    Requires QR code scan"))
	b.WriteString("\n\n")

	b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("Note: Can be configured later"))

	return b.String()
}

func (m model) renderWhatsAppSetup() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("WhatsApp Setup"))
	b.WriteString("\n\n")

	if m.whatsappDone {
		if m.whatsappError != "" {
			b.WriteString(lipgloss.NewStyle().Foreground(ErrorColor).Render(fmt.Sprintf("  ✗ %s\n", m.whatsappStatus)))
			b.WriteString("\n")
			b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render(fmt.Sprintf("  Error: %s\n\n", m.whatsappError)))
			b.WriteString("  Press Enter to continue.\n")
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(SuccessColor).Render(fmt.Sprintf("  ✓ %s\n\n", m.whatsappStatus)))
			b.WriteString("  Press Enter to continue.\n")
		}
		return b.String()
	}

	if m.whatsappQRCode != "" {
		ascii := renderQRForTerminal(m.whatsappQRCode)
		b.WriteString("  Scan this QR code with your WhatsApp app:\n\n")
		b.WriteString(ascii)
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render(fmt.Sprintf("  %s\n", m.whatsappStatus)))
		b.WriteString("\n  Press Enter when done  •  Esc to skip\n")
	} else {
		b.WriteString(fmt.Sprintf("  %s %s\n\n", m.spinner.View(), m.whatsappStatus))
		b.WriteString("  Press Esc to skip.\n")
	}

	return b.String()
}

func (m model) renderSignalSetup() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Signal Setup"))
	b.WriteString("\n\n")

	if m.signalDone {
		if m.signalError != "" {
			b.WriteString(lipgloss.NewStyle().Foreground(ErrorColor).Render(fmt.Sprintf("  ✗ %s\n", m.signalStatus)))
			b.WriteString("\n")
			b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render(fmt.Sprintf("  Error: %s\n\n", m.signalError)))
			b.WriteString("  Press Enter to continue.\n")
		} else {
			b.WriteString(lipgloss.NewStyle().Foreground(SuccessColor).Render(fmt.Sprintf("  ✓ %s\n", m.signalStatus)))
			if m.signalAccount != "" {
				b.WriteString(fmt.Sprintf("  Linked to: %s\n\n", m.signalAccount))
			} else {
				b.WriteString("\n")
			}
			b.WriteString("  Press Enter to continue.\n")
		}
		return b.String()
	}

	if m.signalQRCode != "" {
		ascii := renderQRForTerminal(m.signalQRCode)
		b.WriteString("  Scan this QR code with your Signal app:\n\n")
		b.WriteString(ascii)
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render(fmt.Sprintf("  %s\n", m.signalStatus)))
		b.WriteString("\n  Press Enter when done  •  Esc to skip\n")
	} else {
		b.WriteString(fmt.Sprintf("  %s %s\n\n", m.spinner.View(), m.signalStatus))
		b.WriteString("  Press Esc to skip.\n")
	}

	return b.String()
}

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
	b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("  smolbot-tui"))
	b.WriteString("\n\n")

	b.WriteString("Launch CLI (single command):\n")
	b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("  smolbot run \"your prompt here\""))
	b.WriteString("\n\n")

	b.WriteString("Service management:\n")
	b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("  systemctl --user start smolbot"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("  systemctl --user stop smolbot"))
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("  systemctl --user status smolbot"))
	b.WriteString("\n\n")

	b.WriteString("Edit configuration:\n")
	b.WriteString(lipgloss.NewStyle().Foreground(FgMuted).Render("  nano ~/.smolbot/config.json"))
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
	b.WriteString("    ~/.local/bin/smolbot\n")
	b.WriteString("    ~/.local/bin/smolbot-tui\n\n")
	b.WriteString("  Config:\n")
	b.WriteString("    ~/.smolbot/config.json\n\n")
	b.WriteString("  Workspace:\n")
	b.WriteString("    ~/.smolbot/workspace/\n\n")

	b.WriteString(lipgloss.NewStyle().Foreground(ErrorColor).Render("Warning: This action cannot be undone!"))

	return b.String()
}

func (m model) renderTelegramSetup() string {
	var b strings.Builder
	b.WriteString("Telegram Setup\n\n")
	b.WriteString("Enter the path to your Telegram bot token file.\n")
	b.WriteString("Leave blank and press Enter to skip.\n\n")
	b.WriteString(m.telegramTokenInput.View())
	if m.telegramTokenInput.Err != nil {
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(ErrorColor).Render(
			"Error: " + m.telegramTokenInput.Err.Error(),
		))
	}
	b.WriteString("\n\nEnter: confirm  •  Esc: skip")
	return b.String()
}

func (m model) renderDiscordSetup() string {
	var b strings.Builder
	b.WriteString("Discord Setup\n\n")
	b.WriteString("Enter the path to your Discord bot token file.\n")
	b.WriteString("Leave blank and press Enter to skip.\n\n")
	b.WriteString(m.discordTokenInput.View())
	if m.discordTokenInput.Err != nil {
		b.WriteString("\n\n")
		b.WriteString(lipgloss.NewStyle().Foreground(ErrorColor).Render(
			"Error: " + m.discordTokenInput.Err.Error(),
		))
	}
	b.WriteString("\n\nEnter: confirm  •  Esc: skip")
	return b.String()
}

func renderQRForTerminal(data string) string {
	q, err := qrcode.New(data, qrcode.Medium)
	if err != nil {
		return fmt.Sprintf("  QR error: %v\n", err)
	}
	bitmap := q.Bitmap()
	if len(bitmap) == 0 {
		return "  QR error: empty bitmap\n"
	}

	var buf strings.Builder
	for y := 0; y < len(bitmap); y += 2 {
		buf.WriteString("  ")
		for x := 0; x < len(bitmap[y]); x++ {
			top := bitmap[y][x]
			bot := false
			if y+1 < len(bitmap) {
				bot = bitmap[y+1][x]
			}
			switch {
			case top && bot:
				buf.WriteString("\u2588") // █ full block
			case top:
				buf.WriteString("\u2580") // ▀ upper half
			case bot:
				buf.WriteString("\u2584") // ▄ lower half
			default:
				buf.WriteString(" ")
			}
		}
		buf.WriteString("\n")
	}
	return buf.String()
}
