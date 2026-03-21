package header

import (
	"image/color"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/assets"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

type Model struct {
	width  int
	cached string
	theme  string
}

func New() Model {
	return Model{}
}

func (m *Model) SetWidth(w int) {
	m.width = w
	m.cached = ""
}

func (m Model) Height() int {
	return strings.Count(strings.TrimRight(assets.Header, "\n"), "\n") + 1
}

func (m *Model) View() string {
	t := theme.Current()
	if t == nil {
		return assets.Header
	}
	if m.cached != "" && m.theme == t.Name {
		return m.cached
	}

	m.theme = t.Name
	lines := strings.Split(strings.TrimRight(assets.Header, "\n"), "\n")
	colors := []color.Color{t.Primary, t.Secondary, t.Accent}
	var out strings.Builder
	for i, line := range lines {
		styled := lipgloss.NewStyle().Foreground(colors[i%len(colors)]).Render(line)
		if m.width > 0 {
			pad := (m.width - lipgloss.Width(line)) / 2
			if pad > 0 {
				styled = strings.Repeat(" ", pad) + styled
			}
		}
		out.WriteString(styled)
		if i < len(lines)-1 {
			out.WriteByte('\n')
		}
	}
	m.cached = out.String()
	return m.cached
}
