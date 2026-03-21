package chat

import (
	"fmt"
	"strings"

	viewport "charm.land/bubbles/v2/viewport"
	glamour "charm.land/glamour/v2"
	styles "charm.land/glamour/v2/styles"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

type ChatMessage struct {
	Role    string
	Content string
}

type MessagesModel struct {
	messages      []ChatMessage
	tools         []ToolCall
	width         int
	height        int
	progress      string
	thinking      string
	viewport      viewport.Model
	rendered      string
	dirty         bool
	renderer      *glamour.TermRenderer
	rendererWidth int
	rendererStyle string
}

func NewMessages() MessagesModel {
	vp := viewport.New()
	vp.SoftWrap = true
	vp.FillHeight = true
	return MessagesModel{
		viewport: vp,
		dirty:    true,
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

func (m *MessagesModel) SetProgress(content string) {
	m.progress = content
	m.sync(m.viewport.AtBottom())
}

func (m *MessagesModel) SetThinking(content string) {
	if strings.TrimSpace(content) == "" {
		m.thinking = "complete"
	} else {
		m.thinking = content
	}
	m.sync(m.viewport.AtBottom())
}

func (m *MessagesModel) ReplaceHistory(history []ChatMessage) {
	m.messages = append([]ChatMessage(nil), history...)
	m.tools = nil
	m.progress = ""
	m.thinking = ""
	m.sync(true)
}

func (m *MessagesModel) StartTool(name, _ string) {
	m.tools = append(m.tools, ToolCall{Name: name, Status: "running"})
	m.sync(m.viewport.AtBottom())
}

func (m *MessagesModel) FinishTool(name, status, output string) {
	for i := len(m.tools) - 1; i >= 0; i-- {
		if m.tools[i].Name == name {
			m.tools[i].Status = status
			m.tools[i].Output = output
			m.sync(m.viewport.AtBottom())
			return
		}
	}
	m.tools = append(m.tools, ToolCall{Name: name, Status: status, Output: output})
	m.sync(m.viewport.AtBottom())
}

func (m *MessagesModel) ScrollToBottom() {
	m.sync(true)
}

func (m *MessagesModel) HandleKey(key string) {
	m.sync(false)
	switch key {
	case "pgup":
		m.viewport.PageUp()
	case "pgdown":
		m.viewport.PageDown()
	case "home":
		m.viewport.GotoTop()
	case "end", "esc", "ctrl+l":
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
			lines = append(lines, lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("You: ")+msg.Content)
		case "assistant":
			lines = append(lines, lipgloss.NewStyle().Foreground(t.Secondary).Bold(true).Render("smolbot"))
			lines = append(lines, m.renderAssistant(msg.Content))
		case "error":
			lines = append(lines, lipgloss.NewStyle().Foreground(t.Error).Render("Error: "+msg.Content))
		}
		lines = append(lines, "")
	}
	if m.progress != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true).Render(fmt.Sprintf("  %s", m.progress)))
	}
	if m.thinking != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true).Render("  Thinking: "+m.thinking))
	}
	for _, tool := range m.tools {
		lines = append(lines, renderToolCall(tool, m.width))
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
	style := markdownStyle()
	if m.renderer != nil && m.rendererWidth == width && m.rendererStyle == style {
		return m.renderer
	}
	renderer, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle(style),
		glamour.WithWordWrap(width),
		glamour.WithPreservedNewLines(),
	)
	if err != nil {
		return nil
	}
	m.renderer = renderer
	m.rendererWidth = width
	m.rendererStyle = style
	return m.renderer
}

func markdownStyle() string {
	current := theme.Current()
	if current == nil {
		return styles.DarkStyle
	}
	switch current.Name {
	case "dracula":
		return styles.DraculaStyle
	case "tokyo-night":
		return styles.TokyoNightStyle
	case "monochrome":
		return styles.AsciiStyle
	default:
		return styles.DarkStyle
	}
}

func (m *MessagesModel) View() string {
	if m.dirty {
		m.sync(m.viewport.AtBottom() || m.viewport.TotalLineCount() == 0)
	}
	if m.width <= 0 || m.height <= 0 {
		return m.rendered
	}
	return lipgloss.NewStyle().Width(m.width).Height(m.height).Render(m.viewport.View())
}
