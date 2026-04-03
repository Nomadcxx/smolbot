package dialog

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/internal/theme"
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
	termWidth int
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

	width := dialogWidth(m.termWidth, 60)
	headerStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Width(width).Align(lipgloss.Center)
	lines := []string{
		headerStyle.Render("//// SESSIONS ////"),
		lipgloss.NewStyle().Foreground(t.TextMuted).Render("Filter: " + m.filter),
		"",
	}

	start, end := visibleBounds(len(m.filtered), m.cursor)
	if start > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Render("▲ more above"))
	}
	for i := start; i < end; i++ {
		session := m.filtered[i]
		prefix := "  "
		if i == m.cursor {
			prefix = "› "
		}
		label := session.Key
		if session.Key == m.current {
			label += lipgloss.NewStyle().Foreground(t.Success).Bold(true).Render(" current")
		}
		row := prefix + label
		if i == m.cursor {
			row = lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render(row)
		}
		lines = append(lines, row)
		if session.Preview != "" {
			lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Render("    "+session.Preview))
		}
	}
	if len(m.filtered) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true).Render("No sessions"))
	}
	if end < len(m.filtered) {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Render("▼ more below"))
	}
	lines = append(lines, "", lipgloss.NewStyle().Foreground(t.TextMuted).Render("Up/Down j/k • Enter switch • n new • d reset • Esc close"))
	return lipgloss.NewStyle().
		Background(t.Panel).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Padding(1, 2).
		Width(dialogWidth(m.termWidth, 64)).
		Render(strings.Join(lines, "\n"))
}

func (m *SessionsModel) applyFilter() {
	m.filtered = m.filtered[:0]
	for _, session := range m.sessions {
		if matchesQuery(m.filter, session.Key, session.Preview, session.UpdatedAt) {
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

func (m SessionsModel) WithTerminalWidth(w int) SessionsModel {
	m.termWidth = w
	return m
}
