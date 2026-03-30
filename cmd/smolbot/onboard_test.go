package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/config"
)

func TestOnboardCommandWritesConfig(t *testing.T) {
	origCollect := collectOnboardConfig
	origWrite := writeConfigFile
	defer func() {
		collectOnboardConfig = origCollect
		writeConfigFile = origWrite
	}()

	var wrotePath string
	collectOnboardConfig = func(context.Context, rootOptions) (*config.Config, error) {
		cfg := config.DefaultConfig()
		cfg.Agents.Defaults.Model = "claude-sonnet"
		return &cfg, nil
	}
	writeConfigFile = func(path string, cfg *config.Config) error {
		wrotePath = path
		return os.WriteFile(path, []byte(`{"ok":true}`), 0o644)
	}

	target := filepath.Join(t.TempDir(), "config.json")
	cmd := NewRootCmd("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"onboard", "--config", target})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if wrotePath != target {
		t.Fatalf("expected config write to %q, got %q", target, wrotePath)
	}
	if _, err := os.Stat(target); err != nil {
		t.Fatalf("expected config file to exist: %v", err)
	}
}

func TestCollectOnboardConfigPromptsAndAppliesAnswers(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "nanobot-workspace")
	input := strings.NewReader(strings.Join([]string{
		"openai",
		"gpt-4.1",
		"sk-test",
		workspace,
		"19001",
		"y",
		"slack",
		"n",
		"n",
		"precise",
		"no rewrites",
		"code review",
		"read/write files",
		"y",
		"n",
		"",
	}, "\n"))
	var out bytes.Buffer

	cfg, err := collectOnboardConfigFromIO(context.Background(), rootOptions{}, input, &out)
	if err != nil {
		t.Fatalf("collectOnboardConfigFromIO: %v", err)
	}
	if cfg.Agents.Defaults.Provider != "openai" {
		t.Fatalf("provider = %q, want openai", cfg.Agents.Defaults.Provider)
	}
	if cfg.Agents.Defaults.Model != "gpt-4.1" {
		t.Fatalf("model = %q, want gpt-4.1", cfg.Agents.Defaults.Model)
	}
	if cfg.Providers["openai"].APIKey != "sk-test" {
		t.Fatalf("api key = %q, want sk-test", cfg.Providers["openai"].APIKey)
	}
	if cfg.Agents.Defaults.Workspace != workspace {
		t.Fatalf("workspace = %q, want %q", cfg.Agents.Defaults.Workspace, workspace)
	}
	if cfg.Gateway.Port != 19001 {
		t.Fatalf("port = %d, want 19001", cfg.Gateway.Port)
	}
	if !cfg.Gateway.Heartbeat.Enabled || cfg.Gateway.Heartbeat.Channel != "slack" {
		t.Fatalf("unexpected heartbeat config %#v", cfg.Gateway.Heartbeat)
	}
	if cfg.Channels.Signal.Enabled {
		t.Fatalf("expected signal channel to remain disabled, got %#v", cfg.Channels.Signal)
	}
	if cfg.Channels.WhatsApp.Enabled {
		t.Fatalf("expected whatsapp channel to remain disabled, got %#v", cfg.Channels.WhatsApp)
	}
	soulPath := filepath.Join(workspace, "SOUL.md")
	soulData, err := os.ReadFile(soulPath)
	if err != nil {
		t.Fatalf("expected SOUL.md to be written: %v", err)
	}
	if !strings.Contains(string(soulData), "precise") {
		t.Fatalf("expected SOUL.md to contain persona guidance, got %q", string(soulData))
	}
	for _, prompt := range []string{
		"Provider",
		"Model",
		"API key",
		"Workspace",
		"Gateway port",
		"Enable heartbeat",
		"Enable Signal channel",
		"Enable WhatsApp channel",
		"Tone",
		"Boundaries",
	} {
		if !strings.Contains(out.String(), prompt) {
			t.Fatalf("expected prompt %q in output %q", prompt, out.String())
		}
	}
}

