package dialog

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

type MCPServersModel struct {
	servers []client.MCPServerInfo
	cursor  int
	termWidth int
}

func NewMCPServers(servers []client.MCPServerInfo) MCPServersModel {
	return MCPServersModel{servers: servers}
}

func (m MCPServersModel) Update(msg tea.Msg) (MCPServersModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return m, func() tea.Msg { return CloseDialogMsg{} }
		case "up", "k", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j", "ctrl+n":
			if m.cursor < len(m.servers)-1 {
				m.cursor++
			}
		}
	}
	return m, nil
}

func (m MCPServersModel) View() string {
	t := theme.Current()
	if t == nil {
		return "mcp servers"
	}
	lines := []string{
		lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("MCP Servers"),
		"",
	}
	start, end := visibleBounds(len(m.servers), m.cursor)
	if start > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Render("▲ more above"))
	}
	for i := start; i < end; i++ {
		server := m.servers[i]
		prefix := "  "
		if i == m.cursor {
			prefix = "› "
		}
		statusColor := t.Warning
		if server.Status == "connected" {
			statusColor = t.Success
		} else if server.Status == "error" {
			statusColor = t.Error
		}
		row := prefix + server.Name + " " + lipgloss.NewStyle().Foreground(statusColor).Render("["+server.Status+"]")
		if i == m.cursor {
			row = lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render(row)
		}
		lines = append(lines, row)
		if strings.TrimSpace(server.Command) != "" {
			lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Render("    "+server.Command))
		}
	}
	if len(m.servers) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true).Render("No MCP servers configured"))
	}
	if end < len(m.servers) {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Render("▼ more below"))
	}
	lines = append(lines, "", lipgloss.NewStyle().Foreground(t.TextMuted).Render("↑↓ j/k • Esc close"))
	return lipgloss.NewStyle().
		Background(t.Panel).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Padding(1, 2).
		Width(dialogWidth(m.termWidth, 72)).
		Render(strings.Join(lines, "\n"))
}

func (m MCPServersModel) WithTerminalWidth(w int) MCPServersModel {
	m.termWidth = w
	return m
}
