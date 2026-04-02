package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	dialogcmp "github.com/Nomadcxx/smolbot/internal/components/dialog"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

type menuPage int

const (
	menuPageRoot menuPage = iota
	menuPageThemes
	menuPageSessions
	menuPageContext
)

type menuItem struct {
	label   string
	command string
	page    menuPage
}

type menuDialog struct {
	page       menuPage
	cursors    map[menuPage]int
	standalone bool
}

func newMenuDialog() menuDialog {
	return menuDialog{
		page: menuPageRoot,
		cursors: map[menuPage]int{
			menuPageRoot:     0,
			menuPageThemes:   0,
			menuPageSessions: 0,
			menuPageContext:  0,
		},
	}
}

func newThemesMenuDialog() menuDialog {
	dialog := newMenuDialog()
	dialog.page = menuPageThemes
	dialog.standalone = true
	return dialog
}

func (d menuDialog) Update(msg tea.Msg) (Dialog, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return d, nil
	}

	items := d.items()
	cursor := d.cursor(items)
	switch key.String() {
	case "esc":
		if d.page == menuPageRoot || d.standalone {
			return d, func() tea.Msg { return dialogcmp.CloseDialogMsg{} }
		}
		d.page = menuPageRoot
		return d, nil
	case "up", "k", "ctrl+p":
		if cursor > 0 {
			d.cursors[d.page] = cursor - 1
		}
		return d, nil
	case "down", "j", "ctrl+n":
		if cursor < len(items)-1 {
			d.cursors[d.page] = cursor + 1
		}
		return d, nil
	case "enter":
		if len(items) == 0 {
			return d, nil
		}
		selected := items[cursor]
		if selected.page != menuPageRoot || selected.label == "← Back" {
			if selected.label == "← Back" {
				d.page = menuPageRoot
				return d, nil
			}
			d.page = selected.page
			return d, nil
		}
		if selected.command == "" {
			return d, nil
		}
		return d, func() tea.Msg { return dialogcmp.CommandChosenMsg{Command: selected.command} }
	}
	return d, nil
}

func (d menuDialog) View() string {
	t := theme.Current()
	if t == nil {
		return "menu"
	}

	items := d.items()
	cursor := d.cursor(items)
	title := d.title()
	renderedItems := make([]string, 0, len(items)+4)
	maxWidth := lipgloss.Width(title)

	start, end := menuVisibleBounds(len(items), cursor)
	if start > 0 {
		line := lipgloss.NewStyle().Foreground(t.TextMuted).Render("▲ More above ▲")
		renderedItems = append(renderedItems, line)
		maxWidth = max(maxWidth, lipgloss.Width(line))
	}
	for i := start; i < end; i++ {
		item := items[i]
		style := lipgloss.NewStyle().
			Foreground(t.TextMuted).
			Background(t.Panel).
			Padding(0, 2)
		if i == cursor {
			style = style.Foreground(t.Primary).Bold(true)
		}
		line := style.Render(item.label)
		renderedItems = append(renderedItems, line)
		maxWidth = max(maxWidth, lipgloss.Width(line))
	}
	if end < len(items) {
		line := lipgloss.NewStyle().Foreground(t.TextMuted).Render("▼ More below ▼")
		renderedItems = append(renderedItems, line)
		maxWidth = max(maxWidth, lipgloss.Width(line))
	}
	help := lipgloss.NewStyle().Foreground(t.TextMuted).Width(maxWidth).Align(lipgloss.Center).Render(d.helpText())
	maxWidth = max(maxWidth, lipgloss.Width(help))
	lines := []string{
		lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Width(maxWidth).Align(lipgloss.Center).Render(title),
		"",
	}
	for _, line := range renderedItems {
		lines = append(lines, lipgloss.NewStyle().Width(maxWidth).Render(line))
	}
	lines = append(lines, "", help)

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Background(t.Panel).
		Foreground(t.Text).
		Padding(2, 4).
		Render(lipgloss.JoinVertical(lipgloss.Left, lines...))
}

func (d menuDialog) SetTerminalWidth(int) Dialog { return d }

func (d menuDialog) title() string {
	switch d.page {
	case menuPageThemes:
		return "//// THEMES ////"
	case menuPageSessions:
		return "//// SESSIONS ////"
	case menuPageContext:
		return "//// CONTEXT ////"
	default:
		return "//// MENU ////"
	}
}

func (d menuDialog) helpText() string {
	if d.page == menuPageRoot {
		return "↑↓ Navigate • Enter Select • Esc close"
	}
	if d.standalone {
		return "↑↓ Navigate • Enter Select • Esc close"
	}
	return "↑↓ Navigate • Enter Select • Esc back"
}

func (d menuDialog) items() []menuItem {
	switch d.page {
	case menuPageThemes:
		items := make([]menuItem, 0, len(theme.List())+1)
		if !d.standalone {
			items = append(items, menuItem{label: "← Back"})
		}
		for _, name := range theme.List() {
			items = append(items, menuItem{
				label:   "Theme: " + name,
				command: "/theme " + name,
			})
		}
		return items
	case menuPageSessions:
		return []menuItem{
			{label: "← Back"},
			{label: "New Session", command: "/session new"},
			{label: "Switch Session", command: "/session"},
			{label: "Reset Current Session", command: "/session reset"},
		}
	case menuPageContext:
		return []menuItem{
			{label: "← Back"},
			{label: "Compact Now", command: "/compact"},
			{label: "Mode: Conservative (soon)"},
			{label: "Mode: Default (soon)"},
			{label: "Mode: Aggressive (soon)"},
		}
	default:
		return []menuItem{
			{label: "Close Menu", command: "/menu.close"},
			{label: "Themes", page: menuPageThemes},
			{label: "Sessions", page: menuPageSessions},
			{label: "Models", command: "/model"},
			{label: "Context & Compaction", page: menuPageContext},
			{label: "Skills", command: "/skills"},
			{label: "MCP Servers", command: "/mcps"},
			{label: "Providers", command: "/providers"},
			{label: "Clear Transcript", command: "/clear"},
			{label: "Status", command: "/status"},
			{label: "Keybindings", command: "/keybindings"},
			{label: "Help", command: "/help"},
			{label: "Quit", command: "/quit"},
		}
	}
}

func (d menuDialog) cursor(items []menuItem) int {
	if len(items) == 0 {
		return 0
	}
	cursor := d.cursors[d.page]
	if cursor < 0 {
		return 0
	}
	if cursor >= len(items) {
		return len(items) - 1
	}
	return cursor
}

func menuVisibleBounds(total, cursor int) (int, int) {
	const window = 9
	if total <= window {
		return 0, total
	}
	start := max(0, cursor-window/2)
	end := start + window
	if end > total {
		end = total
		start = max(0, end-window)
	}
	return start, end
}

func isMenuCloseCommand(command string) bool {
	return strings.TrimSpace(command) == "/menu.close"
}
