package main

import (
	"context"
	"fmt"
	"io"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Nomadcxx/smolbot/internal/client"
	"github.com/Nomadcxx/smolbot/pkg/agent"
	"github.com/Nomadcxx/smolbot/pkg/channel"
	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/gateway"
	"github.com/Nomadcxx/smolbot/pkg/provider"
	"github.com/gorilla/websocket"
)

func TestModelsSetSwitchUpdatesAgentLoopModel(t *testing.T) {
	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Provider = "openai"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	app, err := buildRuntime(daemonLaunchOptions{
		ConfigPath: cfgPath,
		Port:       port,
	}, runtimeDeps{
		Provider: &spyProvider{name: "openai", model: "gpt-test"},
	})
	if err != nil {
		t.Fatalf("buildRuntime: %v", err)
	}
	defer app.Close()

	httpServer := httptest.NewServer(app.gateway.Handler())
	defer httpServer.Close()

	cl := connectGatewayClient(t, httpServer.URL)
	defer cl.Close()

	current, err := cl.ModelsSet("claude-sonnet")
	if err != nil {
		t.Fatalf("ModelsSet: %v", err)
	}
	if current != "claude-sonnet" {
		t.Fatalf("current model = %q, want claude-sonnet", current)
	}

	agentModel := app.agent.EffectiveModel()
	if agentModel != "claude-sonnet" {
		t.Fatalf("agent loop effective model = %q, want claude-sonnet after switch", agentModel)
	}
}

func TestModelsSetSwitchUpdatesHeartbeatModel(t *testing.T) {
	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Provider = "openai"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Gateway.Heartbeat.Enabled = true
	cfg.Gateway.Heartbeat.Interval = 1
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	app, err := buildRuntime(daemonLaunchOptions{
		ConfigPath: cfgPath,
		Port:       port,
	}, runtimeDeps{
		Provider: &spyProvider{name: "openai", model: "gpt-test"},
	})
	if err != nil {
		t.Fatalf("buildRuntime: %v", err)
	}
	defer app.Close()

	httpServer := httptest.NewServer(app.gateway.Handler())
	defer httpServer.Close()

	cl := connectGatewayClient(t, httpServer.URL)
	defer cl.Close()

	current, err := cl.ModelsSet("claude-sonnet")
	if err != nil {
		t.Fatalf("ModelsSet: %v", err)
	}
	if current != "claude-sonnet" {
		t.Fatalf("current model = %q, want claude-sonnet", current)
	}

	beatModel := app.heartbeat.EffectiveModel()
	if beatModel != "claude-sonnet" {
		t.Fatalf("heartbeat effective model = %q, want claude-sonnet after switch", beatModel)
	}
}

func TestGatewayStatusReportsModelAfterModelSwitch(t *testing.T) {
	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Provider = "openai"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	app, err := buildRuntime(daemonLaunchOptions{
		ConfigPath: cfgPath,
		Port:       port,
	}, runtimeDeps{
		Provider: &spyProvider{name: "openai", model: "gpt-test"},
	})
	if err != nil {
		t.Fatalf("buildRuntime: %v", err)
	}
	defer app.Close()

	httpServer := httptest.NewServer(app.gateway.Handler())
	defer httpServer.Close()

	cl := connectGatewayClient(t, httpServer.URL)
	defer cl.Close()

	if _, err := cl.ModelsSet("claude-sonnet"); err != nil {
		t.Fatalf("ModelsSet: %v", err)
	}

	status, err := cl.Status("tui:main")
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.Model != "claude-sonnet" {
		t.Fatalf("status model = %q, want claude-sonnet", status.Model)
	}
}

func TestProviderSwitchAcrossFamiliesChangesAgentModel(t *testing.T) {
	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-4o"
	cfg.Agents.Defaults.Provider = "openai"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	app, err := buildRuntime(daemonLaunchOptions{
		ConfigPath: cfgPath,
		Port:       port,
	}, runtimeDeps{
		Provider: &spyProvider{name: "openai", model: "gpt-4o"},
	})
	if err != nil {
		t.Fatalf("buildRuntime: %v", err)
	}
	defer app.Close()

	httpServer := httptest.NewServer(app.gateway.Handler())
	defer httpServer.Close()

	cl := connectGatewayClient(t, httpServer.URL)
	defer cl.Close()

	if _, err := cl.ModelsSet("claude-3-5-sonnet"); err != nil {
		t.Fatalf("ModelsSet: %v", err)
	}

	agentModel := app.agent.EffectiveModel()
	if !strings.HasPrefix(agentModel, "claude") {
		t.Fatalf("agent loop model = %q, want claude prefix after cross-family switch", agentModel)
	}
}

