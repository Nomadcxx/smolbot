package chat

import (
	textarea "charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/nanobot-go/internal/tui/theme"
	"strings"
)

type EditorModel struct {
	textarea  textarea.Model
	width     int
	submitted string
	history   []string
	histIdx   int
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

func (m EditorModel) Height() int {
	return m.textarea.Height() + 2
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

func (m EditorModel) Update(msg tea.Msg) (EditorModel, tea.Cmd) {
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
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus)
	if m.width > 2 {
		border = border.Width(m.width - 2)
	}
	return border.Render(m.textarea.View())
}
