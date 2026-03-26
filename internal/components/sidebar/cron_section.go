package sidebar

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

type CronSection struct {
	jobs []client.CronJob
}

func (s CronSection) Title() string { return "SCHEDULED" }

func (s CronSection) ItemCount() int { return len(s.jobs) }

func (s CronSection) Render(width, maxItems int, t *theme.Theme) string {
	if width <= 0 {
		width = DefaultWidth
	}
	if len(s.jobs) == 0 {
		return styleLine("none", width, t, func(th *theme.Theme) color.Color { return th.TextMuted })
	}

	entries := s.jobs
	overflow := 0
	if maxItems > 0 && len(entries) > maxItems {
		overflow = len(entries) - maxItems
		entries = entries[:max(0, maxItems-1)]
	}

	lines := make([]string, 0, len(entries)*2+1)
	for _, job := range entries {
		icon, iconColor := cronStatus(job.Status, t)
		lines = append(lines, renderEntryLine(fmt.Sprintf("%s %s", icon, job.Name), width, t, iconColor))
		lines = append(lines, styleLine("  "+job.Schedule, width, t, func(th *theme.Theme) color.Color { return th.TextMuted }))
	}
	if overflow > 0 {
		lines = append(lines, styleLine(fmt.Sprintf("…and %d more", overflow), width, t, func(th *theme.Theme) color.Color { return th.TextMuted }))
	}
	return strings.Join(lines, "\n")
}

func cronStatus(state string, t *theme.Theme) (string, color.Color) {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "paused":
		return "⏸", themeColor(t, func(th *theme.Theme) color.Color { return th.TextMuted }, nil)
	case "completed":
		return "✓", themeColor(t, func(th *theme.Theme) color.Color { return th.Success }, nil)
	default:
		return "⏱", themeColor(t, func(th *theme.Theme) color.Color { return th.Accent }, nil)
	}
}
