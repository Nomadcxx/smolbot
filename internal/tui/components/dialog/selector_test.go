package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/Nomadcxx/nanobot-go/internal/tui/client"
	"github.com/Nomadcxx/nanobot-go/internal/tui/theme"
	_ "github.com/Nomadcxx/nanobot-go/internal/tui/theme/themes"
)

func TestSelectorParityUsesSharedNavigationKeys(t *testing.T) {
	_ = theme.Set("nord")

	commands := NewCommands([]string{"/help", "/status"})
	commands, _ = commands.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Mod: tea.ModCtrl}))
	if commands.cursor != 1 {
		t.Fatalf("commands ctrl+n cursor = %d, want 1", commands.cursor)
	}
	commands, _ = commands.Update(tea.KeyPressMsg(tea.Key{Code: 'p', Mod: tea.ModCtrl}))
	if commands.cursor != 0 {
		t.Fatalf("commands ctrl+p cursor = %d, want 0", commands.cursor)
	}

	models := NewModels([]client.ModelInfo{
		{ID: "a", Name: "Alpha"},
		{ID: "b", Name: "Beta"},
	}, "a")
	models, _ = models.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Mod: tea.ModCtrl}))
	if models.cursor != 1 {
		t.Fatalf("models ctrl+n cursor = %d, want 1", models.cursor)
	}
	models, _ = models.Update(tea.KeyPressMsg(tea.Key{Code: 'p', Mod: tea.ModCtrl}))
	if models.cursor != 0 {
		t.Fatalf("models ctrl+p cursor = %d, want 0", models.cursor)
	}

	sessions := NewSessions([]client.SessionInfo{
		{Key: "tui:main"},
		{Key: "tui:alt"},
	}, "tui:main")
	sessions, _ = sessions.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Mod: tea.ModCtrl}))
	if sessions.cursor != 1 {
		t.Fatalf("sessions ctrl+n cursor = %d, want 1", sessions.cursor)
	}
	sessions, _ = sessions.Update(tea.KeyPressMsg(tea.Key{Code: 'p', Mod: tea.ModCtrl}))
	if sessions.cursor != 0 {
		t.Fatalf("sessions ctrl+p cursor = %d, want 0", sessions.cursor)
	}
}

func TestSelectorParityCommandPaletteShowsDescriptions(t *testing.T) {
	_ = theme.Set("nord")

	commands := NewCommands([]string{"/status", "/help"})
	view := commands.View()
	if !strings.Contains(view, "Show runtime metadata") {
		t.Fatalf("expected command palette description, got %q", view)
	}
	if !strings.Contains(view, "Enter=run") {
		t.Fatalf("expected shared footer hints, got %q", view)
	}
}