func TestCollectOnboardConfigAppliesSignalAndWhatsAppAnswers(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "nanobot-workspace")
	signalDir := filepath.Join(t.TempDir(), "signal")
	whatsAppStore := filepath.Join(t.TempDir(), "whatsapp.db")

	if err := os.MkdirAll(signalDir, 0o755); err != nil {
		t.Fatalf("create signal dir: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(whatsAppStore), 0o755); err != nil {
		t.Fatalf("create whatsapp dir: %v", err)
	}

	input := strings.NewReader(strings.Join([]string{
		"openai",
		"gpt-4.1",
		"sk-test",
		workspace,
		"19001",
		"n",
		"y",
		"+15551234567",
		"signal-cli",
		signalDir,
		"y",
		"nanobot-phone",
		whatsAppStore,
		"precise",
		"no rewrites",
		"code review",
		"read/write files",
		"y",
		"n",
		"",
	}, "\n"))
	var out bytes.Buffer

	cfg, err := collectOnboardConfigFromIO(context.Background(), rootOptions{}, input, &out)
	if err != nil {
		t.Fatalf("collectOnboardConfigFromIO: %v", err)
	}

	if !cfg.Channels.Signal.Enabled {
		t.Fatalf("expected signal to be enabled, got %#v", cfg.Channels.Signal)
	}
	if cfg.Channels.Signal.Account != "+15551234567" {
		t.Fatalf("signal account = %q, want +15551234567", cfg.Channels.Signal.Account)
	}
	if cfg.Channels.Signal.CLIPath != "signal-cli" {
		t.Fatalf("signal cli path = %q, want signal-cli", cfg.Channels.Signal.CLIPath)
	}
	if cfg.Channels.Signal.DataDir != signalDir {
		t.Fatalf("signal data dir = %q, want %q", cfg.Channels.Signal.DataDir, signalDir)
	}
	if !cfg.Channels.WhatsApp.Enabled {
		t.Fatalf("expected whatsapp to be enabled, got %#v", cfg.Channels.WhatsApp)
	}
	if cfg.Channels.WhatsApp.DeviceName != "nanobot-phone" {
		t.Fatalf("whatsapp device name = %q, want nanobot-phone", cfg.Channels.WhatsApp.DeviceName)
	}
	if cfg.Channels.WhatsApp.StorePath != whatsAppStore {
		t.Fatalf("whatsapp store path = %q, want %q", cfg.Channels.WhatsApp.StorePath, whatsAppStore)
	}

	for _, prompt := range []string{
		"Enable Signal channel",
		"Signal account",
		"Signal CLI path",
		"Signal data dir",
		"Enable WhatsApp channel",
		"WhatsApp device name",
		"WhatsApp store path",
	} {
		if !strings.Contains(out.String(), prompt) {
			t.Fatalf("expected prompt %q in output %q", prompt, out.String())
		}
	}
}

func TestDefaultSoulTemplateProvidesStructuredGuidance(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "templates", "SOUL.md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	text := string(data)
	for _, needle := range []string{"Who You Are", "Tone", "Boundaries"} {
		if !strings.Contains(text, needle) {
			t.Fatalf("expected SOUL template to include %q, got %q", needle, text)
		}
	}
}

func TestOnboardCommandRefusesIfConfigExists(t *testing.T) {
	origCollect := collectOnboardConfig
	origWrite := writeConfigFile
	defer func() {
		collectOnboardConfig = origCollect
		writeConfigFile = origWrite
	}()

	collectOnboardConfig = func(context.Context, rootOptions) (*config.Config, error) {
		cfg := config.DefaultConfig()
		return &cfg, nil
	}
	var writeCalled bool
	writeConfigFile = func(path string, cfg *config.Config) error {
		writeCalled = true
		return nil
	}

	target := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(target, []byte(`{"ok":true}`), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cmd := NewRootCmd("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"onboard", "--config", target})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when config already exists, got nil")
	}
	if writeCalled {
		t.Fatal("writeConfigFile should not have been called")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected 'already exists' in error, got %q", err.Error())
	}
}

func TestCollectOnboardConfigEnablesOllamaQuota(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "nanobot-workspace")
	input := strings.NewReader(strings.Join([]string{
		"ollama",
		"llama3.2",
		"y",
		"",
		workspace,
		"18790",
		"n",
		"n",
		"n",
		"",
	}, "\n"))
	var out bytes.Buffer

	cfg, err := collectOnboardConfigFromIO(context.Background(), rootOptions{}, input, &out)
	if err != nil {
		t.Fatalf("collectOnboardConfigFromIO: %v", err)
	}
	if cfg.Agents.Defaults.Provider != "ollama" {
		t.Fatalf("provider = %q, want ollama", cfg.Agents.Defaults.Provider)
	}
	if !cfg.Quota.HasEnabledProvider("ollama") {
		t.Fatal("expected ollama quota to be enabled")
	}
	if cfg.Quota.RefreshIntervalMinutes != 60 {
		t.Fatalf("refreshIntervalMinutes = %d, want 60", cfg.Quota.RefreshIntervalMinutes)
	}
	p := cfg.Quota.Provider("ollama")
	if !p.BrowserCookieDiscoveryEnabled {
		t.Fatal("expected browser cookie discovery to be enabled by default")
	}
}

func TestCollectOnboardConfigSkipsQuotaForNonOllama(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "nanobot-workspace")
	input := strings.NewReader(strings.Join([]string{
		"openai",
		"gpt-4",
		"sk-test",
		workspace,
		"18790",
		"n",
		"n",
		"n",
		"",
	}, "\n"))
	var out bytes.Buffer

	cfg, err := collectOnboardConfigFromIO(context.Background(), rootOptions{}, input, &out)
	if err != nil {
		t.Fatalf("collectOnboardConfigFromIO: %v", err)
	}
	if cfg.Quota.HasEnabledProvider("ollama") {
		t.Fatal("expected no ollama quota for openai provider")
	}
	if !strings.Contains(out.String(), "API key") {
		t.Fatalf("expected API key prompt in output, got %q", out.String())
	}
}
