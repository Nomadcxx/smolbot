package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/provider"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

const (
	minimaxAuthTimeout = 5 * time.Minute
	minimaxProfileID   = "minimax-portal:default"
)

func init() {
	// Register minimax subcommand via newAuthCmd hook.
	registerMiniMaxAuth = func(parent *cobra.Command, opts *rootOptions) {
		parent.AddCommand(newAuthMiniMaxCmd(opts))
	}
}

func newAuthMiniMaxCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "minimax",
		Short: "Authenticate with MiniMax via OAuth",
		Long: `Authenticate with MiniMax using OAuth device code flow.

Opens a browser to verify a user code. After authentication,
MiniMax models (MiniMax-M2.5, MiniMax-M2.7) are available via the minimax-portal provider.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath := opts.configPath
			if configPath == "" {
				configPath = defaultConfigPath(*opts)
			}
			cfg, paths, err := loadRuntimeConfig(configPath, opts.workspace, 0)
			if err != nil {
				return err
			}
			return runMiniMaxAuth(cmd.Context(), cfg, paths, configPath)
		},
	}
	return cmd
}

// --- MiniMax device code TUI model ---

type minimaxAuthModel struct {
	provider  *provider.MiniMaxOAuthProvider
	store     config.OAuthTokenStore
	spinner   spinner.Model
	status    string
	dc        *provider.DeviceCodeResponse
	state     string
	err       error
	done      bool
	token     *provider.TokenInfo
	ctx       context.Context
	cancel    context.CancelFunc
	startTime time.Time
}

type minimaxDCMsg struct {
	dc    *provider.DeviceCodeResponse
	state string
}
type minimaxTokenMsg struct{ token *provider.TokenInfo }
type minimaxErrMsg struct{ err error }

func (m *minimaxAuthModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.startDeviceCodeFlow())
}

func (m *minimaxAuthModel) startDeviceCodeFlow() tea.Cmd {
	return func() tea.Msg {
		dc, state, err := m.provider.InitiateAuth(m.ctx)
		if err != nil {
			return minimaxErrMsg{err: fmt.Errorf("initiate auth: %w", err)}
		}
		return minimaxDCMsg{dc: dc, state: state}
	}
}

func (m *minimaxAuthModel) pollForToken() tea.Cmd {
	return func() tea.Msg {
		token, err := m.provider.PollForToken(m.ctx, m.dc, m.state)
		if err != nil {
			return minimaxErrMsg{err: err}
		}
		return minimaxTokenMsg{token: token}
	}
}

func (m *minimaxAuthModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case minimaxDCMsg:
		m.dc = msg.dc
		m.state = msg.state
		m.status = "Waiting for browser verification..."
		_ = openURL(msg.dc.VerificationURI)
		return m, m.pollForToken()

	case minimaxTokenMsg:
		m.token = msg.token
		m.done = true
		m.status = "Authenticated with MiniMax!"
		if err := m.saveToken(msg.token); err != nil {
			m.err = fmt.Errorf("save token: %w", err)
		}
		return m, tea.Quit

	case minimaxErrMsg:
		m.err = msg.err
		return m, tea.Quit

	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "ctrl+c":
			m.cancel()
			m.err = fmt.Errorf("cancelled")
			return m, tea.Quit
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *minimaxAuthModel) View() string {
	if m.err != nil {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("1")).
			Render(fmt.Sprintf("\n  Error: %v\n", m.err))
	}

	if m.done {
		return lipgloss.NewStyle().
			Foreground(lipgloss.Color("2")).
			Render(fmt.Sprintf("\n  ✓ %s\n", m.status))
	}

	var s strings.Builder
	s.WriteString("\n")
	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("208")).
		Render("  MiniMax OAuth Authentication"))
	s.WriteString("\n\n")

	if m.dc != nil {
		s.WriteString("  Visit: ")
		s.WriteString(m.dc.VerificationURI)
		s.WriteString("\n  Code:  ")
		s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3")).
			Render(m.dc.UserCode))
		s.WriteString("\n\n")
	}

	s.WriteString("  ")
	s.WriteString(m.spinner.View())
	s.WriteString(" ")
	s.WriteString(m.status)
	s.WriteString("\n\n")

	remaining := minimaxAuthTimeout - time.Since(m.startTime)
	if remaining < 0 {
		remaining = 0
	}
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).
		Render(fmt.Sprintf("  Timeout: %v remaining", remaining.Round(time.Second))))
	s.WriteString("\n\n")
	s.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Italic(true).
		Render("  Press Esc or Ctrl+C to cancel"))

	return s.String()
}

func (m *minimaxAuthModel) saveToken(tok *provider.TokenInfo) error {
	if m.store == nil {
		return nil
	}
	entry := &config.TokenStoreEntry{
		AccessToken:  tok.AccessToken,
		RefreshToken: tok.RefreshToken,
		ExpiresAt:    tok.ExpiresAt,
		TokenType:    tok.TokenType,
		Scope:        tok.Scope,
		ProviderID:   "minimax-portal",
		ProfileID:    minimaxProfileID,
		UpdatedAt:    time.Now(),
	}
	return m.store.Save("minimax-portal", minimaxProfileID, entry)
}

func runMiniMaxAuth(ctx context.Context, cfg *config.Config, paths *config.Paths, configPath string) error {
	store, err := config.NewOAuthTokenStore(paths)
	if err != nil {
		return fmt.Errorf("init token store: %w", err)
	}

	p := provider.NewMiniMaxOAuthProvider("minimax-portal")

	ctx, cancel := context.WithTimeout(ctx, minimaxAuthTimeout)
	sp := spinner.New()
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("208"))

	model := &minimaxAuthModel{
		provider:  p,
		store:     store,
		spinner:   sp,
		status:    "Initiating device code flow...",
		ctx:       ctx,
		cancel:    cancel,
		startTime: time.Now(),
	}

	prog := tea.NewProgram(model, tea.WithContext(ctx), tea.WithInput(os.Stdin))
	if _, err := prog.Run(); err != nil {
		cancel()
		return err
	}
	cancel()

	if model.err != nil {
		return model.err
	}

	// Ensure main config has the provider entry with authType=oauth
	if cfg.Providers == nil {
		cfg.Providers = make(map[string]config.ProviderConfig)
	}
	if pc, ok := cfg.Providers["minimax-portal"]; !ok || pc.AuthType != "oauth" {
		cfg.Providers["minimax-portal"] = config.ProviderConfig{
			AuthType:  "oauth",
			ProfileID: minimaxProfileID,
			APIBase:   "https://api.minimax.io",
		}
		if configPath != "" {
			if err := config.AtomicWriteConfig(configPath, cfg); err != nil {
				fmt.Fprintf(os.Stderr, "  Warning: could not update config: %v\n", err)
				fmt.Fprintf(os.Stderr, "  Add minimax-portal provider manually with authType: oauth\n")
			} else {
				fmt.Println()
				fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).
					Render("  Provider config updated automatically."))
			}
		}
	}

	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).
		Render("  MiniMax models are now available via the minimax-portal provider."))
	fmt.Println()
	return nil
}
