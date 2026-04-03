package themes

import (
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

func init() {
	register("tokyo_night", [15]string{
		"#000000", "#000000", "#0a0a0a", "#24283B", "#7AA2F7",
		"#7AA2F7", "#BB9AF7", "#F7768E", "#C0CAF5", "#565F89",
		"#F7768E", "#E0AF68", "#9ECE6A", "#7DCFFF", "#24283B",
	}, func(t *theme.Theme) {
		t.SidebarBg = lipgloss.Color("#111111")
		t.ToolName = lipgloss.Color("#7AA2F7")
		t.TranscriptUserAccent = lipgloss.Color("#7AA2F7")
		t.TranscriptAssistantAccent = lipgloss.Color("#BB9AF7")
		t.TranscriptThinking = lipgloss.Color("#565F89")
		t.TranscriptStreaming = lipgloss.Color("#7DCFFF")
		t.TranscriptError = lipgloss.Color("#F7768E")
		t.MarkdownHeading = lipgloss.Color("#7AA2F7")
		t.MarkdownLink = lipgloss.Color("#7DCFFF")
		t.MarkdownCode = lipgloss.Color("#BB9AF7")
		t.SyntaxKeyword = lipgloss.Color("#7AA2F7")
		t.SyntaxString = lipgloss.Color("#9ECE6A")
		t.SyntaxComment = lipgloss.Color("#565F89")
		t.ToolStateRunning = lipgloss.Color("#E0AF68")
		t.ToolStateDone = lipgloss.Color("#9ECE6A")
		t.ToolStateError = lipgloss.Color("#F7768E")
		t.ToolArtifactHeader = lipgloss.Color("#221C2C")
		t.ToolArtifactBody = lipgloss.Color("#141620")
	})
}
