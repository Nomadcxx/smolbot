package app

import (
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
}

func TestSaveStateRoundTrip(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))

	want := State{
		Theme:       "rama",
		LastSession: "tui:test",
		LastModel:   "gpt-test",
	}
	if err := SaveState(want); err != nil {
		t.Fatalf("SaveState returned error: %v", err)
	}

	got := LoadState()
	if got != want {
		t.Fatalf("round trip mismatch: got %#v want %#v", got, want)
	}
}
