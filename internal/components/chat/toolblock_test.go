package chat

import (
	"strings"
	"testing"

	"github.com/Nomadcxx/smolbot/internal/theme"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
)

func TestRenderToolBlockUsesStateChromeAndSpinner(t *testing.T) {
	useTheme(t)

	current := theme.Current()
	if current == nil {
		t.Fatal("expected current theme")
	}

	running0 := RenderToolBlock(ToolBlockOpts{
		Title:        "read_file",
		Content:      "",
		State:        ToolBlockRunning,
		SpinnerFrame: 0,
		Width:        80,
	}, current)
	running1 := RenderToolBlock(ToolBlockOpts{
		Title:        "read_file",
		Content:      "",
		State:        ToolBlockRunning,
		SpinnerFrame: 1,
		Width:        80,
	}, current)
	done := RenderToolBlock(ToolBlockOpts{
		Title:   "read_file",
		Content: "config loaded",
		State:   ToolBlockDone,
		Width:   80,
	}, current)
	failed := RenderToolBlock(ToolBlockOpts{
		Title:   "read_file",
		Content: "permission denied",
		State:   ToolBlockError,
		Width:   80,
	}, current)

	if !strings.Contains(running0, "read_file") || !strings.Contains(running0, "running") {
		t.Fatalf("expected running block to include title and running text, got %q", running0)
	}
	if !strings.Contains(running0, "◐") || !strings.Contains(running1, "◓") {
		t.Fatalf("expected running spinner frames to change, got %q and %q", running0, running1)
	}
	if running0 == running1 {
		t.Fatal("expected different spinner frames to change the rendered block")
	}
	if !strings.Contains(done, "✓") || !strings.Contains(done, "config loaded") {
		t.Fatalf("expected done block to include success chrome and content, got %q", done)
	}
	if !strings.Contains(failed, "✗") || !strings.Contains(failed, "permission denied") {
		t.Fatalf("expected error block to include error chrome and content, got %q", failed)
	}
}

func useTheme(t *testing.T) {
	t.Helper()

	previous := theme.Current()
	previousName := ""
	if previous != nil {
		previousName = previous.Name
	}

	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be registered")
	}

	t.Cleanup(func() {
		if previousName != "" {
			theme.Set(previousName)
		}
	})
}
