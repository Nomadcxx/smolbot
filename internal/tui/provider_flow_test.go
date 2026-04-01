package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/Nomadcxx/smolbot/internal/app"
	"github.com/Nomadcxx/smolbot/internal/client"
	dialogcmp "github.com/Nomadcxx/smolbot/internal/components/dialog"
	"github.com/Nomadcxx/smolbot/internal/theme"
	cfgpkg "github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/provider"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
)

func TestProviderDialogShowsActiveConfiguredUnconfiguredProviders(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	model := New(app.Config{})
	model.client = &fakeClient{
		models: []client.ModelInfo{
			{ID: "gpt-5", Name: "GPT-5", Provider: "openai"},
			{ID: "claude-sonnet", Name: "Claude Sonnet", Provider: "anthropic"},
			{ID: "minimax/M2.7", Name: "MiniMax M2.7", Provider: "minimax"},
			{ID: "deepseek-chat", Name: "DeepSeek Chat", Provider: "deepseek"},
		},
		current: "gpt-5",
		status: client.StatusPayload{
			Model: "gpt-5",
		},
	}
	cfg := cfgpkg.DefaultConfig()
	cfg.Providers = map[string]cfgpkg.ProviderConfig{
		"openai":    {APIKey: "sk-test", APIBase: "https://api.openai.com/v1"},
		"anthropic": {APIKey: "", APIBase: "https://api.anthropic.com"},
		"minimax":   {APIKey: "test-key"},
		"deepseek":  {},
	}
	model.providerConfig = &cfg

	_, cmd := model.handleSlashCommand("/providers")
	msg := cmd()
	updated, _ := model.Update(msg)
	got := updated.(Model)

	view := plain(got.dialog.View())

	if !strings.Contains(view, "Active") {
		t.Fatalf("expected Active section, got %q", view)
	}
	if !strings.Contains(view, "openai (active)") {
		t.Fatalf("expected active provider marker, got %q", view)
	}
	if !strings.Contains(view, "openai") {
		t.Fatalf("expected openai as active provider, got %q", view)
	}
	if !strings.Contains(view, "Configured") {
		t.Fatalf("expected Configured section, got %q", view)
	}
	if !strings.Contains(view, "Anthropic") {
		t.Fatalf("expected anthropic in Configured section, got %q", view)
	}
	if !strings.Contains(view, "MiniMax") {
		t.Fatalf("expected minimax in Configured section, got %q", view)
	}
	if !strings.Contains(view, "Not Configured") {
		t.Fatalf("expected Not Configured section, got %q", view)
	}
	if !strings.Contains(view, "DeepSeek") {
		t.Fatalf("expected deepseek in Not Configured section, got %q", view)
	}
}

func TestProviderDialogShowsMiniMaxWithCorrectType(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	model := New(app.Config{})
	model.client = &fakeClient{
		models: []client.ModelInfo{
			{ID: "minimax/M2.7", Name: "MiniMax M2.7", Provider: "minimax"},
		},
		current: "minimax/M2.7",
		status: client.StatusPayload{
			Model: "minimax/M2.7",
		},
	}
	cfg := cfgpkg.DefaultConfig()
	cfg.Providers = map[string]cfgpkg.ProviderConfig{
		"minimax": {APIKey: "test-key"},
	}
	model.providerConfig = &cfg

	_, cmd := model.handleSlashCommand("/providers")
	msg := cmd()
	updated, _ := model.Update(msg)
	got := updated.(Model)

	view := plain(got.dialog.View())

	if !strings.Contains(view, "MiniMax") {
		t.Fatalf("expected MiniMax provider type, got %q", view)
	}
}

func TestModelPickerGroupsByProviderWithCurrentModelHighlighted(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	model := New(app.Config{})
	model.width = 80
	model.height = 24
	model.client = &fakeClient{
		models: []client.ModelInfo{
			{ID: "gpt-5", Name: "GPT-5", Provider: "openai", Selectable: true},
			{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", Selectable: true},
			{ID: "claude-sonnet", Name: "Claude Sonnet", Provider: "anthropic", Selectable: true},
		},
		current: "gpt-5",
	}

	_, cmd := model.handleSlashCommand("/model")
	updated, _ := model.Update(cmd())
	got := updated.(Model)

	view := plain(got.View().Content)

	if !strings.Contains(view, "OpenAI (current)") {
		t.Fatalf("expected current provider group marker, got %q", view)
	}
	if !strings.Contains(view, "Anthropic") {
		t.Fatalf("expected non-current provider group, got %q", view)
	}
	if !strings.Contains(view, "gpt-5") || !strings.Contains(view, "current") {
		t.Fatalf("expected current model with current marker, got %q", view)
	}
}

func TestModelPickerSkipsConfigOnlyRowsWhenNavigating(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	model := New(app.Config{})
	model.width = 80
	model.height = 24
	model.client = &fakeClient{
		models: []client.ModelInfo{
			{ID: "openai", Name: "OpenRouter", Provider: "openrouter", Description: "Configured provider", Source: "config", Selectable: false},
			{ID: "openrouter/auto", Name: "Auto", Provider: "openrouter", Selectable: true},
			{ID: "gpt-5", Name: "GPT-5", Provider: "openai", Selectable: true},
		},
		current: "gpt-5",
	}

	_, cmd := model.handleSlashCommand("/model")
	updated, _ := model.Update(cmd())
	got := updated.(Model)

	updated, _ = got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	got = updated.(Model)

	_, chooseCmd := got.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if chooseCmd == nil {
		t.Fatal("expected enter on first selectable to produce command")
	}

	msg := chooseCmd()
	chosen, ok := msg.(dialogcmp.ModelChosenMsg)
	if !ok {
		t.Fatalf("expected ModelChosenMsg, got %T", msg)
	}
	if chosen.ID == "openai" {
		t.Fatalf("expected config-only row to be skipped, got %q", chosen.ID)
	}
}

func TestMiniMaxProviderHasDefaultAPIBase(t *testing.T) {
	cfg := cfgpkg.DefaultConfig()
	cfg.Providers = map[string]cfgpkg.ProviderConfig{
		"minimax": {APIKey: "test-key"},
	}

	r := provider.NewRegistryWithDefaults(&cfg)
	p, err := r.ForModel("minimax/M2.7")
	if err != nil {
		t.Fatalf("expected minimax provider to resolve, got error: %v", err)
	}

	if p.Name() != "minimax" {
		t.Fatalf("expected provider name minimax, got %q", p.Name())
	}
}