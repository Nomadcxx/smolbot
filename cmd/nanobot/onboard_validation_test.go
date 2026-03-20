package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOnboardRejectsInvalidSignalCLIPath(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "nanobot-workspace")
	invalidSignalPath := filepath.Join(t.TempDir(), "nonexistent", "signal-cli")
	input := strings.NewReader(strings.Join([]string{
		"openai",
		"gpt-4.1",
		"sk-test",
		workspace,
		"19001",
		"n",
		"y",
		"+15551234567",
		invalidSignalPath,
		filepath.Join(t.TempDir(), "signal"),
		"n",
		"n",
		"precise",
		"technical",
		"no rewrites",
		"code review",
		"y",
		"n",
		"",
	}, "\n"))
	var out bytes.Buffer

	_, err := collectOnboardConfigFromIO(context.Background(), rootOptions{workspace: workspace}, input, &out)
	if err == nil {
		t.Fatalf("expected error for invalid Signal CLI path %q, got nil", invalidSignalPath)
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "signal") && !strings.Contains(errMsg, "cli") && !strings.Contains(errMsg, invalidSignalPath) {
		t.Fatalf("error %q does not mention invalid Signal CLI path", errMsg)
	}
}

func TestOnboardRejectsInvalidWhatsAppStorePath(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "nanobot-workspace")
	invalidStorePath := filepath.Join(t.TempDir(), "nonexistent", "dir", "whatsapp.db")
	input := strings.NewReader(strings.Join([]string{
		"openai",
		"gpt-4.1",
		"sk-test",
		workspace,
		"19001",
		"n",
		"n",
		"y",
		"nanobot-phone",
		invalidStorePath,
		"precise",
		"technical",
		"no rewrites",
		"code review",
		"y",
		"n",
		"",
	}, "\n"))
	var out bytes.Buffer

	_, err := collectOnboardConfigFromIO(context.Background(), rootOptions{workspace: workspace}, input, &out)
	if err == nil {
		t.Fatalf("expected error for invalid WhatsApp store path %q, got nil", invalidStorePath)
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "whatsapp") && !strings.Contains(errMsg, "store") && !strings.Contains(errMsg, invalidStorePath) {
		t.Fatalf("error %q does not mention invalid WhatsApp store path", errMsg)
	}
}

func TestOnboardRejectsEmptySignalAccount(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "nanobot-workspace")
	input := strings.NewReader(strings.Join([]string{
		"openai",
		"gpt-4.1",
		"sk-test",
		workspace,
		"19001",
		"n",
		"y",
		"",
		"signal-cli",
		filepath.Join(t.TempDir(), "signal"),
		"n",
		"n",
		"precise",
		"technical",
		"no rewrites",
		"code review",
		"y",
		"n",
		"",
	}, "\n"))
	var out bytes.Buffer

	_, err := collectOnboardConfigFromIO(context.Background(), rootOptions{workspace: workspace}, input, &out)
	if err == nil {
		t.Fatalf("expected error for empty Signal account, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "account") {
		t.Fatalf("error %q does not mention account", err.Error())
	}
}

func TestOnboardStructuredPersonaPrompts(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "nanobot-workspace")
	input := strings.NewReader(strings.Join([]string{
		"openai",
		"gpt-4.1",
		"sk-test",
		workspace,
		"19001",
		"n",
		"n",
		"n",
		"precise",
		"technical",
		"no rewrites",
		"code review",
		"y",
		"n",
		"",
	}, "\n"))
	var out bytes.Buffer

	_, err := collectOnboardConfigFromIO(context.Background(), rootOptions{workspace: workspace}, input, &out)
	if err != nil {
		t.Fatalf("collectOnboardConfigFromIO: %v", err)
	}

	soulPath := filepath.Join(workspace, "SOUL.md")
	soulData, err := os.ReadFile(soulPath)
	if err != nil {
		t.Fatalf("expected SOUL.md to be written: %v", err)
	}
	soulText := string(soulData)

	for _, section := range []string{"Tone", "Boundaries", "Expertise"} {
		if !strings.Contains(soulText, section) {
			t.Fatalf("expected SOUL.md to contain %q section, got %q", section, soulText)
		}
	}

	if !strings.Contains(soulText, "precise") {
		t.Fatalf("expected SOUL.md to contain tone value 'precise', got %q", soulText)
	}
	if !strings.Contains(soulText, "technical") {
		t.Fatalf("expected SOUL.md to contain tone value 'technical', got %q", soulText)
	}
	if !strings.Contains(soulText, "no rewrites") {
		t.Fatalf("expected SOUL.md to contain boundary 'no rewrites', got %q", soulText)
	}
	if !strings.Contains(soulText, "code review") {
		t.Fatalf("expected SOUL.md to contain expertise 'code review', got %q", soulText)
	}

	for _, prompt := range []string{
		"Tone",
		"Boundaries",
		"Expertise",
		"Can do",
	} {
		if !strings.Contains(out.String(), prompt) {
			t.Fatalf("expected prompt %q in output %q", prompt, out.String())
		}
	}
}

