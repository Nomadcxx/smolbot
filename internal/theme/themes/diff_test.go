package themes

import (
	"fmt"
	"image/color"
	"strconv"
	"testing"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

func TestRegisterDerivesDiffColorsFromThemeOverrides(t *testing.T) {
	previousName := ""
	if current := theme.Current(); current != nil {
		previousName = current.Name
	}
	if !theme.Set("rama") {
		t.Fatal("expected rama theme to be registered")
	}

	const name = "diff-defaults-test"
	register(name, [15]string{
		"#101010", "#202020", "#303030", "#404040", "#505050",
		"#606060", "#707070", "#808080", "#909090", "#A0A0A0",
		"#B0B0B0", "#C0C0C0", "#D0D0D0", "#E0E0E0", "#F0F0F0",
	}, func(t *theme.Theme) {
		t.Success = lipgloss.Color("#11AA33")
		t.Error = lipgloss.Color("#CC2244")
		t.Panel = lipgloss.Color("#222244")
		t.TextMuted = lipgloss.Color("#778899")
	})

	if !theme.Set(name) {
		t.Fatalf("expected to set test theme %q", name)
	}
	t.Cleanup(func() {
		if previousName != "" {
			theme.Set(previousName)
		}
	})

	current := theme.Current()
	if current == nil {
		t.Fatal("expected current theme")
	}

	assertColor(t, "DiffAdded", current.DiffAdded, "#11AA33")
	assertColor(t, "DiffRemoved", current.DiffRemoved, "#CC2244")
	assertColor(t, "DiffContext", current.DiffContext, "#778899")
	assertColor(t, "DiffContextBg", current.DiffContextBg, "#222244")
	assertColor(t, "DiffHighlightAdded", current.DiffHighlightAdded, "#11AA33")
	assertColor(t, "DiffHighlightRemoved", current.DiffHighlightRemoved, "#CC2244")
	assertColor(t, "DiffLineNumber", current.DiffLineNumber, "#778899")
	assertColor(t, "DiffAddedBg", current.DiffAddedBg, darkenHexForTest("#11AA33", 0.85))
	assertColor(t, "DiffRemovedBg", current.DiffRemovedBg, darkenHexForTest("#CC2244", 0.85))
}

func TestBuiltinThemesExposeDiffOverrides(t *testing.T) {
	cases := []struct {
		name string
		want map[string]string
	}{
		{
			name: "dracula",
			want: map[string]string{
				"DiffAdded":     "#50FA7B",
				"DiffRemoved":   "#FF5555",
				"DiffAddedBg":   "#1a3a1a",
				"DiffRemovedBg": "#3a1a1a",
			},
		},
		{
			name: "nord",
			want: map[string]string{
				"DiffAdded":     "#a3be8c",
				"DiffRemoved":   "#bf616a",
				"DiffAddedBg":   "#1a2a1a",
				"DiffRemovedBg": "#2a1a1a",
			},
		},
		{
			name: "rama",
			want: map[string]string{
				"DiffAdded":     "#edf2f4",
				"DiffRemoved":   "#ef233c",
				"DiffAddedBg":   "#1a2b1a",
				"DiffRemovedBg": "#2b1a1a",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			previousName := ""
			if current := theme.Current(); current != nil {
				previousName = current.Name
			}
			if !theme.Set(tc.name) {
				t.Fatalf("expected to set theme %q", tc.name)
			}
			t.Cleanup(func() {
				if previousName != "" {
					theme.Set(previousName)
				}
			})
			current := theme.Current()
			if current == nil {
				t.Fatal("expected current theme")
			}
			assertColor(t, "DiffAdded", current.DiffAdded, tc.want["DiffAdded"])
			assertColor(t, "DiffRemoved", current.DiffRemoved, tc.want["DiffRemoved"])
			assertColor(t, "DiffAddedBg", current.DiffAddedBg, tc.want["DiffAddedBg"])
			assertColor(t, "DiffRemovedBg", current.DiffRemovedBg, tc.want["DiffRemovedBg"])
		})
	}
}

func assertColor(t *testing.T, field string, got color.Color, want string) {
	t.Helper()
	if got == nil {
		t.Fatalf("expected %s to be set", field)
	}
	wantColor := lipgloss.Color(want)
	if fmt.Sprintf("%#v", got) != fmt.Sprintf("%#v", wantColor) {
		t.Fatalf("unexpected %s: got %s want %s", field, fmt.Sprintf("%#v", got), fmt.Sprintf("%#v", wantColor))
	}
}

func darkenHexForTest(hex string, factor float64) string {
	if len(hex) != 7 || hex[0] != '#' {
		return "#000000"
	}
	r, _ := strconv.ParseUint(hex[1:3], 16, 8)
	g, _ := strconv.ParseUint(hex[3:5], 16, 8)
	b, _ := strconv.ParseUint(hex[5:7], 16, 8)
	return fmt.Sprintf("#%02X%02X%02X",
		uint8(float64(r)*factor+0.5),
		uint8(float64(g)*factor+0.5),
		uint8(float64(b)*factor+0.5),
	)
}
