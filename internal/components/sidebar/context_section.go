package sidebar

import (
	"fmt"
	"image/color"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

type ContextSection struct {
	usage       client.UsageInfo
	compression *client.CompressionInfo
}

func (s ContextSection) Title() string { return "CONTEXT" }

func (s ContextSection) ItemCount() int { return 0 }

func (s ContextSection) Render(width, _ int, t *theme.Theme) string {
	if width <= 0 {
		width = DefaultWidth
	}
	if s.usage.ContextWindow <= 0 || s.usage.TotalTokens <= 0 {
		return styleLine("—", width, t, func(th *theme.Theme) color.Color { return th.TextMuted })
	}

	percent := float64(s.usage.TotalTokens) / float64(s.usage.ContextWindow)
	if percent < 0 {
		percent = 0
	}
	if percent > 1 {
		percent = 1
	}
	percentLabel := fmt.Sprintf(" %d%%", int(percent*100+0.5))
	barWidth := width - lipgloss.Width(percentLabel)
	if barWidth < 4 {
		barWidth = 4
	}
	filled := int(percent * float64(barWidth))
	if filled < 0 {
		filled = 0
	}
	if filled > barWidth {
		filled = barWidth
	}
	empty := barWidth - filled

	barColor := contextColor(percent, t)
	bar := styledRepeat("█", filled, barColor, t) + styledRepeat("░", empty, nil, t) + styleSuffix(percentLabel, barColor, t)
	tokens := styleLine(fmt.Sprintf("%s / %s", formatTokens(s.usage.TotalTokens), formatTokens(s.usage.ContextWindow)), width, t, func(th *theme.Theme) color.Color { return th.TextMuted })

	lines := []string{bar, tokens}
	if s.compression != nil && s.compression.Enabled && s.compression.ReductionPercent > 0 {
		lines = append(lines, styleLine(fmt.Sprintf("↓ %.0f%% compacted", s.compression.ReductionPercent), width, t, func(th *theme.Theme) color.Color { return th.CompressionSuccess }))
	}
	return joinNonEmpty(lines...)
}

func contextColor(percent float64, t *theme.Theme) color.Color {
	if t == nil {
		return nil
	}
	switch {
	case percent >= 0.9:
		return t.TokenHighUsage
	case percent >= 0.8:
		return t.Warning
	case percent >= 0.6:
		return t.CompressionWarning
	default:
		return t.Accent
	}
}

func styledRepeat(char string, count int, colorValue color.Color, t *theme.Theme) string {
	if count <= 0 {
		return ""
	}
	text := strings.Repeat(char, count)
	if t == nil {
		return text
	}
	style := lipgloss.NewStyle()
	if colorValue != nil {
		style = style.Foreground(colorValue)
	}
	return style.Render(text)
}

func styleSuffix(text string, colorValue color.Color, t *theme.Theme) string {
	if t == nil {
		return text
	}
	style := lipgloss.NewStyle()
	if colorValue != nil {
		style = style.Foreground(colorValue)
	}
	return style.Render(text)
}

func styleLine(text string, width int, t *theme.Theme, colorFn func(*theme.Theme) color.Color) string {
	text = truncateVisible(text, width)
	if text == "" {
		return ""
	}
	if t == nil {
		return text
	}
	style := lipgloss.NewStyle()
	if colorFn != nil {
		style = style.Foreground(colorFn(t))
	}
	return style.Render(text)
}
