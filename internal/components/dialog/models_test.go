package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/internal/theme"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
)

func TestModelsModelShowsCurrentModel(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	model := NewModels([]client.ModelInfo{
		{ID: "claude-sonnet", Name: "Claude Sonnet", Provider: "anthropic", Selectable: true},
		{ID: "gpt-5", Name: "GPT-5", Provider: "openai", Selectable: true},
	}, "gpt-5")

	view := model.View()
	if !strings.Contains(view, "current") {
		t.Fatalf("expected current marker in dialog, got %q", view)
	}
	if !strings.Contains(view, "openai") {
		t.Fatalf("expected provider description in dialog, got %q", view)
	}
	if !strings.Contains(view, "Enter save") {
		t.Fatalf("expected selector footer help row, got %q", view)
	}
}

func TestModelsModelGroupsByProvider(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	model := NewModels([]client.ModelInfo{
		{ID: "claude-sonnet", Name: "Claude Sonnet", Provider: "anthropic", Selectable: true},
		{ID: "gpt-5", Name: "GPT-5", Provider: "openai", Selectable: true},
		{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", Selectable: true},
	}, "gpt-5")

	view := model.View()
	if !strings.Contains(view, "Provider: anthropic") {
		t.Fatalf("expected anthropic provider section, got %q", view)
	}
	if !strings.Contains(view, "Provider: openai (current)") {
		t.Fatalf("expected current provider section, got %q", view)
	}
	if !strings.Contains(view, "GPT-5") || !strings.Contains(view, "current") {
		t.Fatalf("expected current model row inside provider group, got %q", view)
	}

	anthropicIdx := strings.Index(view, "Provider: anthropic")
	openaiIdx := strings.Index(view, "Provider: openai (current)")
	if anthropicIdx == -1 || openaiIdx == -1 || anthropicIdx > openaiIdx {
		t.Fatalf("expected provider groups in stable order, got %q", view)
	}
}

func TestModelsModelSkipsInfoOnlyRowsWhenChoosing(t *testing.T) {
	model := NewModels([]client.ModelInfo{
		{ID: "openrouter", Name: "OpenRouter", Provider: "openrouter", Description: "Configured provider", Source: "config", Selectable: false},
		{ID: "openrouter/auto", Name: "Auto", Provider: "openrouter", Selectable: true},
	}, "openrouter/auto")

	view := model.View()
	if !strings.Contains(strings.ToLower(view), "configured provider") {
		t.Fatalf("expected provider info row to remain visible, got %q", view)
	}

	_, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("expected enter to choose the first selectable model, not the provider info row")
	}
	msg := cmd()
	chosen, ok := msg.(ModelChosenMsg)
	if !ok {
		t.Fatalf("expected ModelChosenMsg, got %T", msg)
	}
	if chosen.ID != "openrouter/auto" {
		t.Fatalf("chosen id = %q, want openrouter/auto", chosen.ID)
	}
}

func TestModelsModelSpaceMarksPendingSelectionAndEnterConfirmsIt(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	model := NewModels([]client.ModelInfo{
		{ID: "gpt-5", Name: "GPT-5", Provider: "openai", Selectable: true},
		{ID: "claude-3-7-sonnet", Name: "Claude 3.7 Sonnet", Provider: "anthropic", Selectable: true},
	}, "gpt-5")

	model, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	model, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: ' ', Text: " "}))
	if cmd != nil {
		t.Fatal("expected space to mark a pending selection without saving immediately")
	}

	view := model.View()
	if !strings.Contains(view, "pending") {
		t.Fatalf("expected pending marker after space selection, got %q", view)
	}

	model, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp}))
	_, cmd = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("expected enter to confirm the pending selection")
	}
	msg := cmd()
	chosen, ok := msg.(ModelChosenMsg)
	if !ok {
		t.Fatalf("expected ModelChosenMsg, got %T", msg)
	}
	if chosen.ID != "claude-3-7-sonnet" {
		t.Fatalf("chosen id = %q, want claude-3-7-sonnet", chosen.ID)
	}
}

