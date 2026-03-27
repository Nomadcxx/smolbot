package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestTelegramChannelSetupStoresState(t *testing.T) {
	m := newModel()
	m.step = stepChannels
	m.channelIndex = 2

	updated, _ := m.handleChannelsKeys(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(model)
	if !m.telegramEnabled {
		t.Fatal("telegramEnabled = false, want true after toggling the Telegram channel")
	}

	updated, _ = m.handleChannelsKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.step != stepTelegramSetup {
		t.Fatalf("step = %v, want stepTelegramSetup", m.step)
	}

	if !m.telegramTokenInput.Focused() {
		t.Fatal("telegramTokenInput should be focused on entering Telegram setup")
	}

	tokenFile := filepath.Join(t.TempDir(), "telegram.token")
	if err := os.WriteFile(tokenFile, []byte("bot-token"), 0600); err != nil {
		t.Fatalf("write token file: %v", err)
	}
	m.telegramTokenInput.SetValue(tokenFile)

	updated, _ = m.handleTelegramSetupKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.telegramTokenFile != tokenFile {
		t.Fatalf("telegramTokenFile = %q, want %q", m.telegramTokenFile, tokenFile)
	}
	if m.step != stepService {
		t.Fatalf("step = %v, want stepService", m.step)
	}
}

func TestTelegramSetupEnterRejectsMissingTokenFile(t *testing.T) {
	m := newModel()
	m.step = stepTelegramSetup
	m.telegramEnabled = true

	m.telegramTokenInput.SetValue(filepath.Join(t.TempDir(), "missing.token"))

	updated, _ := m.handleTelegramSetupKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.step != stepTelegramSetup {
		t.Fatalf("step = %v, want stepTelegramSetup for missing token file", m.step)
	}
	if m.telegramEnabled {
		t.Fatal("telegramEnabled should be false when token file validation fails")
	}
	if m.telegramTokenFile != "" {
		t.Fatalf("telegramTokenFile = %q, want empty after missing-token validation failure", m.telegramTokenFile)
	}
	if m.telegramTokenInput.Err == nil {
		t.Fatal("telegramTokenInput.Err = nil, want visible validation error for missing token file")
	}
	if got := m.renderTelegramSetup(); !strings.Contains(got, "read Telegram token file") {
		t.Fatalf("renderTelegramSetup() missing validation error, got:\n%s", got)
	}
}

func TestTelegramSetupEnterRejectsUnreadableTokenFile(t *testing.T) {
	m := newModel()
	m.step = stepTelegramSetup
	m.telegramEnabled = true

	tokenFile := filepath.Join(t.TempDir(), "telegram.token")
	if err := os.WriteFile(tokenFile, []byte("bot-token"), 0600); err != nil {
		t.Fatalf("write token file: %v", err)
	}
	if err := os.Chmod(tokenFile, 0000); err != nil {
		t.Fatalf("chmod token file unreadable: %v", err)
	}

	m.telegramTokenInput.SetValue(tokenFile)

	updated, _ := m.handleTelegramSetupKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.step != stepTelegramSetup {
		t.Fatalf("step = %v, want stepTelegramSetup for unreadable token file", m.step)
	}
	if m.telegramEnabled {
		t.Fatal("telegramEnabled should be false when token file validation fails")
	}
	if m.telegramTokenFile != "" {
		t.Fatalf("telegramTokenFile = %q, want empty after unreadable-token validation failure", m.telegramTokenFile)
	}
	if m.telegramTokenInput.Err == nil {
		t.Fatal("telegramTokenInput.Err = nil, want visible validation error for unreadable token file")
	}
	if got := m.renderTelegramSetup(); !strings.Contains(got, "read Telegram token file") {
		t.Fatalf("renderTelegramSetup() missing validation error, got:\n%s", got)
	}
}

func TestTelegramSetupEnterRejectsWhitespaceOnlyTokenFile(t *testing.T) {
	m := newModel()
	m.step = stepTelegramSetup
	m.telegramEnabled = true

	tokenFile := filepath.Join(t.TempDir(), "telegram.token")
	if err := os.WriteFile(tokenFile, []byte("   \n\t  "), 0600); err != nil {
		t.Fatalf("write whitespace token file: %v", err)
	}

	m.telegramTokenInput.SetValue(tokenFile)

	updated, _ := m.handleTelegramSetupKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.step != stepTelegramSetup {
		t.Fatalf("step = %v, want stepTelegramSetup for whitespace token file", m.step)
	}
	if m.telegramEnabled {
		t.Fatal("telegramEnabled should be false when token file is whitespace-only")
	}
	if m.telegramTokenFile != "" {
		t.Fatalf("telegramTokenFile = %q, want empty after whitespace-token validation failure", m.telegramTokenFile)
	}
	if m.telegramTokenInput.Err == nil {
		t.Fatal("telegramTokenInput.Err = nil, want visible validation error for whitespace token file")
	}
	if got := m.renderTelegramSetup(); !strings.Contains(got, "empty") {
		t.Fatalf("renderTelegramSetup() missing whitespace validation error, got:\n%s", got)
	}
}

func TestTelegramSetupEnterWithEmptyInputSkipsTelegram(t *testing.T) {
	m := newModel()
	m.step = stepTelegramSetup
	m.telegramEnabled = true

	updated, _ := m.handleTelegramSetupKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.step != stepService {
		t.Fatalf("step = %v, want stepService after empty Telegram setup", m.step)
	}
	if m.telegramEnabled {
		t.Fatal("telegramEnabled should be false when Telegram setup is skipped by leaving the field empty")
	}
	if m.telegramTokenFile != "" {
		t.Fatalf("telegramTokenFile = %q, want empty after empty-input skip", m.telegramTokenFile)
	}
}

func TestTelegramSetupEscSkipsTelegram(t *testing.T) {
	m := newModel()
	m.telegramEnabled = true
	m.step = stepTelegramSetup
	m.telegramTokenInput.SetValue("ignored")

	updated, _ := m.handleTelegramSetupKeys(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(model)
	if m.step != stepService {
		t.Fatalf("step = %v, want stepService after escape skip", m.step)
	}
	if m.telegramEnabled {
		t.Fatal("telegramEnabled should be false after escape skip")
	}
	if m.telegramTokenFile != "" {
		t.Fatalf("telegramTokenFile = %q, want empty after escape skip", m.telegramTokenFile)
	}
}

func TestWriteConfigIncludesTelegramWhenEnabled(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "telegram-token.txt")
	if err := os.WriteFile(tokenFile, []byte("bot-token"), 0600); err != nil {
		t.Fatalf("write token file: %v", err)
	}

	m := &model{
		step:              stepChannels,
		projectDir:        t.TempDir(),
		configPath:        filepath.Join(t.TempDir(), "config.json"),
		workspacePath:     filepath.Join(t.TempDir(), "workspace"),
		provider:          providerOllama,
		selectedModel:     "llama3.2",
		ollamaURL:         "http://localhost:11434",
		port:              18790,
		telegramEnabled:   true,
		telegramTokenFile: tokenFile,
	}

	if err := writeConfig(m); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	channels := cfg["channels"].(map[string]any)
	telegram := channels["telegram"].(map[string]any)
	if enabled, ok := telegram["enabled"].(bool); !ok || !enabled {
		t.Fatalf("telegram.enabled = %#v, want true", telegram["enabled"])
	}
	if got := telegram["tokenFile"]; got != tokenFile {
		t.Fatalf("telegram.tokenFile = %#v, want %q", got, tokenFile)
	}
	if _, ok := telegram["botToken"]; ok {
		t.Fatalf("telegram.botToken should be omitted, got %#v", telegram["botToken"])
	}
}

func TestWriteConfigRejectsWhitespaceOnlyTelegramTokenFile(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "telegram-token.txt")
	if err := os.WriteFile(tokenFile, []byte(" \n\t "), 0600); err != nil {
		t.Fatalf("write whitespace token file: %v", err)
	}

	m := &model{
		step:              stepChannels,
		projectDir:        t.TempDir(),
		configPath:        filepath.Join(t.TempDir(), "config.json"),
		workspacePath:     filepath.Join(t.TempDir(), "workspace"),
		provider:          providerOllama,
		selectedModel:     "llama3.2",
		ollamaURL:         "http://localhost:11434",
		port:              18790,
		telegramEnabled:   true,
		telegramTokenFile: tokenFile,
	}

	if err := writeConfig(m); err == nil {
		t.Fatal("writeConfig succeeded, want whitespace-only Telegram token file to be rejected")
	}
}

