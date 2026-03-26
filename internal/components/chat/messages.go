package chat

import (
	"image/color"
	"strconv"
	"strings"
	"time"

	viewport "charm.land/bubbles/v2/viewport"
	glamour "charm.land/glamour/v2"
	"charm.land/glamour/v2/ansi"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

type ChatMessage struct {
	Role     string
	Content  string
	Duration time.Duration
}

type MessagesModel struct {
	messages      []ChatMessage
	tools         []ToolCall
	width         int
	height        int
	progress      string
	thinking      string
	thinkingStart time.Time
	viewport      viewport.Model
	rendered      string
	dirty         bool
	renderer      *glamour.TermRenderer
	rendererWidth int
	rendererStyle string
	expandedTools map[string]bool
}

func NewMessages() MessagesModel {
	vp := viewport.New()
	vp.SoftWrap = true
	vp.FillHeight = true
	return MessagesModel{
		viewport:      vp,
		dirty:         true,
		expandedTools: make(map[string]bool),
	}
}

func (m *MessagesModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.viewport.SetWidth(max(1, w))
	m.viewport.SetHeight(max(1, h))
	m.dirty = true
}

func (m *MessagesModel) AppendUser(content string) {
	m.messages = append(m.messages, ChatMessage{Role: "user", Content: content})
	m.progress = ""
	m.thinking = ""
	m.sync(true)
}

func (m *MessagesModel) AppendAssistant(content string) {
	m.messages = append(m.messages, ChatMessage{Role: "assistant", Content: content})
	m.progress = ""
	m.thinking = ""
	m.tools = nil
	m.sync(true)
}

func (m *MessagesModel) AppendError(content string) {
	m.messages = append(m.messages, ChatMessage{Role: "error", Content: content})
	m.progress = ""
	m.thinking = ""
	m.sync(true)
}

func (m *MessagesModel) AppendSystem(content string) {
	if strings.TrimSpace(content) == "" {
		return
	}
	m.messages = append(m.messages, ChatMessage{Role: "system", Content: content})
	m.progress = ""
	m.thinking = ""
	m.sync(m.viewport.AtBottom())
}

func (m *MessagesModel) AppendThinking(content string) {
	if strings.TrimSpace(content) == "" {
		return
	}
	dur := time.Duration(0)
	if !m.thinkingStart.IsZero() {
		dur = time.Since(m.thinkingStart)
		m.thinkingStart = time.Time{}
	}
	m.messages = append(m.messages, ChatMessage{Role: "thinking", Content: content, Duration: dur})
	m.sync(m.viewport.AtBottom())
}

func (m *MessagesModel) SetProgress(content string) {
	m.progress = content
	m.sync(m.viewport.AtBottom())
}

// SetThinking sets ephemeral in-progress thinking text shown during an active
// streaming run. It is cleared when the run completes. For finalized thinking
// content that should persist in the transcript, use AppendThinking instead.
func (m *MessagesModel) SetThinking(content string) {
	if m.thinking == "" && content != "" {
		m.thinkingStart = time.Now()
	}
	m.thinking = content
	m.sync(m.viewport.AtBottom())
}

func (m *MessagesModel) GetThinking() string {
	return m.thinking
}

func (m *MessagesModel) GetProgress() string {
	return m.progress
}

func (m *MessagesModel) ReplaceHistory(history []ChatMessage) {
	m.messages = append([]ChatMessage(nil), history...)
	m.tools = nil
	m.progress = ""
	m.thinking = ""
	m.sync(true)
}

func (m *MessagesModel) StartTool(id, name, input string) {
	m.tools = append(m.tools, ToolCall{ID: id, Name: name, Input: input, Status: "running"})
	m.sync(m.viewport.AtBottom())
}

func (m *MessagesModel) FinishTool(id, name, status, output string) {
	for i := len(m.tools) - 1; i >= 0; i-- {
		if m.tools[i].ID == id {
			m.tools[i].Status = status
			m.tools[i].Output = output
			m.sync(m.viewport.AtBottom())
			return
		}
	}
	for i := len(m.tools) - 1; i >= 0; i-- {
		if m.tools[i].Name == name && m.tools[i].Status == "running" {
			m.tools[i].Status = status
			m.tools[i].Output = output
			m.sync(m.viewport.AtBottom())
			return
		}
	}
	m.tools = append(m.tools, ToolCall{ID: id, Name: name, Status: status, Output: output})
	m.sync(m.viewport.AtBottom())
}

func (m *MessagesModel) ToggleToolExpand(index int) {
	key := strconv.Itoa(index)
	m.expandedTools[key] = !m.expandedTools[key]
	m.dirty = true
}

func (m *MessagesModel) ScrollToBottom() {
	m.sync(true)
}

func (m *MessagesModel) ViewportOffset() int {
	return m.viewport.YOffset()
}

func (m *MessagesModel) HasContentAbove() bool {
	return m.viewport.YOffset() > 0
}

func (m *MessagesModel) InvalidateTheme() {
	m.renderer = nil
	m.dirty = true
}

func (m *MessagesModel) IsDirty() bool {
	return m.dirty
}

func (m *MessagesModel) HandleKey(key string) {
	switch key {
	case "pgup":
		m.viewport.PageUp()
	case "pgdown":
		m.viewport.PageDown()
	case "home":
		m.viewport.GotoTop()
	case "end", "ctrl+l":
		m.viewport.GotoBottom()
	}
}

func (m *MessagesModel) sync(follow bool) {
	offset := m.viewport.YOffset()
	m.rendered = m.renderContent()
	m.viewport.SetContent(m.rendered)
	if follow {
		m.viewport.GotoBottom()
	} else {
		m.viewport.SetYOffset(offset)
	}
	m.dirty = false
}

func (m *MessagesModel) renderContent() string {
	t := theme.Current()
	if t == nil {
		return ""
	}

	var lines []string
	for _, msg := range m.messages {
		switch msg.Role {
		case "user":
			lines = append(lines, renderRoleBlock("USER", msg.Content, t.TranscriptUserAccent, m.width))
		case "assistant":
			lines = append(lines, renderRoleBlock("ASSISTANT", m.renderAssistant(msg.Content), t.TranscriptAssistantAccent, m.width))
		case "system":
			lines = append(lines, renderSystemMessage(msg.Content, m.width))
		case "error":
			lines = append(lines, renderMessageBlock("ERROR", msg.Content, t.Error, m.width))
		case "thinking":
			lines = append(lines, renderThinkingBlock(msg.Content, msg.Duration, t.TranscriptThinking, m.width))
		}
		lines = append(lines, "")
	}
	if m.progress != "" {
		lines = append(lines, renderRoleBlock("ASSISTANT", m.renderAssistant(m.progress), t.TranscriptStreaming, m.width))
	}
	if m.thinking != "" {
		lines = append(lines, renderRoleBlock("THINKING", m.thinking, t.TranscriptThinking, m.width))
	}
	for i, tool := range m.tools {
		expanded := m.expandedTools[strconv.Itoa(i)]
		lines = append(lines, renderToolCall(tool, m.width, expanded))
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

func (m *MessagesModel) renderAssistant(content string) string {
	renderer := m.markdownRenderer()
	if renderer == nil || strings.TrimSpace(content) == "" {
		return content
	}
	rendered, err := renderer.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimSpace(rendered)
}

func (m *MessagesModel) markdownRenderer() *glamour.TermRenderer {
	width := max(20, m.width-2)
	signature := markdownSignature()
	if m.renderer != nil && m.rendererWidth == width && m.rendererStyle == signature {
		return m.renderer
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStyles(markdownStyleConfig()),
		glamour.WithWordWrap(width),
		glamour.WithPreservedNewLines(),
		glamour.WithChromaFormatter("terminal16m"),
	)
	if err != nil {
		return nil
	}
	m.renderer = renderer
	m.rendererWidth = width
	m.rendererStyle = signature
	return m.renderer
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

func markdownSignature() string {
	current := theme.Current()
	if current == nil {
		return "default"
	}
	return strings.Join([]string{
		current.Name,
		colorHex(current.Text),
		colorHex(current.TextMuted),
		colorHex(current.MarkdownHeading),
		colorHex(current.MarkdownLink),
		colorHex(current.MarkdownCode),
		colorHex(current.SyntaxKeyword),
		colorHex(current.SyntaxString),
		colorHex(current.SyntaxComment),
	}, ":")
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
				Background:       ansi.StylePrimitive{BackgroundColor: background},
				Text:             ansi.StylePrimitive{BackgroundColor: background, Color: text},
				Error:            ansi.StylePrimitive{BackgroundColor: background, Color: colorPtr(colorHex(current.TranscriptError))},
				Comment:          ansi.StylePrimitive{BackgroundColor: background, Color: comment},
				Keyword:          ansi.StylePrimitive{BackgroundColor: background, Color: keyword},
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

func (m *MessagesModel) View() string {
	if m.dirty {
		m.sync(m.viewport.AtBottom() || m.viewport.TotalLineCount() == 0)
	}
	if m.width <= 0 || m.height <= 0 {
		return m.rendered
	}
	if strings.TrimSpace(m.rendered) == "" {
		return strings.Repeat("\n", max(0, m.height-1))
	}
	return m.viewport.View()
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
