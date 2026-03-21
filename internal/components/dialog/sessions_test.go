package dialog

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/internal/theme"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
)

func plainDialog(text string) string {
	text = strings.ReplaceAll(text, "\x1b[m", "")
	for {
		start := strings.Index(text, "\x1b[")
		if start == -1 {
			return text
		}
		end := strings.Index(text[start:], "m")
		if end == -1 {
			return text[:start]
		}
		text = text[:start] + text[start+end+1:]
	}
}

func TestSessionsModelShowsCurrentMarkerAndFooterHelp(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be available")
	}

	model := NewSessions([]client.SessionInfo{
		{Key: "tui:main"},
		{Key: "tui:alt"},
	}, "tui:main")

	view := plainDialog(model.View())
	if !strings.Contains(view, "current") {
		t.Fatalf("expected current marker in session dialog, got %q", view)
	}
	if !strings.Contains(view, "Enter switch") {
		t.Fatalf("expected footer help row, got %q", view)
	}
	if !strings.Contains(view, "Esc close") {
		t.Fatalf("expected close help treatment in footer, got %q", view)
	}
}

func TestSessionsModelShowsOverflowCues(t *testing.T) {
	sessions := make([]client.SessionInfo, 0, 10)
	for i := 0; i < 10; i++ {
		sessions = append(sessions, client.SessionInfo{Key: "tui:session-" + string(rune('a'+i))})
	}

	model := NewSessions(sessions, "tui:session-a")

	view := plainDialog(model.View())
	if !strings.Contains(view, "▼ more below") {
		t.Fatalf("expected lower overflow cue, got %q", view)
	}

	for i := 0; i < 7; i++ {
		model, _ = model.Update(tea.KeyPressMsg(tea.Key{Code: 'j', Text: "j"}))
	}

	view = plainDialog(model.View())
	if !strings.Contains(view, "▲ more above") {
		t.Fatalf("expected upper overflow cue after scrolling, got %q", view)
	}
}
