// cmd/installer/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
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

	// Default paths - use ~/.nanobot/ per spec (not ~/.nanobot-go/)
	workspacePath := filepath.Join(os.Getenv("HOME"), ".nanobot", "workspace")
	configPath := filepath.Join(os.Getenv("HOME"), ".nanobot", "config.json")

	// Create temp directory for build - will be set after clone
	tempDir := ""

	// Initialize text inputs
	inputs := make([]textinput.Model, 4)
	
	// Port input
	inputs[0] = textinput.New()
	inputs[0].Placeholder = "18791"
	inputs[0].CharLimit = 5
	inputs[0].Width = 20
	
	// Workspace path input
	inputs[1] = textinput.New()
	inputs[1].Placeholder = workspacePath
	inputs[1].CharLimit = 256
	inputs[1].Width = 50
	
	// Config path input
	inputs[2] = textinput.New()
	inputs[2].Placeholder = configPath
	inputs[2].CharLimit = 256
	inputs[2].Width = 50
	
	// API Key input (for cloud providers)
	inputs[3] = textinput.New()
	inputs[3].Placeholder = "sk-..."
	inputs[3].CharLimit = 256
	inputs[3].Width = 50
	inputs[3].EchoMode = textinput.EchoPassword

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
		provider:         providerOllama, // Default to Ollama
		inputs:           inputs,
		focusedInput:     0,
	}

	// Create temp log file with proper cleanup
	logFile, err := os.CreateTemp("", "nanobot-installer-*.log")
	if err == nil {
		fmt.Fprintf(logFile, "=== Nanobot Installer Log ===\n")
		fmt.Fprintf(logFile, "Started: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
		// Note: logFile will be closed when installer exits
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
		case "left", "h":
			return m.handleLeft()
		case "right", "l":
			return m.handleRight()
		case "tab":
			return m.handleTab()
		case "shift+tab":
			return m.handleShiftTab()
		case "s":
			return m.handleSkip()
		case "r":
			return m.handleRetry()
		case "b":
			return m.handleBack()
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tickMsg:
		return m, tickCmd()

	case taskCompleteMsg:
		return m.handleTaskComplete(msg)

	case ollamaModelsMsg:
		m.ollamaModels = msg.models
		if len(m.ollamaModels) > 0 {
			m.ollamaModelIndex = 0
			m.selectedModel = m.ollamaModels[0]
		}
		m.ollamaDetecting = false
		return m, nil

	case ollamaDetectMsg:
		m.ollamaDetected = msg.detected
		m.ollamaDetecting = false
		if msg.detected && msg.models != nil {
			m.ollamaModels = msg.models
			if len(m.ollamaModels) > 0 {
				m.ollamaModelIndex = 0
				m.selectedModel = m.ollamaModels[0]
			}
		}
		return m, nil
	}

	// Update focused input if in configuration step
	if m.step == stepConfiguration && m.focusedInput >= 0 && m.focusedInput < len(m.inputs) {
		var cmd tea.Cmd
		m.inputs[m.focusedInput], cmd = m.inputs[m.focusedInput].Update(msg)
		return m, cmd
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
	case stepProvider:
		return m.providerView()
	case stepConfiguration:
		return m.configurationView()
	case stepChannels:
		return m.channelsView()
	case stepService:
		return m.serviceView()
	case stepInstalling:
		return m.installingView()
	case stepComplete:
		return m.completeView()
	case stepUninstall:
		return m.uninstallView()
	}
	return "Unknown step"
}

// Handle Enter key
func (m model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepWelcome:
		m.step = stepPrerequisites
		// Try to detect Ollama
		return m, tea.Batch(detectOllamaCmd(m.ollamaURL), tickCmd())

	case stepPrerequisites:
		m.step = stepProvider
		return m, nil

	case stepProvider:
		m.step = stepConfiguration
		// If Ollama selected, try to fetch models
		if m.provider == providerOllama {
			return m, tea.Batch(fetchOllamaModelsCmd(m.ollamaURL), tickCmd())
		}
		return m, nil

	case stepConfiguration:
		// Validate configuration before proceeding
		if m.validateConfiguration() {
			m.step = stepChannels
		}
		return m, nil

	case stepChannels:
		m.step = stepService
		return m, nil

	case stepService:
		// Update paths from inputs
		if m.inputs[0].Value() != "" {
			fmt.Sscanf(m.inputs[0].Value(), "%d", &m.port)
		}
		if m.inputs[1].Value() != "" {
			m.workspacePath = m.inputs[1].Value()
		}
		if m.inputs[2].Value() != "" {
			m.configPath = m.inputs[2].Value()
		}
		m.step = stepInstalling
		m.initTasks()
		m.currentTaskIndex = 0
		if len(m.tasks) > 0 {
			m.tasks[0].status = statusRunning
			return m, tea.Batch(m.spinner.Tick, executeTaskCmd(0, &m))
		}
		return m, nil

	case stepComplete:
		return m, tea.Quit
	}

	return m, nil
}

