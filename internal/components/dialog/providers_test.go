package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/internal/theme"
	cfgpkg "github.com/Nomadcxx/smolbot/pkg/config"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
)

func TestProvidersModelRendersActiveProviderSection(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	info := []ProviderInfo{
		{Name: "openai", Type: "OpenAI Compatible", APIBase: "https://api.openai.com/v1", HasAuth: true, IsActive: true},
		{Name: "anthropic", Type: "Anthropic", APIBase: "https://api.anthropic.com", HasAuth: false, IsActive: false},
	}

	model := NewProviders(info, "openai", "gpt-5")
	view := model.View()

	if !strings.Contains(view, "Active") {
		t.Fatalf("expected Active section, got %q", view)
	}
	if !strings.Contains(view, "openai (active)") {
		t.Fatalf("expected active provider marker, got %q", view)
	}
	if !strings.Contains(view, "Type:") {
		t.Fatalf("expected Type field, got %q", view)
	}
	if !strings.Contains(view, "Model:") {
		t.Fatalf("expected Model field, got %q", view)
	}
	if !strings.Contains(view, "gpt-5") {
		t.Fatalf("expected model value, got %q", view)
	}
	if !strings.Contains(view, "API Base:") {
		t.Fatalf("expected API Base field, got %q", view)
	}
}

func TestProvidersModelShowsAuthStatus(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	info := []ProviderInfo{
		{Name: "openai", Type: "OpenAI Compatible", HasAuth: true, IsActive: true},
		{Name: "anthropic", Type: "Anthropic", HasAuth: false, IsActive: false},
	}

	model := NewProviders(info, "openai", "gpt-5")
	view := model.View()

	if !strings.Contains(view, "Auth:") {
		t.Fatalf("expected Auth field, got %q", view)
	}
}

func TestProvidersModelDistinguishesConfiguredAndUnconfigured(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	info := []ProviderInfo{
		{Name: "openai", Type: "OpenAI Compatible", HasAuth: true, IsActive: true},
		{Name: "anthropic", Type: "Anthropic", HasAuth: true, APIBase: "https://api.anthropic.com", IsActive: false},
		{Name: "groq", Type: "Groq", HasAuth: false, IsActive: false},
	}

	model := NewProviders(info, "openai", "gpt-5")
	view := model.View()

	if !strings.Contains(view, "Configured") {
		t.Fatalf("expected Configured section, got %q", view)
	}
	if !strings.Contains(view, "Not Configured") {
		t.Fatalf("expected Not Configured section, got %q", view)
	}
	if !strings.Contains(view, "Groq") {
		t.Fatalf("expected unconfigured provider in Not Configured section, got %q", view)
	}
}

func TestProvidersModelEscToClose(t *testing.T) {
	model := NewProviders([]ProviderInfo{}, "", "")

	_, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	if cmd == nil {
		t.Fatal("expected esc to return close dialog command")
	}

	msg := cmd()
	if _, ok := msg.(CloseDialogMsg); !ok {
		t.Fatalf("expected CloseDialogMsg, got %T", msg)
	}
}

func TestProvidersModelIgnoresOtherKeys(t *testing.T) {
	model := NewProviders([]ProviderInfo{{Name: "openai", Type: "OpenAI Compatible", IsActive: true}}, "openai", "gpt-5")

	_, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("expected enter on active provider to produce SwitchProviderMsg")
	}
}

func TestProvidersModelFromDataBuildsActiveAndConfiguredSections(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	models := []client.ModelInfo{
		{ID: "gpt-5", Name: "GPT-5", Provider: "openai"},
		{ID: "claude-sonnet", Name: "Claude Sonnet", Provider: "anthropic"},
	}
	status := client.StatusPayload{Model: "gpt-5"}
	cfg := cfgpkg.DefaultConfig()
	cfg.Providers = map[string]cfgpkg.ProviderConfig{
		"openai":    {APIKey: "sk-test", APIBase: "https://api.openai.com/v1"},
		"anthropic": {APIKey: "", APIBase: "https://api.anthropic.com"},
	}

	model := NewProvidersFromData(models, "gpt-5", status, &cfg)
	view := model.View()

	if !strings.Contains(view, "Active") {
		t.Fatalf("expected Active section, got %q", view)
	}
	if !strings.Contains(view, "openai (active)") {
		t.Fatalf("expected active provider marker, got %q", view)
	}
	if !strings.Contains(view, "Configured") {
		t.Fatalf("expected Configured section, got %q", view)
	}
}

func TestProvidersModelHandlesEmptyProviders(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	model := NewProviders([]ProviderInfo{}, "", "")
	view := model.View()

	if !strings.Contains(view, "No providers found") {
		t.Fatalf("expected empty state message, got %q", view)
	}
}