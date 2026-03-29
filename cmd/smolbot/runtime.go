package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/agent"
	"github.com/Nomadcxx/smolbot/pkg/channel"
	discordchannel "github.com/Nomadcxx/smolbot/pkg/channel/discord"
	signalchannel "github.com/Nomadcxx/smolbot/pkg/channel/signal"
	telegramchannel "github.com/Nomadcxx/smolbot/pkg/channel/telegram"
	whatsappchannel "github.com/Nomadcxx/smolbot/pkg/channel/whatsapp"
	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/cron"
	"github.com/Nomadcxx/smolbot/pkg/gateway"
	"github.com/Nomadcxx/smolbot/pkg/heartbeat"
	"github.com/Nomadcxx/smolbot/pkg/mcp"
	"github.com/Nomadcxx/smolbot/pkg/provider"
	"github.com/Nomadcxx/smolbot/pkg/session"
	"github.com/Nomadcxx/smolbot/pkg/skill"
	"github.com/Nomadcxx/smolbot/pkg/tokenizer"
	"github.com/Nomadcxx/smolbot/pkg/tool"
	"github.com/Nomadcxx/smolbot/pkg/usage"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

type daemonLaunchOptions struct {
	ConfigPath string
	Workspace  string
	Verbose    bool
	Port       int
}

type statusReport struct {
	Model            string
	UptimeSeconds    int
	Channels         []string
	ChannelStates    map[string]map[string]string
	ConnectedClients int
}

type channelStatus struct {
	Name   string
	State  string
	Detail string
}

type chatRequest struct {
	Session    string
	Message    string
	Markdown   bool
	ConfigPath string
	Workspace  string
}

type runtimeDeps struct {
	Provider          provider.Provider
	ProviderRegistry  *provider.Registry
	Channels          []channel.Channel
	CronRun           func(context.Context, time.Time) error
	CronInterval      time.Duration
	HeartbeatRun      func(context.Context) error
	HeartbeatInterval time.Duration
	HeartbeatEnabled  bool
	QuotaRun          func(context.Context) error
	QuotaInterval     time.Duration
	SetModelCallback  func(model string) (string, error)
}

type runtimeApp struct {
	config           *config.Config
	paths            *config.Paths
	sessions         *session.Store
	usage            *usage.Store
	mcpCleanup       func()
	channels         *channel.Manager
	tools            *tool.Registry
	agent            *agent.AgentLoop
	providerRegistry *provider.Registry
	cron             *cron.Service
	heartbeat        *heartbeat.Service
	runCron          func(context.Context, time.Time) error
	runBeat          func(context.Context) error
	runQuota         func(context.Context) error
	cronEvery        time.Duration
	beatEvery        time.Duration
	quotaEvery       time.Duration
	beatOn           bool
	gateway          *gateway.Server
}

type runtimeSpawner struct {
	loop *agent.AgentLoop
}

type modelRoutingProvider struct {
	cfg      *config.Config
	resolver interface {
		ForModel(model string) (provider.Provider, error)
	}
}

func (p *modelRoutingProvider) Chat(ctx context.Context, req provider.ChatRequest) (*provider.Response, error) {
	resolved, err := p.resolve(req.Model)
	if err != nil {
		return nil, err
	}
	return resolved.Chat(ctx, req)
}

func (p *modelRoutingProvider) ChatStream(ctx context.Context, req provider.ChatRequest) (*provider.Stream, error) {
	resolved, err := p.resolve(req.Model)
	if err != nil {
		return nil, err
	}
	return resolved.ChatStream(ctx, req)
}

func (p *modelRoutingProvider) Name() string {
	return p.NameForModel("")
}

func (p *modelRoutingProvider) NameForModel(model string) string {
	resolved, err := p.resolve(model)
	if err != nil || resolved == nil {
		if p.cfg != nil {
			return p.cfg.Agents.Defaults.Provider
		}
		return ""
	}
	return resolved.Name()
}

func (p *modelRoutingProvider) resolve(model string) (provider.Provider, error) {
	if p == nil || p.resolver == nil {
		return nil, fmt.Errorf("provider resolver unavailable")
	}
	if strings.TrimSpace(model) == "" && p.cfg != nil {
		model = p.cfg.Agents.Defaults.Model
	}
	return p.resolver.ForModel(model)
}

type mcpDiscoveryManager interface {
	DiscoverAndRegister(ctx context.Context, registry *tool.Registry, servers map[string]config.MCPServerConfig) ([]string, error)
}

var newMCPMgr = func() (mcpDiscoveryManager, func()) {
	client := mcp.NewStdioDiscoveryClient(slog.Default())
	return mcp.NewManager(client), client.Close
}

var launchRuntimeDeps = func() runtimeDeps {
	return runtimeDeps{}
}

var launchDaemon = launchDaemonImpl

var fetchStatus = fetchStatusImpl

var fetchChannelStatuses = fetchChannelStatusesImpl

var runChannelLogin = runChannelLoginImpl

var newSignalChannel = func(cfg config.SignalChannelConfig) channel.Channel {
	return signalchannel.NewAdapter(cfg, nil)
}

