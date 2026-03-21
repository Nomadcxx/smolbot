package header

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/assets"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

type Model struct {
	width   int
	cached  string
	theme   string
	compact bool
}

func New() Model {
	return Model{}
}

func (m *Model) SetWidth(w int) {
	m.width = w
	m.cached = ""
}

func (m *Model) SetCompact(v bool) {
	if m.compact == v {
		return
	}
	m.compact = v
	m.cached = ""
}

func (m Model) Height() int {
	if m.compact {
		return 1
	}
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
	if m.compact {
		if m.width > 0 {
			line := lipgloss.NewStyle().
				Width(m.width).
				Align(lipgloss.Center).
				Foreground(t.Primary).
				Bold(true).
				Render("nanobot tui")
			m.cached = line
			return m.cached
		}
		line := lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true).
			Render("nanobot tui")
		m.cached = line
		return m.cached
	}
	lines := strings.Split(strings.TrimRight(assets.Header, "\n"), "\n")
	var out strings.Builder
	for i, line := range lines {
		style := lipgloss.NewStyle().Foreground(t.Primary)
		if m.width > 0 {
			style = style.Width(m.width).Align(lipgloss.Center)
		}
		styled := style.Render(line)
		out.WriteString(styled)
		if i < len(lines)-1 {
			out.WriteByte('\n')
		}
	}
	m.cached = out.String()
	return m.cached
}
