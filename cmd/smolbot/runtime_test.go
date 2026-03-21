package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/channel"
	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/gateway"
	"github.com/gorilla/websocket"
)

func TestLaunchDaemonServesHealthAndStops(t *testing.T) {
	port := freePort(t)
	cfgPath := writeTestConfig(t, port)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- launchDaemon(ctx, daemonLaunchOptions{
			ConfigPath: cfgPath,
			Port:       port,
		})
	}()

	waitForHealth(t, port)
	cancel()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("launchDaemon returned error: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("launchDaemon did not stop after context cancellation")
	}
}

func TestFetchStatusQueriesGateway(t *testing.T) {
	port := freePort(t)
	server := &http.Server{Addr: "127.0.0.1:" + strconv.Itoa(port)}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Upgrade: %v", err)
		}
		defer conn.Close()

		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage: %v", err)
		}
		frame, err := gateway.DecodeFrame(data)
		if err != nil {
			t.Fatalf("DecodeFrame: %v", err)
		}
		if frame.Request.Method != "status" {
			t.Fatalf("expected status request, got %#v", frame.Request)
		}

		payload, err := json.Marshal(map[string]any{
			"model":            "claude-sonnet",
			"uptimeSeconds":    42,
			"channels":         []string{"slack", "discord"},
			"channelStates":    map[string]map[string]string{"slack": {"state": "connected"}, "discord": {"state": "error"}},
			"connectedClients": 2,
		})
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		wire, err := gateway.EncodeResponse(gateway.ResponseFrame{
			ID:     frame.Request.ID,
			Result: payload,
		})
		if err != nil {
			t.Fatalf("EncodeResponse: %v", err)
		}
		if err := conn.WriteMessage(websocket.TextMessage, wire); err != nil {
			t.Fatalf("WriteMessage: %v", err)
		}
	})
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()
	defer func() {
		_ = server.Close()
		<-errCh
	}()

	cfgPath := writeTestConfig(t, port)
	report, err := fetchStatus(context.Background(), rootOptions{configPath: cfgPath})
	if err != nil {
		t.Fatalf("fetchStatus: %v", err)
	}
	if report.Model != "claude-sonnet" || report.UptimeSeconds != 42 || report.ConnectedClients != 2 {
		t.Fatalf("unexpected status report %#v", report)
	}
	if got := strings.Join(report.Channels, ","); got != "slack,discord" {
		t.Fatalf("unexpected channels %q", got)
	}
}

func TestFetchChannelStatusesQueriesGatewayStatus(t *testing.T) {
	port := freePort(t)
	server := &http.Server{Addr: "127.0.0.1:" + strconv.Itoa(port)}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("Upgrade: %v", err)
		}
		defer conn.Close()

		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("ReadMessage: %v", err)
		}
		frame, err := gateway.DecodeFrame(data)
		if err != nil {
			t.Fatalf("DecodeFrame: %v", err)
		}
		payload, err := json.Marshal(map[string]any{
			"model":            "claude-sonnet",
			"uptimeSeconds":    42,
			"channels":         []string{"slack", "discord"},
			"channelStates":    map[string]map[string]string{"slack": {"state": "connected"}, "discord": {"state": "error"}},
			"connectedClients": 2,
		})
		if err != nil {
			t.Fatalf("Marshal: %v", err)
		}
		wire, err := gateway.EncodeResponse(gateway.ResponseFrame{
			ID:     frame.Request.ID,
			Result: payload,
		})
		if err != nil {
			t.Fatalf("EncodeResponse: %v", err)
		}
		if err := conn.WriteMessage(websocket.TextMessage, wire); err != nil {
			t.Fatalf("WriteMessage: %v", err)
		}
	})
	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe()
	}()
	defer func() {
		_ = server.Close()
		<-errCh
	}()

	cfgPath := writeTestConfig(t, port)
	statuses, err := fetchChannelStatuses(context.Background(), rootOptions{configPath: cfgPath})
	if err != nil {
		t.Fatalf("fetchChannelStatuses: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("expected two channel statuses, got %#v", statuses)
	}
	if statuses[0].Name != "slack" || statuses[0].State != "connected" {
		t.Fatalf("unexpected first channel status %#v", statuses[0])
	}
	if statuses[1].Name != "discord" || statuses[1].State != "error" {
		t.Fatalf("unexpected second channel status %#v", statuses[1])
	}
}