var newWhatsAppChannel = func(cfg config.WhatsAppChannelConfig) (channel.Channel, error) {
	return whatsappchannel.NewProductionAdapter(cfg)
}

var newTelegramChannel = func(cfg config.TelegramChannelConfig) (channel.Channel, error) {
	return telegramchannel.NewProductionAdapter(cfg)
}

var newDiscordChannel = func(cfg config.DiscordChannelConfig) (channel.Channel, error) {
	return discordchannel.NewProductionAdapter(cfg)
}

var runChatRuntimeDeps = func() runtimeDeps {
	return runtimeDeps{}
}

var runChatMessage = func(ctx context.Context, req chatRequest) (string, error) {
	opts := daemonLaunchOptions{
		ConfigPath: firstNonEmpty(req.ConfigPath, defaultConfigPath(rootOptions{})),
		Workspace:  req.Workspace,
	}
	app, err := buildRuntime(opts, runChatRuntimeDeps())
	if err != nil {
		return "", err
	}
	defer app.Close()

	sessionKey := req.Session
	if sessionKey == "" {
		sessionKey = "cli:main"
	}
	return app.agent.ProcessDirect(ctx, agent.Request{
		Content:    req.Message,
		SessionKey: sessionKey,
	}, nil)
}

var collectOnboardConfig = func(ctx context.Context, opts rootOptions) (*config.Config, error) {
	return collectOnboardConfigFromIO(ctx, opts, os.Stdin, os.Stdout)
}

