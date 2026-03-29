// cmd/installer/types.go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Install steps in the wizard flow
type installStep int

const (
	stepWelcome installStep = iota
	stepPrerequisites
	stepProvider
	stepConfiguration
	stepMiniMaxOAuth
	stepChannels
	stepTelegramSetup
	stepDiscordSetup
	stepWhatsAppSetup
	stepService
	stepInstalling
	stepComplete
	stepUninstall
)

// Provider types
const (
	providerOllama       = "ollama"
	providerOpenAI       = "openai"
	providerAnthropic    = "anthropic"
	providerAzure        = "azure_openai"
	providerMiniMax      = "minimax"
	providerMiniMaxOAuth = "minimax-portal"
	providerCustom       = "custom"
)

var providers = []string{
	providerOllama,
	providerOpenAI,
	providerAnthropic,
	providerAzure,
	providerMiniMax,
	providerMiniMaxOAuth,
	providerCustom,
}

// Task execution status
type taskStatus int

const (
	statusPending taskStatus = iota
	statusRunning
	statusComplete
	statusFailed
	statusSkipped
)

// Message types for async communication
type taskCompleteMsg struct {
	index   int
	success bool
	skipped bool
	err     error
}

type tickMsg time.Time

type ollamaModelsMsg struct {
	models []string
}

type ollamaDetectMsg struct {
	detected bool
	models   []string
}

// Install task with error handling
type installTask struct {
	name         string
	description  string
	execute      func(*model) error
	optional     bool
	status       taskStatus
	errorDetails *errorInfo
}

type errorInfo struct {
	message string
	command string
	logFile string
}

// CommandError provides detailed error information
type CommandError struct {
	Command  string
	ExitCode int
	Output   string
	Duration time.Duration
	Err      error
}

func (e CommandError) Error() string {
	return fmt.Sprintf("command failed: %s (exit %d): %v", e.Command, e.ExitCode, e.Err)
}

// Main model state
type model struct {
	// Wizard state
	step             installStep
	tasks            []installTask
	currentTaskIndex int
	width, height    int

	// Tea program reference (for sending messages from goroutines)
	program *tea.Program

	// Visual elements
	spinner        spinner.Model
	inputs         []textinput.Model
	focusedInput   int
	selectedOption int
	providerIndex  int
	serviceOption  int
	channelIndex   int
	tickerIndex    int

	// Animations
	beams  *BeamsTextEffect
	ticker *TypewriterTicker

	// Provider configuration
	provider    string
	apiKey      string
	apiEndpoint string

	// Configuration
	selectedModel    string
	ollamaURL        string
	ollamaDetected   bool
	ollamaDetecting  bool
	ollamaModels     []string
	ollamaModelIndex int
	workspacePath    string
	configPath       string
	port             int

	// Quota configuration
	quotaEnabled bool

	// Channel configuration
	signalEnabled      bool
	whatsappEnabled    bool
	telegramEnabled    bool
	discordEnabled     bool
	signalCLIPath      string
	whatsappDBPath     string
	telegramTokenFile  string
	telegramTokenInput textinput.Model
	discordTokenFile   string
	discordTokenInput  textinput.Model

	// MiniMax OAuth state
	oauthFlow  *oauthFlowState
	oauthToken *oauthToken
	oauthError string
	oauthURL   string

	// WhatsApp setup state
	whatsappQRCode string
	whatsappStatus string
	whatsappDone   bool
	whatsappError  string
	whatsappLinker *WhatsAppLinker

	// Service options
	enableService bool
	startNow      bool
	// Prerequisites
	hasGo  bool
	hasGit bool

	// Upgrade detection
	existingInstall  bool
	existingVersion  string
	daemonWasRunning bool
	configExists     bool
	updateMode       bool

	// Error tracking
	errors    []string
	debugMode bool
	logFile   *os.File
	ctx       context.Context
	cancel    context.CancelFunc

	// Build info
	projectDir string
}