func TestProviderSwitchAcrossFamiliesRoutesChatThroughResolvedProvider(t *testing.T) {
	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-4o"
	cfg.Agents.Defaults.Provider = "openai"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	openaiProvider := &recordingProvider{name: "openai"}
	anthropicProvider := &recordingProvider{name: "anthropic"}
	registry := provider.NewRegistry(&cfg)
	registry.RegisterFactory("openai", func(config.ProviderConfig) provider.Provider { return openaiProvider })
	registry.RegisterFactory("anthropic", func(config.ProviderConfig) provider.Provider { return anthropicProvider })

	app, err := buildRuntime(daemonLaunchOptions{
		ConfigPath: cfgPath,
		Port:       port,
	}, runtimeDeps{
		Provider:         openaiProvider,
		ProviderRegistry: registry,
	})
	if err != nil {
		t.Fatalf("buildRuntime: %v", err)
	}
	defer app.Close()

	initial, err := app.agent.ProcessDirect(context.Background(), agent.Request{
		Content:    "hello",
		SessionKey: "s1",
	}, nil)
	if err != nil {
		t.Fatalf("initial ProcessDirect: %v", err)
	}
	if initial != "openai:gpt-4o" {
		t.Fatalf("initial response = %q, want openai:gpt-4o", initial)
	}

	httpServer := httptest.NewServer(app.gateway.Handler())
	defer httpServer.Close()

	cl := connectGatewayClient(t, httpServer.URL)
	defer cl.Close()

	if _, err := cl.ModelsSet("claude-3-5-sonnet"); err != nil {
		t.Fatalf("ModelsSet: %v", err)
	}

	switched, err := app.agent.ProcessDirect(context.Background(), agent.Request{
		Content:    "hello again",
		SessionKey: "s2",
	}, nil)
	if err != nil {
		t.Fatalf("switched ProcessDirect: %v", err)
	}
	if switched != "anthropic:claude-3-5-sonnet" {
		t.Fatalf("switched response = %q, want anthropic:claude-3-5-sonnet", switched)
	}
	if last := openaiProvider.lastStreamModel(); last != "gpt-4o" {
		t.Fatalf("openai stream model = %q, want gpt-4o", last)
	}
	if last := anthropicProvider.lastStreamModel(); last != "claude-3-5-sonnet" {
		t.Fatalf("anthropic stream model = %q, want claude-3-5-sonnet", last)
	}
}

func TestModelsSetSwitchUpdatesHeartbeatEvaluatorProvider(t *testing.T) {
	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-4o"
	cfg.Agents.Defaults.Provider = "openai"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Gateway.Heartbeat.Enabled = true
	cfg.Gateway.Heartbeat.Interval = 1
	cfg.Gateway.Heartbeat.Channel = "slack"
	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	openaiProvider := &recordingProvider{name: "openai"}
	anthropicProvider := &recordingProvider{name: "anthropic"}
	anthropicProvider.chatResponder = func(req provider.ChatRequest) (*provider.Response, error) {
		if req.MaxTokens == 32 {
			return &provider.Response{Content: "run"}, nil
		}
		return &provider.Response{Content: "deliver=true"}, nil
	}

	registry := provider.NewRegistry(&cfg)
	registry.RegisterFactory("openai", func(config.ProviderConfig) provider.Provider { return openaiProvider })
	registry.RegisterFactory("anthropic", func(config.ProviderConfig) provider.Provider { return anthropicProvider })

	slack := &fakeRuntimeChannel{name: "slack"}
	app, err := buildRuntime(daemonLaunchOptions{
		ConfigPath: cfgPath,
		Port:       port,
	}, runtimeDeps{
		Provider:         openaiProvider,
		ProviderRegistry: registry,
		Channels:         []channel.Channel{slack},
	})
	if err != nil {
		t.Fatalf("buildRuntime: %v", err)
	}
	defer app.Close()

	httpServer := httptest.NewServer(app.gateway.Handler())
	defer httpServer.Close()

	cl := connectGatewayClient(t, httpServer.URL)
	defer cl.Close()

	if _, err := cl.ModelsSet("claude-3-5-sonnet"); err != nil {
		t.Fatalf("ModelsSet: %v", err)
	}
	if err := app.heartbeat.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if got := anthropicProvider.lastStreamModel(); got != "claude-3-5-sonnet" {
		t.Fatalf("heartbeat processor stream model = %q, want claude-3-5-sonnet", got)
	}
	if got := anthropicProvider.lastChatModel(); got != "claude-3-5-sonnet" {
		t.Fatalf("heartbeat evaluator/decider model = %q, want claude-3-5-sonnet", got)
	}
	if got := openaiProvider.lastChatModel(); got != "" {
		t.Fatalf("openai provider received heartbeat chat after switch: %q", got)
	}
	if slack.sendCount != 1 {
		t.Fatalf("slack send count = %d, want 1", slack.sendCount)
	}
}

