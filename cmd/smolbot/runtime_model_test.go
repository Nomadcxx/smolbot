package main

import (
	"context"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nomadcxx/smolbot/internal/client"
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

	previous, err := cl.ModelsSet("claude-sonnet")
	if err != nil {
		t.Fatalf("ModelsSet: %v", err)
	}
	if previous != "gpt-test" {
		t.Fatalf("previous model = %q, want gpt-test", previous)
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

	previous, err := cl.ModelsSet("claude-sonnet")
	if err != nil {
		t.Fatalf("ModelsSet: %v", err)
	}
	if previous != "gpt-test" {
		t.Fatalf("previous model = %q, want gpt-test", previous)
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