var writeConfigFile = func(path string, cfg *config.Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

var writeSoulFile = func(workspace, content string) error {
	if err := os.MkdirAll(workspace, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(workspace, "SOUL.md"), []byte(content), 0o644)
}

func collectOnboardConfigFromIO(ctx context.Context, opts rootOptions, in io.Reader, out io.Writer) (*config.Config, error) {
	_ = ctx
	cfg := config.DefaultConfig()
	if opts.workspace != "" {
		cfg.Agents.Defaults.Workspace = opts.workspace
	}

	reader := bufio.NewReader(in)
	providerName, err := promptWithDefault(reader, out, "Provider", firstNonEmpty(cfg.Agents.Defaults.Provider, "openai"))
	if err != nil {
		return nil, err
	}
	cfg.Agents.Defaults.Provider = providerName

	modelName, err := promptWithDefault(reader, out, "Model", cfg.Agents.Defaults.Model)
	if err != nil {
		return nil, err
	}
	cfg.Agents.Defaults.Model = modelName

	if strings.EqualFold(strings.TrimSpace(providerName), "ollama") {
		quotaEnabled, err := promptWithDefault(reader, out, "Enable quota setup", "y")
		if err != nil {
			return nil, err
		}
		if isYes(quotaEnabled) {
			cfg.Quota.RefreshIntervalMinutes = 60
			cfg.Quota.Providers = map[string]config.ProviderQuotaConfig{
				"ollama": {
					Enabled:                        true,
					BrowserCookieDiscoveryEnabled:  true,
				},
			}
		}
	}

	providerCfg := cfg.Providers[providerName]
	apiKey, err := promptWithDefault(reader, out, "API key", providerCfg.APIKey)
	if err != nil {
		return nil, err
	}
	providerCfg.APIKey = apiKey
	cfg.Providers[providerName] = providerCfg

	workspace, err := promptWithDefault(reader, out, "Workspace", cfg.Agents.Defaults.Workspace)
	if err != nil {
		return nil, err
	}
	cfg.Agents.Defaults.Workspace = workspace

	portValue, err := promptWithDefault(reader, out, "Gateway port", strconv.Itoa(cfg.Gateway.Port))
	if err != nil {
		return nil, err
	}
	if portValue != "" {
		port, err := strconv.Atoi(portValue)
		if err != nil {
			return nil, fmt.Errorf("invalid gateway port %q", portValue)
		}
		cfg.Gateway.Port = port
	}

	heartbeatEnabled, err := promptWithDefault(reader, out, "Enable heartbeat", "n")
	if err != nil {
		return nil, err
	}
	cfg.Gateway.Heartbeat.Enabled = strings.EqualFold(strings.TrimSpace(heartbeatEnabled), "y") || strings.EqualFold(strings.TrimSpace(heartbeatEnabled), "yes")
	if cfg.Gateway.Heartbeat.Enabled {
		channelName, err := promptWithDefault(reader, out, "Heartbeat channel", cfg.Gateway.Heartbeat.Channel)
		if err != nil {
			return nil, err
		}
		cfg.Gateway.Heartbeat.Channel = channelName
	}

	signalEnabled, err := promptWithDefault(reader, out, "Enable Signal channel", yesNoDefault(cfg.Channels.Signal.Enabled))
	if err != nil {
		return nil, err
	}
	cfg.Channels.Signal.Enabled = isYes(signalEnabled)
	if cfg.Channels.Signal.Enabled {
		account, err := promptWithDefault(reader, out, "Signal account", cfg.Channels.Signal.Account)
		if err != nil {
			return nil, err
		}
		cfg.Channels.Signal.Account = account

		cliPath, err := promptWithDefault(reader, out, "Signal CLI path", cfg.Channels.Signal.CLIPath)
		if err != nil {
			return nil, err
		}
		cfg.Channels.Signal.CLIPath = cliPath

		dataDir, err := promptWithDefault(reader, out, "Signal data dir", cfg.Channels.Signal.DataDir)
		if err != nil {
			return nil, err
		}
		cfg.Channels.Signal.DataDir = dataDir
	}

	whatsAppEnabled, err := promptWithDefault(reader, out, "Enable WhatsApp channel", yesNoDefault(cfg.Channels.WhatsApp.Enabled))
	if err != nil {
		return nil, err
	}
	cfg.Channels.WhatsApp.Enabled = isYes(whatsAppEnabled)
	if cfg.Channels.WhatsApp.Enabled {
		deviceName, err := promptWithDefault(reader, out, "WhatsApp device name", cfg.Channels.WhatsApp.DeviceName)
		if err != nil {
			return nil, err
		}
		cfg.Channels.WhatsApp.DeviceName = deviceName

		storePath, err := promptWithDefault(reader, out, "WhatsApp store path", cfg.Channels.WhatsApp.StorePath)
		if err != nil {
			return nil, err
		}
		cfg.Channels.WhatsApp.StorePath = storePath
	}

	tone, err := promptWithDefault(reader, out, "Tone", "Be direct and calm. Prefer clarity over flourish.")
	if err != nil {
		return nil, err
	}

	boundaries, err := promptWithDefault(reader, out, "Boundaries", "Do not invent capabilities that are not wired. Surface gaps plainly when incomplete.")
	if err != nil {
		return nil, err
	}

	expertise, err := promptWithDefault(reader, out, "Expertise", "General coding assistance, debugging, and system maintenance.")
	if err != nil {
		return nil, err
	}

	canDo, err := promptWithDefault(reader, out, "Can do", "Read and write files, execute commands, search the web, manage sessions.")
	if err != nil {
		return nil, err
	}

	if err := validateChannelConfig(cfg); err != nil {
		return nil, err
	}

	if err := writeSoulFile(cfg.Agents.Defaults.Workspace, renderSoulDocument(tone, boundaries, expertise, canDo)); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func renderSoulDocument(tone, boundaries, expertise, canDo string) string {
	tone = strings.TrimSpace(tone)
	if tone == "" {
		tone = "Be direct and calm. Prefer clarity over flourish. Stay technically precise and grounded in what is actually implemented."
	}
	boundaries = strings.TrimSpace(boundaries)
	if boundaries == "" {
		boundaries = "Do not invent capabilities that are not wired. Surface gaps plainly when the runtime or product is still incomplete. Prefer pragmatic, testable changes over broad rewrites."
	}
	expertise = strings.TrimSpace(expertise)
	if expertise == "" {
		expertise = "General coding assistance, debugging, and system maintenance."
	}
	canDo = strings.TrimSpace(canDo)
	if canDo == "" {
		canDo = "Read and write files, execute commands, search the web, manage channels and sessions, run tests and linters, analyze code structure."
	}
	return strings.TrimSpace(fmt.Sprintf(`# SOUL.md - Who You Are

## Who You Are

You are smolbot, a practical coding agent with a clear working style.

## Tone

- %s

## Boundaries

- %s

## Expertise

- %s

## Can do

- %s
`, tone, boundaries, expertise, canDo)) + "\n"
}

func validateChannelConfig(cfg config.Config) error {
	if cfg.Channels.Signal.Enabled {
		if strings.TrimSpace(cfg.Channels.Signal.Account) == "" {
			return fmt.Errorf("signal account cannot be empty when Signal channel is enabled")
		}
		cliPath := strings.TrimSpace(cfg.Channels.Signal.CLIPath)
		if cliPath != "" && cliPath != "signal-cli" {
			if _, err := os.Stat(cliPath); err != nil {
				return fmt.Errorf("signal CLI path %q is not accessible: %w", cliPath, err)
			}
		}
	}
	if cfg.Channels.WhatsApp.Enabled {
		storePath := strings.TrimSpace(cfg.Channels.WhatsApp.StorePath)
		if storePath == "" {
			return fmt.Errorf("whatsapp store path cannot be empty when WhatsApp channel is enabled")
		}
		parentDir := filepath.Dir(storePath)
		if parentDir != "." {
			parentInfo, err := os.Stat(parentDir)
			if err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("whatsapp store path %q has nonexistent parent directory: %s", storePath, parentDir)
				}
				return fmt.Errorf("whatsapp store path %q parent is inaccessible: %w", storePath, err)
			}
			if !parentInfo.IsDir() {
				return fmt.Errorf("whatsapp store path %q parent %q is a file, not a directory", storePath, parentDir)
			}
		}
	}
	return nil
}

func promptWithDefault(reader *bufio.Reader, out io.Writer, label, defaultValue string) (string, error) {
	prompt := label
	if defaultValue != "" {
		prompt += " [" + defaultValue + "]"
	}
	prompt += ": "
	if _, err := fmt.Fprint(out, prompt); err != nil {
		return "", err
	}
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultValue, nil
	}
	return line, nil
}

func yesNoDefault(enabled bool) string {
	if enabled {
		return "y"
	}
	return "n"
}

