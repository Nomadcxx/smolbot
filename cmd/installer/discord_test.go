package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestDiscordChannelSetupStoresState(t *testing.T) {
	m := newModel()
	m.step = stepChannels
	m.channelIndex = 3

	updated, _ := m.handleChannelsKeys(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(model)
	if !m.discordEnabled {
		t.Fatal("discordEnabled = false, want true after toggling the Discord channel")
	}

	updated, _ = m.handleChannelsKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.step != stepDiscordSetup {
		t.Fatalf("step = %v, want stepDiscordSetup", m.step)
	}
	if !m.discordTokenInput.Focused() {
		t.Fatal("discordTokenInput should be focused on entering Discord setup")
	}

	tokenFile := filepath.Join(t.TempDir(), "discord.token")
	if err := os.WriteFile(tokenFile, []byte("discord-bot-token"), 0600); err != nil {
		t.Fatalf("write token file: %v", err)
	}
	m.discordTokenInput.SetValue(tokenFile)

	updated, _ = m.handleDiscordSetupKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.discordTokenFile != tokenFile {
		t.Fatalf("discordTokenFile = %q, want %q", m.discordTokenFile, tokenFile)
	}
	if m.step != stepService {
		t.Fatalf("step = %v, want stepService", m.step)
	}
}

func TestDiscordSetupEnterRejectsMissingTokenFile(t *testing.T) {
	m := newModel()
	m.step = stepDiscordSetup
	m.discordEnabled = true
	m.discordTokenInput.SetValue(filepath.Join(t.TempDir(), "missing.token"))

	updated, _ := m.handleDiscordSetupKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.step != stepDiscordSetup {
		t.Fatalf("step = %v, want stepDiscordSetup for missing token file", m.step)
	}
	if m.discordEnabled {
		t.Fatal("discordEnabled should be false when token file validation fails")
	}
	if m.discordTokenFile != "" {
		t.Fatalf("discordTokenFile = %q, want empty after missing-token validation failure", m.discordTokenFile)
	}
	if m.discordTokenInput.Err == nil {
		t.Fatal("discordTokenInput.Err = nil, want visible validation error for missing token file")
	}
	if got := m.renderDiscordSetup(); !strings.Contains(got, "read Discord token file") {
		t.Fatalf("renderDiscordSetup() missing validation error, got:\n%s", got)
	}
}

func TestDiscordSetupEnterRejectsWhitespaceOnlyTokenFile(t *testing.T) {
	m := newModel()
	m.step = stepDiscordSetup
	m.discordEnabled = true

	tokenFile := filepath.Join(t.TempDir(), "discord.token")
	if err := os.WriteFile(tokenFile, []byte(" \n\t "), 0600); err != nil {
		t.Fatalf("write token file: %v", err)
	}
	m.discordTokenInput.SetValue(tokenFile)

	updated, _ := m.handleDiscordSetupKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.step != stepDiscordSetup {
		t.Fatalf("step = %v, want stepDiscordSetup for whitespace token file", m.step)
	}
	if m.discordEnabled {
		t.Fatal("discordEnabled should be false when token file is whitespace-only")
	}
	if m.discordTokenInput.Err == nil {
		t.Fatal("discordTokenInput.Err = nil, want validation error")
	}
}

func TestDiscordSetupEnterWithEmptyInputSkipsDiscord(t *testing.T) {
	m := newModel()
	m.step = stepDiscordSetup
	m.discordEnabled = true

	updated, _ := m.handleDiscordSetupKeys(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(model)
	if m.step != stepService {
		t.Fatalf("step = %v, want stepService after empty Discord setup", m.step)
	}
	if m.discordEnabled {
		t.Fatal("discordEnabled should be false when Discord setup is skipped by leaving the field empty")
	}
	if m.discordTokenFile != "" {
		t.Fatalf("discordTokenFile = %q, want empty after empty-input skip", m.discordTokenFile)
	}
}

func TestWriteConfigIncludesDiscordWhenEnabled(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "discord-token.txt")
	if err := os.WriteFile(tokenFile, []byte("discord-bot-token"), 0600); err != nil {
		t.Fatalf("write token file: %v", err)
	}

	m := &model{
		step:             stepChannels,
		projectDir:       t.TempDir(),
		configPath:       filepath.Join(t.TempDir(), "config.json"),
		workspacePath:    filepath.Join(t.TempDir(), "workspace"),
		provider:         providerOllama,
		selectedModel:    "llama3.2",
		ollamaURL:        "http://localhost:11434",
		port:             18790,
		discordEnabled:   true,
		discordTokenFile: tokenFile,
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
	discord := channels["discord"].(map[string]any)
	if enabled, ok := discord["enabled"].(bool); !ok || !enabled {
		t.Fatalf("discord.enabled = %#v, want true", discord["enabled"])
	}
	if got := discord["tokenFile"]; got != tokenFile {
		t.Fatalf("discord.tokenFile = %#v, want %q", got, tokenFile)
	}
	if _, ok := discord["botToken"]; ok {
		t.Fatalf("discord.botToken should be omitted, got %#v", discord["botToken"])
	}
}

func TestWriteConfigDoesNotInjectDiscordTokenWhenDisabled(t *testing.T) {
	m := &model{
		step:             stepChannels,
		projectDir:       t.TempDir(),
		configPath:       filepath.Join(t.TempDir(), "config.json"),
		workspacePath:    filepath.Join(t.TempDir(), "workspace"),
		provider:         providerOllama,
		selectedModel:    "llama3.2",
		ollamaURL:        "http://localhost:11434",
		port:             18790,
		discordEnabled:   false,
		discordTokenFile: "/tmp/discord-token.txt",
	}

	if err := writeConfig(m); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	if strings.Contains(string(data), "/tmp/discord-token.txt") {
		t.Fatalf("disabled Discord config leaked token file path into JSON: %s", data)
	}
}

func TestWriteConfigUsesPrivatePermissions(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "discord-token.txt")
	if err := os.WriteFile(tokenFile, []byte("discord-bot-token"), 0600); err != nil {
		t.Fatalf("write token file: %v", err)
	}

	m := &model{
		step:             stepChannels,
		projectDir:       t.TempDir(),
		configPath:       filepath.Join(t.TempDir(), "config.json"),
		workspacePath:    filepath.Join(t.TempDir(), "workspace"),
		provider:         providerOpenAI,
		apiKey:           "sk-test",
		selectedModel:    "gpt-test",
		port:             18790,
		discordEnabled:   true,
		discordTokenFile: tokenFile,
	}

	if err := writeConfig(m); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}

	info, err := os.Stat(m.configPath)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("config mode = %#o, want 0600", got)
	}
}

