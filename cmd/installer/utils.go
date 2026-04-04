// cmd/installer/utils.go
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// runCommand executes a command with logging
type commandResult struct {
	Output   string
	ExitCode int
	Duration time.Duration
	Err      error
}

func runCommand(m *model, name string, args ...string) commandResult {
	start := time.Now()

	// Check for context cancellation
	select {
	case <-m.ctx.Done():
		return commandResult{Err: m.ctx.Err()}
	default:
	}

	cmd := exec.CommandContext(m.ctx, name, args...)
	if m.projectDir != "" {
		cmd.Dir = m.projectDir
	}

	// Log command
	if m.logFile != nil {
		fmt.Fprintf(m.logFile, "[CMD] %s %s\n", name, strings.Join(args, " "))
		if cmd.Dir != "" {
			fmt.Fprintf(m.logFile, "[CWD] %s\n", cmd.Dir)
		}
	}

	output, err := cmd.CombinedOutput()
	duration := time.Since(start)

	result := commandResult{
		Output:   string(output),
		Duration: duration,
		Err:      err,
	}

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	// Log result
	if m.logFile != nil {
		if err != nil {
			fmt.Fprintf(m.logFile, "[FAIL] Exit code %d, duration %v\n%s\n\n",
				result.ExitCode, duration, output)
		} else {
			fmt.Fprintf(m.logFile, "[OK] Duration %v\n\n", duration)
		}
	}

	return result
}

// validatePort checks if a port is available
func validatePort(port int) error {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("port %d is already in use: %w", port, err)
	}
	listener.Close()
	return nil
}

// getLastCommand returns the last executed command for error display
var lastCommand string

func setLastCommand(name string, args ...string) {
	lastCommand = name + " " + strings.Join(args, " ")
}

func getLastCommand() string {
	return lastCommand
}

// detectExistingInstall checks for existing installation
func detectExistingInstall() (exists bool, version string, daemonRunning bool, configExists bool, err error) {
	// Check binary
	binPath := filepath.Join(os.Getenv("HOME"), ".local", "bin", "smolbot")
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		return false, "", false, false, nil
	}
	exists = true

	// Get version
	cmd := exec.Command(binPath, "--version")
	output, err := cmd.Output()
	if err == nil {
		version = strings.TrimSpace(string(output))
	}

	// Check if service is running
	cmd = exec.Command("systemctl", "--user", "is-active", "smolbot")
	if err := cmd.Run(); err == nil {
		daemonRunning = true
	}

	// Check config (current location)
	configPath := filepath.Join(os.Getenv("HOME"), ".smolbot", "config.json")
	if _, err := os.Stat(configPath); err == nil {
		configExists = true
	}
	
	// Also check legacy location
	legacyConfigPath := filepath.Join(os.Getenv("HOME"), ".nanobot", "config.json")
	if _, err := os.Stat(legacyConfigPath); err == nil {
		configExists = true
	}

	return exists, version, daemonRunning, configExists, nil
}

// detectOllamaCmd returns a command to detect Ollama
func detectOllamaCmd(url string) tea.Cmd {
	return func() tea.Msg {
		client := &http.Client{Timeout: 2 * time.Second}
		resp, err := client.Get(url + "/api/tags")
		if err != nil {
			return ollamaDetectMsg{detected: false}
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return ollamaDetectMsg{detected: false}
		}

		// Parse models from response
		var result struct {
			Models []struct {
				Name string `json:"name"`
			} `json:"models"`
		}

		body, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(body, &result); err != nil {
			return ollamaDetectMsg{detected: true}
		}

		models := make([]string, 0, len(result.Models))
		for _, m := range result.Models {
			models = append(models, m.Name)
		}

		return ollamaDetectMsg{detected: true, models: models}
	}
}

// fetchOllamaModelsCmd returns a command to fetch Ollama models
func fetchOllamaModelsCmd(url string) tea.Cmd {
	return detectOllamaCmd(url) // Same implementation
}

// Global channel state (persisted across views)
var (
	signalEnabled   = false
	whatsappEnabled = false
)

// executeTaskCmd executes a task and returns a message when complete
func executeTaskCmd(index int, m *model) tea.Cmd {
	return func() tea.Msg {
		task := &m.tasks[index]
		err := task.execute(m)
		skipped := err != nil && task.optional
		return taskCompleteMsg{
			index:   index,
			success: err == nil,
			skipped: skipped,
			err:     err,
		}
	}
}

// validateConfiguration validates the current configuration
func (m *model) validateConfiguration() bool {
	// Basic validation - ensure required fields are set
	switch m.provider {
	case providerOllama:
		// Ollama just needs a model selected
		return m.selectedModel != "" || len(m.ollamaModels) > 0
	case providerOpenAI, providerAnthropic, providerMiniMax:
		// These need an API key
		return m.apiKey != ""
	case providerMiniMaxOAuth:
		// OAuth uses browser-based sign-in
		return true
	case providerAzure:
		// Azure needs endpoint and key
		return m.apiKey != "" && m.apiEndpoint != ""
	case providerCustom:
		// Custom needs endpoint
		return m.apiEndpoint != ""
	}
	return true
}

func defaultModelFor(provider string) string {
	switch provider {
	case providerOpenAI:
		return "gpt-4o"
	case providerAnthropic:
		return "claude-opus-4-6"
	case providerAzure:
		return "gpt-4o"
	case providerMiniMax:
		return "MiniMax-M2.7"
	case providerMiniMaxOAuth:
		return "minimax-portal/MiniMax-M2.7"
	case providerOpenAICodex:
		return "openai-codex/gpt-5.2-codex"
	case providerCustom:
		return "gpt-4o"
	default:
		return ""
	}
}
