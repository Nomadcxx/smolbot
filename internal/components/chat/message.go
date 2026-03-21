package chat

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"

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

	icon, stateColor, statusLabel := toolStateTokens(tc.Status, t)
	innerWidth := max(0, width-5)

	label := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Bold(true).
		Render("TOOL")
	name := lipgloss.NewStyle().
		Foreground(t.ToolName).
		Bold(true).
		Render(tc.Name)
	state := lipgloss.NewStyle().
		Background(stateColor).
		Foreground(t.Background).
		Bold(true).
		Padding(0, 1).
		Render(statusLabel)
	headerContent := lipgloss.JoinHorizontal(
		lipgloss.Left,
		lipgloss.NewStyle().Foreground(stateColor).Bold(true).Render(icon),
		" ",
		label,
		"  ",
		name,
		" ",
		state,
	)
	header := lipgloss.NewStyle().
		Background(t.ToolArtifactHeader).
		Foreground(t.Text).
		Width(innerWidth).
		Padding(0, 1).
		Render(headerContent)

	bodyText := tc.Output
	if strings.TrimSpace(bodyText) == "" {
		switch tc.Status {
		case "running":
			bodyText = "waiting for tool output..."
		case "error":
			bodyText = "tool execution failed"
		default:
			bodyText = "no output"
		}
	}
	body := lipgloss.NewStyle().
		Background(t.ToolArtifactBody).
		Foreground(t.Text).
		Width(innerWidth).
		Padding(0, 1).
		Render(bodyText)

	content := lipgloss.JoinVertical(lipgloss.Left, header, body)
	style := lipgloss.NewStyle().
		Background(t.ToolArtifactBody).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.ToolArtifactBorder).
		Padding(0, 0)
	if width > 4 {
		style = style.Width(width - 2)
	}
	return style.Render(content)
}

func renderRoleBlock(label, body string, accent color.Color, width int) string {
	t := theme.Current()
	if t == nil {
		return label + "\n" + body
	}
	if semanticAccent := transcriptRoleAccent(label, t); semanticAccent != nil {
		accent = semanticAccent
	}
	innerWidth := max(0, width-5)
	badge := lipgloss.NewStyle().
		Background(accent).
		Foreground(t.Background).
		Bold(true).
		Padding(0, 1).
		Render(label)

	header := lipgloss.NewStyle().
		Background(subtleWash(accent)).
		Width(innerWidth).
		Padding(0, 1).
		Render(badge)
	contentBody := lipgloss.NewStyle().
		Background(t.Panel).
		Foreground(t.Text).
		Width(innerWidth).
		Padding(0, 1).
		Render(body)
	content := lipgloss.JoinVertical(lipgloss.Left, header, contentBody)
	style := lipgloss.NewStyle().
		Background(t.Panel).
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(accent).
		Padding(0, 0)
	if width > 4 {
		style = style.Width(width - 2)
	}
	return style.Render(content)
}

func transcriptRoleAccent(label string, t *theme.Theme) color.Color {
	switch label {
	case "USER":
		return t.TranscriptUserAccent
	case "ASSISTANT":
		return t.TranscriptAssistantAccent
	default:
		return nil
	}
}

func subtleWash(accent color.Color) color.Color {
	hex := colorHex(accent)
	if len(hex) != 7 || hex[0] != '#' {
		return lipgloss.Color("#111111")
	}
	r, _ := strconv.ParseInt(hex[1:3], 16, 64)
	g, _ := strconv.ParseInt(hex[3:5], 16, 64)
	b, _ := strconv.ParseInt(hex[5:7], 16, 64)
	return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", int(r)/5, int(g)/5, int(b)/5))
}

func colorHex(value color.Color) string {
	r, g, b, _ := value.RGBA()
	return fmt.Sprintf("#%02X%02X%02X", uint8(r>>8), uint8(g>>8), uint8(b>>8))
}

func toolStateTokens(status string, t *theme.Theme) (icon string, accent color.Color, label string) {
	switch status {
	case "running":
		return "●", t.ToolStateRunning, "RUNNING"
	case "done":
		return "✓", t.ToolStateDone, "DONE"
	case "error":
		return "✗", t.ToolStateError, "ERROR"
	default:
		return "•", t.ToolArtifactBorder, "INFO"
	}
}
