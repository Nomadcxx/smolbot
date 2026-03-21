package themes

import (
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

func init() {
	register("rama", [15]string{
		"#000000", "#000000", "#000000", "#3B3D52", "#F4A261",
		"#F4A261", "#2A9D8F", "#E76F51", "#F1FAEE", "#C9D6DF",
		"#E63946", "#E9C46A", "#8AB17D", "#4CC9F0", "#3B3D52",
	}, func(t *theme.Theme) {
		t.ToolName = lipgloss.Color("#F4A261")
		t.TranscriptUserAccent = lipgloss.Color("#F4A261")
		t.TranscriptAssistantAccent = lipgloss.Color("#2A9D8F")
		t.TranscriptThinking = lipgloss.Color("#C9D6DF")
		t.TranscriptStreaming = lipgloss.Color("#4CC9F0")
		t.TranscriptError = lipgloss.Color("#E63946")
		t.MarkdownHeading = lipgloss.Color("#F4A261")
		t.MarkdownLink = lipgloss.Color("#E76F51")
		t.MarkdownCode = lipgloss.Color("#2A9D8F")
		t.SyntaxKeyword = lipgloss.Color("#E76F51")
		t.SyntaxString = lipgloss.Color("#4CC9F0")
		t.SyntaxComment = lipgloss.Color("#8D99AE")
		t.ToolStateRunning = lipgloss.Color("#E9C46A")
		t.ToolStateDone = lipgloss.Color("#8AB17D")
		t.ToolStateError = lipgloss.Color("#E63946")
		t.ToolArtifactHeader = lipgloss.Color("#2C1D11")
		t.ToolArtifactBody = lipgloss.Color("#20222D")
	})
}
