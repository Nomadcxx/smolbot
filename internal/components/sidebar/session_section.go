package sidebar

import (
	"image/color"
	"os"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

// cachedHomeDir is resolved once at init to avoid a syscall on every frame.
var cachedHomeDir string

func init() {
	cachedHomeDir, _ = os.UserHomeDir()
}

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
	if cachedHomeDir != "" && strings.HasPrefix(path, cachedHomeDir) {
		return "~" + strings.TrimPrefix(path, cachedHomeDir)
	}
	return path
}