func connectGatewayClient(t *testing.T, rawURL string) *client.Client {
	t.Helper()

	wsURL := "ws" + strings.TrimPrefix(rawURL, "http") + "/ws"
	cl := client.New(wsURL)
	if _, err := cl.Connect(); err != nil {
		t.Fatalf("Connect: %v", err)
	}
	return cl
}

type spyProvider struct {
	name  string
	model string
}

func (p *spyProvider) Name() string  { return p.name }
func (p *spyProvider) Model() string { return p.model }

func (p *spyProvider) Chat(_ context.Context, _ provider.ChatRequest) (*provider.Response, error) {
	return &provider.Response{Content: "spy response"}, nil
}

func (p *spyProvider) ChatStream(_ context.Context, _ provider.ChatRequest) (*provider.Stream, error) {
	return provider.NewStream(
		func() (*provider.StreamDelta, error) { return nil, fmt.Errorf("stream done") },
		func() error { return nil },
	), nil
}

type recordingProvider struct {
	name          string
	chatResponder func(req provider.ChatRequest) (*provider.Response, error)

	mu           sync.Mutex
	streamModels []string
	chatModels   []string
}

func (p *recordingProvider) Name() string { return p.name }

func (p *recordingProvider) Chat(_ context.Context, req provider.ChatRequest) (*provider.Response, error) {
	p.mu.Lock()
	p.chatModels = append(p.chatModels, req.Model)
	p.mu.Unlock()
	if p.chatResponder != nil {
		return p.chatResponder(req)
	}
	return &provider.Response{Content: p.name + ":" + req.Model}, nil
}

func (p *recordingProvider) ChatStream(_ context.Context, req provider.ChatRequest) (*provider.Stream, error) {
	p.mu.Lock()
	p.streamModels = append(p.streamModels, req.Model)
	p.mu.Unlock()

	sent := false
	finish := "stop"
	return provider.NewStream(
		func() (*provider.StreamDelta, error) {
			if sent {
				return nil, io.EOF
			}
			sent = true
			return &provider.StreamDelta{
				Content:      p.name + ":" + req.Model,
				FinishReason: &finish,
			}, nil
		},
		func() error { return nil },
	), nil
}

func (p *recordingProvider) lastStreamModel() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.streamModels) == 0 {
		return ""
	}
	return p.streamModels[len(p.streamModels)-1]
}

func (p *recordingProvider) lastChatModel() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.chatModels) == 0 {
		return ""
	}
	return p.chatModels[len(p.chatModels)-1]
}

type fakeRuntimeChannel struct {
	name string

	sendCount int
}

func (c *fakeRuntimeChannel) Name() string { return c.name }

func (c *fakeRuntimeChannel) Start(context.Context, channel.Handler) error { return nil }

func (c *fakeRuntimeChannel) Stop(context.Context) error { return nil }

func (c *fakeRuntimeChannel) Send(_ context.Context, msg channel.OutboundMessage) error {
	c.sendCount++
	if msg.Content == "" {
		return fmt.Errorf("empty message")
	}
	return nil
}

func dialWebsocketGateway(t *testing.T, rawURL string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(rawURL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	return conn
}

func writeFrameGateway(t *testing.T, conn *websocket.Conn, req gateway.RequestFrame) {
	t.Helper()
	data, err := gateway.EncodeRequest(req)
	if err != nil {
		t.Fatalf("EncodeRequest: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}
}

func readResponseFrameGateway(t *testing.T, conn *websocket.Conn, id string) *gateway.DecodedFrame {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	_ = conn.SetReadDeadline(deadline)
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage: %v", err)
		}
		frame, err := gateway.DecodeFrame(data)
		if err != nil {
			t.Fatalf("DecodeFrame: %v", err)
		}
		if frame.Kind == gateway.FrameResponse && frame.Response.ID == id {
			return frame
		}
	}
}
