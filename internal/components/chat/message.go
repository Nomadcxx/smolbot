package chat

import (
	"fmt"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

type ToolCall struct {
	Name   string
	Status string
	Output string
}

func renderToolCall(tc ToolCall, width int) string {
	t := theme.Current()
	if t == nil {
		return tc.Name
	}

	var icon string
	var iconColor = t.Info
	switch tc.Status {
	case "running":
		icon = "●"
		iconColor = t.Warning
	case "done":
		icon = "✓"
		iconColor = t.Success
	case "error":
		icon = "✗"
		iconColor = t.Error
	default:
		icon = "•"
	}

	header := fmt.Sprintf(
		"%s %s",
		lipgloss.NewStyle().Foreground(iconColor).Render(icon),
		lipgloss.NewStyle().Foreground(t.ToolName).Bold(true).Render(tc.Name),
	)

	content := header
	if tc.Output != "" && tc.Status != "running" {
		content += "\n" + lipgloss.NewStyle().Foreground(t.TextMuted).Render(tc.Output)
	}

	style := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.ToolBorder).
		Padding(0, 1)
	if width > 4 {
		style = style.Width(width - 4)
	}
	return style.Render(content)
}
