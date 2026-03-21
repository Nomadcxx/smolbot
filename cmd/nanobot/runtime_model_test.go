package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	conn := dialWebsocketGateway(t, httpServer.URL+"/ws")
	defer conn.Close()

	writeFrameGateway(t, conn, gateway.RequestFrame{
		ID:     "1",
		Method: "models.set",
		Params: json.RawMessage(`{"model":"claude-sonnet"}`),
	})
	frame := readResponseFrameGateway(t, conn, "1")
	if frame.Kind != gateway.FrameResponse || frame.Response.Error != nil {
		t.Fatalf("models.set failed: %#v", frame)
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

	conn := dialWebsocketGateway(t, httpServer.URL+"/ws")
	defer conn.Close()

	writeFrameGateway(t, conn, gateway.RequestFrame{
		ID:     "1",
		Method: "models.set",
		Params: json.RawMessage(`{"model":"claude-sonnet"}`),
	})
	frame := readResponseFrameGateway(t, conn, "1")
	if frame.Kind != gateway.FrameResponse || frame.Response.Error != nil {
		t.Fatalf("models.set failed: %#v", frame)
	}

	beatModel := app.heartbeat.EffectiveModel()
	if beatModel != "claude-sonnet" {
		t.Fatalf("heartbeat effective model = %q, want claude-sonnet after switch", beatModel)
	}
}

func TestGatewayStatusReportsEffectiveProviderAfterModelSwitch(t *testing.T) {
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

	conn := dialWebsocketGateway(t, httpServer.URL+"/ws")
	defer conn.Close()

	writeFrameGateway(t, conn, gateway.RequestFrame{
		ID:     "1",
		Method: "models.set",
		Params: json.RawMessage(`{"model":"claude-sonnet"}`),
	})
	readResponseFrameGateway(t, conn, "1")

	writeFrameGateway(t, conn, gateway.RequestFrame{ID: "2", Method: "status"})
	frame := readResponseFrameGateway(t, conn, "2")
	if frame.Kind != gateway.FrameResponse || frame.Response.Error != nil {
		t.Fatalf("status failed: %#v", frame)
	}

	var result map[string]any
	if err := json.Unmarshal(frame.Response.Result, &result); err != nil {
		t.Fatalf("Unmarshal result: %v", err)
	}

	reportedModel, _ := result["model"].(string)
	reportedProvider, _ := result["provider"].(string)

	if reportedModel != "claude-sonnet" {
		t.Fatalf("status model = %q, want claude-sonnet", reportedModel)
	}
	if reportedProvider != "anthropic" {
		t.Fatalf("status provider = %q, want anthropic (detected from model name)", reportedProvider)
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

	conn := dialWebsocketGateway(t, httpServer.URL+"/ws")
	defer conn.Close()

	writeFrameGateway(t, conn, gateway.RequestFrame{
		ID:     "1",
		Method: "models.set",
		Params: json.RawMessage(`{"model":"claude-3-5-sonnet"}`),
	})
	frame := readResponseFrameGateway(t, conn, "1")
	if frame.Kind != gateway.FrameResponse || frame.Response.Error != nil {
		t.Fatalf("models.set failed: %#v", frame)
	}

	agentModel := app.agent.EffectiveModel()
	if !strings.HasPrefix(agentModel, "claude") {
		t.Fatalf("agent loop model = %q, want claude prefix after cross-family switch", agentModel)
	}
}

type spyProvider struct {
	name  string
	model string
}

func (p *spyProvider) Name() string { return p.name }
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
