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
	artLines := strings.Count(strings.TrimRight(assets.Header, "\n"), "\n") + 1
	return artLines + 1
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
	artStyle := lipgloss.NewStyle().Foreground(t.Primary)
	diagStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	artWidth := lipgloss.Width(lines[0])
	fillChars := "╱╱╱╱╱╱"
	fillLen := lipgloss.Width(fillChars)

	for i, line := range lines {
		rendered := artStyle.Render(line)
		out.WriteString(rendered)
		if m.width > artWidth {
			padWidth := m.width - artWidth
			for padWidth > 0 {
				chunkLen := min(fillLen, padWidth)
				out.WriteString(diagStyle.Render(fillChars[:chunkLen]))
				padWidth -= chunkLen
			}
		}
		if i < len(lines)-1 {
			out.WriteByte('\n')
		}
	}

	info := m.renderInfoLine(t)
	if info != "" {
		out.WriteByte('\n')
		out.WriteString(info)
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

func (m *Model) renderInfoLine(t *theme.Theme) string {
	var parts []string
	sep := lipgloss.NewStyle().Foreground(t.TextMuted).Render(" • ")

	if m.model != "" {
		parts = append(parts, lipgloss.NewStyle().Foreground(t.Secondary).Render(m.model))
	}
	if m.contextPct > 0 {
		pctStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
		if m.contextPct >= 90 {
			pctStyle = lipgloss.NewStyle().Foreground(t.Error).Bold(true)
		} else if m.contextPct >= 60 {
			pctStyle = lipgloss.NewStyle().Foreground(t.Warning)
		}
		parts = append(parts, pctStyle.Render(fmt.Sprintf("%d%%", m.contextPct)))
	}
	if m.workDir != "" {
		parts = append(parts, lipgloss.NewStyle().Foreground(t.TextMuted).Render(trimWorkDir(m.workDir, 4)))
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, sep)
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