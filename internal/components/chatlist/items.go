package chatlist

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"time"

	"charm.land/glamour/v2/ansi"
	glamour "charm.land/glamour/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

const maxTextWidth = 120
const maxToolOutputLines = 10

func cappedWidth(available int) int {
	if available <= 0 {
		return maxTextWidth
	}
	w := available - 2
	if w > maxTextWidth {
		return maxTextWidth
	}
	return max(20, w)
}

type UserItem struct {
	Content string
	cached  string
	width   int
}

func (t *UserItem) Render(width int) string {
	if t.width == width && t.cached != "" {
		return t.cached
	}
	t.cached = renderRoleBlock("USER", t.Content, theme.Current().Primary, width)
	t.width = width
	return t.cached
}

type AssistantItem struct {
	Content  string
	cached   string
	width    int
	renderer *glamour.TermRenderer
}

func (t *AssistantItem) Invalidate() {
	t.cached = ""
}

func (t *AssistantItem) Render(width int) string {
	if t.width == width && t.cached != "" {
		return t.cached
	}
	md := t.renderMarkdown(width)
	t.cached = renderRoleBlock("ASSISTANT", md, theme.Current().Secondary, width)
	t.width = width
	return t.cached
}

func (t *AssistantItem) renderMarkdown(width int) string {
	r := t.markdownRenderer(width)
	if r == nil || strings.TrimSpace(t.Content) == "" {
		return t.Content
	}
	rendered, err := r.Render(t.Content)
	if err != nil {
		return t.Content
	}
	return strings.TrimSpace(rendered)
}

func (t *AssistantItem) markdownRenderer(width int) *glamour.TermRenderer {
	w := max(20, width-2)
	if t.renderer != nil && t.width == width {
		return t.renderer
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(markdownStyleConfig()),
		glamour.WithWordWrap(w),
		glamour.WithPreservedNewLines(),
		glamour.WithChromaFormatter("terminal16m"),
	)
	if err != nil {
		return nil
	}
	t.renderer = renderer
	return renderer
}

type ThinkingItem struct {
	Content  string
	Duration time.Duration
	cached   string
	width    int
}

func (t *ThinkingItem) Render(width int) string {
	if t.width == width && t.cached != "" {
		return t.cached
	}
	t.cached = renderThinkingBlock(t.Content, t.Duration, theme.Current().TranscriptThinking, width)
	t.width = width
	return t.cached
}

type EphemeralItem struct {
	Label   string
	Content string
}

func (t *EphemeralItem) Render(width int) string {
	return renderRoleBlock(t.Label, t.Content, theme.Current().TranscriptThinking, width)
}

type ToolItem struct {
	ID     string
	Name   string
	Input  string
	Status string
	Output string
	cached string
	width  int
}

func (t *ToolItem) Render(width int) string {
	if t.width == width && t.cached != "" {
		return t.cached
	}
	t.cached = renderToolCall(ToolCall{t.ID, t.Name, t.Input, t.Status, t.Output}, width, false)
	t.width = width
	return t.cached
}

type ToolCall struct {
	ID, Name, Input, Status, Output string
}

type ErrorItem struct {
	Content string
	cached  string
	width   int
}

func (t *ErrorItem) Render(width int) string {
	if t.width == width && t.cached != "" {
		return t.cached
	}
	t.cached = renderMessageBlock("ERROR", t.Content, theme.Current().Error, width)
	t.width = width
	return t.cached
}

func renderToolCall(tc ToolCall, width int, expanded bool) string {
	t := theme.Current()
	if t == nil {
		return tc.Name
	}

	icon, iconColor := toolIcon(tc.Status, t)
	iconStr := lipgloss.NewStyle().Foreground(iconColor).Bold(true).Render(icon)
	nameStr := lipgloss.NewStyle().Foreground(t.ToolName).Bold(true).Render(tc.Name)

	// Format: ● Name(input)
	inputSummary := tc.Input
	if len(inputSummary) > 80 {
		inputSummary = inputSummary[:77] + "..."
	}
	header := iconStr + " " + nameStr
	if strings.TrimSpace(inputSummary) != "" {
		header += lipgloss.NewStyle().Foreground(t.TextMuted).Render("(" + inputSummary + ")")
	}

	bodyText := tc.Output
	if strings.TrimSpace(bodyText) == "" {
		if tc.Status == "running" {
			return header
		}
		return header
	}

	truncHint := ""
	if !expanded {
		outputLines := strings.Split(bodyText, "\n")
		if len(outputLines) > maxToolOutputLines {
			hidden := len(outputLines) - maxToolOutputLines
			bodyText = strings.Join(outputLines[:maxToolOutputLines], "\n")
			truncHint = fmt.Sprintf("… +%d lines", hidden)
		}
	}

	bodyStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	indent := "  ⎿  "
	var lines []string
	lines = append(lines, header)

	for _, line := range strings.Split(bodyText, "\n") {
		lines = append(lines, bodyStyle.Render(indent+line))
	}
	if truncHint != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true).Render(indent+truncHint))
	}
	return strings.Join(lines, "\n")
}

