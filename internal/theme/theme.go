package theme

import "image/color"

type Theme struct {
	Name        string
	Background  color.Color
	Panel       color.Color
	Element     color.Color
	Border      color.Color
	BorderFocus color.Color
	Primary     color.Color
	Secondary   color.Color
	Accent      color.Color
	Text        color.Color
	TextMuted   color.Color
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

	// Compression indicator colors (inspired by nanocoder)
	CompressionActive  color.Color // Light compression (20-40%)
	CompressionSuccess color.Color // Moderate compression (40-60%)
	CompressionWarning color.Color // High compression (>60%)
	TokenHighUsage     color.Color // Token usage critical (>90%)
}
