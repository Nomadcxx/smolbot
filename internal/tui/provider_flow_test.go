package tui

import (
	"encoding/json"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/Nomadcxx/smolbot/internal/app"
	"github.com/Nomadcxx/smolbot/internal/client"
	dialogcmp "github.com/Nomadcxx/smolbot/internal/components/dialog"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
	"github.com/Nomadcxx/smolbot/internal/theme"
	cfgpkg "github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/provider"
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
	if !strings.Contains(view, "Not Configured") && !strings.Contains(view, "Popular") {
		t.Fatalf("expected Not Configured or Popular section, got %q", view)
	}
	if !strings.Contains(view, "DeepSeek") {
		t.Fatalf("expected deepseek in unconfigured section, got %q", view)
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
func TestTUICtrlAFromModelDialogTransitionsToProviderDialog(t *testing.T) {
if !theme.Set("nord") {
t.Fatal("expected nord theme to be available")
}

model := New(app.Config{})
model.width = 80
model.height = 40
model.client = &fakeClient{
models: []client.ModelInfo{
{ID: "gpt-5", Name: "GPT-5", Provider: "openai", Selectable: true},
},
current: "gpt-5",
}

// Open model dialog
_, cmd := model.handleSlashCommand("/model")
updated, _ := model.Update(cmd())
got := updated.(Model)

// Verify model dialog is open
if _, ok := got.dialog.(modelsDialog); !ok {
t.Fatalf("expected models dialog to be open, got %T", got.dialog)
}

// Press Ctrl+A — should emit RequestProviderAddMsg and open provider dialog
ctrlA := tea.Key{Code: 'a', Mod: tea.ModCtrl}
updated2, cmd2 := got.Update(tea.KeyPressMsg(ctrlA))
got2 := updated2.(Model)

// After ctrl+a, model dialog is closed and requestProviderAdd is in flight
if cmd2 == nil {
t.Fatal("expected ctrl+a to produce a command")
}
msg := cmd2()
if _, ok := msg.(dialogcmp.RequestProviderAddMsg); !ok {
t.Fatalf("expected RequestProviderAddMsg from ctrl+a, got %T", msg)
}

// Feed the message back to TUI — it should set returnToModelsAfterProvider and start loading providers
updated3, loadCmd := got2.Update(msg)
got3 := updated3.(Model)
if !got3.returnToModelsAfterProvider {
t.Fatal("expected returnToModelsAfterProvider to be set after RequestProviderAddMsg")
}
if loadCmd == nil {
t.Fatal("expected load providers command after RequestProviderAddMsg")
}
}

func TestTUIReturnsToModelsAfterProviderConfig(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	model := New(app.Config{})
	model.width = 80
	model.height = 40
	model.client = &fakeClient{
		models: []client.ModelInfo{
			{ID: "claude-opus", Name: "Claude Opus", Provider: "anthropic", Selectable: true},
		},
		current: "claude-opus",
	}
	// Set up state: returnToModelsAfterProvider=true with a providers dialog open
	model.returnToModelsAfterProvider = true
	model.dialog = providersDialog{dialogcmp.NewProviders(
		[]dialogcmp.ProviderInfo{{Name: "anthropic", Type: "Anthropic", HasAuth: true}},
		"anthropic", "claude-opus",
	)}

	// Simulate models.updated server event — the handler checks returnToModelsAfterProvider
	// and replaces the providers dialog with a models dialog.
	payload, _ := json.Marshal(client.ModelsUpdatedPayload{
		Models:  []client.ModelInfo{{ID: "claude-opus", Name: "Claude Opus", Provider: "anthropic", Selectable: true}},
		Current: "claude-opus",
	})
	eventMsg := EventMsg{Event: client.Event{
		Type:    client.FrameEvent,
		Event:   "models.updated",
		Payload: payload,
	}}
	updated, _ := model.Update(eventMsg)
	got := updated.(Model)

	if got.returnToModelsAfterProvider {
		t.Fatal("expected returnToModelsAfterProvider to be cleared after models.updated")
	}
	if _, ok := got.dialog.(modelsDialog); !ok {
		t.Fatalf("expected models dialog to reopen after models.updated with returnToModelsAfterProvider=true, got %T", got.dialog)
	}
}