func isYes(value string) bool {
	value = strings.TrimSpace(value)
	return strings.EqualFold(value, "y") || strings.EqualFold(value, "yes")
}

func defaultConfigPath(opts rootOptions) string {
	if opts.configPath != "" {
		return opts.configPath
	}
	return config.DefaultPaths().ConfigFile()
}

func formatStatus(report *statusReport) string {
	return fmt.Sprintf(
		"model: %s\nuptime: %d\nchannels: %v\nconnected clients: %d\n",
		report.Model,
		report.UptimeSeconds,
		report.Channels,
		report.ConnectedClients,
	)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func launchDaemonImpl(ctx context.Context, opts daemonLaunchOptions) error {
	app, err := buildRuntime(opts, launchRuntimeDeps())
	if err != nil {
		return err
	}
	defer app.Close()
	app.channels.SetInboundHandler(app.handleInbound)
	if err := app.channels.Start(ctx); err != nil {
		return err
	}
	defer func() {
		_ = app.channels.Stop(context.Background())
	}()
	bgErrCh := make(chan error, 2)
	bgCtx, bgCancel := context.WithCancel(ctx)
	defer bgCancel()
	startRuntimeLoops(bgCtx, app, bgErrCh)

	addr := app.config.Gateway.Host + ":" + strconv.Itoa(app.config.Gateway.Port)
	httpServer := &http.Server{
		Addr:    addr,
		Handler: app.gateway.Handler(),
	}

	errCh := make(chan error, 1)
	go func() {
		err := httpServer.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	shutdownServer := func() error {
		bgCancel()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownCtx); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		if err := app.gateway.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return nil
	}

	select {
	case <-ctx.Done():
		if err := shutdownServer(); err != nil {
			return err
		}
		select {
		case err := <-bgErrCh:
			if err != nil {
				return err
			}
		default:
		}
		return <-errCh
	case err := <-bgErrCh:
		if shutdownErr := shutdownServer(); shutdownErr != nil {
			return shutdownErr
		}
		if err != nil {
			return err
		}
		return <-errCh
	case err := <-errCh:
		if shutdownErr := shutdownServer(); shutdownErr != nil {
			return shutdownErr
		}
		return err
	}
}

func buildRuntime(opts daemonLaunchOptions, deps runtimeDeps) (*runtimeApp, error) {
	cfg, paths, err := loadRuntimeConfig(opts.ConfigPath, opts.Workspace, opts.Port)
	if err != nil {
		return nil, err
	}
	if err := ensureRuntimePaths(paths); err != nil {
		return nil, err
	}

	// Validate heartbeat config
	if cfg.Gateway.Heartbeat.Enabled && cfg.Gateway.Heartbeat.Interval <= 0 {
		return nil, fmt.Errorf("heartbeat enabled but interval is %d (must be > 0)", cfg.Gateway.Heartbeat.Interval)
	}

	if err := agent.SyncWorkspaceTemplates(paths.Workspace()); err != nil {
		return nil, err
	}

	skills, err := skill.NewRegistry(paths)
	if err != nil {
		return nil, err
	}
	sessions, err := session.NewStore(paths.SessionsDB())
	if err != nil {
		return nil, err
	}
	usageStore, err := usage.NewStore(paths.UsageDB())
	if err != nil {
		_ = sessions.Close()
		return nil, err
	}

	providerRegistry := deps.ProviderRegistry
	if providerRegistry == nil && deps.Provider == nil {
		providerRegistry = provider.NewRegistryWithDefaults(cfg)
	}

	runtimeProvider := deps.Provider
	if providerRegistry != nil {
		runtimeProvider = &modelRoutingProvider{
			cfg:      cfg,
			resolver: providerRegistry,
		}
	} else if runtimeProvider == nil {
		_ = usageStore.Close()
		_ = sessions.Close()
		return nil, fmt.Errorf("provider unavailable")
	}

	channels := channel.NewManager()
	configured, err := configuredChannels(cfg, false)
	if err != nil {
		_ = usageStore.Close()
		_ = sessions.Close()
		return nil, err
	}
	for _, registered := range configured {
		if registered != nil {
			channels.Register(registered)
		}
	}
	for _, registered := range deps.Channels {
		if registered != nil && channelEnabled(cfg, registered.Name()) {
			channels.Register(registered)
		}
	}
	tools := tool.NewRegistry()
	registerRuntimeTools(tools, cfg)
	mcpCleanup := func() {}
	if len(cfg.Tools.MCPServers) > 0 {
		mgr, cleanup := newMCPMgr()
		mcpCleanup = cleanup
		warnings, err := mgr.DiscoverAndRegister(context.Background(), tools, cfg.Tools.MCPServers)
		if err != nil {
			slog.Warn("mcp discovery failed; continuing without discovered tools", "error", err)
		} else {
			for _, warning := range warnings {
				slog.Warn("mcp discovery warning", "msg", warning)
			}
			registered := make([]string, 0)
			for _, def := range tools.Definitions() {
				if strings.HasPrefix(def.Name, "mcp_") {
					registered = append(registered, def.Name)
				}
			}
			slog.Info("mcp discovery completed", "servers", len(cfg.Tools.MCPServers), "count", len(registered), "tools", registered)
		}
	}
	spawner := &runtimeSpawner{}
	loop := agent.NewAgentLoop(agent.LoopDeps{
		Provider:      runtimeProvider,
		Tools:         tools,
		Sessions:      sessions,
		Config:        cfg,
		Skills:        skills,
		Tokenizer:     tokenizer.New(),
		UsageRecorder: usageStore,
		Workspace:     paths.Workspace(),
		MessageRouter: channels,
		Spawner:       spawner,
	})
	spawner.loop = loop
	tools.SetCancelSession(loop.CancelSession)
	cronService := cron.NewService(cron.ServiceDeps{
		JobsFile:  paths.JobsFile(),
		Processor: loop,
		Router:    channels,
	})
	tools.Register(tool.NewCronTool(cronService))
	heartbeatDecider := &heartbeat.ProviderDecider{
		Provider:     runtimeProvider,
		Model:        cfg.Agents.Defaults.Model,
		SystemPrompt: buildHeartbeatDecisionPrompt(paths.Workspace(), skills),
	}
	heartbeatEvaluator := agent.NewEvaluator(&agent.ProviderDecisionProvider{
		Provider:     runtimeProvider,
		Model:        cfg.Agents.Defaults.Model,
		SystemPrompt: buildBackgroundDeliveryPrompt(paths.Workspace(), skills),
	})
	heartbeatService := heartbeat.NewService(heartbeat.ServiceDeps{
		Decider:   heartbeatDecider,
		Processor: loop,
		Evaluator: heartbeatEvaluator,
		Router:    channels,
		Channel:   cfg.Gateway.Heartbeat.Channel,
		ChatID:    cfg.Gateway.Heartbeat.Channel,
	})

	app := &runtimeApp{
		config:     cfg,
		paths:      paths,
		sessions:   sessions,
		usage:      usageStore,
		mcpCleanup: mcpCleanup,
		channels:   channels,
		tools:      tools,
		agent:      loop,
		providerRegistry: providerRegistry,
		cron:       cronService,
		heartbeat:  heartbeatService,
		runCron:    cronService.RunDue,
		runBeat:    heartbeatService.RunOnce,
		cronEvery:  time.Second,
		beatEvery:  time.Duration(cfg.Gateway.Heartbeat.Interval) * time.Minute,
		quotaEvery: time.Duration(cfg.Quota.RefreshIntervalMinutes) * time.Minute,
		beatOn:     cfg.Gateway.Heartbeat.Enabled,
		gateway: gateway.NewServer(gateway.ServerDeps{
			Agent:     loop,
			Sessions:  sessions,
			Channels:  channels,
			Config:    cfg,
			Usage:     usageStore,
			Version:   version,
			StartedAt: time.Now(),
			SetModelCallback: func(model string) (string, error) {
				loop.SetActiveModel(model)
				heartbeatService.SetActiveModel(model)
				return loop.EffectiveModel(), nil
			},
		}),
	}
	if deps.CronRun != nil {
		app.runCron = deps.CronRun
	}
	if deps.CronInterval > 0 {
		app.cronEvery = deps.CronInterval
	}
	if deps.HeartbeatRun != nil {
		app.runBeat = deps.HeartbeatRun
	}
	if deps.HeartbeatInterval > 0 {
		app.beatEvery = deps.HeartbeatInterval
	}
	if deps.HeartbeatEnabled {
		app.beatOn = true
	}
	if app.runQuota == nil && shouldEnableOllamaQuota(cfg, usageStore) {
		app.runQuota = newOllamaQuotaRunner(cfg, paths, usageStore)
	}
	if deps.QuotaRun != nil {
		app.runQuota = deps.QuotaRun
	}
	if deps.QuotaInterval > 0 {
		app.quotaEvery = deps.QuotaInterval
	}
	return app, nil
}

func fetchStatusImpl(ctx context.Context, opts rootOptions) (*statusReport, error) {
	cfg, _, err := loadRuntimeConfig(defaultConfigPath(opts), opts.workspace, 0)
	if err != nil {
		return nil, err
	}

	conn, err := dialGateway(ctx, cfg)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	wire, err := gateway.EncodeRequest(gateway.RequestFrame{
		ID:     "status-1",
		Method: "status",
	})
	if err != nil {
		return nil, err
	}
	if err := conn.WriteMessage(websocket.TextMessage, wire); err != nil {
		return nil, err
	}

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return nil, err
		}
		frame, err := gateway.DecodeFrame(data)
		if err != nil {
			return nil, err
		}
		if frame.Kind != gateway.FrameResponse || frame.Response.ID != "status-1" {
			continue
		}
		if frame.Response.Error != nil {
			return nil, fmt.Errorf("%s: %s", frame.Response.Error.Code, frame.Response.Error.Message)
		}
		report := &statusReport{}
		if err := json.Unmarshal(frame.Response.Result, report); err != nil {
			return nil, err
		}
		return report, nil
	}
}

