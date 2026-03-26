package chat

import (
	textarea "charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
	"strings"
)

type EditorModel struct {
	textarea  textarea.Model
	width     int
	submitted string
	history   []string
	histIdx   int
	compact   bool
}

func NewEditor() EditorModel {
	ta := textarea.New()
	ta.Placeholder = "Send a message..."
	ta.CharLimit = 0
	ta.SetHeight(3)
	ta.ShowLineNumbers = false
	ta.Focus()
	return EditorModel{textarea: ta, histIdx: -1}
}

func (m EditorModel) Init() tea.Cmd {
	return textarea.Blink
}

func (m *EditorModel) SetWidth(w int) {
	m.width = w
	if w > 4 {
		m.textarea.SetWidth(w - 4)
	}
}

func (m *EditorModel) SetCompact(v bool) {
	if m.compact == v {
		return
	}
	m.compact = v
	if v {
		m.textarea.SetHeight(1)
		return
	}
	m.textarea.SetHeight(3)
}

func (m EditorModel) Height() int {
	if m.compact {
		return 1
	}
	return m.textarea.Height() + 3
}

func (m *EditorModel) Submitted() string {
	s := m.submitted
	m.submitted = ""
	return s
}

func (m EditorModel) Value() string {
	return m.textarea.Value()
}

func (m *EditorModel) SetValue(value string) {
	m.textarea.SetValue(value)
	m.textarea.MoveToEnd()
}

func (m *EditorModel) Focus() tea.Cmd {
	return m.textarea.Focus()
}

func (m *EditorModel) Blur() {
	m.textarea.Blur()
}

func (m EditorModel) Focused() bool {
	return m.textarea.Focused()
}

func (m EditorModel) Update(msg tea.Msg) (EditorModel, tea.Cmd) {
	if paste, ok := msg.(tea.PasteMsg); ok {
		m.textarea.InsertString(paste.Content)
		return m, nil
	}
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "shift+enter", "alt+enter":
			m.textarea.InsertRune('\n')
			return m, nil
		case "enter":
			val := strings.TrimSpace(m.textarea.Value())
			if val != "" {
				m.submitted = val
				m.history = append(m.history, val)
				m.histIdx = len(m.history)
				m.textarea.Reset()
				return m, nil
			}
		case "up":
			if len(m.history) == 0 {
				return m, nil
			}
			if strings.TrimSpace(m.textarea.Value()) != "" {
				break
			}
			if m.histIdx == -1 || m.histIdx > len(m.history) {
				m.histIdx = len(m.history)
			}
			if m.histIdx > 0 {
				m.histIdx--
				m.textarea.SetValue(m.history[m.histIdx])
			}
			return m, nil
		case "down":
			if len(m.history) == 0 {
				return m, nil
			}
			if strings.TrimSpace(m.textarea.Value()) != "" && (m.histIdx < 0 || m.histIdx >= len(m.history)) {
				break
			}
			if m.histIdx >= 0 && m.histIdx < len(m.history)-1 {
				m.histIdx++
				m.textarea.SetValue(m.history[m.histIdx])
				return m, nil
			}
			if m.histIdx == len(m.history)-1 {
				m.histIdx = len(m.history)
				m.textarea.SetValue("")
				return m, nil
			}
		}
	}

	var cmd tea.Cmd
	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

func (m EditorModel) View() string {
	t := theme.Current()
	if t == nil {
		return m.textarea.View()
	}
	if m.compact {
		line := lipgloss.NewStyle().
			Foreground(t.BorderFocus).
			Bold(true).
			Render("› ")
		line += m.textarea.View()
		return lipgloss.NewStyle().
			Width(m.width).
			Background(t.Element).
			Foreground(t.Text).
			Render(line)
	}
	border := lipgloss.NewStyle().
		Background(t.Element).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)
	if m.width > 2 {
		border = border.Width(m.width - 2)
	}
	body := lipgloss.NewStyle().
		Background(t.Element).
		Foreground(t.Text).
		Render(m.textarea.View())
	input := border.Render(body)
	hint := renderQuickStartHint(m.width)
	if hint == "" {
		return input
	}
	return lipgloss.JoinVertical(lipgloss.Left, input, hint)
}

func renderQuickStartHint(width int) string {
	t := theme.Current()
	if t == nil || width <= 0 {
		return ""
	}
	base := lipgloss.NewStyle().
		Foreground(t.Text).
		Background(t.Panel).
		Bold(true).
		Render
	muted := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Background(t.Panel).
		Render

	hint := base("f1") + muted(" menu   ")
	hint += base("/model") + muted(" models   ")
	hint += base("/theme") + muted(" themes   ")
	hint += base("ctrl+c") + muted(" abort/quit")

	return lipgloss.NewStyle().
		Width(width).
		Background(t.Panel).
		Foreground(t.TextMuted).
		Padding(0, 1).
		Render(hint)
}
