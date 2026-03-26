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
		{ID: "claude-sonnet", Name: "Claude Sonnet", Provider: "anthropic"},
		{ID: "gpt-5", Name: "GPT-5", Provider: "openai"},
	}, "gpt-5")

	view := model.View()
	if !strings.Contains(view, "current") {
		t.Fatalf("expected current marker in dialog, got %q", view)
	}
	if !strings.Contains(view, "openai") {
		t.Fatalf("expected provider description in dialog, got %q", view)
	}
	if !strings.Contains(view, "Enter switch") {
		t.Fatalf("expected selector footer help row, got %q", view)
	}
}

func TestModelsModelGroupsByProvider(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	model := NewModels([]client.ModelInfo{
		{ID: "claude-sonnet", Name: "Claude Sonnet", Provider: "anthropic"},
		{ID: "gpt-5", Name: "GPT-5", Provider: "openai"},
		{ID: "gpt-4o", Name: "GPT-4o", Provider: "openai"},
	}, "gpt-5")

	view := model.View()
	if !strings.Contains(view, "Provider: anthropic") {
		t.Fatalf("expected anthropic provider section, got %q", view)
	}
	if !strings.Contains(view, "Provider: openai (current)") {
		t.Fatalf("expected current provider section, got %q", view)
	}
	if !strings.Contains(view, "[openai] GPT-5") || !strings.Contains(view, "current") {
		t.Fatalf("expected current model row inside provider group, got %q", view)
	}

	anthropicIdx := strings.Index(view, "Provider: anthropic")
	openaiIdx := strings.Index(view, "Provider: openai (current)")
	if anthropicIdx == -1 || openaiIdx == -1 || anthropicIdx > openaiIdx {
		t.Fatalf("expected provider groups in stable order, got %q", view)
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
