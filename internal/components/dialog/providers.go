package dialog

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

type ProvidersModel struct {
	lines []string
}

func NewProviders(lines []string) ProvidersModel {
	return ProvidersModel{lines: append([]string(nil), lines...)}
}

func (m ProvidersModel) Update(msg tea.Msg) (ProvidersModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		return m, func() tea.Msg { return CloseDialogMsg{} }
	}
	return m, nil
}

func (m ProvidersModel) View() string {
	t := theme.Current()
	if t == nil {
		return "providers"
	}
	lines := []string{
		lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("Providers"),
		"",
	}
	if len(m.lines) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true).Render("No provider information available"))
	} else {
		for _, line := range m.lines {
			lines = append(lines, line)
		}
	}
	lines = append(lines, "", lipgloss.NewStyle().Foreground(t.TextMuted).Render("Esc close"))
	return lipgloss.NewStyle().
		Background(t.Panel).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Padding(1, 2).
		Width(72).
		Render(strings.Join(lines, "\n"))
}