func TestSoulBootstrapOutputIsDeterministic(t *testing.T) {
	workspace1 := filepath.Join(t.TempDir(), "nanobot-workspace1")
	workspace2 := filepath.Join(t.TempDir(), "nanobot-workspace2")

	input1 := strings.NewReader(strings.Join([]string{
		"openai", "gpt-4.1", "sk-test", workspace1, "19001",
		"n", "n", "n",
		"precise", "technical", "no rewrites", "code review",
		"y", "n",
		"",
	}, "\n"))
	input2 := strings.NewReader(strings.Join([]string{
		"openai", "gpt-4.1", "sk-test", workspace2, "19001",
		"n", "n", "n",
		"precise", "technical", "no rewrites", "code review",
		"y", "n",
		"",
	}, "\n"))

	var out1, out2 bytes.Buffer
	_, err := collectOnboardConfigFromIO(context.Background(), rootOptions{workspace: workspace1}, input1, &out1)
	if err != nil {
		t.Fatalf("first collectOnboardConfigFromIO: %v", err)
	}
	_, err = collectOnboardConfigFromIO(context.Background(), rootOptions{workspace: workspace2}, input2, &out2)
	if err != nil {
		t.Fatalf("second collectOnboardConfigFromIO: %v", err)
	}

	soul1, err := os.ReadFile(filepath.Join(workspace1, "SOUL.md"))
	if err != nil {
		t.Fatalf("read soul1: %v", err)
	}
	soul2, err := os.ReadFile(filepath.Join(workspace2, "SOUL.md"))
	if err != nil {
		t.Fatalf("read soul2: %v", err)
	}

	if string(soul1) != string(soul2) {
		t.Fatalf("SOUL.md outputs differ:\n--- first ---\n%s\n--- second ---\n%s", soul1, soul2)
	}
}

func TestSoulBootstrapOutputContainsRequiredSections(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "nanobot-workspace")
	input := strings.NewReader(strings.Join([]string{
		"openai", "gpt-4.1", "sk-test", workspace, "19001",
		"n", "n", "n",
		"precise", "technical", "no rewrites", "refactoring, testing",
		"y", "n",
		"",
	}, "\n"))
	var out bytes.Buffer

	_, err := collectOnboardConfigFromIO(context.Background(), rootOptions{workspace: workspace}, input, &out)
	if err != nil {
		t.Fatalf("collectOnboardConfigFromIO: %v", err)
	}

	soulPath := filepath.Join(workspace, "SOUL.md")
	soulData, err := os.ReadFile(soulPath)
	if err != nil {
		t.Fatalf("expected SOUL.md to be written: %v", err)
	}
	soulText := string(soulData)

	requiredSections := []string{"# SOUL.md", "Who You Are", "Tone", "Boundaries", "Expertise", "Can do"}
	for _, section := range requiredSections {
		if !strings.Contains(soulText, section) {
			t.Fatalf("expected SOUL.md to contain %q, got %q", section, soulText)
		}
	}
}

