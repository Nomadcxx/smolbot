package dialog

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/nanobot-go/internal/theme"
)

type CommandChosenMsg struct {
	Command string
}

type CommandsModel struct {
	commands []string
	filtered []string
	filter   string
	cursor   int
}

func NewCommands(commands []string) CommandsModel {
	m := CommandsModel{commands: commands}
	m.applyFilter()
	return m
}

func (m CommandsModel) Update(msg tea.Msg) (CommandsModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return m, func() tea.Msg { return CloseDialogMsg{} }
		case "up", "k", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j", "ctrl+n":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		case "enter", "tab":
			if len(m.filtered) == 0 {
				return m, nil
			}
			return m, func() tea.Msg { return CommandChosenMsg{Command: m.filtered[m.cursor]} }
		case "backspace":
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
				m.applyFilter()
			}
			return m, nil
		default:
			k := key.String()
			if len(k) == 1 && k >= " " {
				m.filter += k
				m.applyFilter()
			}
			return m, nil
		}
	}
	return m, nil
}

func (m CommandsModel) View() string {
	t := theme.Current()
	if t == nil {
		return "commands"
	}
	lines := []string{
		lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("Commands"),
		lipgloss.NewStyle().Foreground(t.TextMuted).Render("Filter: " + m.filter),
		"",
	}
	for i, command := range m.filtered {
		prefix := "  "
		if i == m.cursor {
			prefix = "› "
		}
		lines = append(lines, prefix+command)
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Padding(1, 2).
		Width(48).
		Render(strings.Join(lines, "\n"))
}

func (m *CommandsModel) SetFilter(filter string) {
	m.filter = filter
	m.applyFilter()
}

func (m CommandsModel) Current() string {
	if len(m.filtered) == 0 {
		return ""
	}
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		return ""
	}
	return m.filtered[m.cursor]
}

func (m *CommandsModel) applyFilter() {
	m.filtered = m.filtered[:0]
	needle := strings.ToLower(m.filter)
	for _, command := range m.commands {
		if needle == "" || strings.Contains(strings.ToLower(command), needle) {
			m.filtered = append(m.filtered, command)
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}
