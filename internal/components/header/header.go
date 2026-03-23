package header

import (
	"fmt"
	"os"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/assets"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

type Model struct {
	width        int
	cached      string
	theme       string
	compact     bool
	model       string
	contextPct  int
	workDir     string
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

func (m *Model) SetModel(v string) {
	if m.model != v {
		m.model = v
		m.cached = ""
	}
}

func (m *Model) SetContextPercent(v int) {
	if m.contextPct != v {
		m.contextPct = v
		m.cached = ""
	}
}

func (m *Model) SetWorkDir(v string) {
	if m.workDir != v {
		m.workDir = v
		m.cached = ""
	}
}

func (m Model) IsCompact() bool {
	return m.compact
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
		return "smolbot"
	}
	if m.cached != "" && m.theme == t.Name {
		return m.cached
	}
	m.theme = t.Name

	if m.compact {
		m.cached = m.renderCompact(t)
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

func (m *Model) renderCompact(t *theme.Theme) string {
	logo := lipgloss.NewStyle().
		Foreground(t.Primary).
		Bold(true).
		Render("smolbot")

	var details []string

	if m.model != "" {
		details = append(details, lipgloss.NewStyle().
			Foreground(t.Secondary).
			Render(m.model))
	}

	if m.contextPct > 0 {
		pctStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
		if m.contextPct >= 90 {
			pctStyle = lipgloss.NewStyle().Foreground(t.Error).Bold(true)
		} else if m.contextPct >= 60 {
			pctStyle = lipgloss.NewStyle().Foreground(t.Warning)
		}
		details = append(details, pctStyle.Render(fmt.Sprintf("%d%%", m.contextPct)))
	}

	if m.workDir != "" {
		trimmed := trimWorkDir(m.workDir, 4)
		details = append(details, lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Render(trimmed))
	}

	diag := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Render(" ╱╱╱ ")

	sep := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Render(" • ")

	right := strings.Join(details, sep)
	line := logo + diag + right

	if m.width > 0 {
		return lipgloss.NewStyle().
			Width(m.width).
			Background(t.Panel).
			Padding(0, 1).
			Render(line)
	}
	return line
}

func trimWorkDir(path string, segments int) string {
	home, _ := os.UserHomeDir()
	display := path
	if home != "" && strings.HasPrefix(path, home) {
		display = "~" + path[len(home):]
	}
	parts := strings.Split(display, "/")
	if len(parts) > segments+1 {
		parts = append([]string{parts[0], "…"}, parts[len(parts)-segments:]...)
	}
	return strings.Join(parts, "/")
}