func TestModelsModelAllowsLegacyRowsWithoutSelectableMetadata(t *testing.T) {
	model := NewModels([]client.ModelInfo{
		{ID: "gpt-5", Name: "GPT-5", Provider: "openai"},
	}, "gpt-5")

	_, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("expected legacy row without selectable metadata to remain selectable")
	}
	msg := cmd()
	chosen, ok := msg.(ModelChosenMsg)
	if !ok {
		t.Fatalf("expected ModelChosenMsg, got %T", msg)
	}
	if chosen.ID != "gpt-5" {
		t.Fatalf("chosen id = %q, want gpt-5", chosen.ID)
	}
}

func TestModelsModelFilterNarrowsByProviderAndModel(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	model := NewModels([]client.ModelInfo{
		{ID: "gpt-5", Name: "GPT-5", Provider: "openai", Selectable: true},
		{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", Selectable: true},
		{ID: "claude-opus", Name: "Claude Opus", Provider: "anthropic", Selectable: true},
	}, "gpt-5")

	for _, ch := range "openai" {
		model, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: ch, Text: string(ch)}))
	}

	view := model.View()
	if !strings.Contains(view, "Provider: openai (current)") {
		t.Fatalf("expected provider filter to keep the openai group, got %q", view)
	}
	if strings.Contains(view, "Provider: anthropic") {
		t.Fatalf("expected provider filter to hide non-matching providers, got %q", view)
	}

	for range "openai" {
		model, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyBackspace}))
	}
	for _, ch := range "claude" {
		model, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: ch, Text: string(ch)}))
	}

	view = model.View()
	if !strings.Contains(view, "Claude Opus") {
		t.Fatalf("expected model filter to keep matching model rows, got %q", view)
	}
	if strings.Contains(view, "GPT-5") || strings.Contains(view, "GPT-4o") {
		t.Fatalf("expected model filter to hide non-matching models, got %q", view)
	}
}

func TestModelsModelShowsOverflowCues(t *testing.T) {
	models := make([]client.ModelInfo, 0, 10)
	for i := 0; i < 10; i++ {
		models = append(models, client.ModelInfo{
			ID:       "model-" + string(rune('a'+i)),
			Name:     "Model " + string(rune('A'+i)),
			Provider: "provider",
		})
	}

	model := NewModels(models, "model-a")
	view := model.View()
	if !strings.Contains(view, "▼ more below") {
		t.Fatalf("expected lower overflow cue, got %q", view)
	}

	for i := 0; i < 6; i++ {
		model, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	}

	view = model.View()
	if !strings.Contains(view, "▲ more above") {
		t.Fatalf("expected upper overflow cue after scrolling, got %q", view)
	}
}

func TestModelsModelSeparatesMinimaxAndMinimaxPortal(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	model := NewModels([]client.ModelInfo{
		{ID: "minimax/M2.7", Name: "MiniMax M2.7", Provider: "minimax", Selectable: true},
		{ID: "minimax-portal/MiniMax-M2.7", Name: "MiniMax M2.7 (OAuth)", Provider: "minimax-portal", Selectable: true},
	}, "minimax-portal/MiniMax-M2.7")

	view := model.View()

	if !strings.Contains(view, "Provider: minimax") {
		t.Fatalf("expected minimax provider group, got %q", view)
	}
	if !strings.Contains(view, "Provider: minimax-portal (current)") {
		t.Fatalf("expected minimax-portal provider group with current marker, got %q", view)
	}

	minimaxIdx := strings.Index(view, "Provider: minimax")
	minimaxPortalIdx := strings.Index(view, "Provider: minimax-portal")
	if minimaxIdx == -1 || minimaxPortalIdx == -1 {
		t.Fatalf("both providers should appear in view")
	}
	if minimaxIdx > minimaxPortalIdx {
		t.Fatalf("minimax should appear before minimax-portal alphabetically")
	}
}

func TestOptionalModelDescription(t *testing.T) {
	type describedModel struct {
		Description string
	}

	if got := optionalModelDescription(describedModel{Description: "High reasoning"}); got != "High reasoning" {
		t.Fatalf("expected description field to be used, got %q", got)
	}
	if got := optionalModelDescription(client.ModelInfo{ID: "fast"}); got != "" {
		t.Fatalf("expected client model info to have no optional description, got %q", got)
	}
}
