package footer

import (
	"fmt"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/nanobot-go/internal/tui/theme"
)

type Model struct {
	width            int
	model            string
	provider         string
	session          string
	uptimeSeconds    int
	connectedClients int
	channels         []string
}

func New() Model { return Model{} }

func (m *Model) SetWidth(w int)                 { m.width = w }
func (m *Model) SetModel(v string)              { m.model = v }
func (m *Model) SetProvider(v string)           { m.provider = v }
func (m *Model) SetSession(v string)            { m.session = v }
func (m *Model) SetUptimeSeconds(v int)         { m.uptimeSeconds = v }
func (m *Model) SetConnectedClients(v int)      { m.connectedClients = v }
func (m *Model) SetChannels(v []string)         { m.channels = append([]string(nil), v...) }
func (m Model) Height() int                     { return 1 }

func (m Model) View() string {
	t := theme.Current()
	if t == nil {
		return ""
	}

	model := m.model
	if model == "" {
		model = "connecting..."
	}
	provider := m.provider
	if provider == "" {
		provider = "provider unknown"
	}
	session := m.session
	if session == "" {
		session = "session unknown"
	}
	channels := "none"
	if len(m.channels) > 0 {
		channels = strings.Join(m.channels, ",")
	}

	bar := fmt.Sprintf(
		" model %s │ provider %s │ session %s │ uptime %ds │ clients %d │ channels %s ",
		model,
		provider,
		session,
		m.uptimeSeconds,
		m.connectedClients,
		channels,
	)

	return lipgloss.NewStyle().
		Width(m.width).
		Foreground(t.TextMuted).
		Background(t.Panel).
		Render(bar)
}
