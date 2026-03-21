package chat

import (
	"strings"
	"testing"

	"github.com/Nomadcxx/smolbot/internal/theme"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
)

func TestMessagesModelRendersToolLifecycle(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be registered")
	}

	model := NewMessages()
	model.SetSize(80, 20)
	model.AppendUser("hello")
	model.StartTool("read_file", "")
	model.FinishTool("read_file", "done", "loaded config")

	view := model.View()
	if !strings.Contains(view, "read_file") {
		t.Fatalf("expected tool name in view, got %q", view)
	}
	if !strings.Contains(view, "loaded config") {
		t.Fatalf("expected tool output in view, got %q", view)
	}
}
