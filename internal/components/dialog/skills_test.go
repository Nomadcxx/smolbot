package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/internal/theme"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
)

func TestSkillsModelRendersAndFilters(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}

	model := NewSkills([]client.SkillInfo{
		{Name: "brainstorming", Description: "Design work", Status: "always"},
		{Name: "hybrid-memory-ops", Description: "Memory recall", Status: "available"},
	})
	view := plainDialog(model.View())
	if !strings.Contains(view, "brainstorming") || !strings.Contains(view, "[always]") {
		t.Fatalf("expected skill list in view, got %q", view)
	}

	model, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'm', Text: "m"}))
	view = plainDialog(model.View())
	if !strings.Contains(view, "hybrid-memory-ops") || strings.Contains(view, "brainstorming") {
		t.Fatalf("expected filter to narrow results, got %q", view)
	}
}
