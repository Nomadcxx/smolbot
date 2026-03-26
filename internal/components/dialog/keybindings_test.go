package dialog

import (
	"strings"
	"testing"

	"github.com/Nomadcxx/smolbot/internal/theme"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
)

func TestKeybindingsModelRendersCompactCommand(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}

	view := plainDialog(NewKeybindings().View())
	if !strings.Contains(view, "/compact") || !strings.Contains(view, "F1 / Ctrl+M") {
		t.Fatalf("expected keybindings content, got %q", view)
	}
}
