package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/channel"
	"github.com/Nomadcxx/smolbot/pkg/channel/whatsapp"
	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

const loginTimeout = 5 * time.Minute

type whatsappLoginModel struct {
	cfg       *config.Config
	qrCode    string
	qrASCII   string
	status    string
	err       error
	done      bool
	connected bool
	spinner   spinner.Model
	ctx       context.Context
	cancel    context.CancelFunc
	renderer  *whatsapp.QRRenderer
	program   *tea.Program
	startTime time.Time
}

func newWhatsAppLoginModel(cfg *config.Config) (*whatsappLoginModel, error) {
	ctx, cancel := context.WithTimeout(context.Background(), loginTimeout)

	m := &whatsappLoginModel{
		cfg:       cfg,
		status:    "Initializing...",
		spinner:   spinner.New(),
		ctx:       ctx,
		cancel:    cancel,
		renderer:  whatsapp.NewQRRenderer(256),
		startTime: time.Now(),
	}
	m.spinner.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

	return m, nil
}

func (m *whatsappLoginModel) Init() tea.Cmd {
	return m.startLogin()
}

func (m *whatsappLoginModel) startLogin() tea.Cmd {
	return func() tea.Msg {
		waCfg := m.cfg.Channels.WhatsApp
		adapter, err := whatsapp.NewProductionAdapter(waCfg)
		if err != nil {
			return errorMsg{err: fmt.Errorf("create adapter: %w", err)}
		}

		err = adapter.LoginWithUpdates(m.ctx, func(status channel.Status) error {
			if m.program != nil {
				switch status.State {
				case "qr":
					m.program.Send(qrMsg{code: status.Detail})
					m.program.Send(statusMsg{text: "Scan the QR code with your phone"})
				case "connected":
					m.program.Send(connectedMsg{})
				case "device-link":
					m.program.Send(statusMsg{text: "Linking device..."})
				default:
					if status.Detail != "" {
						m.program.Send(statusMsg{text: status.Detail})
					}
				}
			}
			return nil
		})

		if err != nil {
			if m.ctx.Err() == context.DeadlineExceeded {
				return errorMsg{err: fmt.Errorf("login timed out after %v", loginTimeout)}
			}
			return errorMsg{err: err}
		}

		return connectedMsg{}
	}
}

func (m *whatsappLoginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case qrMsg:
		m.qrCode = msg.code
		m.qrASCII, _ = m.renderer.RenderToASCII(msg.code)
		return m, nil

	case statusMsg:
		m.status = msg.text
		return m, nil

	case connectedMsg:
		m.connected = true
		m.done = true
		m.status = "Device linked successfully!"
		return m, tea.Quit

	case errorMsg:
		m.err = msg.err
		m.done = true
		return m, tea.Quit

	case timeoutMsg:
		m.err = fmt.Errorf("login timed out after %v", loginTimeout)
		m.done = true
		return m, tea.Quit

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			m.cancel()
			m.err = fmt.Errorf("cancelled")
			m.done = true
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m *whatsappLoginModel) View() string {
	if m.err != nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Render(fmt.Sprintf("\n  Error: %v\n\n  Press any key to exit.\n", m.err))
	}

	if m.connected {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("2")).
			Render(fmt.Sprintf("\n  ✓ %s\n\n  Press any key to exit.\n", m.status))
	}

	var s strings.Builder

	s.WriteString("\n")
	s.WriteString(lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Render("  WhatsApp Device Linking"))
	s.WriteString("\n\n")

	if m.qrASCII != "" {
		s.WriteString(m.qrASCII)
		s.WriteString("\n")
	} else {
		s.WriteString("  ")
		s.WriteString(m.spinner.View())
		s.WriteString(" ")
		s.WriteString(m.status)
		s.WriteString("\n")
	}

	remaining := loginTimeout - time.Since(m.startTime)
	if remaining < 0 {
		remaining = 0
	}

	s.WriteString("\n")
	s.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render(fmt.Sprintf("  Timeout: %v remaining", remaining.Round(time.Second))))

	s.WriteString("\n\n")
	s.WriteString(lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true).
		Render("  Press Esc or Ctrl+C to cancel"))

	return s.String()
}

type qrMsg struct{ code string }
type statusMsg struct{ text string }
type connectedMsg struct{}
type timeoutMsg struct{}
type errorMsg struct{ err error }

func runWhatsAppLogin(ctx context.Context, opts rootOptions) error {
	configPath := opts.configPath
	if configPath == "" {
		configPath = defaultConfigPath(opts)
	}

	cfg, _, err := loadRuntimeConfig(configPath, opts.workspace, 0)
	if err != nil {
		return err
	}

	model, err := newWhatsAppLoginModel(cfg)
	if err != nil {
		return err
	}

	p := tea.NewProgram(model,
		tea.WithContext(ctx),
		tea.WithInput(os.Stdin),
	)

	model.program = p

	if _, err := p.Run(); err != nil {
		return err
	}

	if model.err != nil {
		return model.err
	}

	return nil
}
