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
	width := dialogWidth(m.termWidth, 72) - 6 // Account for padding/border
	headerStyle := lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Width(width).Align(lipgloss.Center)
	hintsStyle := lipgloss.NewStyle().Foreground(t.TextMuted).Width(width).Align(lipgloss.Center)
	lines := []string{
		headerStyle.Render("//// KEYBINDINGS ////"),
		"",
		"Global",
		"  F1 / Ctrl+M    Open menu",
		"  Ctrl+C         Stop / Quit",
		"  Ctrl+D         Toggle sidebar",
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
		"Transcript",
		"  Ctrl+O         Expand/collapse tool output",
		"  Ctrl+E         Toggle verbose tool view",
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
		hintsStyle.Render("Esc close"),
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