func TestBuildRuntimeRegistersOnlyEnabledConfiguredChannels(t *testing.T) {
	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Channels.Signal.Enabled = true
	cfg.Channels.WhatsApp.Enabled = false

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	app, err := buildRuntime(daemonLaunchOptions{
		ConfigPath: cfgPath,
		Port:       port,
	}, runtimeDeps{
		Provider: &fakeRuntimeProvider{},
		Channels: []channel.Channel{
			&runtimeLifecycleChannel{name: "signal"},
			&runtimeLifecycleChannel{name: "whatsapp"},
			&runtimeLifecycleChannel{name: "slack"},
		},
	})
	if err != nil {
		t.Fatalf("buildRuntime: %v", err)
	}
	defer app.Close()

	got := strings.Join(app.channels.ChannelNames(), ",")
	if got != "signal,slack" {
		t.Fatalf("registered channels = %q, want signal,slack", got)
	}
}

func TestBuildRuntimeConstructsConfiguredSignalChannel(t *testing.T) {
	orig := newSignalChannel
	defer func() { newSignalChannel = orig }()

	fakeSignal := &runtimeLoginStatusChannel{name: "signal", status: channel.Status{State: "connected"}}
	newSignalChannel = func(cfg config.SignalChannelConfig) channel.Channel {
		if cfg.Account != "+15551234567" {
			t.Fatalf("unexpected signal config %#v", cfg)
		}
		return fakeSignal
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Channels.Signal.Enabled = true
	cfg.Channels.Signal.Account = "+15551234567"

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	app, err := buildRuntime(daemonLaunchOptions{
		ConfigPath: cfgPath,
		Port:       port,
	}, runtimeDeps{
		Provider: &fakeRuntimeProvider{},
	})
	if err != nil {
		t.Fatalf("buildRuntime: %v", err)
	}
	defer app.Close()

	if got := strings.Join(app.channels.ChannelNames(), ","); got != "signal" {
		t.Fatalf("registered channels = %q, want signal", got)
	}
	if state := app.channels.Statuses(context.Background())["signal"].State; state != "connected" {
		t.Fatalf("signal status = %q, want connected", state)
	}
}

func TestRunChannelLoginUsesConfiguredSignalChannel(t *testing.T) {
	orig := newSignalChannel
	defer func() { newSignalChannel = orig }()

	fakeSignal := &runtimeLoginStatusChannel{name: "signal"}
	newSignalChannel = func(cfg config.SignalChannelConfig) channel.Channel {
		if cfg.Account != "+15551234567" {
			t.Fatalf("unexpected signal config %#v", cfg)
		}
		return fakeSignal
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Channels.Signal.Enabled = true
	cfg.Channels.Signal.Account = "+15551234567"

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	if err := runChannelLoginImpl(context.Background(), rootOptions{configPath: cfgPath}, "signal", io.Discard); err != nil {
		t.Fatalf("runChannelLoginImpl: %v", err)
	}
	if fakeSignal.logins != 1 {
		t.Fatalf("signal login calls = %d, want 1", fakeSignal.logins)
	}
}

func TestRunChannelLoginUsesConfiguredSignalChannelWhenDisabled(t *testing.T) {
	orig := newSignalChannel
	defer func() { newSignalChannel = orig }()

	fakeSignal := &runtimeLoginStatusChannel{name: "signal"}
	newSignalChannel = func(cfg config.SignalChannelConfig) channel.Channel {
		if cfg.Account != "+15551234567" {
			t.Fatalf("unexpected signal config %#v", cfg)
		}
		return fakeSignal
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Channels.Signal.Enabled = false
	cfg.Channels.Signal.Account = "+15551234567"

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	if err := runChannelLoginImpl(context.Background(), rootOptions{configPath: cfgPath}, "signal", io.Discard); err != nil {
		t.Fatalf("runChannelLoginImpl: %v", err)
	}
	if fakeSignal.logins != 1 {
		t.Fatalf("signal login calls = %d, want 1", fakeSignal.logins)
	}
}

func TestBuildRuntimeConstructsConfiguredWhatsAppChannel(t *testing.T) {
	orig := newWhatsAppChannel
	defer func() { newWhatsAppChannel = orig }()

	fakeWhatsApp := &runtimeLoginStatusChannel{name: "whatsapp", status: channel.Status{State: "connected"}}
	newWhatsAppChannel = func(cfg config.WhatsAppChannelConfig) (channel.Channel, error) {
		if cfg.DeviceName != "smolbot-test" {
			t.Fatalf("unexpected whatsapp config %#v", cfg)
		}
		return fakeWhatsApp, nil
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Channels.WhatsApp.Enabled = true
	cfg.Channels.WhatsApp.DeviceName = "smolbot-test"

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	app, err := buildRuntime(daemonLaunchOptions{
		ConfigPath: cfgPath,
		Port:       port,
	}, runtimeDeps{
		Provider: &fakeRuntimeProvider{},
	})
	if err != nil {
		t.Fatalf("buildRuntime: %v", err)
	}
	defer app.Close()

	if got := strings.Join(app.channels.ChannelNames(), ","); got != "whatsapp" {
		t.Fatalf("registered channels = %q, want whatsapp", got)
	}
	if state := app.channels.Statuses(context.Background())["whatsapp"].State; state != "connected" {
		t.Fatalf("whatsapp status = %q, want connected", state)
	}
}

func TestRunChannelLoginUsesConfiguredWhatsAppChannelWhenDisabled(t *testing.T) {
	orig := newWhatsAppChannel
	defer func() { newWhatsAppChannel = orig }()

	fakeWhatsApp := &runtimeLoginStatusChannel{name: "whatsapp"}
	newWhatsAppChannel = func(cfg config.WhatsAppChannelConfig) (channel.Channel, error) {
		if cfg.DeviceName != "smolbot-test" {
			t.Fatalf("unexpected whatsapp config %#v", cfg)
		}
		return fakeWhatsApp, nil
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Channels.WhatsApp.Enabled = false
	cfg.Channels.WhatsApp.DeviceName = "smolbot-test"

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	if err := runChannelLoginImpl(context.Background(), rootOptions{configPath: cfgPath}, "whatsapp", io.Discard); err != nil {
		t.Fatalf("runChannelLoginImpl: %v", err)
	}
	if fakeWhatsApp.logins != 1 {
		t.Fatalf("whatsapp login calls = %d, want 1", fakeWhatsApp.logins)
	}
}

func TestRunChannelLoginIgnoresDisabledBrokenWhatsAppWhenLoggingIntoSignal(t *testing.T) {
	origSignal := newSignalChannel
	origWhatsApp := newWhatsAppChannel
	defer func() {
		newSignalChannel = origSignal
		newWhatsAppChannel = origWhatsApp
	}()

	fakeSignal := &runtimeLoginStatusChannel{name: "signal"}
	newSignalChannel = func(cfg config.SignalChannelConfig) channel.Channel {
		if cfg.Account != "+15551234567" {
			t.Fatalf("unexpected signal config %#v", cfg)
		}
		return fakeSignal
	}
	newWhatsAppChannel = func(config.WhatsAppChannelConfig) (channel.Channel, error) {
		return nil, errors.New("broken whatsapp")
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Channels.Signal.Enabled = false
	cfg.Channels.Signal.Account = "+15551234567"
	cfg.Channels.WhatsApp.Enabled = false
	cfg.Channels.WhatsApp.StorePath = ""

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	if err := runChannelLoginImpl(context.Background(), rootOptions{configPath: cfgPath}, "signal", io.Discard); err != nil {
		t.Fatalf("runChannelLoginImpl: %v", err)
	}
	if fakeSignal.logins != 1 {
		t.Fatalf("signal login calls = %d, want 1", fakeSignal.logins)
	}
}

func TestBuildRuntimeReturnsConfiguredWhatsAppChannelError(t *testing.T) {
	orig := newWhatsAppChannel
	defer func() { newWhatsAppChannel = orig }()

	wantErr := errors.New("whatsapp seam failed")
	newWhatsAppChannel = func(cfg config.WhatsAppChannelConfig) (channel.Channel, error) {
		if cfg.StorePath == "" {
			t.Fatalf("expected store path in whatsapp config %#v", cfg)
		}
		return nil, wantErr
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Channels.WhatsApp.Enabled = true

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	_, err := buildRuntime(daemonLaunchOptions{
		ConfigPath: cfgPath,
		Port:       port,
	}, runtimeDeps{
		Provider: &fakeRuntimeProvider{},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("buildRuntime error = %v, want %v", err, wantErr)
	}
}

func writeTestConfig(t *testing.T, port int) string {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port

	path := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(path, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}
	return path
}

func writeTestFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}

func waitForHealth(t *testing.T, port int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	url := "http://127.0.0.1:" + strconv.Itoa(port) + "/health"
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("health endpoint did not become ready at %s", url)
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

type runtimeLoginStatusChannel struct {
	name   string
	status channel.Status
	logins int
}

func (f *runtimeLoginStatusChannel) Name() string { return f.name }

func (f *runtimeLoginStatusChannel) Start(context.Context, channel.Handler) error { return nil }

func (f *runtimeLoginStatusChannel) Stop(context.Context) error { return nil }

func (f *runtimeLoginStatusChannel) Send(context.Context, channel.OutboundMessage) error { return nil }

func (f *runtimeLoginStatusChannel) Status(context.Context) (channel.Status, error) {
	return f.status, nil
}

func (f *runtimeLoginStatusChannel) Login(context.Context) error {
	f.logins++
	return nil
}