func toolIcon(status string, t *theme.Theme) (string, color.Color) {
	switch status {
	case "running":
		return "●", t.ToolStateRunning
	case "done":
		return "✓", t.ToolStateDone
	case "error":
		return "✗", t.ToolStateError
	default:
		return "•", t.TextMuted
	}
}

func renderRoleBlock(label, body string, accent color.Color, width int) string {
	t := theme.Current()
	if t == nil {
		return label + "\n" + body
	}
	if semanticAccent := transcriptRoleAccent(label, t); semanticAccent != nil {
		accent = semanticAccent
	}
	innerWidth := cappedWidth(width)
	badge := lipgloss.NewStyle().
		Background(accent).
		Foreground(t.Background).
		Bold(true).
		Padding(0, 1).
		Render(label)

	header := lipgloss.NewStyle().
		Background(subtleWash(accent)).
		Width(innerWidth).
		Padding(0, 1).
		Render(badge)
	contentBody := lipgloss.NewStyle().
		Foreground(t.Text).
		Width(innerWidth).
		Padding(0, 1).
		Render(body)
	content := lipgloss.JoinVertical(lipgloss.Left, header, contentBody)
	style := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(accent).
		Padding(0, 0)
	if width > 4 {
		style = style.Width(width - 2)
	}
	return style.Render(content)
}