// Handle Back key
func (m model) handleBack() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepPrerequisites:
		m.step = stepWelcome
	case stepProvider:
		m.step = stepPrerequisites
	case stepConfiguration:
		m.step = stepProvider
	case stepChannels:
		m.step = stepConfiguration
	case stepService:
		m.step = stepChannels
	}
	return m, nil
}

// Handle Up key
func (m model) handleUp() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepProvider:
		if m.providerIndex > 0 {
			m.providerIndex--
		}
	case stepConfiguration:
		if m.ollamaModelIndex > 0 {
			m.ollamaModelIndex--
		}
	case stepService:
		if m.serviceOption > 0 {
			m.serviceOption--
		}
	}
	return m, nil
}

// Handle Down key
func (m model) handleDown() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepProvider:
		if m.providerIndex < len(providers)-1 {
			m.providerIndex++
		}
	case stepConfiguration:
		if m.ollamaModelIndex < len(m.ollamaModels)-1 {
			m.ollamaModelIndex++
		}
	case stepService:
		if m.serviceOption < 2 {
			m.serviceOption++
		}
	}
	return m, nil
}

// Handle Left key
func (m model) handleLeft() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepService:
		switch m.serviceOption {
		case 0:
			m.enableService = !m.enableService
		case 1:
			m.startNow = !m.startNow
		}
	case stepChannels:
		if m.channelIndex == 0 {
			signalEnabled = !signalEnabled
		} else if m.channelIndex == 1 {
			whatsappEnabled = !whatsappEnabled
		}
	}
	return m, nil
}

// Handle Right key
func (m model) handleRight() (tea.Model, tea.Cmd) {
	return m.handleLeft() // Toggle same as left
}

// Handle Tab key
func (m model) handleTab() (tea.Model, tea.Cmd) {
	if m.step == stepConfiguration {
		m.focusedInput++
		if m.focusedInput >= len(m.inputs) {
			m.focusedInput = 0
		}
		for i := range m.inputs {
			if i == m.focusedInput {
				m.inputs[i].Focus()
			} else {
				m.inputs[i].Blur()
			}
		}
	}
	return m, nil
}

// Handle Shift+Tab key
func (m model) handleShiftTab() (tea.Model, tea.Cmd) {
	if m.step == stepConfiguration {
		m.focusedInput--
		if m.focusedInput < 0 {
			m.focusedInput = len(m.inputs) - 1
		}
		for i := range m.inputs {
			if i == m.focusedInput {
				m.inputs[i].Focus()
			} else {
				m.inputs[i].Blur()
			}
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

// Handle Retry key
func (m model) handleRetry() (tea.Model, tea.Cmd) {
	if m.step == stepInstalling && m.currentTaskIndex < len(m.tasks) {
		if m.tasks[m.currentTaskIndex].status == statusFailed {
			m.tasks[m.currentTaskIndex].status = statusRunning
			m.tasks[m.currentTaskIndex].errorDetails = nil
			return m, tea.Batch(m.spinner.Tick, executeTaskCmd(m.currentTaskIndex, &m))
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

// Validate configuration
func (m *model) validateConfiguration() bool {
	// Validate port
	if m.inputs[0].Value() != "" {
		var port int
		if _, err := fmt.Sscanf(m.inputs[0].Value(), "%d", &port); err != nil {
			m.errors = append(m.errors, "Invalid port number")
			return false
		}
		if err := validatePort(port); err != nil {
			m.errors = append(m.errors, err.Error())
			return false
		}
		m.port = port
	}

	// Update selected model
	if m.provider == providerOllama && len(m.ollamaModels) > 0 && m.ollamaModelIndex >= 0 {
		m.selectedModel = m.ollamaModels[m.ollamaModelIndex]
	}

	m.errors = nil
	return true
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
