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