func TestWriteConfigTightensExistingConfigPermissions(t *testing.T) {
	tokenFile := filepath.Join(t.TempDir(), "discord-token.txt")
	if err := os.WriteFile(tokenFile, []byte("discord-bot-token"), 0600); err != nil {
		t.Fatalf("write token file: %v", err)
	}

	configPath := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(configPath, []byte(`{"stale":true}`), 0644); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	m := &model{
		step:             stepChannels,
		projectDir:       t.TempDir(),
		configPath:       configPath,
		workspacePath:    filepath.Join(t.TempDir(), "workspace"),
		provider:         providerOpenAI,
		apiKey:           "sk-test",
		selectedModel:    "gpt-test",
		port:             18790,
		discordEnabled:   true,
		discordTokenFile: tokenFile,
	}

	if err := writeConfig(m); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}

	info, err := os.Stat(configPath)
	if err != nil {
		t.Fatalf("stat config: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("config mode = %#o, want 0600 after rewrite", got)
	}
}

func TestBackupConfigUsesPrivatePermissions(t *testing.T) {
	m := &model{configPath: filepath.Join(t.TempDir(), "config.json")}
	if err := os.WriteFile(m.configPath, []byte(`{"providers":{"openai":{"apiKey":"sk-test"}}}`), 0600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if err := backupConfig(m); err != nil {
		t.Fatalf("backupConfig: %v", err)
	}

	matches, err := filepath.Glob(m.configPath + ".backup.*")
	if err != nil {
		t.Fatalf("glob backups: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("backup files = %#v, want 1", matches)
	}
	info, err := os.Stat(matches[0])
	if err != nil {
		t.Fatalf("stat backup: %v", err)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("backup mode = %#o, want 0600", got)
	}
}

func TestSetupSystemdCapturesInstallerPath(t *testing.T) {
	home := t.TempDir()
	systemctlDir := filepath.Join(t.TempDir(), "bin")
	if err := os.MkdirAll(systemctlDir, 0755); err != nil {
		t.Fatalf("mkdir systemctl dir: %v", err)
	}

	systemctlPath := filepath.Join(systemctlDir, "systemctl")
	systemctlScript := "#!/bin/sh\nexit 0\n"
	if err := os.WriteFile(systemctlPath, []byte(systemctlScript), 0755); err != nil {
		t.Fatalf("write fake systemctl: %v", err)
	}

	oldHome := os.Getenv("HOME")
	oldPath := os.Getenv("PATH")
	t.Cleanup(func() {
		_ = os.Setenv("HOME", oldHome)
		_ = os.Setenv("PATH", oldPath)
	})
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("set HOME: %v", err)
	}
	wantPath := systemctlDir + string(os.PathListSeparator) + "/opt/fnm/bin"
	if err := os.Setenv("PATH", wantPath); err != nil {
		t.Fatalf("set PATH: %v", err)
	}

	m := newModel()
	m.configPath = filepath.Join(home, ".smolbot", "config.json")
	m.workspacePath = filepath.Join(home, ".smolbot", "workspace")
	m.port = 18790

	if err := setupSystemd(&m); err != nil {
		t.Fatalf("setupSystemd: %v", err)
	}

	servicePath := filepath.Join(home, ".config", "systemd", "user", "smolbot.service")
	data, err := os.ReadFile(servicePath)
	if err != nil {
		t.Fatalf("read service: %v", err)
	}
	if !strings.Contains(string(data), "Environment=PATH="+wantPath) {
		t.Fatalf("service file missing captured PATH, got:\n%s", data)
	}
}

func TestHandleTaskCompleteSkipsOptionalFailure(t *testing.T) {
	m := newModel()
	m.tasks = []installTask{
		{name: "Optional", optional: true, status: statusRunning},
		{name: "Next", status: statusPending},
	}
	if m.logFile == nil {
		t.Fatal("expected installer log file")
	}

	updated, cmd := m.handleTaskComplete(taskCompleteMsg{
		index:   0,
		success: false,
		skipped: true,
		err:     errors.New("optional step failed"),
	})
	m = updated.(model)
	if m.tasks[0].status != statusSkipped {
		t.Fatalf("task[0] status = %v, want statusSkipped", m.tasks[0].status)
	}
	if m.tasks[1].status != statusRunning {
		t.Fatalf("task[1] status = %v, want statusRunning", m.tasks[1].status)
	}
	if cmd == nil {
		t.Fatal("expected follow-up command to start the next task")
	}
}
