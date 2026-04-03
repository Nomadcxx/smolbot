package sidebar

import (
	"fmt"
	"image/color"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
	"github.com/charmbracelet/x/ansi"
)

// Section represents a discrete block within the sidebar.
type Section interface {
	Title() string
	Render(width, maxItems int, t *theme.Theme) string
	ItemCount() int
}

func renderSectionHeader(title string, width int, t *theme.Theme) string {
	label := strings.ToUpper(strings.TrimSpace(title))
	if label == "" {
		return ""
	}

	styled := lipgloss.NewStyle().Bold(true)
	if t != nil {
		styled = styled.Foreground(t.Accent)
	}
	return styled.Render(label)
}

func truncateVisible(text string, width int) string {
	if width <= 0 || text == "" {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	if width == 1 {
		return ansi.Cut(text, 0, 1)
	}
	return ansi.Cut(text, 0, width-1) + "…"
}

func joinNonEmpty(lines ...string) string {
	compact := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			compact = append(compact, line)
		}
	}
	return strings.Join(compact, "\n")
}

func styleText(text string, width int, t *theme.Theme, colorFn func(*theme.Theme) color.Color) string {
	if strings.TrimSpace(text) == "" {
		return ""
	}
	rendered := truncateVisible(text, width)
	if t == nil {
		return rendered
	}
	st := lipgloss.NewStyle()
	if colorFn != nil {
		st = st.Foreground(colorFn(t))
	}
	return st.Render(rendered)
}

func formatTokens(value int) string {
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
