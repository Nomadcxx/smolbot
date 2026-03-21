package status

import (
	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/app"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

type Model struct {
	width        int
	connected    bool
	reconnecting bool
	streaming    bool
	quitHint     bool
}

func New(a *app.App) Model {
	_ = a
	return Model{}
}

func (m *Model) SetWidth(w int)         { m.width = w }
func (m *Model) SetConnected(v bool)    { m.connected = v }
func (m *Model) SetReconnecting(v bool) { m.reconnecting = v }
func (m *Model) SetStreaming(v bool)    { m.streaming = v }
func (m *Model) SetQuitHint(v bool)     { m.quitHint = v }

func (m Model) View() string {
	t := theme.Current()
	if t == nil {
		return ""
	}
	connStatus := lipgloss.NewStyle().Foreground(t.Error).Render("● disconnected")
	if m.streaming {
		connStatus = lipgloss.NewStyle().Foreground(t.Warning).Render("● streaming")
	} else if m.reconnecting {
		connStatus = lipgloss.NewStyle().Foreground(t.Warning).Render("● reconnecting")
	} else if m.connected {
		connStatus = lipgloss.NewStyle().Foreground(t.Success).Render("● connected")
	}
	bar := " " + connStatus
	if m.quitHint {
		bar += lipgloss.NewStyle().Foreground(t.Warning).Render("  Press Ctrl+C again to quit")
	}
	return lipgloss.NewStyle().Width(m.width).Background(t.Panel).Foreground(t.Text).Render(bar)
}
