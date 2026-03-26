package dialog

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

type CommandChosenMsg struct {
	Command string
}

type CommandsModel struct {
	commands []string
	filtered []string
	filter   string
	cursor   int
}

var commandDescriptions = map[string]string{
	"/compact":       "compress context to free tokens",
	"/session":       "switch sessions",
	"/session new":   "start a fresh session",
	"/session reset": "reset the current session transcript",
	"/model":         "choose the active model",
	"/skills":        "browse available skills",
	"/mcps":          "show configured MCP servers",
	"/providers":     "show provider and model context",
	"/keybindings":   "show keyboard shortcuts",
	"/clear":         "clear the visible transcript",
	"/status":        "show gateway status",
	"/help":          "list available slash commands",
	"/quit":          "quit nanobot tui",
}

var commandAliases = map[string][]string{
	"/compact":       {"compress"},
	"/session new":   {"new session"},
	"/session reset": {"wipe", "restart"},
	"/clear":         {"cls"},
	"/quit":          {"exit"},
}

func NewCommands(commands []string) CommandsModel {
	m := CommandsModel{commands: commands}
	m.applyFilter()
	return m
}

func (m CommandsModel) Update(msg tea.Msg) (CommandsModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc":
			return m, func() tea.Msg { return CloseDialogMsg{} }
		case "up", "k", "ctrl+p":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j", "ctrl+n":
			if m.cursor < len(m.filtered)-1 {
				m.cursor++
			}
			return m, nil
		case "enter", "tab":
			if len(m.filtered) == 0 {
				return m, nil
			}
			return m, func() tea.Msg { return CommandChosenMsg{Command: m.filtered[m.cursor]} }
		case "backspace":
			if len(m.filter) > 0 {
				m.filter = m.filter[:len(m.filter)-1]
				m.applyFilter()
			}
			return m, nil
		default:
			k := key.String()
			if len(k) == 1 && k >= " " {
				m.filter += k
				m.applyFilter()
			}
			return m, nil
		}
	}
	return m, nil
}

func (m CommandsModel) View() string {
	t := theme.Current()
	if t == nil {
		return "commands"
	}
	lines := []string{
		lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("Commands"),
		lipgloss.NewStyle().Foreground(t.TextMuted).Render("Filter: " + m.filter),
		"",
	}
	if len(m.filtered) == 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true).Render("No matches"))
		lines = append(lines, "", lipgloss.NewStyle().Foreground(t.TextMuted).Render("↑↓ j/k • Enter/Tab run • Esc close"))
		return lipgloss.NewStyle().
			Background(t.Panel).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(t.BorderFocus).
			Padding(1, 2).
			Width(64).
			Render(strings.Join(lines, "\n"))
	}

	start, end := visibleBounds(len(m.filtered), m.cursor)
	if start > 0 {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Render("▲ more above"))
	}
	for i := start; i < end; i++ {
		command := m.filtered[i]
		prefix := "  "
		if i == m.cursor {
			prefix = "› "
		}
		desc := commandDescription(command)
		if desc == "" {
			lines = append(lines, prefix+command)
			continue
		}
		row := prefix + lipgloss.NewStyle().Foreground(t.Text).Render(command)
		row += lipgloss.NewStyle().Foreground(t.TextMuted).Render("  " + desc)
		if i == m.cursor {
			row = lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render(row)
		}
		lines = append(lines, row)
	}
	if end < len(m.filtered) {
		lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Render("▼ more below"))
	}
	lines = append(lines, "", lipgloss.NewStyle().Foreground(t.TextMuted).Render("↑↓ j/k • Enter/Tab run • Esc close"))
	return lipgloss.NewStyle().
		Background(t.Panel).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Padding(1, 2).
		Width(64).
		Render(strings.Join(lines, "\n"))
}

func (m *CommandsModel) SetFilter(filter string) {
	m.filter = filter
	m.applyFilter()
}

func (m CommandsModel) Current() string {
	if len(m.filtered) == 0 {
		return ""
	}
	if m.cursor < 0 || m.cursor >= len(m.filtered) {
		return ""
	}
	return m.filtered[m.cursor]
}

func (m CommandsModel) Filter() string {
	return m.filter
}

func (m *CommandsModel) applyFilter() {
	m.filtered = m.filtered[:0]
	labelMatches := make([]string, 0, len(m.commands))
	metaMatches := make([]string, 0, len(m.commands))
	for _, command := range m.commands {
		if matchesQuery(m.filter, command) {
			labelMatches = append(labelMatches, command)
			continue
		}
		if matchesQuery(m.filter, commandDescription(command), strings.Join(commandAliases[command], " ")) {
			metaMatches = append(metaMatches, command)
		}
	}
	if len(labelMatches) > 0 || strings.TrimSpace(m.filter) == "" {
		m.filtered = append(m.filtered, labelMatches...)
	} else {
		m.filtered = append(m.filtered, metaMatches...)
	}
	if m.cursor >= len(m.filtered) {
		m.cursor = max(0, len(m.filtered)-1)
	}
}

func commandDescription(command string) string {
	if desc, ok := commandDescriptions[command]; ok {
		return desc
	}
	if strings.HasPrefix(command, "/theme ") {
		return "switch theme"
	}
	return ""
}
