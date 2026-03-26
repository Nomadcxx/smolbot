package theme_test

import (
	"fmt"
	"testing"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
)

func TestBuiltInThemesRegister(t *testing.T) {
	names := theme.List()
	if len(names) != 9 {
		t.Fatalf("expected 9 themes, got %d: %v", len(names), names)
	}
	if !theme.Set("nord") {
		t.Fatal("expected to set nord theme")
	}
	if current := theme.Current(); current == nil || current.Name != "nord" {
		t.Fatalf("unexpected current theme: %#v", current)
	}
}

func TestBuiltInThemesConfigureSurfaceColors(t *testing.T) {
	for _, name := range theme.List() {
		if !theme.Set(name) {
			t.Fatalf("expected to set theme %q", name)
		}
		current := theme.Current()
		if got := fmt.Sprintf("%#v", current.Background); got == fmt.Sprintf("%#v", lipgloss.NoColor{}) {
			t.Fatalf("expected %s background to be configured", name)
		}
		if got := fmt.Sprintf("%#v", current.Panel); got == fmt.Sprintf("%#v", lipgloss.NoColor{}) {
			t.Fatalf("expected %s panel to be configured", name)
		}
		if got := fmt.Sprintf("%#v", current.Element); got == fmt.Sprintf("%#v", lipgloss.NoColor{}) {
			t.Fatalf("expected %s element to be configured", name)
		}
	}
}
