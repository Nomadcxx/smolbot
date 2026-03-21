package status

import (
	"fmt"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/nanobot-go/internal/app"
	"github.com/Nomadcxx/nanobot-go/internal/theme"
)

type Model struct {
	app          *app.App
	width        int
	connected    bool
	reconnecting bool
	streaming    bool
	quitHint     bool
}

func New(a *app.App) Model {
	return Model{app: a}
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
	model := m.app.Model
	if model == "" {
		model = "connecting..."
	}
	session := m.app.Session
	connStatus := lipgloss.NewStyle().Foreground(t.Error).Render("● disconnected")
	if m.streaming {
		connStatus = lipgloss.NewStyle().Foreground(t.Warning).Render("● streaming")
	} else if m.reconnecting {
		connStatus = lipgloss.NewStyle().Foreground(t.Warning).Render("● reconnecting")
	} else if m.connected {
		connStatus = lipgloss.NewStyle().Foreground(t.Success).Render("● connected")
	}
	bar := fmt.Sprintf(" %s │ %s │ %s", model, session, connStatus)
	if m.quitHint {
		bar += lipgloss.NewStyle().Foreground(t.Warning).Render("  Press Ctrl+C again to quit")
	}
	return lipgloss.NewStyle().Width(m.width).Background(t.Panel).Foreground(t.Text).Render(bar)
}
