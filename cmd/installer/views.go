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

// Configuration view
func (m model) configurationView() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("Configuration"))
	b.WriteString("\n\n")

	// Model selection
	b.WriteString("Select Default Model:\n\n")

	for i, model := range m.ollamaModels {
		marker := "○"
		if i == m.ollamaModelIndex {
			marker = "●"
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", marker, model))
	}

	if len(m.ollamaModels) == 0 {
		b.WriteString("  No Ollama models detected\n")
		b.WriteString("  Enter model name manually: " + m.selectedModel + "\n")
	}

	b.WriteString("\n")
	b.WriteString(mutedStyle.Render("↑/↓ to select, Enter to continue"))

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
				status += " (optional, skipped)"
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
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color(defaultTheme.ErrorColor)).
			Render("Error: " + errInfo.message))
		if m.tasks[m.currentTaskIndex].optional {
			b.WriteString("\n" + mutedStyle.Render("Press 's' to skip this optional task"))
		}
	}

	return b.String()
}

// Complete view
func (m model) completeView() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render("✓ Installation Complete!"))
	b.WriteString("\n\n")

	if m.updateMode {
		b.WriteString(fmt.Sprintf("Successfully upgraded nanobot-go\n"))
		b.WriteString(fmt.Sprintf("From: v%s\n", m.existingVersion))
		b.WriteString("To:   v1.0.0\n\n")
	} else {
		b.WriteString("nanobot-go is installed and ready to use!\n\n")
	}

	b.WriteString("Installation Summary:\n")
	b.WriteString("  Binaries: ~/.local/bin/nanobot, ~/.local/bin/nanobot-tui\n")
	b.WriteString("  Config: ~/.nanobot-go/config.json\n")
	b.WriteString("  Service: systemd user service enabled\n\n")

	b.WriteString("Quick Start:\n")
	b.WriteString("  nanobot-tui              # Launch TUI\n")
	b.WriteString("  nanobot chat \"hello\"     # Quick CLI chat\n\n")

	b.WriteString("Systemd Commands:\n")
	b.WriteString("  systemctl --user status nanobot-go\n")
	b.WriteString("  systemctl --user stop nanobot-go\n")
	b.WriteString("  systemctl --user restart nanobot-go\n\n")

	b.WriteString(mutedStyle.Render("Press Enter or 'q' to exit"))

	return b.String()
}
