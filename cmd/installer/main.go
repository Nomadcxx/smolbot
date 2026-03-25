// cmd/installer/main.go
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	s.Style = lipgloss.NewStyle().Foreground(Secondary)

	// Detect existing installation
	exists, version, daemonRunning, configExists, _ := detectExistingInstall()

	// Default paths
	workspacePath := filepath.Join(os.Getenv("HOME"), ".smolbot", "workspace")
	configPath := filepath.Join(os.Getenv("HOME"), ".smolbot", "config.json")

	// Check prerequisites
	hasGo := false
	hasGit := false
	if _, err := exec.LookPath("go"); err == nil {
		hasGo = true
	}
	if _, err := exec.LookPath("git"); err == nil {
		hasGit = true
	}

	// Initialize animations
	asciiHeader := strings.Join(asciiHeaderLines, "\n")
	beams := NewBeamsTextEffect(80, 8, asciiHeader)
	ticker := NewTypewriterTicker(tickerMessages)

	m := model{
		ctx:              ctx,
		cancel:           cancel,
		spinner:          s,
		step:             stepWelcome,
		projectDir:       ".",
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
		provider:         providerOllama,
		hasGo:            hasGo,
		hasGit:           hasGit,
		selectedOption:   0,
		providerIndex:    0,
		tickerIndex:      0,
		channelIndex:     0,
		signalEnabled:    signalEnabled,
		whatsappEnabled:  whatsappEnabled,
		beams:            beams,
		ticker:           ticker,
	}

	// Create temp log file
	logFile, err := os.CreateTemp("", "smolbot-installer-*.log")
	if err == nil {
		fmt.Fprintf(logFile, "=== SMOLBOT Installer Log ===\n")
		fmt.Fprintf(logFile, "Started: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
	}
	m.logFile = logFile

	return m
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tickCmd(),
		tickerCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Millisecond*50, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func tickerCmd() tea.Cmd {
	return tea.Tick(time.Second*3, func(t time.Time) tea.Msg {
		return tickerMsg{}
	})
}

type tickerMsg struct{}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle quit
		if msg.String() == "ctrl+c" {
			m.cancel()
			return m, tea.Quit
		}

		// Step-specific key handling
		switch m.step {
		case stepWelcome:
			return m.handleWelcomeKeys(msg)
		case stepPrerequisites:
			return m.handlePrerequisitesKeys(msg)
		case stepProvider:
			return m.handleProviderKeys(msg)
		case stepConfiguration:
			return m.handleConfigurationKeys(msg)
		case stepChannels:
			return m.handleChannelsKeys(msg)
		case stepWhatsAppSetup:
			return m.handleWhatsAppSetupKeys(msg)
		case stepService:
			return m.handleServiceKeys(msg)
		case stepInstalling:
			return m.handleInstallingKeys(msg)
		case stepComplete:
			return m.handleCompleteKeys(msg)
		case stepUninstall:
			return m.handleUninstallKeys(msg)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		// Resize beams animation
		if m.beams != nil {
			m.beams.Resize(m.width, 8)
		}

	case tickMsg:
		// Update animations (beams and ticker both update every 50ms)
		if m.beams != nil {
			m.beams.Update()
		}
		if m.ticker != nil {
			m.ticker.Update()
		}
		return m, tickCmd()

	case tickerMsg:
		// Ticker messages cycle handled internally by TypewriterTicker
		return m, tickerCmd()

	case taskCompleteMsg:
		return m.handleTaskComplete(msg)

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

	case whatsappQRMsg:
		m.whatsappQRCode = msg.code
		m.whatsappStatus = "Scan the QR code with your phone"
		return m, nil

	case whatsappStatusMsg:
		m.whatsappStatus = msg.text
		return m, nil

	case whatsappLoginResult:
		m.whatsappDone = true
		if msg.success {
			m.whatsappStatus = "Device linked successfully!"
			m.whatsappError = ""
		} else {
			m.whatsappStatus = "Setup failed"
			m.whatsappError = msg.message
		}
		return m, nil
	}

	// Update spinner
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

// Key handlers for each step
func (m model) handleWelcomeKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "up", "k":
		if m.selectedOption > 0 {
			m.selectedOption--
		}
		return m, nil
	case "down", "j":
		maxOptions := 1
		if m.existingInstall {
			maxOptions = 3
		}
		if m.selectedOption < maxOptions-1 {
			m.selectedOption++
		}
		return m, nil
	case "enter":
		// Determine what was selected
		if m.existingInstall {
			switch m.selectedOption {
			case 0: // Update
				m.updateMode = true
				m.step = stepInstalling
				m.initTasks()
				return m, startFirstTask(&m)
			case 1: // Install
				m.updateMode = false
				m.step = stepPrerequisites
				return m, tea.Batch(detectOllamaCmd(m.ollamaURL), tickCmd())
			case 2: // Uninstall
				m.step = stepUninstall
				return m, nil
			}
		} else {
			m.step = stepPrerequisites
			return m, tea.Batch(detectOllamaCmd(m.ollamaURL), tickCmd())
		}
	}
	return m, nil
}