func fetchChannelStatusesImpl(ctx context.Context, opts rootOptions) ([]channelStatus, error) {
	report, err := fetchStatus(ctx, opts)
	if err != nil {
		return nil, err
	}
	statuses := make([]channelStatus, 0, len(report.Channels))
	for _, name := range report.Channels {
		state := "registered"
		detail := ""
		if report.ChannelStates != nil {
			if ch, ok := report.ChannelStates[name]; ok {
				if s, ok := ch["state"]; ok {
					state = s
				}
				if d, ok := ch["detail"]; ok {
					detail = d
				}
			}
		}
		statuses = append(statuses, channelStatus{
			Name:   name,
			State:  state,
			Detail: detail,
		})
	}
	return statuses, nil
}

func runChannelLoginImpl(ctx context.Context, opts rootOptions, channelName string, out io.Writer) error {
	configPath := opts.configPath
	if configPath == "" {
		configPath = defaultConfigPath(opts)
	}
	cfg, _, err := loadRuntimeConfig(configPath, opts.workspace, 0)
	if err != nil {
		return err
	}

	deps := launchRuntimeDeps()
	manager := channel.NewManager()
	var selected channel.Channel
	if configured, err := configuredChannel(cfg, channelName, true); err != nil {
		return err
	} else if configured != nil {
		selected = configured
		manager.Register(configured)
	}
	for _, registered := range deps.Channels {
		if registered != nil && channelEnabled(cfg, registered.Name()) {
			if strings.EqualFold(strings.TrimSpace(registered.Name()), strings.TrimSpace(channelName)) {
				selected = registered
			}
			manager.Register(registered)
		}
	}
	err = manager.LoginWithUpdates(ctx, channelName, func(status channel.Status) error {
		if out == nil || status.State == "" {
			return nil
		}
		if status.Detail != "" {
			_, err := fmt.Fprintf(out, "%s: %s\n", status.State, status.Detail)
			return err
		}
		_, err := fmt.Fprintf(out, "%s\n", status.State)
		return err
	})
	if err != nil && selected != nil {
		if _, interactive := selected.(channel.InteractiveLoginHandler); interactive {
			return err
		}
		if reporter, ok := selected.(channel.StatusReporter); ok && out != nil {
			if status, statusErr := reporter.Status(ctx); statusErr == nil && status.State != "" {
				if status.Detail != "" {
					_, _ = fmt.Fprintf(out, "%s: %s\n", status.State, status.Detail)
				} else {
					_, _ = fmt.Fprintf(out, "%s\n", status.State)
				}
			}
		}
	}
	return err
}

