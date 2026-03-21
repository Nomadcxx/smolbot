package themes

import (
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

func register(name string, colors [15]string) {
	theme.Register(&theme.Theme{
		Name:        name,
		Background:  lipgloss.Color(colors[0]),
		Panel:       lipgloss.Color(colors[1]),
		Element:     lipgloss.Color(colors[2]),
		Border:      lipgloss.Color(colors[3]),
		BorderFocus: lipgloss.Color(colors[4]),
		Primary:     lipgloss.Color(colors[5]),
		Secondary:   lipgloss.Color(colors[6]),
		Accent:      lipgloss.Color(colors[7]),
		Text:        lipgloss.Color(colors[8]),
		TextMuted:   lipgloss.Color(colors[9]),
		Error:       lipgloss.Color(colors[10]),
		Warning:     lipgloss.Color(colors[11]),
		Success:     lipgloss.Color(colors[12]),
		Info:        lipgloss.Color(colors[13]),
		ToolBorder:  lipgloss.Color(colors[14]),
		ToolName:    lipgloss.Color(colors[5]),
	})
}