func (m model) handlePrerequisitesKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.step = stepProvider
		return m, nil
	}
	return m, nil
}

func (m model) handleProviderKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.providerIndex > 0 {
			m.providerIndex--
		}
		return m, nil
	case "down", "j":
		if m.providerIndex < len(providers)-1 {
			m.providerIndex++
		}
		return m, nil
	case "enter":
		m.step = stepConfiguration
		if m.providerIndex == 0 {
			return m, tea.Batch(fetchOllamaModelsCmd(m.ollamaURL), tickCmd())
		}
		return m, nil
	case "esc":
		m.step = stepPrerequisites
		return m, nil
	}
	return m, nil
}

func (m model) handleConfigurationKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.providerIndex == 0 && m.ollamaModelIndex > 0 {
			m.ollamaModelIndex--
			m.selectedModel = m.ollamaModels[m.ollamaModelIndex]
		}
		return m, nil
	case "down", "j":
		if m.providerIndex == 0 && m.ollamaModelIndex < len(m.ollamaModels)-1 {
			m.ollamaModelIndex++
			m.selectedModel = m.ollamaModels[m.ollamaModelIndex]
		}
		return m, nil
	case "enter":
		if m.validateConfiguration() {
			m.step = stepChannels
		}
		return m, nil
	case "esc":
		m.step = stepProvider
		return m, nil
	}
	return m, nil
}

func (m model) handleChannelsKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.channelIndex > 0 {
			m.channelIndex--
		}
		return m, nil
	case "down", "j":
		if m.channelIndex < 1 {
			m.channelIndex++
		}
		return m, nil
	case " ":
		if m.channelIndex == 0 {
			m.signalEnabled = !m.signalEnabled
		} else if m.channelIndex == 1 {
			m.whatsappEnabled = !m.whatsappEnabled
		}
		return m, nil
	case "enter":
		signalEnabled = m.signalEnabled
		whatsappEnabled = m.whatsappEnabled
		if m.whatsappEnabled {
			m.step = stepWhatsAppSetup
			return m, nil
		}
		m.step = stepService
		return m, nil
	case "esc":
		signalEnabled = m.signalEnabled
		whatsappEnabled = m.whatsappEnabled
		m.step = stepConfiguration
		return m, nil
	}
	return m, nil
}

func (m model) handleWhatsAppSetupKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	fmt.Printf("DEBUG handleWhatsAppSetupKeys: msg=%s, whatsappDone=%v, whatsappQRCode=%q\n", msg.String(), m.whatsappDone, m.whatsappQRCode)
	switch msg.String() {
	case "enter":
		fmt.Printf("DEBUG: enter pressed, condition=%v\n", !m.whatsappDone && m.whatsappQRCode == "")
		if !m.whatsappDone && m.whatsappQRCode == "" {
			return m, m.startWhatsAppLink()
		}
		m.step = stepService
		return m, nil
	case "esc":
		m.whatsappDone = false
		m.whatsappQRCode = ""
		m.whatsappStatus = ""
		m.step = stepService
		return m, nil
	}
	return m, nil
}