func loadRuntimeConfig(configPath, workspace string, port int) (*config.Config, *config.Paths, error) {
	var (
		cfg *config.Config
		err error
	)
	if configPath != "" {
		cfg, err = config.Load(configPath)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, nil, err
		}
	}
	if cfg == nil {
		defaultCfg := config.DefaultConfig()
		cfg = &defaultCfg
	}

	paths := config.DefaultPaths()
	if configPath != "" {
		paths = config.NewPaths(filepath.Dir(configPath))
	}

	if workspace != "" {
		cfg.Agents.Defaults.Workspace = workspace
	}
	if cfg.Agents.Defaults.Workspace != "" {
		paths.SetWorkspace(cfg.Agents.Defaults.Workspace)
	}
	if port > 0 {
		cfg.Gateway.Port = port
	}
	if cfg.Gateway.Host == "" {
		cfg.Gateway.Host = "127.0.0.1"
	}
	if cfg.Gateway.Port == 0 {
		cfg.Gateway.Port = 18790
	}
	return cfg, paths, nil
}

func ensureRuntimePaths(paths *config.Paths) error {
	for _, path := range []string{
		paths.Root(),
		paths.Workspace(),
		paths.MemoryDir(),
		filepath.Dir(paths.SessionsDB()),
		paths.SkillsDir(),
	} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func buildHeartbeatDecisionPrompt(workspace string, skills *skill.Registry) string {
	systemPrompt, err := agent.BuildSystemPrompt(agent.BuildContext{
		Workspace: workspace,
		Skills:    skills,
	})
	if err != nil {
		systemPrompt = agent.DefaultIdentityBlock
	}
	heartbeatText := readOptionalWorkspaceFile(filepath.Join(workspace, "HEARTBEAT.md"))
	if heartbeatText == "" {
		heartbeatText = "Check in periodically and only act when useful."
	}
	return strings.TrimSpace(systemPrompt + "\n\nHeartbeat policy:\n" + heartbeatText)
}

func buildBackgroundDeliveryPrompt(workspace string, skills *skill.Registry) string {
	systemPrompt, err := agent.BuildSystemPrompt(agent.BuildContext{
		Workspace: workspace,
		Skills:    skills,
	})
	if err != nil {
		systemPrompt = agent.DefaultIdentityBlock
	}
	return strings.TrimSpace(systemPrompt + "\n\nDecide whether background output should be delivered to the configured channel. Reply with exactly deliver=true or deliver=false.")
}

func readOptionalWorkspaceFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func channelEnabled(cfg *config.Config, name string) bool {
	if cfg == nil {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "signal":
		return cfg.Channels.Signal.Enabled
	case "whatsapp":
		return cfg.Channels.WhatsApp.Enabled
	case "telegram":
		return cfg.Channels.Telegram.Enabled
	case "discord":
		return cfg.Channels.Discord.Enabled
	default:
		return true
	}
}

func configuredChannels(cfg *config.Config, includeDisabled bool) ([]channel.Channel, error) {
	if cfg == nil {
		return nil, nil
	}

	var out []channel.Channel
	if includeDisabled || cfg.Channels.Signal.Enabled {
		out = append(out, newSignalChannel(cfg.Channels.Signal))
	}
	if includeDisabled || cfg.Channels.WhatsApp.Enabled {
		whatsApp, err := newWhatsAppChannel(cfg.Channels.WhatsApp)
		if err != nil {
			return nil, err
		}
		out = append(out, whatsApp)
	}
	if includeDisabled || cfg.Channels.Telegram.Enabled {
		telegram, err := newTelegramChannel(cfg.Channels.Telegram)
		if err != nil {
			return nil, err
		}
		out = append(out, telegram)
	}
	if includeDisabled || cfg.Channels.Discord.Enabled {
		discord, err := newDiscordChannel(cfg.Channels.Discord)
		if err != nil {
			return nil, err
		}
		out = append(out, discord)
	}
	return out, nil
}

func configuredChannel(cfg *config.Config, name string, includeDisabled bool) (channel.Channel, error) {
	if cfg == nil {
		return nil, nil
	}

	switch strings.ToLower(strings.TrimSpace(name)) {
	case "signal":
		if includeDisabled || cfg.Channels.Signal.Enabled {
			return newSignalChannel(cfg.Channels.Signal), nil
		}
	case "whatsapp":
		if includeDisabled || cfg.Channels.WhatsApp.Enabled {
			return newWhatsAppChannel(cfg.Channels.WhatsApp)
		}
	case "telegram":
		if includeDisabled || cfg.Channels.Telegram.Enabled {
			return newTelegramChannel(cfg.Channels.Telegram)
		}
	case "discord":
		if includeDisabled || cfg.Channels.Discord.Enabled {
			return newDiscordChannel(cfg.Channels.Discord)
		}
	}
	return nil, nil
}

func registerRuntimeTools(registry *tool.Registry, cfg *config.Config) {
	restrict := cfg.Tools.RestrictToWorkspace
	registry.Register(tool.NewExecTool(cfg.Tools.Exec, restrict))
	registry.Register(tool.NewReadFileTool(restrict))
	registry.Register(tool.NewWriteFileTool(restrict))
	registry.Register(tool.NewEditFileTool(restrict))
	registry.Register(tool.NewListDirTool(restrict))
	registry.Register(tool.NewWebSearchTool(cfg.Tools.Web, tool.WebDependencies{}))
	registry.Register(tool.NewWebFetchTool(cfg.Tools.Web, tool.WebDependencies{}))
	registry.Register(tool.NewMessageTool())
	registry.Register(tool.NewSpawnTool(uuid.NewString))
}

func (a *runtimeApp) Close() error {
	if a == nil {
		return nil
	}
	if a.mcpCleanup != nil {
		a.mcpCleanup()
		a.mcpCleanup = nil
	}
	var errs []error
	if a.usage != nil {
		errs = append(errs, a.usage.Close())
		a.usage = nil
	}
	if a.sessions != nil {
		errs = append(errs, a.sessions.Close())
		a.sessions = nil
	}
	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func (s *runtimeSpawner) ProcessDirect(ctx context.Context, req tool.SpawnRequest) (string, error) {
	if s == nil || s.loop == nil {
		return "", errors.New("spawner unavailable")
	}
	return s.loop.ProcessDirect(ctx, agent.Request{
		Content:    req.Message,
		SessionKey: req.ChildSessionKey,
	}, nil)
}

func (a *runtimeApp) handleInbound(ctx context.Context, msg channel.InboundMessage) {
	if a == nil || a.agent == nil || a.channels == nil {
		return
	}
	log.Printf("[channel] inbound from %s/%s: %s", msg.Channel, msg.ChatID, truncateLog(msg.Content, 120))
	if a.gateway != nil {
		a.gateway.BroadcastEvent("channel.inbound", map[string]any{
			"channel": msg.Channel,
			"chatID":  msg.ChatID,
			"content": msg.Content,
		})
	}
	go func() {
		sessionKey := firstNonEmpty(msg.Channel+":"+msg.ChatID, msg.ChatID, "channel:unknown")
		cb := a.channelEventCallback(msg.Channel, msg.ChatID)
		response, err := a.agent.ProcessDirect(ctx, agent.Request{
			Content:    msg.Content,
			SessionKey: sessionKey,
			Channel:    msg.Channel,
			ChatID:     msg.ChatID,
		}, cb)
		if err != nil {
			log.Printf("[channel] agent error for %s/%s: %v", msg.Channel, msg.ChatID, err)
			if a.gateway != nil {
				a.gateway.BroadcastEvent("channel.error", map[string]any{
					"channel": msg.Channel,
					"chatID":  msg.ChatID,
					"error":   err.Error(),
				})
			}
			return
		}
		if strings.TrimSpace(response) == "" {
			log.Printf("[channel] empty response for %s/%s", msg.Channel, msg.ChatID)
			return
		}
		if a.gateway != nil {
			a.gateway.BroadcastEvent("channel.outbound", map[string]any{
				"channel": msg.Channel,
				"chatID":  msg.ChatID,
				"content": response,
			})
		}
		if err := a.channels.Route(ctx, msg.Channel, msg.ChatID, response); err != nil {
			log.Printf("[channel] route error for %s/%s: %v", msg.Channel, msg.ChatID, err)
		}
	}()
}

func (a *runtimeApp) channelEventCallback(ch, chatID string) agent.EventCallback {
	if a.gateway == nil {
		return nil
	}
	return func(event agent.Event) {
		switch event.Type {
		case agent.EventProgress:
			a.gateway.BroadcastEvent("channel.progress", map[string]any{
				"channel": ch,
				"chatID":  chatID,
				"content": event.Content,
			})
		case agent.EventThinking:
			a.gateway.BroadcastEvent("channel.thinking", map[string]any{
				"channel": ch,
				"chatID":  chatID,
				"content": event.Content,
			})
		}
	}
}

func truncateLog(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}
	return s
}

func startRuntimeLoops(ctx context.Context, app *runtimeApp, errCh chan<- error) {
	if app == nil {
		return
	}
	reportLoopError := func(err error) {
		if err == nil {
			return
		}
		select {
		case errCh <- err:
		default:
		}
	}
	if app.runCron != nil && app.cronEvery > 0 {
		go func() {
			ticker := time.NewTicker(app.cronEvery)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case now := <-ticker.C:
					if err := app.runCron(ctx, now); err != nil {
						log.Printf("[runtime] cron run failed: %v", err)
						reportLoopError(err)
						return
					}
				}
			}
		}()
	}
	if app.beatOn && app.runBeat != nil && app.beatEvery > 0 {
		go func() {
			ticker := time.NewTicker(app.beatEvery)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if err := app.runBeat(ctx); err != nil {
						log.Printf("[runtime] heartbeat run failed: %v", err)
						reportLoopError(err)
						return
					}
				}
			}
		}()
	}
	if app.runQuota != nil && app.quotaEvery > 0 {
		go func() {
			ticker := time.NewTicker(app.quotaEvery)
			defer ticker.Stop()
			run := func() {
				if err := app.runQuota(ctx); err != nil {
					log.Printf("[runtime] quota refresh failed: %v", err)
				}
			}
			run()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					run()
				}
			}
		}()
	}
}

