package dialog

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

type SkillsModel struct {
	skills   []client.SkillInfo
	filtered []client.SkillInfo
	filter   string
	cursor   int
	termWidth int
}

func NewSkills(skills []client.SkillInfo) SkillsModel {
	m := SkillsModel{skills: skills}
	m.applyFilter()
	return m
}

func (m SkillsModel) Update(msg tea.Msg) (SkillsModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return m, func() tea.Msg { return CloseDialogMsg{} }
		case "up", "k", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j", "ctrl+n":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
		case "backspace":
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
				m.applyFilter()
			}
		default:
			k := key.String()
			if len(k) == 1 && k >= " " {
				m.filter += k
				m.applyFilter()
			}
		}
	}
	return m, nil
}

func (m SkillsModel) View() string {
	t := theme.Current()
	if t == nil {
		return "skills"
	}

	lines := []string{
		lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("Skills"),
		lipgloss.NewStyle().Foreground(t.TextMuted).Render("Filter: " + m.filter),
		"",
	}
	start, end := visibleBounds(len(m.filtered), m.cursor)
	if start > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Render("▲ more above"))
	}
	for i := start; i < end; i++ {
		skill := m.filtered[i]
		prefix := "  "
		if i == m.cursor {
			prefix = "› "
		}
		statusColor := t.TextMuted
		if skill.Status == "loaded" || skill.Status == "always" {
			statusColor = t.Success
		}
		row := prefix + skill.Name + " " + lipgloss.NewStyle().Foreground(statusColor).Render("["+skill.Status+"]")
		if i == m.cursor {
			row = lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render(row)
		}
		lines = append(lines, row)
		if strings.TrimSpace(skill.Description) != "" {
			lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Render("    "+skill.Description))
		}
	}
	if len(m.filtered) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true).Render("No skills"))
	}
	if end < len(m.filtered) {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Render("▼ more below"))
	}
	lines = append(lines, "", lipgloss.NewStyle().Foreground(t.TextMuted).Render("Type to filter • Esc close"))
	return lipgloss.NewStyle().
		Background(t.Panel).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Padding(1, 2).
		Width(dialogWidth(m.termWidth, 72)).
		Render(strings.Join(lines, "\n"))
}

func (m *SkillsModel) applyFilter() {
	m.filtered = m.filtered[:0]
	for _, skill := range m.skills {
		if matchesQuery(m.filter, skill.Name, skill.Description, skill.Status) {
			m.filtered = append(m.filtered, skill)
		}
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func (m SkillsModel) WithTerminalWidth(w int) SkillsModel {
	m.termWidth = w
	return m
}
