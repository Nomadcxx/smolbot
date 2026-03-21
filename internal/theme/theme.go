package theme

import "image/color"

type Theme struct {
	Name        string
	Background  color.Color
	Panel       color.Color
	Surface     color.Color
	Element     color.Color
	Border      color.Color
	BorderFocus color.Color
	Primary     color.Color
	Secondary   color.Color
	Accent      color.Color
	Text        color.Color
	TextMuted   color.Color
	Subtle      color.Color
	Error       color.Color
	Warning     color.Color
	Success     color.Color
	Info        color.Color
	ToolBorder  color.Color
	ToolName    color.Color

	// Transcript colors
	TranscriptUserAccent      color.Color
	TranscriptAssistantAccent color.Color
	TranscriptThinking        color.Color
	TranscriptStreaming       color.Color
	TranscriptError           color.Color

	// Markdown colors
	MarkdownHeading color.Color
	MarkdownLink    color.Color
	MarkdownCode    color.Color

	// Syntax colors
	SyntaxKeyword color.Color
	SyntaxString  color.Color
	SyntaxComment color.Color

	// Tool state colors
	ToolStateRunning color.Color
	ToolStateDone    color.Color
	ToolStateError   color.Color

	// Tool artifact colors
	ToolArtifactBorder color.Color
	ToolArtifactHeader color.Color
	ToolArtifactBody   color.Color
}
