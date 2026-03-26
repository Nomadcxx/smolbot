package sidebar

import (
	"image/color"
	"os"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

type SessionSection struct {
	sessionKey string
	cwd        string
	model      string
}

func (s SessionSection) Title() string { return "SESSION" }

func (s SessionSection) ItemCount() int { return 0 }

func (s SessionSection) Render(width, _ int, t *theme.Theme) string {
	if width <= 0 {
		width = DefaultWidth
	}

	name := strings.TrimSpace(s.sessionKey)
	if name == "" {
		name = "—"
	}
	cwd := prettyPath(s.cwd)
	if cwd == "" {
		cwd = "—"
	}
	model := strings.TrimSpace(s.model)
	if model == "" {
		model = "—"
	}

	primary := renderValue(name, width, t, func(th *theme.Theme) color.Color { return th.Text })
	secondary := renderValue(cwd, width, t, func(th *theme.Theme) color.Color { return th.TextMuted })
	tertiary := renderValue(model, width, t, func(th *theme.Theme) color.Color { return th.Accent })
	return joinNonEmpty(primary, secondary, tertiary)
}

func renderValue(value string, width int, t *theme.Theme, colorFn func(*theme.Theme) color.Color) string {
	value = truncateVisible(value, width)
	if value == "" {
		return ""
	}
	if t == nil {
		return value
	}
	style := lipgloss.NewStyle()
	if colorFn != nil {
		style = style.Foreground(colorFn(t))
	}
	return style.Render(value)
}

func prettyPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	home, err := os.UserHomeDir()
	if err == nil && home != "" && strings.HasPrefix(path, home) {
		return "~" + strings.TrimPrefix(path, home)
	}
	return path
}