func renderThinkingBlock(body string, dur time.Duration, accent color.Color, width int) string {
	t := theme.Current()
	if t == nil {
		return "THINKING\n" + body
	}
	innerWidth := cappedWidth(width)
	badge := lipgloss.NewStyle().
		Background(accent).
		Foreground(t.Background).
		Bold(true).
		Padding(0, 1).
		Render("THINKING")

	header := lipgloss.NewStyle().
		Background(subtleWash(accent)).
		Width(innerWidth).
		Padding(0, 1).
		Render(badge)

	bodyLines := strings.Split(body, "\n")
	truncHint := ""
	if len(bodyLines) > maxToolOutputLines {
		hidden := len(bodyLines) - maxToolOutputLines
		body = strings.Join(bodyLines[:maxToolOutputLines], "\n")
		truncHint = fmt.Sprintf("… (%d lines hidden)", hidden)
	}

	contentBody := lipgloss.NewStyle().
		Foreground(t.Text).
		Width(innerWidth).
		Padding(0, 1).
		Render(body)

	var rows []string
	rows = append(rows, header, contentBody)

	if truncHint != "" {
		rows = append(rows, lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Italic(true).
			Width(innerWidth).
			Padding(0, 1).
			Render(truncHint))
	}

	if dur > 0 {
		footer := lipgloss.NewStyle().
			Background(subtleWash(accent)).
			Foreground(t.TextMuted).
			Width(innerWidth).
			Padding(0, 1).
			Render("Thought for "+formatDuration(dur))
		rows = append(rows, footer)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	style := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(accent).
		Padding(0, 0)
	if width > 4 {
		style = style.Width(width - 2)
	}
	return style.Render(content)
}

func renderMessageBlock(label, body string, accent color.Color, width int) string {
	t := theme.Current()
	if t == nil {
		return label + "\n" + body
	}
	head := lipgloss.NewStyle().
		Foreground(accent).
		Bold(true).
		Render(label)
	content := lipgloss.JoinVertical(lipgloss.Left, head, body)
	style := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(accent).
		Padding(0, 1)
	style = style.Width(cappedWidth(width))
	return style.Render(content)
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func transcriptRoleAccent(label string, t *theme.Theme) color.Color {
	switch label {
	case "USER":
		return t.TranscriptUserAccent
	case "ASSISTANT":
		return t.TranscriptAssistantAccent
	case "THINKING":
		return t.TranscriptThinking
	default:
		return nil
	}
}

func subtleWash(accent color.Color) color.Color {
	hex := colorHex(accent)
	if len(hex) != 7 || hex[0] != '#' {
		return lipgloss.Color("#111111")
	}
	r, _ := strconv.ParseInt(hex[1:3], 16, 64)
	g, _ := strconv.ParseInt(hex[3:5], 16, 64)
	b, _ := strconv.ParseInt(hex[5:7], 16, 64)
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", int(r)/5, int(g)/5, int(b)/5))
}

func colorHex(value color.Color) string {
	r, g, b, _ := value.RGBA()
	return fmt.Sprintf("#%02X%02X%02X", uint8(r>>8), uint8(g>>8), uint8(b>>8))
}

func markdownStyleConfig() ansi.StyleConfig {
	current := theme.Current()
	if current == nil {
		return ansi.StyleConfig{}
	}
	background := colorPtr(colorHex(current.Background))
	text := colorPtr(colorHex(current.Text))
	muted := colorPtr(colorHex(current.TextMuted))
	heading := colorPtr(colorHex(current.MarkdownHeading))
	link := colorPtr(colorHex(current.MarkdownLink))
	code := colorPtr(colorHex(current.MarkdownCode))
	keyword := colorPtr(colorHex(current.SyntaxKeyword))
	stringColor := colorPtr(colorHex(current.SyntaxString))
	comment := colorPtr(colorHex(current.SyntaxComment))

	return ansi.StyleConfig{
		Document: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:           text,
				BackgroundColor: background,
			},
		},
		BlockQuote: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:  muted,
				Italic: boolPtr(true),
				Prefix: "┃ ",
			},
			Indent:      uintPtr(1),
			IndentToken: stringPtr(" "),
		},
		List: ansi.StyleList{
			LevelIndent: 2,
			StyleBlock: ansi.StyleBlock{
				IndentToken: stringPtr(" "),
				StylePrimitive: ansi.StylePrimitive{
					Color: text,
				},
			},
		},
		Heading: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color: heading,
				Bold:  boolPtr(true),
			},
		},
		H1: headingBlock("# ", heading),
		H2: headingBlock("## ", heading),
		H3: headingBlock("### ", heading),
		H4: headingBlock("#### ", heading),
		H5: headingBlock("##### ", heading),
		H6: headingBlock("###### ", heading),
		Emph: ansi.StylePrimitive{
			Color:  heading,
			Italic: boolPtr(true),
		},
		Strong: ansi.StylePrimitive{
			Color: text,
			Bold:  boolPtr(true),
		},
		HorizontalRule: ansi.StylePrimitive{
			Color:  muted,
			Format: "\n─────────────────────────────────────────\n",
		},
		Item: ansi.StylePrimitive{
			BlockPrefix: "• ",
			Color:       link,
		},
		Enumeration: ansi.StylePrimitive{
			BlockPrefix: ". ",
			Color:       link,
		},
		Task: ansi.StyleTask{
			Ticked:   "[✓] ",
			Unticked: "[ ] ",
		},
		Link: ansi.StylePrimitive{
			Color:     link,
			Underline: boolPtr(true),
		},
		LinkText: ansi.StylePrimitive{
			Color: link,
			Bold:  boolPtr(true),
		},
		Code: ansi.StyleBlock{
			StylePrimitive: ansi.StylePrimitive{
				Color:           code,
				BackgroundColor: background,
			},
		},
		CodeBlock: ansi.StyleCodeBlock{
			StyleBlock: ansi.StyleBlock{
				StylePrimitive: ansi.StylePrimitive{
					Color:           code,
					BackgroundColor: background,
					Prefix:          " ",
				},
			},
			Chroma: &ansi.Chroma{
				Background: ansi.StylePrimitive{BackgroundColor: background},
				Text:       ansi.StylePrimitive{BackgroundColor: background, Color: text},
				Error:      ansi.StylePrimitive{BackgroundColor: background, Color: colorPtr(colorHex(current.TranscriptError))},
				Comment:    ansi.StylePrimitive{BackgroundColor: background, Color: comment},
				Keyword:    ansi.StylePrimitive{BackgroundColor: background, Color: keyword},
				KeywordReserved:  ansi.StylePrimitive{BackgroundColor: background, Color: keyword},
				KeywordNamespace: ansi.StylePrimitive{BackgroundColor: background, Color: keyword},
				KeywordType:      ansi.StylePrimitive{BackgroundColor: background, Color: keyword},
				Name:             ansi.StylePrimitive{BackgroundColor: background, Color: text},
				NameBuiltin:      ansi.StylePrimitive{BackgroundColor: background, Color: link},
				NameFunction:     ansi.StylePrimitive{BackgroundColor: background, Color: link},
				LiteralString:    ansi.StylePrimitive{BackgroundColor: background, Color: stringColor},
				LiteralNumber:    ansi.StylePrimitive{BackgroundColor: background, Color: stringColor},
				Operator:         ansi.StylePrimitive{BackgroundColor: background, Color: text},
				Punctuation:      ansi.StylePrimitive{BackgroundColor: background, Color: muted},
			},
		},
	}
}

func headingBlock(prefix string, color *string) ansi.StyleBlock {
	return ansi.StyleBlock{
		StylePrimitive: ansi.StylePrimitive{
			Prefix: prefix,
			Color:  color,
			Bold:   boolPtr(true),
		},
	}
}

func colorPtr(hex string) *string {
	return &hex
}

func boolPtr(value bool) *bool {
	return &value
}

func stringPtr(value string) *string {
	return &value
}

func uintPtr(value uint) *uint {
	return &value
}
