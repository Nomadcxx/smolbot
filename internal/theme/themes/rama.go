package themes

import (
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

func init() {
	register("rama", [15]string{
		"#2b2d42", "#2b2d42", "#2b2d42", "#8d99ae", "#ef233c",
		"#ef233c", "#8d99ae", "#d90429", "#edf2f4", "#8d99ae",
		"#d90429", "#ffd700", "#8d99ae", "#edf2f4", "#8d99ae",
	}, func(t *theme.Theme) {
		t.ToolName = lipgloss.Color("#ef233c")
		t.TranscriptUserAccent = lipgloss.Color("#ef233c")
		t.TranscriptAssistantAccent = lipgloss.Color("#8d99ae")
		t.TranscriptThinking = lipgloss.Color("#8d99ae")
		t.TranscriptStreaming = lipgloss.Color("#edf2f4")
		t.TranscriptError = lipgloss.Color("#d90429")
		t.MarkdownHeading = lipgloss.Color("#ef233c")
		t.MarkdownLink = lipgloss.Color("#d90429")
		t.MarkdownCode = lipgloss.Color("#edf2f4")
		t.SyntaxKeyword = lipgloss.Color("#ef233c")
		t.SyntaxString = lipgloss.Color("#edf2f4")
		t.SyntaxComment = lipgloss.Color("#8d99ae")
		t.ToolStateRunning = lipgloss.Color("#ffd700")
		t.ToolStateDone = lipgloss.Color("#8d99ae")
		t.ToolStateError = lipgloss.Color("#d90429")
		t.ToolArtifactHeader = lipgloss.Color("#1a1c2a")
		t.ToolArtifactBody = lipgloss.Color("#2b2d42")
	})
}
