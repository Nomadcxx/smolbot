package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
)

func TestCommandsModelShowsEmptyState(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	model := NewCommands([]string{"/status", "/model", "/session"})
	model.SetFilter("zzz")

	view := model.View()
	if !strings.Contains(view, "No matches") {
		t.Fatalf("expected explicit empty state, got %q", view)
	}
	if !strings.Contains(view, "Esc close") {
		t.Fatalf("expected footer help row in empty state, got %q", view)
	}
}

func TestCommandsModelSupportsNavigationKeys(t *testing.T) {
	model := NewCommands([]string{"/status", "/model", "/session"})

	model, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	if got := model.Current(); got != "/model" {
		t.Fatalf("expected j to move selection to /model, got %q", got)
	}

	model, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'k', Text: "k"}))
	if got := model.Current(); got != "/status" {
		t.Fatalf("expected k to move selection back to /status, got %q", got)
	}

	model, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'n', Mod: tea.ModCtrl}))
	if got := model.Current(); got != "/model" {
		t.Fatalf("expected ctrl+n to move selection to /model, got %q", got)
	}

	model, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'p', Mod: tea.ModCtrl}))
	if got := model.Current(); got != "/status" {
		t.Fatalf("expected ctrl+p to move selection back to /status, got %q", got)
	}
}

func TestCommandsModelTabAndEnterChooseCurrentCommand(t *testing.T) {
	model := NewCommands([]string{"/status", "/model", "/session"})

	_, cmd := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyTab}))
	if cmd == nil {
		t.Fatal("expected tab to choose the current command")
	}
	msg := cmd()
	chosen, ok := msg.(CommandChosenMsg)
	if !ok {
		t.Fatalf("expected CommandChosenMsg from tab, got %T", msg)
	}
	if chosen.Command != "/status" {
		t.Fatalf("expected tab to choose /status, got %q", chosen.Command)
	}

	model, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	_, cmd = model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("expected enter to choose the current command")
	}
	msg = cmd()
	chosen, ok = msg.(CommandChosenMsg)
	if !ok {
		t.Fatalf("expected CommandChosenMsg from enter, got %T", msg)
	}
	if chosen.Command != "/model" {
		t.Fatalf("expected enter to choose /model, got %q", chosen.Command)
	}
}

func TestCommandPaletteFiltersFullQuery(t *testing.T) {
	model := NewCommands([]string{"/session", "/session new", "/session reset", "/status"})
	model.SetFilter("session r")

	if len(model.filtered) != 1 {
		t.Fatalf("expected a single match for full query, got %v", model.filtered)
	}
	if got := model.Current(); got != "/session reset" {
		t.Fatalf("expected /session reset to match, got %q", got)
	}
}

func TestCommandPaletteShowsOverflowCues(t *testing.T) {
	model := NewCommands([]string{
		"/session",
		"/session new",
		"/session reset",
		"/model",
		"/clear",
		"/status",
		"/help",
		"/quit",
		"/theme nord",
	})

	view := model.View()
	if !strings.Contains(view, "▼ more below") {
		t.Fatalf("expected lower overflow cue, got %q", view)
	}
	if !strings.Contains(view, "Enter/Tab run") {
		t.Fatalf("expected footer help row, got %q", view)
	}

	for i := 0; i < 7; i++ {
		model, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	}

	view = model.View()
	if !strings.Contains(view, "▲ more above") {
		t.Fatalf("expected upper overflow cue after scrolling, got %q", view)
	}
}