func shouldEnableOllamaQuota(cfg *config.Config, store *usage.Store) bool {
	if cfg == nil || store == nil {
		return false
	}
	if cfg.Quota.RefreshIntervalMinutes <= 0 {
		return false
	}
	return cfg.Quota.HasEnabledProvider("ollama")
}

func newOllamaQuotaRunner(cfg *config.Config, paths *config.Paths, store *usage.Store) func(context.Context) error {
	cookiePath := paths.OllamaCookieJar()
	cookieLoader := usage.NewCookieJarStore(cookiePath)
	fetcher := &usage.OllamaQuotaFetcher{
		Clock:        time.Now,
		Signer:       usage.NewOllamaKeySigner(""),
		CookieLoader: cookieLoader,
	}
	providerCfg := config.ProviderQuotaConfig{}
	if cfg != nil {
		providerCfg = cfg.Quota.Provider("ollama")
	}

	return func(ctx context.Context) error {
		if cfg != nil {
			cookieHeader := strings.TrimSpace(providerCfg.CookieHeader)
			if cookieHeader == "" {
				cookieHeader = strings.TrimSpace(cfg.Quota.OllamaCookieHeader)
			}
			if cookieHeader != "" {
				if err := usage.WriteOllamaCookieHeader(cookiePath, cookieHeader); err != nil {
					log.Printf("[runtime] ollama cookie header override failed: %v", err)
				}
			} else {
				if providerCfg.BrowserCookieDiscoveryEnabled {
					cookies, err := cookieLoader.Load()
					if err != nil || len(cookies) == 0 {
						if _, importErr := usage.ImportOllamaCookiesFromLinuxBrowsers("", cookiePath); importErr != nil {
							log.Printf("[runtime] ollama cookie import skipped: %v", importErr)
						}
					}
				}
			}
		}

		summary, err := fetcher.Fetch(ctx)
		if summary.ProviderID != "" {
			if saveErr := store.SaveQuotaSummary(ctx, summary); saveErr != nil {
				return fmt.Errorf("save quota summary: %w", saveErr)
			}
		}
		return err
	}
}

func dialGateway(ctx context.Context, cfg *config.Config) (*websocket.Conn, error) {
	url := "ws://" + cfg.Gateway.Host + ":" + strconv.Itoa(cfg.Gateway.Port) + "/ws"
	dialer := websocket.Dialer{HandshakeTimeout: 2 * time.Second}

	var lastErr error
	for attempt := 0; attempt < 10; attempt++ {
		conn, _, err := dialer.DialContext(ctx, url, nil)
		if err == nil {
			return conn, nil
		}
		lastErr = err
		time.Sleep(25 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = errors.New("gateway dial failed")
	}
	return nil, lastErr
}
