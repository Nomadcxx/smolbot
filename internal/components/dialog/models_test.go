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

func TestModelsModelShowsCurrentModel(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	model := NewModels(nil, []client.ModelInfo{
		{ID: "claude-sonnet", Name: "Claude Sonnet", Provider: "anthropic", Selectable: true},
		{ID: "gpt-5", Name: "GPT-5", Provider: "openai", Selectable: true},
	}, "gpt-5")

	view := model.View()
	if !strings.Contains(view, "●") {
		t.Fatalf("expected ● current marker in dialog, got %q", view)
	}
	if !strings.Contains(view, "OpenAI") {
		t.Fatalf("expected OpenAI provider display name in dialog, got %q", view)
	}
	if !strings.Contains(view, "Enter save") {
		t.Fatalf("expected selector footer help row, got %q", view)
	}
}

func TestModelsModelGroupsByProvider(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	model := NewModels(nil, []client.ModelInfo{
		{ID: "claude-sonnet", Name: "Claude Sonnet", Provider: "anthropic", Selectable: true},
		{ID: "gpt-5", Name: "GPT-5", Provider: "openai", Selectable: true},
		{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", Selectable: true},
	}, "gpt-5")

	view := model.View()
	if !strings.Contains(view, "Anthropic") {
		t.Fatalf("expected Anthropic provider display name, got %q", view)
	}
	if !strings.Contains(view, "OpenAI") {
		t.Fatalf("expected OpenAI provider display name, got %q", view)
	}
	if !strings.Contains(view, "(current)") {
		t.Fatalf("expected (current) suffix on active provider, got %q", view)
	}
	if !strings.Contains(view, "GPT-5") || !strings.Contains(view, "●") {
		t.Fatalf("expected current model row with ● marker, got %q", view)
	}

	anthropicIdx := strings.Index(view, "Anthropic")
	openaiIdx := strings.Index(view, "OpenAI")
	if anthropicIdx == -1 || openaiIdx == -1 || anthropicIdx > openaiIdx {
		t.Fatalf("expected provider groups in stable order, got %q", view)
	}
}

func TestModelsModelSkipsInfoOnlyRowsWhenChoosing(t *testing.T) {
	model := NewModels(nil, []client.ModelInfo{
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

	model := NewModels(nil, []client.ModelInfo{
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
	model := NewModels(nil, []client.ModelInfo{
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

	model := NewModels(nil, []client.ModelInfo{
		{ID: "gpt-5", Name: "GPT-5", Provider: "openai", Selectable: true},
		{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", Selectable: true},
		{ID: "claude-opus", Name: "Claude Opus", Provider: "anthropic", Selectable: true},
	}, "gpt-5")

	for _, ch := range "openai" {
		model, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: ch, Text: string(ch)}))
	}

	view := model.View()
	if !strings.Contains(view, "OpenAI") {
		t.Fatalf("expected provider filter to keep the openai group, got %q", view)
	}
	if !strings.Contains(view, "(current)") {
		t.Fatalf("expected (current) suffix on active provider, got %q", view)
	}
	if strings.Contains(view, "Anthropic") {
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

	model := NewModels(nil, models, "model-a")
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

	model := NewModels(nil, []client.ModelInfo{
		{ID: "minimax/M2.7", Name: "MiniMax M2.7", Provider: "minimax", Selectable: true},
		{ID: "minimax-portal/MiniMax-M2.7", Name: "MiniMax M2.7 (OAuth)", Provider: "minimax-portal", Selectable: true},
	}, "minimax-portal/MiniMax-M2.7")

	view := model.View()

	if !strings.Contains(view, "MiniMax") {
		t.Fatalf("expected minimax provider group, got %q", view)
	}
	if !strings.Contains(view, "(current)") {
		t.Fatalf("expected (current) marker on active provider, got %q", view)
	}

	minimaxIdx := strings.Index(view, "MiniMax")
	minimaxPortalIdx := strings.LastIndex(view, "MiniMax")
	if minimaxIdx == -1 || minimaxPortalIdx == -1 || minimaxIdx == minimaxPortalIdx {
		t.Fatalf("both minimax providers should appear in view, got %q", view)
	}
}

func TestModelsModelOAuthFilterHidesMinimaxWhenPortalIsOAuth(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	oauthConfig := &cfgpkg.Config{
		Providers: map[string]cfgpkg.ProviderConfig{
			"minimax-portal": {AuthType: "oauth", ProfileID: "minimax-portal:default"},
		},
	}

	models := []client.ModelInfo{
		{ID: "minimax/M2.7", Name: "MiniMax M2.7", Provider: "minimax", Selectable: true},
		{ID: "minimax-portal/MiniMax-M2.7", Name: "MiniMax M2.7 (OAuth)", Provider: "minimax-portal", Selectable: true},
		{ID: "claude-sonnet", Name: "Claude Sonnet", Provider: "anthropic", Selectable: true},
	}

	model := NewModels(oauthConfig, models, "claude-sonnet")

	view := model.View()
	if !strings.Contains(view, "MiniMax") {
		t.Fatalf("expected minimax provider group to appear, got %q", view)
	}
	if !strings.Contains(view, "Claude Sonnet") {
		t.Fatalf("expected anthropic models to remain visible, got %q", view)
	}
}

func TestOptionalModelDescription(t *testing.T) {
	if got := optionalModelDescription(client.ModelInfo{ID: "test", Description: "High reasoning"}); got != "High reasoning" {
		t.Fatalf("expected description field to be used, got %q", got)
	}
	if got := optionalModelDescription(client.ModelInfo{ID: "fast"}); got != "" {
		t.Fatalf("expected empty description for model without one, got %q", got)
	}
}

func TestModelsModelShowsFavoritesSection(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	models := []client.ModelInfo{
		{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", Selectable: true},
		{ID: "claude-sonnet", Name: "Claude Sonnet", Provider: "anthropic", Selectable: true},
	}
	m := NewModelsWithState(nil, models, "gpt-4o", []string{"claude-sonnet"}, nil)

	view := m.View()
	if !strings.Contains(view, "★ Favorites") {
		t.Fatalf("expected Favorites section header, got %q", view)
	}
	if !strings.Contains(view, "Claude Sonnet") {
		t.Fatalf("expected favorite model in section, got %q", view)
	}
	// Favorites section should appear before the OpenAI provider section
	favIdx := strings.Index(view, "★ Favorites")
	openaiIdx := strings.Index(view, "OpenAI")
	if favIdx == -1 || openaiIdx == -1 || favIdx > openaiIdx {
		t.Fatalf("expected Favorites before provider sections, got %q", view)
	}
}

func TestModelsModelShowsRecentsSection(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	models := []client.ModelInfo{
		{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", Selectable: true},
		{ID: "claude-sonnet", Name: "Claude Sonnet", Provider: "anthropic", Selectable: true},
	}
	m := NewModelsWithState(nil, models, "gpt-4o", nil, []string{"claude-sonnet"})

	view := m.View()
	if !strings.Contains(view, "⏱ Recent") {
		t.Fatalf("expected Recent section header, got %q", view)
	}
	if !strings.Contains(view, "Claude Sonnet") {
		t.Fatalf("expected recent model in section, got %q", view)
	}
}

func TestModelsModelDeduplicatesFavoritesFromProviderGroups(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	models := []client.ModelInfo{
		{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", Selectable: true},
		{ID: "claude-sonnet", Name: "Claude Sonnet", Provider: "anthropic", Selectable: true},
	}
	m := NewModelsWithState(nil, models, "gpt-4o", []string{"claude-sonnet"}, nil)

	view := m.View()
	// Claude Sonnet should appear exactly once (in Favorites, not again in Anthropic section)
	count := strings.Count(view, "Claude Sonnet")
	if count != 1 {
		t.Fatalf("expected Claude Sonnet to appear once, got %d times in %q", count, view)
	}
}

func TestModelsModelCtrlFSendsFavoriteToggleMsg(t *testing.T) {
	models := []client.ModelInfo{
		{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", Selectable: true},
	}
	m := NewModels(nil, models, "gpt-4o")

	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'f', Mod: tea.ModCtrl}))
	if cmd == nil {
		t.Fatal("expected ctrl+f to produce a command")
	}
	msg := cmd()
	fav, ok := msg.(ModelFavoriteToggledMsg)
	if !ok {
		t.Fatalf("expected ModelFavoriteToggledMsg, got %T", msg)
	}
	if fav.ID != "gpt-4o" {
		t.Fatalf("expected fav ID gpt-4o, got %q", fav.ID)
	}
}

func TestModelsModelCtrlXSendsRemoveFromRecentsMsg(t *testing.T) {
	models := []client.ModelInfo{
		{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", Selectable: true},
	}
	m := NewModels(nil, models, "gpt-4o")

	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'x', Mod: tea.ModCtrl}))
	if cmd == nil {
		t.Fatal("expected ctrl+x to produce a command")
	}
	msg := cmd()
	rem, ok := msg.(ModelRemovedFromRecentsMsg)
	if !ok {
		t.Fatalf("expected ModelRemovedFromRecentsMsg, got %T", msg)
	}
	if rem.ID != "gpt-4o" {
		t.Fatalf("expected removed ID gpt-4o, got %q", rem.ID)
	}
}

func TestModelsModelCtrlASendsRequestProviderAddMsg(t *testing.T) {
	m := NewModels(nil, nil, "")

	_, cmd := m.Update(tea.KeyPressMsg(tea.Key{Code: 'a', Mod: tea.ModCtrl}))
	if cmd == nil {
		t.Fatal("expected ctrl+a to produce a command")
	}
	msg := cmd()
	if _, ok := msg.(RequestProviderAddMsg); !ok {
		t.Fatalf("expected RequestProviderAddMsg, got %T", msg)
	}
}

func TestModelsModelFilterIncludesFavoritesAndRecents(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	models := []client.ModelInfo{
		{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai", Selectable: true},
		{ID: "claude-sonnet", Name: "Claude Sonnet", Provider: "anthropic", Selectable: true},
	}
	m := NewModelsWithState(nil, models, "gpt-4o", []string{"claude-sonnet"}, []string{"gpt-4o"})

	// Filter by "claude" — should find the favorite
	for _, ch := range "claude" {
		m, _ = m.Update(tea.KeyPressMsg(tea.Key{Code: ch, Text: string(ch)}))
	}

	view := m.View()
	if !strings.Contains(view, "Claude Sonnet") {
		t.Fatalf("expected filter to include favorites, got %q", view)
	}
	if strings.Contains(view, "GPT-4o") {
		t.Fatalf("expected non-matching models to be hidden, got %q", view)
	}
}

