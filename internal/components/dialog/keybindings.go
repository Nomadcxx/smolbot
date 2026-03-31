package dialog

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

type KeybindingsModel struct {
	termWidth int
}

func NewKeybindings() KeybindingsModel {
	return KeybindingsModel{}
}

func (m KeybindingsModel) Update(msg tea.Msg) (KeybindingsModel, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
		return m, func() tea.Msg { return CloseDialogMsg{} }
	}
	return m, nil
}

func (m KeybindingsModel) View() string {
	t := theme.Current()
	if t == nil {
		return "keybindings"
	}
	lines := []string{
		lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("Keybindings"),
		"",
		"Global",
		"  F1 / Ctrl+M    Open menu",
		"  Ctrl+C         Stop / Quit",
		"  Esc / i        Leave / enter insert mode",
		"  y / c          Copy last assistant reply",
		"  PgUp/PgDn      Scroll transcript",
		"  Home/End       Top/Bottom of transcript",
		"  Ctrl+L         Jump to latest",
		"",
		"Editor",
		"  Enter          Send message",
		"  Shift+Enter    New line",
		"  Up/Down        Input history",
		"",
		"Dialogs",
		"  Esc            Close / Back",
		"  ↑/↓ or j/k     Navigate",
		"  Enter          Select",
		"  Type           Filter",
		"",
		"Commands",
		"  /compact       Compress context",
		"  /session       Switch session",
		"  /model         Change model",
		"  /theme         Change theme",
		"  /status        Show status",
		"  /clear         Clear transcript",
		"  /help          Show help",
		"  /quit          Exit",
		"",
		lipgloss.NewStyle().Foreground(t.TextMuted).Render("Esc close"),
	}
	return lipgloss.NewStyle().
		Background(t.Panel).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocus).
		Padding(1, 2).
		Width(dialogWidth(m.termWidth, 72)).
		Render(strings.Join(lines, "\n"))
}

func (m KeybindingsModel) WithTerminalWidth(w int) KeybindingsModel {
	m.termWidth = w
	return m
}
