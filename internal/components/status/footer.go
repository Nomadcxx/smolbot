package status

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/app"
	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

type FooterModel struct {
	app      *app.App
	width    int
	metadata string
	usage    client.UsageInfo
}

func NewFooter(a *app.App) FooterModel {
	return FooterModel{app: a}
}

func (m *FooterModel) SetWidth(w int) {
	m.width = w
}

func (m *FooterModel) SetMetadata(value string) {
	m.metadata = value
}

func (m *FooterModel) SetUsage(value client.UsageInfo) {
	m.usage = value
}

func (m FooterModel) View() string {
	t := theme.Current()
	if t == nil || m.app == nil {
		return ""
	}
	parts := []string{
		"model " + footerValue(m.app.Model, "connecting..."),
		"session " + footerValue(m.app.Session, "none"),
	}
	if strings.TrimSpace(m.metadata) != "" {
		parts = append(parts, strings.TrimSpace(m.metadata))
	}
	left := " " + strings.Join(parts, " | ")
	right := m.renderUsage(t, false)
	if right == "" {
		return lipgloss.NewStyle().
			Width(m.width).
			Background(t.Panel).
			Foreground(t.TextMuted).
			Render(left)
	}

	if m.width > 0 && lipgloss.Width(left)+1+lipgloss.Width(right) > m.width {
		right = m.renderUsage(t, true)
	}
	if m.width > 0 && lipgloss.Width(right)+1 >= m.width {
		return lipgloss.NewStyle().
			Width(m.width).
			Background(t.Panel).
			Foreground(t.TextMuted).
			Align(lipgloss.Right).
			Render(right)
	}

	if m.width > 0 {
		availableLeft := max(1, m.width-lipgloss.Width(right)-1)
		left = truncateFooterText(left, availableLeft)
	}
	gap := " "
	if m.width > 0 {
		gapWidth := m.width - lipgloss.Width(left) - lipgloss.Width(right)
		if gapWidth > 0 {
			gap = strings.Repeat(" ", gapWidth)
		}
	}
	return lipgloss.NewStyle().
		Width(m.width).
		Background(t.Panel).
		Foreground(t.TextMuted).
		Render(left + gap + right)
}

func footerValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func truncateFooterText(text string, width int) string {
	if width <= 0 || lipgloss.Width(text) <= width {
		return text
	}
	if width <= 1 {
		return text[:width]
	}
	runes := []rune(text)
	if len(runes) > width-1 {
		runes = runes[:width-1]
	}
	return string(runes) + "…"
}

func (m FooterModel) renderUsage(t *theme.Theme, compact bool) string {
	if m.usage.ContextWindow <= 0 || m.usage.TotalTokens <= 0 {
		return ""
	}

	percentage := (float64(m.usage.TotalTokens) / float64(m.usage.ContextWindow)) * 100
	percentageStyle := lipgloss.NewStyle().
		Background(t.Panel).
		Foreground(t.TextMuted).
		Bold(true)
	if percentage >= 80 {
		percentageStyle = percentageStyle.Foreground(t.Warning)
	}

	percentageText := percentageStyle.Render(fmt.Sprintf("%d%%", int(percentage+0.5)))
	if compact {
		return percentageText + " " + lipgloss.NewStyle().
			Background(t.Panel).
			Foreground(t.TextMuted).
			Render("("+formatUsageTokens(m.usage.TotalTokens)+")")
	}

	return percentageText + " " + lipgloss.NewStyle().
		Background(t.Panel).
		Foreground(t.TextMuted).
		Render(fmt.Sprintf("(%s/%s)", formatUsageTokens(m.usage.TotalTokens), formatUsageTokens(m.usage.ContextWindow)))
}

func formatUsageTokens(value int) string {
	switch {
	case value >= 1_000_000:
		text := fmt.Sprintf("%.1fM", float64(value)/1_000_000)
		return strings.Replace(text, ".0M", "M", 1)
	case value >= 1_000:
		text := fmt.Sprintf("%.1fK", float64(value)/1_000)
		return strings.Replace(text, ".0K", "K", 1)
	default:
		return fmt.Sprintf("%d", value)
	}
}