func (m model) startWhatsAppLink() tea.Cmd {
	return func() tea.Msg {
		fmt.Printf("DEBUG: startWhatsAppLink called\n")
		storePath := filepath.Join(os.Getenv("HOME"), ".smolbot", "whatsapp.db")
		fmt.Printf("DEBUG: storePath=%s\n", storePath)
		
		linker, err := NewWhatsAppLinker(storePath)
		fmt.Printf("DEBUG: NewWhatsAppLinker err=%v\n", err)
		if err != nil {
			return whatsappLoginResult{success: false, message: err.Error()}
		}

		if linker.IsLinked() {
			m.whatsappDone = true
			m.whatsappQRCode = ""
			m.whatsappStatus = "Already linked!"
			m.whatsappError = ""
			return whatsappLoginResult{success: true}
		}

		// Run linking in goroutine and send messages via program
		go func() {
			onQR := func(code string) {
				if m.program != nil {
					m.program.Send(whatsappQRMsg{code: code})
				}
			}
			onStatus := func(status string) {
				if m.program != nil {
					m.program.Send(whatsappStatusMsg{text: status})
				}
			}

			err := linker.StartLinking(onQR, onStatus)
			if err != nil {
				if m.program != nil {
					m.program.Send(whatsappLoginResult{success: false, message: err.Error()})
				}
				return
			}
			if m.program != nil {
				m.program.Send(whatsappLoginResult{success: true})
			}
		}()

		return whatsappLoginResult{success: true}
	}
}

func (m model) startWhatsAppLogin() tea.Cmd {
	return func() tea.Msg {
		// Use direct linking instead of subprocess
		storePath := filepath.Join(os.Getenv("HOME"), ".smolbot", "whatsapp.db")
		linker, err := NewWhatsAppLinker(storePath)
		if err != nil {
			return whatsappLoginResult{success: false, message: err.Error()}
		}

		if linker.IsLinked() {
			return whatsappLoginResult{success: true}
		}

		// QR callback - send to model
		onQR := func(code string) {
		}

		// Status callback - update model
		onStatus := func(status string) {
		}

		err = linker.StartLinking(onQR, onStatus)
		if err != nil {
			return whatsappLoginResult{success: false, message: err.Error()}
		}
		return whatsappLoginResult{success: true}
	}
}

type whatsappQRMsg struct{ code string }
type whatsappStatusMsg struct{ text string }
type whatsappLoginResult struct {
	success bool
	message string
}

func findSmolbotBinary() (string, error) {
	paths := []string{
		filepath.Join(os.Getenv("HOME"), ".local", "bin", "smolbot"),
		filepath.Join(os.Getenv("HOME"), ".local", "bin", "smolbot-tui"),
		"./smolbot",
		"smolbot",
	}
	// First try direct paths
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	// Fall back to PATH lookup
	if smolbotPath, err := exec.LookPath("smolbot"); err == nil {
		return smolbotPath, nil
	}
	return "", fmt.Errorf("smolbot binary not found in ~/.local/bin or PATH")
}

func (m model) handleServiceKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		m.serviceOption = (m.serviceOption + 1) % 3
		return m, nil
	case "left", "h", "right", "l":
		switch m.serviceOption {
		case 0:
			m.enableService = !m.enableService
		case 1:
			m.startNow = !m.startNow
		}
		return m, nil
	case "enter":
		m.step = stepInstalling
		m.initTasks()
		return m, startFirstTask(&m)
	case "esc":
		m.step = stepChannels
		return m, nil
	}
	return m, nil
}

func (m model) handleInstallingKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// During installation, only Ctrl+C works
	return m, nil
}

func (m model) handleCompleteKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m model) handleUninstallKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.initUninstallTasks()
		m.step = stepInstalling
		return m, startFirstTask(&m)
	case "esc":
		m.step = stepWelcome
		return m, nil
	}
	return m, nil
}

func (m model) handleTaskComplete(msg taskCompleteMsg) (tea.Model, tea.Cmd) {
	if msg.success {
		m.tasks[msg.index].status = statusComplete
		m.currentTaskIndex++

		if m.currentTaskIndex < len(m.tasks) {
			m.tasks[m.currentTaskIndex].status = statusRunning
			return m, tea.Batch(m.spinner.Tick, executeTaskCmd(m.currentTaskIndex, &m))
		}

		m.step = stepComplete
		return m, nil
	}

	m.tasks[msg.index].status = statusFailed
	m.tasks[msg.index].errorDetails = &errorInfo{
		message: msg.err.Error(),
		command: getLastCommand(),
		logFile: m.logFile.Name(),
	}

	return m, nil
}

func startFirstTask(m *model) tea.Cmd {
	if len(m.tasks) == 0 {
		return nil
	}
	m.currentTaskIndex = 0
	m.tasks[0].status = statusRunning
	return tea.Batch(m.spinner.Tick, executeTaskCmd(0, m))
}

func main() {
	m := newModel()
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.program = p

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
