package chat

import (
	"strings"
	"testing"

	"github.com/Nomadcxx/smolbot/internal/theme"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
)

func TestRenderToolCallIncludesStatusAndOutput(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be registered")
	}

	rendered := renderToolCall(ToolCall{
		Name:   "read_file",
		Status: "done",
		Output: "contents loaded",
	}, 80)

	if !strings.Contains(rendered, "read_file") {
		t.Fatalf("expected tool name in render, got %q", rendered)
	}
	if !strings.Contains(rendered, "contents loaded") {
		t.Fatalf("expected tool output in render, got %q", rendered)
	}
	if !strings.Contains(rendered, "✓") {
		t.Fatalf("expected done icon in render, got %q", rendered)
	}
}
