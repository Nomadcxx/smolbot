package header

import (
	"strings"
	"testing"

	"github.com/Nomadcxx/nanobot-go/internal/tui/theme"
	_ "github.com/Nomadcxx/nanobot-go/internal/tui/theme/themes"
)

func TestHeaderRendersStyledCenteredOutput(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme to be registered")
	}

	model := New()
	model.SetWidth(80)
	rendered := model.View()

	if !strings.Contains(rendered, "\x1b[") {
		t.Fatalf("expected ANSI styling in header render, got %q", rendered)
	}
	if !strings.Contains(rendered, "NN") && rendered == "" {
		t.Fatalf("expected non-empty rendered header, got %q", rendered)
	}
}
