package dialog

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/nanobot-go/internal/client"
	"github.com/Nomadcxx/nanobot-go/internal/theme"
)

type SessionChosenMsg struct {
	Key string
}

type SessionNewMsg struct{}

type SessionResetMsg struct {
	Key string
}

type CloseDialogMsg struct{}

type SessionsModel struct {
	sessions []client.SessionInfo
	filtered []client.SessionInfo
	filter   string
	cursor   int
	current  string
}

func NewSessions(sessions []client.SessionInfo, current string) SessionsModel {
	m := SessionsModel{sessions: sessions, current: current}
	m.applyFilter()
	return m
}

func (m SessionsModel) Update(msg tea.Msg) (SessionsModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return m, func() tea.Msg { return CloseDialogMsg{} }
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		case "n":
			return m, func() tea.Msg { return SessionNewMsg{} }
		case "d":
			if len(m.filtered) == 0 {
				return m, nil
			}
			key := m.filtered[m.cursor].Key
			return m, func() tea.Msg { return SessionResetMsg{Key: key} }
		case "enter":
			if len(m.filtered) == 0 {
				return m, nil
			}
			key := m.filtered[m.cursor].Key
			return m, func() tea.Msg { return SessionChosenMsg{Key: key} }
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

func (m SessionsModel) View() string {
	t := theme.Current()
	if t == nil {
		return "sessions"
	}

	lines := []string{
		lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("Sessions"),
		lipgloss.NewStyle().Foreground(t.TextMuted).Render("Filter: " + m.filter),
		"",
	}
	for i, session := range m.filtered {
		prefix := "  "
		if i == m.cursor {
			prefix = "› "
		}
		label := session.Key
		if session.Key == m.current {
			label += " (current)"
		}
		lines = append(lines, prefix+label)
	}
	if len(m.filtered) == 0 {
		lines = append(lines, "  No sessions")
	}
	lines = append(lines, "", "Enter=switch  n=new  d=reset  Esc=close")
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Padding(1, 2).
		Width(48).
		Render(strings.Join(lines, "\n"))
}

func (m *SessionsModel) applyFilter() {
	m.filtered = m.filtered[:0]
	needle := strings.ToLower(m.filter)
	for _, session := range m.sessions {
		if needle == "" || strings.Contains(strings.ToLower(session.Key), needle) {
			m.filtered = append(m.filtered, session)
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func (m SessionsModel) String() string {
	return fmt.Sprintf("SessionsDialog(%d)", len(m.filtered))
}
