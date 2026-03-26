package sidebar

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/Nomadcxx/smolbot/internal/theme"
)

type MCPEntry struct {
	Name   string
	Status string
	Tools  int
}

type MCPsSection struct {
	servers []MCPEntry
}

func (s MCPsSection) Title() string { return "MCPS" }

func (s MCPsSection) ItemCount() int { return len(s.servers) }

func (s MCPsSection) Render(width, maxItems int, t *theme.Theme) string {
	if width <= 0 {
		width = DefaultWidth
	}
	if len(s.servers) == 0 {
		return styleLine("none configured", width, t, func(th *theme.Theme) color.Color { return th.TextMuted })
	}

	entries := s.servers
	overflow := 0
	if maxItems > 0 && len(entries) > maxItems {
		overflow = len(entries) - maxItems
		entries = entries[:max(0, maxItems-1)]
	}

	lines := make([]string, 0, len(entries)+1)
	for _, entry := range entries {
		icon, color := mcpStatus(entry.Status, t)
		parts := []string{icon, entry.Name}
		if entry.Tools > 0 {
			parts = append(parts, fmt.Sprintf("(%d tools)", entry.Tools))
		}
		lines = append(lines, renderEntryLine(strings.Join(parts, " "), width, t, color))
	}
	if overflow > 0 {
		lines = append(lines, styleLine(fmt.Sprintf("…and %d more", overflow), width, t, func(th *theme.Theme) color.Color { return th.TextMuted }))
	}
	return strings.Join(lines, "\n")
}

func mcpStatus(state string, t *theme.Theme) (string, color.Color) {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "connected":
		return "●", themeColor(t, func(th *theme.Theme) color.Color { return th.Success }, nil)
	case "error":
		return "●", themeColor(t, func(th *theme.Theme) color.Color { return th.Error }, nil)
	case "configured":
		return "●", themeColor(t, func(th *theme.Theme) color.Color { return th.Accent }, nil)
	case "disabled":
		return "○", themeColor(t, func(th *theme.Theme) color.Color { return th.TextMuted }, nil)
	default:
		return "●", themeColor(t, func(th *theme.Theme) color.Color { return th.TextMuted }, nil)
	}
}
