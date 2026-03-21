package themes

import (
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

func init() {
	register("monochrome", [15]string{
		"#000000", "#000000", "#000000", "#333333", "#F5F5F5",
		"#F5F5F5", "#DDDDDD", "#B0B0B0", "#F5F5F5", "#777777",
		"#999999", "#A0A0A0", "#B8B8B8", "#8C8C8C", "#333333",
	}, func(t *theme.Theme) {
		t.ToolName = lipgloss.Color("#F5F5F5")
		t.TranscriptUserAccent = lipgloss.Color("#F5F5F5")
		t.TranscriptAssistantAccent = lipgloss.Color("#DDDDDD")
		t.TranscriptThinking = lipgloss.Color("#777777")
		t.TranscriptStreaming = lipgloss.Color("#8C8C8C")
		t.TranscriptError = lipgloss.Color("#999999")
		t.MarkdownHeading = lipgloss.Color("#F5F5F5")
		t.MarkdownLink = lipgloss.Color("#C7C7C7")
		t.MarkdownCode = lipgloss.Color("#B8B8B8")
		t.SyntaxKeyword = lipgloss.Color("#DDDDDD")
		t.SyntaxString = lipgloss.Color("#C7C7C7")
		t.SyntaxComment = lipgloss.Color("#777777")
		t.ToolStateRunning = lipgloss.Color("#A0A0A0")
		t.ToolStateDone = lipgloss.Color("#B8B8B8")
		t.ToolStateError = lipgloss.Color("#999999")
		t.ToolArtifactBorder = lipgloss.Color("#333333")
		t.ToolArtifactHeader = lipgloss.Color("#2C2C2C")
		t.ToolArtifactBody = lipgloss.Color("#1C1C1C")
	})
}