func TestOnboardNoInteractionDefaults(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "nanobot-workspace")
	emptyInput := strings.NewReader("")
	var out bytes.Buffer

	cfg, err := collectOnboardConfigFromIO(context.Background(), rootOptions{workspace: workspace}, emptyInput, &out)
	if err != nil {
		t.Fatalf("collectOnboardConfigFromIO with empty input: %v", err)
	}

	if cfg.Agents.Defaults.Provider != "openai" {
		t.Fatalf("expected default provider openai, got %q", cfg.Agents.Defaults.Provider)
	}
	if cfg.Channels.Signal.Enabled {
		t.Fatalf("expected signal disabled by default, got enabled")
	}
	if cfg.Channels.WhatsApp.Enabled {
		t.Fatalf("expected whatsapp disabled by default, got enabled")
	}

	soulPath := filepath.Join(workspace, "SOUL.md")
	if _, err := os.Stat(soulPath); os.IsNotExist(err) {
		t.Fatalf("expected SOUL.md to be written with defaults")
	}
}

func TestOnboardValidatesSignalCLIPathExists(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "nanobot-workspace")
	nonexistentCLI := filepath.Join(t.TempDir(), "nonexistent", "signal-cli")
	input := strings.NewReader(strings.Join([]string{
		"openai", "gpt-4.1", "sk-test", workspace, "19001",
		"n", "y", "+15551234567", nonexistentCLI, filepath.Join(t.TempDir(), "signal"),
		"n", "n",
		"precise", "technical", "no rewrites", "code review",
		"y", "n",
		"",
	}, "\n"))
	var out bytes.Buffer

	_, err := collectOnboardConfigFromIO(context.Background(), rootOptions{workspace: workspace}, input, &out)
	if err == nil {
		t.Fatalf("expected error for nonexistent Signal CLI path, got nil")
	}
}

func TestOnboardValidatesWhatsAppStorePathHasValidParent(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "nanobot-workspace")
	invalidStorePath := filepath.Join(t.TempDir(), "nonexistent_parent", "whatsapp.db")
	input := strings.NewReader(strings.Join([]string{
		"openai", "gpt-4.1", "sk-test", workspace, "19001",
		"n", "n", "y", "nanobot-phone", invalidStorePath,
		"precise", "technical", "no rewrites", "code review",
		"y", "n",
		"",
	}, "\n"))
	var out bytes.Buffer

	_, err := collectOnboardConfigFromIO(context.Background(), rootOptions{workspace: workspace}, input, &out)
	if err == nil {
		t.Fatalf("expected error for WhatsApp store path with nonexistent parent dir, got nil")
	}
}

func TestOnboardConfigDoesNotLeakToRuntime(t *testing.T) {
	workspace := filepath.Join(t.TempDir(), "nanobot-workspace")
	validSignalCLI := filepath.Join(t.TempDir(), "signal-cli")
	if err := os.WriteFile(validSignalCLI, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("create fake signal-cli: %v", err)
	}

	soulWrittenForInvalidConfig := false
	origWriteSoul := writeSoulFile
	writeSoulFile = func(ws, content string) error {
		if strings.Contains(content, "precise") && strings.Contains(content, "technical") {
			soulWrittenForInvalidConfig = true
		}
		return origWriteSoul(ws, content)
	}
	defer func() { writeSoulFile = origWriteSoul }()

	invalidStorePath := filepath.Join(t.TempDir(), "nonexistent", "whatsapp.db")
	input := strings.NewReader(strings.Join([]string{
		"openai", "gpt-4.1", "sk-test", workspace, "19001",
		"n", "y", "+15551234567", validSignalCLI, filepath.Join(t.TempDir(), "signal"),
		"y", "nanobot-phone", invalidStorePath,
		"precise", "technical", "no rewrites", "code review",
		"y", "n",
		"",
	}, "\n"))
	var out bytes.Buffer

	_, err := collectOnboardConfigFromIO(context.Background(), rootOptions{workspace: workspace}, input, &out)
	if err == nil {
		t.Fatalf("expected error for invalid WhatsApp store path, got nil")
	}
	if soulWrittenForInvalidConfig {
		t.Fatalf("SOUL.md was written even though config validation failed - validation should prevent config from being finalized")
	}
}