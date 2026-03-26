package themes_test

import (
	"fmt"
	"testing"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
)

func TestRAMAThemeCanonicalPalette(t *testing.T) {
	if !theme.Set("rama") {
		t.Fatal("expected to set rama theme")
	}

	current := theme.Current()
	if current == nil {
		t.Fatal("expected current theme")
	}

	want := map[string]string{
		"Background":                "#2b2d42",
		"Panel":                     "#2b2d42",
		"Element":                   "#2b2d42",
		"Border":                    "#8d99ae",
		"BorderFocus":               "#ef233c",
		"Primary":                   "#ef233c",
		"Secondary":                 "#8d99ae",
		"Accent":                    "#d90429",
		"Text":                      "#edf2f4",
		"TextMuted":                 "#8d99ae",
		"Error":                     "#d90429",
		"Warning":                   "#ffd700",
		"Success":                   "#8d99ae",
		"Info":                      "#edf2f4",
		"ToolBorder":                "#8d99ae",
		"ToolName":                  "#ef233c",
		"TranscriptUserAccent":      "#ef233c",
		"TranscriptAssistantAccent": "#8d99ae",
		"TranscriptThinking":        "#8d99ae",
		"TranscriptStreaming":       "#edf2f4",
		"TranscriptError":           "#d90429",
		"MarkdownHeading":           "#ef233c",
		"MarkdownLink":              "#d90429",
		"MarkdownCode":              "#edf2f4",
		"SyntaxKeyword":             "#ef233c",
		"SyntaxString":              "#edf2f4",
		"SyntaxComment":             "#8d99ae",
		"ToolStateRunning":          "#ffd700",
		"ToolStateDone":             "#8d99ae",
		"ToolStateError":            "#d90429",
		"ToolArtifactBorder":        "#8d99ae",
		"ToolArtifactHeader":        "#1a1c2a",
		"ToolArtifactBody":          "#2b2d42",
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
	}

	for field, wantValue := range want {
		if gotValue := got[field]; gotValue != fmt.Sprintf("%#v", lipgloss.Color(wantValue)) {
			t.Fatalf("unexpected %s: got %s want %s", field, gotValue, fmt.Sprintf("%#v", lipgloss.Color(wantValue)))
		}
	}
}
