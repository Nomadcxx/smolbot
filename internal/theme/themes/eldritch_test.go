package themes_test

import (
	"fmt"
	"testing"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
)

func TestEldritchThemeCanonicalPalette(t *testing.T) {
	if !theme.Set("eldritch") {
		t.Fatal("expected to set eldritch theme")
	}

	current := theme.Current()
	if current == nil {
		t.Fatal("expected current theme")
	}

	want := map[string]string{
		"Background":                "#212337",
		"Panel":                     "#212337",
		"Element":                   "#323449",
		"Border":                    "#3b4261",
		"BorderFocus":               "#37f499",
		"Primary":                   "#37f499",
		"Secondary":                 "#04d1f9",
		"Accent":                    "#a48cf2",
		"Text":                      "#ebfafa",
		"TextMuted":                 "#7081d0",
		"Error":                     "#f16c75",
		"Warning":                   "#f1fc79",
		"Success":                   "#37f499",
		"Info":                      "#04d1f9",
		"ToolBorder":                "#3b4261",
		"ToolName":                  "#37f499",
		"TranscriptUserAccent":      "#37f499",
		"TranscriptAssistantAccent": "#04d1f9",
		"TranscriptThinking":        "#7081d0",
		"TranscriptStreaming":       "#ebfafa",
		"TranscriptError":           "#f16c75",
		"MarkdownHeading":           "#37f499",
		"MarkdownLink":              "#04d1f9",
		"MarkdownCode":              "#a48cf2",
		"SyntaxKeyword":             "#a48cf2",
		"SyntaxString":              "#37f499",
		"SyntaxComment":             "#7081d0",
		"ToolStateRunning":          "#f1fc79",
		"ToolStateDone":             "#37f499",
		"ToolStateError":            "#f16c75",
		"ToolArtifactBorder":        "#3b4261",
		"ToolArtifactHeader":        "#1a1b2e",
		"ToolArtifactBody":          "#212337",
		"DiffAdded":                 "#37f499",
		"DiffRemoved":               "#f16c75",
		"DiffAddedBg":               "#1a2b1a",
		"DiffRemovedBg":             "#2b1a1a",
	}

	got := map[string]string{
		"Background":                fmt.Sprintf("%#v", current.Background),
		"Panel":                     fmt.Sprintf("%#v", current.Panel),
		"Element":                   fmt.Sprintf("%#v", current.Element),
		"Border":                    fmt.Sprintf("%#v", current.Border),
		"BorderFocus":               fmt.Sprintf("%#v", current.BorderFocus),
		"Primary":                   fmt.Sprintf("%#v", current.Primary),
		"Secondary":                 fmt.Sprintf("%#v", current.Secondary),
		"Accent":                    fmt.Sprintf("%#v", current.Accent),
		"Text":                      fmt.Sprintf("%#v", current.Text),
		"TextMuted":                 fmt.Sprintf("%#v", current.TextMuted),
		"Error":                     fmt.Sprintf("%#v", current.Error),
		"Warning":                   fmt.Sprintf("%#v", current.Warning),
		"Success":                   fmt.Sprintf("%#v", current.Success),
		"Info":                      fmt.Sprintf("%#v", current.Info),
		"ToolBorder":                fmt.Sprintf("%#v", current.ToolBorder),
		"ToolName":                  fmt.Sprintf("%#v", current.ToolName),
		"TranscriptUserAccent":      fmt.Sprintf("%#v", current.TranscriptUserAccent),
		"TranscriptAssistantAccent": fmt.Sprintf("%#v", current.TranscriptAssistantAccent),
		"TranscriptThinking":        fmt.Sprintf("%#v", current.TranscriptThinking),
		"TranscriptStreaming":       fmt.Sprintf("%#v", current.TranscriptStreaming),
		"TranscriptError":           fmt.Sprintf("%#v", current.TranscriptError),
		"MarkdownHeading":           fmt.Sprintf("%#v", current.MarkdownHeading),
		"MarkdownLink":              fmt.Sprintf("%#v", current.MarkdownLink),
		"MarkdownCode":              fmt.Sprintf("%#v", current.MarkdownCode),
		"SyntaxKeyword":             fmt.Sprintf("%#v", current.SyntaxKeyword),
		"SyntaxString":              fmt.Sprintf("%#v", current.SyntaxString),
		"SyntaxComment":             fmt.Sprintf("%#v", current.SyntaxComment),
		"ToolStateRunning":          fmt.Sprintf("%#v", current.ToolStateRunning),
		"ToolStateDone":             fmt.Sprintf("%#v", current.ToolStateDone),
		"ToolStateError":            fmt.Sprintf("%#v", current.ToolStateError),
		"ToolArtifactBorder":        fmt.Sprintf("%#v", current.ToolArtifactBorder),
		"ToolArtifactHeader":        fmt.Sprintf("%#v", current.ToolArtifactHeader),
		"ToolArtifactBody":          fmt.Sprintf("%#v", current.ToolArtifactBody),
		"DiffAdded":                 fmt.Sprintf("%#v", current.DiffAdded),
		"DiffRemoved":               fmt.Sprintf("%#v", current.DiffRemoved),
		"DiffAddedBg":               fmt.Sprintf("%#v", current.DiffAddedBg),
		"DiffRemovedBg":             fmt.Sprintf("%#v", current.DiffRemovedBg),
	}

	for field, wantValue := range want {
		if gotValue := got[field]; gotValue != fmt.Sprintf("%#v", lipgloss.Color(wantValue)) {
			t.Errorf("unexpected %s: got %s want %s", field, gotValue, fmt.Sprintf("%#v", lipgloss.Color(wantValue)))
		}
	}
}

func TestEldritchThemeSidebarBgDistinctFromBackground(t *testing.T) {
	if !theme.Set("eldritch") {
		t.Fatal("expected to set eldritch theme")
	}
	current := theme.Current()
	if current == nil {
		t.Fatal("expected current theme")
	}
	bg := fmt.Sprintf("%#v", current.Background)
	sidebarBg := fmt.Sprintf("%#v", current.SidebarBg)
	if bg == sidebarBg {
		t.Errorf("eldritch: Background (%s) should differ from SidebarBg (%s) for sidebar contrast", bg, sidebarBg)
	}
}
