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
	app          *app.App
	width        int
	metadata     string
	usage        client.UsageInfo
	compression  *client.CompressionInfo
	compacting   bool
	compactFrame int
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

func (m *FooterModel) SetCompression(info *client.CompressionInfo) {
	m.compression = info
}

func (m *FooterModel) SetCompacting(v bool) {
	m.compacting = v
	if !v {
		m.compactFrame = 0
	}
}

func (m *FooterModel) IsCompacting() bool {
	return m.compacting
}

func (m *FooterModel) SetCompactionFrame(frame int) {
	m.compactFrame = frame
}

func (m FooterModel) renderCompression(t *theme.Theme) string {
	if m.compacting {
		frames := []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}
		frame := frames[m.compactFrame%len(frames)]
		return lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true).
			Render(frame + " compacting...")
	}
	if m.compression == nil || !m.compression.Enabled {
		return ""
	}

	pct := m.compression.ReductionPercent

	// Choose style based on reduction percentage (inspired by nanocoder)
	var style lipgloss.Style
	indicator := "↓" // Down arrow indicates compression

	switch {
	case pct >= 60: // Heavy compression
		style = lipgloss.NewStyle().
			Foreground(t.CompressionWarning).
			Bold(true)
		indicator += fmt.Sprintf("%.0f%%", pct)
	case pct >= 30: // Moderate compression
		style = lipgloss.NewStyle().
			Foreground(t.CompressionSuccess)
		indicator += fmt.Sprintf("%.0f%%", pct)
	default: // Light compression
		style = lipgloss.NewStyle().
			Foreground(t.CompressionActive)
		indicator += fmt.Sprintf("%.0f%%", pct)
	}

	return style.Render(indicator)
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
	// Add compression indicator if available
	if comp := m.renderCompression(t); comp != "" {
		parts = append(parts, comp)
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

	// Color coding inspired by nanocoder (green → yellow → red)
	var percentStyle lipgloss.Style
	if percentage >= 90 {
		percentStyle = lipgloss.NewStyle().Foreground(t.TokenHighUsage).Bold(true)
	} else if percentage >= 80 {
		percentStyle = lipgloss.NewStyle().Foreground(t.Warning).Bold(true)
	} else if percentage >= 60 {
		percentStyle = lipgloss.NewStyle().Foreground(t.CompressionWarning)
	} else {
		percentStyle = lipgloss.NewStyle().Foreground(t.TextMuted)
	}

	percentText := percentStyle.Render(fmt.Sprintf("%d%%", int(percentage+0.5)))
	warning := ""
	switch {
	case percentage >= 95:
		warning = lipgloss.NewStyle().
			Foreground(t.TokenHighUsage).
			Bold(true).
			Blink(true).
			Render(" ⚠ /compact")
	case percentage >= 90:
		warning = lipgloss.NewStyle().
			Foreground(t.Warning).
			Bold(true).
			Render(" ⚠")
	}

	if compact {
		return percentText + warning + " " + lipgloss.NewStyle().
			Background(t.Panel).
			Foreground(t.TextMuted).
			Render("("+formatUsageTokens(m.usage.TotalTokens)+")")
	}

	return percentText + warning + " " + lipgloss.NewStyle().
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
