// cmd/installer/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func newModel() model {
	// Initialize context for cancellation
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(defaultTheme.Primary))

	// Detect existing installation
	exists, version, daemonRunning, configExists, _ := detectExistingInstall()

	// Default paths
	workspacePath := filepath.Join(os.Getenv("HOME"), ".nanobot-go")
	configPath := filepath.Join(workspacePath, "config.json")

	// Create temp directory for build
	tempDir, err := os.MkdirTemp("", "nanobot-install-*")
	if err != nil {
		tempDir = "/tmp/nanobot-install"
		os.MkdirAll(tempDir, 0755)
	}

	m := model{
		ctx:              ctx,
		cancel:           cancel,
		spinner:          s,
		step:             stepWelcome,
		projectDir:       tempDir,
		workspacePath:    workspacePath,
		configPath:       configPath,
		port:             18791,
		ollamaURL:        "http://localhost:11434",
		existingInstall:  exists,
		existingVersion:  version,
		daemonWasRunning: daemonRunning,
		configExists:     configExists,
		updateMode:       exists,
		enableService:    true,
		startNow:         true,
	}

	// Create temp log file
	logFile, _ := os.CreateTemp("", "nanobot-installer-*.log")
	if logFile != nil {
		fmt.Fprintf(logFile, "=== Nanobot Installer Log ===\n")
		fmt.Fprintf(logFile, "Started: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
	}
	m.logFile = logFile

	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tickCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*50, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.cancel()
			return m, tea.Quit
		case "enter":
			return m.handleEnter()
		case "up", "k":
			return m.handleUp()
		case "down", "j":
			return m.handleDown()
		case "s":
			return m.handleSkip()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		return m, tickCmd()

	case taskCompleteMsg:
		return m.handleTaskComplete(msg)
	}

	// Update spinner
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)

	return m, cmd
}

func (m model) View() string {
	switch m.step {
	case stepWelcome:
		return m.welcomeView()
	case stepPrerequisites:
		return m.prerequisitesView()
	case stepConfiguration:
		return m.configurationView()
	case stepInstalling:
		return m.installingView()
	case stepComplete:
		return m.completeView()
	}
	return "Unknown step"
}

// Handle Enter key
func (m model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepWelcome:
		m.step = stepPrerequisites
		// Try to detect Ollama
		m.detectOllama()
		return m, nil

	case stepPrerequisites:
		m.step = stepConfiguration
		// Try to detect Ollama models
		m.detectOllamaModels()
		return m, nil

	case stepConfiguration:
		if len(m.ollamaModels) > 0 && m.ollamaModelIndex >= 0 {
			m.selectedModel = m.ollamaModels[m.ollamaModelIndex]
		}
		if m.selectedModel == "" {
			m.selectedModel = "llama2:7b" // Default
		}
		m.step = stepInstalling
		m.initTasks()
		m.currentTaskIndex = 0
		m.tasks[0].status = statusRunning
		return m, tea.Batch(m.spinner.Tick, executeTaskCmd(0, &m))

	case stepComplete:
		return m, tea.Quit
	}

	return m, nil
}

// Handle Up key
func (m model) handleUp() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepConfiguration:
		if m.ollamaModelIndex > 0 {
			m.ollamaModelIndex--
		}
	}
	return m, nil
}

// Handle Down key
func (m model) handleDown() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepConfiguration:
		if m.ollamaModelIndex < len(m.ollamaModels)-1 {
			m.ollamaModelIndex++
		}
	}
	return m, nil
}

// Handle Skip key
func (m model) handleSkip() (tea.Model, tea.Cmd) {
	if m.step == stepInstalling && m.currentTaskIndex < len(m.tasks) {
		if m.tasks[m.currentTaskIndex].optional {
			m.tasks[m.currentTaskIndex].status = statusSkipped
			m.currentTaskIndex++
			if m.currentTaskIndex < len(m.tasks) {
				m.tasks[m.currentTaskIndex].status = statusRunning
				return m, tea.Batch(m.spinner.Tick, executeTaskCmd(m.currentTaskIndex, &m))
			}
			m.step = stepComplete
		}
	}
	return m, nil
}

// Handle task completion
func (m model) handleTaskComplete(msg taskCompleteMsg) (tea.Model, tea.Cmd) {
	if msg.success {
		// Mark current task as complete
		m.tasks[msg.index].status = statusComplete
		m.currentTaskIndex++

		// Start next task if available
		if m.currentTaskIndex < len(m.tasks) {
			m.tasks[m.currentTaskIndex].status = statusRunning
			return m, tea.Batch(m.spinner.Tick, executeTaskCmd(m.currentTaskIndex, &m))
		}

		// All tasks complete
		m.step = stepComplete
		return m, nil
	}

	// Task failed
	m.tasks[msg.index].status = statusFailed
	m.tasks[msg.index].errorDetails = &errorInfo{
		message: msg.err.Error(),
		command: getLastCommand(),
		logFile: m.logFile.Name(),
	}

	// If optional, allow skip
	if m.tasks[msg.index].optional {
		return m, nil
	}

	// Required task failed - stop
	return m, nil
}

// Detect Ollama
func (m *model) detectOllama() {
	// Simple check - in real implementation, try HTTP request
	m.ollamaDetected = true
}

// Detect Ollama models
func (m *model) detectOllamaModels() {
	// TODO: Implement Ollama API call to fetch models
	// For now, use sensible defaults
	m.ollamaModels = []string{
		"llama2:7b",
		"mixtral:8x7b",
		"codellama:7b",
		"mistral:7b",
	}
	m.ollamaModelIndex = 0
}

// Execute task command
func executeTaskCmd(index int, m *model) tea.Cmd {
	return func() tea.Msg {
		err := m.tasks[index].execute(m)
		return taskCompleteMsg{index: index, success: err == nil, err: err}
	}
}

func main() {
	m := newModel()
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
