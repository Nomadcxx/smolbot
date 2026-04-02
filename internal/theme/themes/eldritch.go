package themes

import (
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

func init() {
	// Eldritch — Lovecraftian horror theme
	// Source palette: https://github.com/eldritch-theme/eldritch
	register("eldritch", [15]string{
		"#212337", // [0]  Background  - Sunken Depths Grey
		"#323449", // [1]  Panel       - Shallow Depths Grey (elevated)
		"#323449", // [2]  Element     - same as panel
		"#3b4261", // [3]  Border      - subtle separator
		"#37f499", // [4]  BorderFocus - Great Old One Green
		"#37f499", // [5]  Primary     - Great Old One Green
		"#04d1f9", // [6]  Secondary   - Watery Tomb Blue
		"#a48cf2", // [7]  Accent      - Lovecraft Purple
		"#ebfafa", // [8]  Text        - Lighthouse White
		"#7081d0", // [9]  TextMuted   - The Old One Purple
		"#f16c75", // [10] Error       - R'lyeh Red
		"#f1fc79", // [11] Warning     - Gold of Yuggoth
		"#37f499", // [12] Success     - Great Old One Green
		"#04d1f9", // [13] Info        - Watery Tomb Blue
		"#3b4261", // [14] ToolBorder  - same as border
	}, func(t *theme.Theme) {
		t.ToolName = lipgloss.Color("#37f499")
		t.TranscriptUserAccent = lipgloss.Color("#37f499")
		t.TranscriptAssistantAccent = lipgloss.Color("#04d1f9")
		t.TranscriptThinking = lipgloss.Color("#7081d0")
		t.TranscriptStreaming = lipgloss.Color("#ebfafa")
		t.TranscriptError = lipgloss.Color("#f16c75")
		t.MarkdownHeading = lipgloss.Color("#37f499")
		t.MarkdownLink = lipgloss.Color("#04d1f9")
		t.MarkdownCode = lipgloss.Color("#a48cf2")
		t.SyntaxKeyword = lipgloss.Color("#a48cf2")
		t.SyntaxString = lipgloss.Color("#37f499")
		t.SyntaxComment = lipgloss.Color("#7081d0")
		t.ToolStateRunning = lipgloss.Color("#f1fc79")
		t.ToolStateDone = lipgloss.Color("#37f499")
		t.ToolStateError = lipgloss.Color("#f16c75")
		t.DiffAdded = lipgloss.Color("#37f499")
		t.DiffRemoved = lipgloss.Color("#f16c75")
		t.DiffAddedBg = lipgloss.Color("#1a2b1a")
		t.DiffRemovedBg = lipgloss.Color("#2b1a1a")
		t.ToolArtifactHeader = lipgloss.Color("#1a1b2e")
		t.ToolArtifactBody = lipgloss.Color("#212337")
		t.AgentBlue = lipgloss.Color("#04d1f9")
		t.AgentGreen = lipgloss.Color("#37f499")
		t.AgentPurple = lipgloss.Color("#a48cf2")
		t.AgentOrange = lipgloss.Color("#f7c67f")
		t.AgentPink = lipgloss.Color("#f265b5")
		t.AgentCyan = lipgloss.Color("#04d1f9")
		t.AgentYellow = lipgloss.Color("#f1fc79")
		t.AgentRed = lipgloss.Color("#f16c75")
	})
}
