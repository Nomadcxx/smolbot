package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestSlashPaletteUsesDialogOwnership(t *testing.T) {
	model := New(Config{})
	model.width = 80
	model.height = 24

	updated, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: '/', Text: "/"}))
	got := updated.(Model)

	if got.dialog == nil {
		t.Fatal("expected slash palette to open through the shared dialog host")
	}
	if !strings.Contains(got.View().Content, "Commands") {
		t.Fatalf("expected slash palette overlay to render commands dialog, got %q", got.View().Content)
	}
}

func TestDirectBootstrapUsesLocalConfigState(t *testing.T) {
	model := New(Config{
		Host:    "gateway.local",
		Port:    18888,
		Theme:   "dracula",
		Session: "cli:session",
	})

	if model.app.Config.Host != "gateway.local" {
		t.Fatalf("host = %q, want gateway.local", model.app.Config.Host)
	}
	if model.app.Config.Port != 18888 {
		t.Fatalf("port = %d, want 18888", model.app.Config.Port)
	}
	if model.app.Theme != "dracula" {
		t.Fatalf("theme = %q, want dracula", model.app.Theme)
	}
	if model.app.Session != "cli:session" {
		t.Fatalf("session = %q, want cli:session", model.app.Session)
	}
}

func TestStatusMetadataUpdatesStructuredState(t *testing.T) {
	model := New(Config{})
	model.client = &fakeClient{status: `{"model":"gpt-4o","provider":"openai","uptimeSeconds":42,"channels":["slack"],"connectedClients":3}`}
	session := model.app.Session

	_, cmd := model.handleSlashCommand("/status")
	if cmd == nil {
		t.Fatal("expected status command to return a loader")
	}

	msg := cmd()
	updated, _ := model.Update(msg)
	got := updated.(Model)

	if got.runtime.Model != "gpt-4o" {
		t.Fatalf("runtime model = %q, want gpt-4o", got.runtime.Model)
	}
	if got.runtime.Provider != "openai" {
		t.Fatalf("runtime provider = %q, want openai", got.runtime.Provider)
	}
	if got.runtime.ConnectedClients != 3 {
		t.Fatalf("runtime connected clients = %d, want 3", got.runtime.ConnectedClients)
	}
	if len(got.runtime.Channels) != 1 || got.runtime.Channels[0] != "slack" {
		t.Fatalf("runtime channels = %#v, want [slack]", got.runtime.Channels)
	}
	if strings.Contains(got.messages.View(), `"model":"gpt-4o"`) {
		t.Fatalf("expected status payload to stay out of transcript, got %q", got.messages.View())
	}
	footer := got.footer.View()
	for _, token := range []string{"gpt-4o", "openai", session, "42s", "clients 3", "slack"} {
		if !strings.Contains(footer, token) {
			t.Fatalf("expected footer to include %q, got %q", token, footer)
		}
	}
	statusView := got.status.View()
	if strings.Contains(statusView, "gpt-4o") || strings.Contains(statusView, "openai") || strings.Contains(statusView, "slack") {
		t.Fatalf("expected status bar to stay focused on connection/activity, got %q", statusView)
	}
}
