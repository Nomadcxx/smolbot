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
	stepConfiguration
	stepInstalling
	stepComplete
)

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
	err     error
}

type tickMsg time.Time

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

	// Visual elements
	spinner        spinner.Model
	inputs         []textinput.Model
	focusedInput   int
	selectedOption int

	// Configuration
	selectedModel    string
	ollamaURL        string
	ollamaDetected   bool
	ollamaModels     []string
	ollamaModelIndex int
	workspacePath    string
	configPath       string
	port             int

	// Service options
	enableService bool
	startNow      bool

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
