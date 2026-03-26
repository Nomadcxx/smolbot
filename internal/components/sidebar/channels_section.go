package sidebar

import (
	"fmt"
	"image/color"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

type ChannelEntry struct {
	Name  string
	State string
}

type ChannelsSection struct {
	channels []ChannelEntry
}

func (s ChannelsSection) Title() string { return "CHANNELS" }

func (s ChannelsSection) ItemCount() int { return len(s.channels) }

func (s ChannelsSection) Render(width, maxItems int, t *theme.Theme) string {
	if width <= 0 {
		width = DefaultWidth
	}
	if len(s.channels) == 0 {
		return styleLine("none configured", width, t, func(th *theme.Theme) color.Color { return th.TextMuted })
	}

	entries := s.channels
	overflow := 0
	if maxItems > 0 && len(entries) > maxItems {
		overflow = len(entries) - maxItems
		entries = entries[:max(0, maxItems-1)]
	}

	lines := make([]string, 0, len(entries)+1)
	for _, entry := range entries {
		icon, color := channelStatus(entry.State, t)
		line := fmt.Sprintf("%s %s %s", icon, entry.Name, strings.TrimSpace(entry.State))
		lines = append(lines, renderEntryLine(line, width, t, color))
	}
	if overflow > 0 {
		lines = append(lines, styleLine(fmt.Sprintf("…and %d more", overflow), width, t, func(th *theme.Theme) color.Color { return th.TextMuted }))
	}
	return strings.Join(lines, "\n")
}

func channelStatus(state string, t *theme.Theme) (string, color.Color) {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "connected":
		return "●", themeColor(t, func(th *theme.Theme) color.Color { return th.Success }, nil)
	case "error":
		return "●", themeColor(t, func(th *theme.Theme) color.Color { return th.Error }, nil)
	case "qr", "starting", "busy":
		return "●", themeColor(t, func(th *theme.Theme) color.Color { return th.Warning }, nil)
	default:
		return "●", themeColor(t, func(th *theme.Theme) color.Color { return th.TextMuted }, nil)
	}
}

func renderEntryLine(text string, width int, t *theme.Theme, colorValue color.Color) string {
	text = truncateVisible(text, width)
	if text == "" {
		return ""
	}
	if t == nil {
		return text
	}
	style := lipgloss.NewStyle()
	if colorValue != nil {
		style = style.Foreground(colorValue)
	}
	return style.Render(text)
}

func themeColor(t *theme.Theme, fn func(*theme.Theme) color.Color, fallback color.Color) color.Color {
	if t == nil {
		return fallback
	}
	return fn(t)
}
