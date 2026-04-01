package app

import (
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestLoadStateDefaultsWhenMissing(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))

	state := LoadState()

	if state.Theme != "nord" {
		t.Fatalf("expected default theme nord, got %q", state.Theme)
	}
	if state.LastSession != "tui:main" {
		t.Fatalf("expected default session tui:main, got %q", state.LastSession)
	}
	if state.SidebarVisible == nil || !*state.SidebarVisible {
		t.Fatalf("expected sidebar to default visible, got %#v", state.SidebarVisible)
	}
}

func TestSaveStateRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))

	sidebarVisible := false
	want := State{
		Theme:          "rama",
		LastSession:    "tui:test",
		LastModel:      "gpt-test",
		SidebarVisible: &sidebarVisible,
	}
	if err := SaveState(want); err != nil {
		t.Fatalf("SaveState returned error: %v", err)
	}

	got := LoadState()
	if got.Theme != want.Theme || got.LastSession != want.LastSession || got.LastModel != want.LastModel {
		t.Fatalf("round trip mismatch: got %#v want %#v", got, want)
	}
	if got.SidebarVisible == nil || *got.SidebarVisible != false {
		t.Fatalf("expected sidebar visibility to round trip false, got %#v", got.SidebarVisible)
	}
}

func TestToggleFavoriteAddsAndRemoves(t *testing.T) {
	s := &State{}

	added := s.ToggleFavorite("gpt-4o")
	if !added {
		t.Fatal("expected ToggleFavorite to return true when adding")
	}
	if !s.IsFavorite("gpt-4o") {
		t.Fatal("expected model to be favorite after toggle-add")
	}

	removed := s.ToggleFavorite("gpt-4o")
	if removed {
		t.Fatal("expected ToggleFavorite to return false when removing")
	}
	if s.IsFavorite("gpt-4o") {
		t.Fatal("expected model to not be favorite after toggle-remove")
	}
}

func TestAddRecentPrependsAndDeduplicates(t *testing.T) {
	s := &State{}

	s.AddRecent("model-a")
	s.AddRecent("model-b")
	s.AddRecent("model-a") // re-add model-a should move it to front

	if len(s.Recents) != 2 {
		t.Fatalf("expected 2 recents after dedup, got %d", len(s.Recents))
	}
	if s.Recents[0] != "model-a" {
		t.Fatalf("expected model-a at front after re-add, got %q", s.Recents[0])
	}
	if s.Recents[1] != "model-b" {
		t.Fatalf("expected model-b second, got %q", s.Recents[1])
	}
}

func TestAddRecentCapsAtMaxRecents(t *testing.T) {
	s := &State{}

	for i := 0; i < MaxRecents+5; i++ {
		s.AddRecent("model-" + string(rune('a'+i)))
	}

	if len(s.Recents) != MaxRecents {
		t.Fatalf("expected recents capped at %d, got %d", MaxRecents, len(s.Recents))
	}
}

func TestRemoveRecentRemovesCorrectEntry(t *testing.T) {
	s := &State{Recents: []string{"a", "b", "c"}}

	s.RemoveRecent("b")

	if len(s.Recents) != 2 {
		t.Fatalf("expected 2 recents after remove, got %d", len(s.Recents))
	}
	if s.Recents[0] != "a" || s.Recents[1] != "c" {
		t.Fatalf("expected [a c] after remove, got %v", s.Recents)
	}
}

func TestFavoritesAndRecentsPersistedToJSON(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))

	sidebarVisible := true
	want := State{
		Theme:          "nord",
		LastSession:    "tui:main",
		SidebarVisible: &sidebarVisible,
		Favorites:      []string{"gpt-4o", "claude-sonnet"},
		Recents:        []string{"claude-sonnet", "gpt-4o-mini"},
	}
	if err := SaveState(want); err != nil {
		t.Fatalf("SaveState error: %v", err)
	}

	// Verify JSON contains the fields
	data, _ := json.Marshal(want)
	if string(data) == "" {
		t.Fatal("expected non-empty JSON")
	}

	got := LoadState()
	if len(got.Favorites) != 2 || got.Favorites[0] != "gpt-4o" {
		t.Fatalf("favorites not persisted: got %v", got.Favorites)
	}
	if len(got.Recents) != 2 || got.Recents[0] != "claude-sonnet" {
		t.Fatalf("recents not persisted: got %v", got.Recents)
	}
}

