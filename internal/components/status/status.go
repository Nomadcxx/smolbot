package status

import (
	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/app"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

type Model struct {
	width         int
	connected     bool
	reconnecting  bool
	streaming     bool
	quitHint      bool
	flash         string
	interruptHint bool
	spinner       SpinnerModel
}

func New(a *app.App) Model {
	_ = a
	return Model{spinner: NewSpinner()}
}

func (m *Model) SetWidth(w int)           { m.width = w }
func (m *Model) SetConnected(v bool)      { m.connected = v }
func (m *Model) SetReconnecting(v bool)   { m.reconnecting = v }
func (m *Model) SetQuitHint(v bool)       { m.quitHint = v }
func (m *Model) SetFlash(v string)        { m.flash = v }
func (m *Model) ClearFlash()              { m.flash = "" }
func (m *Model) SetInterruptHint(v bool)  { m.interruptHint = v }

func (m *Model) SetStreaming(v bool) {
	m.streaming = v
	if !v {
		m.spinner = NewSpinner()
	}
}

// StatusSpinnerTick returns a command that starts the streaming status spinner.
func (m *Model) StatusSpinnerTick() tea.Cmd {
	return m.spinner.Tick()
}

// AdvanceSpinner advances the spinner frame by one step.
// Called from tui.go on each status.SpinnerTickMsg while streaming.
func (m *Model) AdvanceSpinner(msg SpinnerTickMsg) {
	m.spinner, _ = m.spinner.Update(msg)
}

func (m Model) View() string {
	t := theme.Current()
	if t == nil {
		return ""
	}

	var connStatus string
	if m.streaming {
		// Animated spinner with styled "working" label
		workingStyle := lipgloss.NewStyle().Foreground(t.Warning).Bold(true)
		connStatus = workingStyle.Render("working") + " " + m.spinner.View()
	} else if m.reconnecting {
		connStatus = lipgloss.NewStyle().Foreground(t.Warning).Render("● reconnecting")
	} else if m.connected {
		connStatus = lipgloss.NewStyle().Foreground(t.Success).Render("● connected")
	} else {
		connStatus = lipgloss.NewStyle().Foreground(t.Error).Render("● disconnected")
	}

	bar := " " + connStatus

	if m.interruptHint {
		keyStyle := lipgloss.NewStyle().Foreground(t.Warning).Bold(true)
		mutedStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
		bar += "  " + keyStyle.Render("esc") + mutedStyle.Render(" again to ") + keyStyle.Render("interrupt")
	}

	if m.quitHint {
		bar += lipgloss.NewStyle().Foreground(t.Warning).Render("  Press Ctrl+C again to quit")
	}
	if m.flash != "" {
		bar += lipgloss.NewStyle().
			Foreground(t.Primary).
			Bold(true).
			Render("  " + m.flash)
	}
	return lipgloss.NewStyle().Width(m.width).Background(t.Panel).Foreground(t.Text).Render(bar)
}
