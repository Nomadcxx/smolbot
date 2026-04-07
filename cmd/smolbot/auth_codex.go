package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/provider"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

const codexAuthTimeout = 5 * time.Minute

// registerMiniMaxAuth is set by auth_minimax.go init; called from newAuthCmd.
var registerMiniMaxAuth func(parent *cobra.Command, opts *rootOptions)

func newAuthCmd(opts *rootOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Authenticate with OAuth providers",
	}
	cmd.AddCommand(newAuthCodexCmd(opts))
	if registerMiniMaxAuth != nil {
		registerMiniMaxAuth(cmd, opts)
	}
	return cmd
}

func newAuthCodexCmd(opts *rootOptions) *cobra.Command {
	var useDeviceCode bool

	cmd := &cobra.Command{
		Use:   "codex",
		Short: "Authenticate with OpenAI Codex via OAuth",
		Long: `Authenticate with OpenAI Codex using your ChatGPT Plus/Pro subscription.

By default, opens a browser for login. Use --device-code for headless environments.
After authentication, Codex models (gpt-5.x) are available at zero additional cost.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			configPath := opts.configPath
			if configPath == "" {
				configPath = defaultConfigPath(*opts)
			}
			cfg, paths, err := loadRuntimeConfig(configPath, opts.workspace, 0)
			if err != nil {
				return err
			}

			if !useDeviceCode && !canOpenBrowser() {
				useDeviceCode = true
			}

			if useDeviceCode {
				return runCodexDeviceCodeAuth(cmd.Context(), cfg, paths)
			}
			return runCodexBrowserAuth(cmd.Context(), cfg, paths)
		},
	}

	cmd.Flags().BoolVar(&useDeviceCode, "device-code", false, "Use device code flow (for headless/SSH environments)")
	return cmd
}

func canOpenBrowser() bool {
	if os.Getenv("SSH_CONNECTION") != "" || os.Getenv("SSH_TTY") != "" {
		return false
	}
	if runtime.GOOS == "linux" && os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		return false
	}
	return true
}

// --- Browser flow ---

type codexBrowserModel struct {
	client    *provider.CodexOAuthClient
	store     config.OAuthTokenStore
	spinner   spinner.Model
	status    string
	authURL   string
	err       error
	done      bool
	token     *provider.CodexTokenResponse
	ctx       context.Context
	cancel    context.CancelFunc
	startTime time.Time
}

type codexAuthURLMsg struct{ url string }
type codexTokenMsg struct{ token *provider.CodexTokenResponse }
type codexErrMsg struct{ err error }

func (m *codexBrowserModel) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, m.startBrowserFlow())
}

func (m *codexBrowserModel) startBrowserFlow() tea.Cmd {
	return func() tea.Msg {
		auth, err := m.client.StartBrowserAuthorization(m.ctx)
		if err != nil {
			return codexErrMsg{err: fmt.Errorf("start browser auth: %w", err)}
		}

		_ = openURL(auth.AuthURL)

		token, err := auth.Wait(m.ctx)
		if err != nil {
			return codexErrMsg{err: err}
		}
		return codexTokenMsg{token: token}
	}
}

func (m *codexBrowserModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case codexTokenMsg:
		m.token = msg.token
		m.done = true
		m.status = "Authenticated successfully!"
		if err := m.saveToken(msg.token); err != nil {
			m.err = fmt.Errorf("save token: %w", err)
		}
		return m, tea.Quit

	case codexErrMsg:
		m.err = msg.err
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

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *codexBrowserModel) View() string {
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
	s.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).
		Render("  OpenAI Codex Authentication"))
	s.WriteString("\n\n")
	s.WriteString("  ")
	s.WriteString(m.spinner.View())
	s.WriteString(" Waiting for browser login...")
	s.WriteString("\n\n")

	remaining := codexAuthTimeout - time.Since(m.startTime)
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

func (m *codexBrowserModel) saveToken(tok *provider.CodexTokenResponse) error {
	if m.store == nil {
		return nil
	}
	info := tok.TokenInfo(time.Now())
	entry := &config.TokenStoreEntry{
		AccessToken:  info.AccessToken,
		RefreshToken: info.RefreshToken,
		ExpiresAt:    info.ExpiresAt,
		TokenType:    info.TokenType,
		Scope:        info.Scope,
		ProviderID:   "openai-codex",
		ProfileID:    "openai-codex:default",
		UpdatedAt:    time.Now(),
	}
	return m.store.Save("openai-codex", "openai-codex:default", entry)
}

func runCodexBrowserAuth(ctx context.Context, _ *config.Config, paths *config.Paths) error {
	store, err := config.NewOAuthTokenStore(paths)
	if err != nil {
		return fmt.Errorf("init token store: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, codexAuthTimeout)
	sp := spinner.New()
	sp.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("86"))

	model := &codexBrowserModel{
		client:    provider.NewCodexOAuthClient(),
		store:     store,
		spinner:   sp,
		status:    "Opening browser...",
		ctx:       ctx,
		cancel:    cancel,
		startTime: time.Now(),
	}

	p := tea.NewProgram(model, tea.WithContext(ctx), tea.WithInput(os.Stdin))
	if _, err := p.Run(); err != nil {
		cancel()
		return err
	}
	cancel()

	if model.err != nil {
		return model.err
	}
	return nil
}

// --- Device code flow ---

func runCodexDeviceCodeAuth(ctx context.Context, _ *config.Config, paths *config.Paths) error {
	store, err := config.NewOAuthTokenStore(paths)
	if err != nil {
		return fmt.Errorf("init token store: %w", err)
	}

	client := provider.NewCodexOAuthClient()
	ctx, cancel := context.WithTimeout(ctx, codexAuthTimeout)
	defer cancel()

	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).
		Render("  OpenAI Codex Authentication (Device Code)"))
	fmt.Println()

	dc, err := client.InitiateDeviceAuthorization(ctx)
	if err != nil {
		return fmt.Errorf("initiate device auth: %w", err)
	}

	fmt.Printf("  Visit: %s\n", dc.VerificationURL)
	fmt.Printf("  Code:  %s\n\n", lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("3")).
		Render(dc.UserCode))
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).
		Render("  Waiting for authorization..."))

	tokenResp, err := client.PollDeviceAuthorization(ctx, dc)
	if err != nil {
		return fmt.Errorf("device auth: %w", err)
	}

	info := tokenResp.TokenInfo(time.Now())
	entry := &config.TokenStoreEntry{
		AccessToken:  info.AccessToken,
		RefreshToken: info.RefreshToken,
		ExpiresAt:    info.ExpiresAt,
		TokenType:    info.TokenType,
		Scope:        info.Scope,
		ProviderID:   "openai-codex",
		ProfileID:    "openai-codex:default",
		UpdatedAt:    time.Now(),
	}
	if err := store.Save("openai-codex", "openai-codex:default", entry); err != nil {
		return fmt.Errorf("save token: %w", err)
	}

	fmt.Println()
	fmt.Println(lipgloss.NewStyle().Foreground(lipgloss.Color("2")).
		Render("  ✓ Authenticated successfully!"))
	fmt.Println()
	return nil
}

func openURL(u string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd = "rundll32"
		args = []string{"url.dll,FileProtocolHandler"}
	default:
		cmd = "xdg-open"
	}
	args = append(args, u)
	return exec.Command(cmd, args...).Start()
}