func TestWriteConfigDoesNotInjectTelegramTokenWhenDisabled(t *testing.T) {
	m := &model{
		step:              stepChannels,
		projectDir:        t.TempDir(),
		configPath:        filepath.Join(t.TempDir(), "config.json"),
		workspacePath:     filepath.Join(t.TempDir(), "workspace"),
		provider:          providerOllama,
		selectedModel:     "llama3.2",
		ollamaURL:         "http://localhost:11434",
		port:              18790,
		telegramEnabled:   false,
		telegramTokenFile: "/tmp/telegram-token.txt",
	}

	if err := writeConfig(m); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	if strings.Contains(string(data), "/tmp/telegram-token.txt") {
		t.Fatalf("disabled Telegram config leaked token file path into JSON: %s", data)
	}

	var cfg map[string]any
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	channels := cfg["channels"].(map[string]any)
	telegram := channels["telegram"].(map[string]any)
	if enabled, ok := telegram["enabled"].(bool); !ok || enabled {
		t.Fatalf("telegram.enabled = %#v, want false", telegram["enabled"])
	}
	if _, ok := telegram["tokenFile"]; ok {
		t.Fatalf("telegram.tokenFile should be omitted when disabled, got %#v", telegram["tokenFile"])
	}
	if _, ok := telegram["botToken"]; ok {
		t.Fatalf("telegram.botToken should be omitted when disabled, got %#v", telegram["botToken"])
	}
}
