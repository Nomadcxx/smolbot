package chat

import (
	"fmt"
	"image/color"
	"strconv"
	"strings"
	"time"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

const maxTextWidth = 120

func cappedWidth(available int) int {
	if available <= 0 {
		return maxTextWidth
	}
	w := available - 2
	if w > maxTextWidth {
		return maxTextWidth
	}
	return max(20, w)
}

type ToolCall struct {
	ID     string
	Name   string
	Input  string
	Status string
	Output string
}

const maxToolOutputLines = 10

func renderToolCall(tc ToolCall, width int, expanded bool) string {
	t := theme.Current()
	if t == nil {
		return tc.Name
	}

	icon, iconColor := toolIcon(tc.Status, t)
	iconStr := lipgloss.NewStyle().Foreground(iconColor).Bold(true).Render(icon)
	nameStr := lipgloss.NewStyle().Foreground(t.ToolName).Bold(true).Render(tc.Name)

	header := "  " + iconStr + " " + nameStr

	bodyText := tc.Output
	if strings.TrimSpace(bodyText) == "" {
		if tc.Status == "running" {
			bodyText = "running…"
		} else {
			return header
		}
	}

	truncHint := ""
	if !expanded {
		outputLines := strings.Split(bodyText, "\n")
		if len(outputLines) > maxToolOutputLines {
			hidden := len(outputLines) - maxToolOutputLines
			bodyText = strings.Join(outputLines[:maxToolOutputLines], "\n")
			truncHint = fmt.Sprintf("… (%d lines hidden)", hidden)
		}
	}

	bodyStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	indent := "    "
	var lines []string
	lines = append(lines, header)

	if strings.TrimSpace(tc.Input) != "" {
		lines = append(lines, bodyStyle.Render(indent+tc.Input))
	}

	for _, line := range strings.Split(bodyText, "\n") {
		lines = append(lines, bodyStyle.Render(indent+line))
	}
	if truncHint != "" {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true).Render(indent+truncHint))
	}
	return strings.Join(lines, "\n")
}

func toolIcon(status string, t *theme.Theme) (string, color.Color) {
	switch status {
	case "running":
		return "●", t.ToolStateRunning
	case "done":
		return "✓", t.ToolStateDone
	case "error":
		return "✗", t.ToolStateError
	default:
		return "•", t.TextMuted
	}
}

func renderRoleBlock(label, body string, accent color.Color, width int) string {
	t := theme.Current()
	if t == nil {
		return label + "\n" + body
	}
	if semanticAccent := transcriptRoleAccent(label, t); semanticAccent != nil {
		accent = semanticAccent
	}
	innerWidth := cappedWidth(width)
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
		Foreground(t.Text).
		Width(innerWidth).
		Padding(0, 1).
		Render(body)
	content := lipgloss.JoinVertical(lipgloss.Left, header, contentBody)
	style := lipgloss.NewStyle().
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
	case "THINKING":
		return t.TranscriptThinking
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

func renderThinkingBlock(body string, dur time.Duration, accent color.Color, width int) string {
	t := theme.Current()
	if t == nil {
		return "THINKING\n" + body
	}
	innerWidth := cappedWidth(width)
	badge := lipgloss.NewStyle().
		Background(accent).
		Foreground(t.Background).
		Bold(true).
		Padding(0, 1).
		Render("THINKING")

	header := lipgloss.NewStyle().
		Background(subtleWash(accent)).
		Width(innerWidth).
		Padding(0, 1).
		Render(badge)

	bodyLines := strings.Split(body, "\n")
	truncHint := ""
	if len(bodyLines) > maxToolOutputLines {
		hidden := len(bodyLines) - maxToolOutputLines
		body = strings.Join(bodyLines[:maxToolOutputLines], "\n")
		truncHint = fmt.Sprintf("… (%d lines hidden)", hidden)
	}

	contentBody := lipgloss.NewStyle().
		Foreground(t.Text).
		Width(innerWidth).
		Padding(0, 1).
		Render(body)

	var rows []string
	rows = append(rows, header, contentBody)

	if truncHint != "" {
		rows = append(rows, lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Italic(true).
			Width(innerWidth).
			Padding(0, 1).
			Render(truncHint))
	}

	if dur > 0 {
		footer := lipgloss.NewStyle().
			Background(subtleWash(accent)).
			Foreground(t.TextMuted).
			Width(innerWidth).
			Padding(0, 1).
			Render("Thought for "+formatDuration(dur))
		rows = append(rows, footer)
	}

	content := lipgloss.JoinVertical(lipgloss.Left, rows...)
	style := lipgloss.NewStyle().
		Border(lipgloss.ThickBorder(), false, false, false, true).
		BorderForeground(accent).
		Padding(0, 0)
	if width > 4 {
		style = style.Width(width - 2)
	}
	return style.Render(content)
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
