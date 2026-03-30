package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
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
			"model":    "claude-sonnet",
			"uptime":   42,
			"channels": []map[string]string{
				{"name": "slack", "status": "connected"},
				{"name": "discord", "status": "error"},
			},
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
	if report.Model != "claude-sonnet" || report.Uptime != 42 {
		t.Fatalf("unexpected status report %#v", report)
	}
	if len(report.Channels) != 2 || report.Channels[0].Name != "slack" || report.Channels[1].Name != "discord" {
		t.Fatalf("unexpected channels %#v", report.Channels)
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
			"model":    "claude-sonnet",
			"uptime":   42,
			"channels": []map[string]string{
				{"name": "slack", "status": "connected"},
				{"name": "discord", "status": "error"},
			},
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
	cfg.Channels.Discord.Enabled = false

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
			&runtimeLifecycleChannel{name: "discord"},
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

func TestBuildRuntimeRegistersSignalWhatsAppAndTelegramTogether(t *testing.T) {
	origSignal := newSignalChannel
	origWhatsApp := newWhatsAppChannel
	origTelegram := newTelegramChannel
	origDiscord := newDiscordChannel
	defer func() {
		newSignalChannel = origSignal
		newWhatsAppChannel = origWhatsApp
		newTelegramChannel = origTelegram
		newDiscordChannel = origDiscord
	}()

	newSignalChannel = func(cfg config.SignalChannelConfig) channel.Channel {
		if cfg.Account != "+15551234567" {
			t.Fatalf("unexpected signal config %#v", cfg)
		}
		return &runtimeLoginStatusChannel{name: "signal", status: channel.Status{State: "connected"}}
	}
	newWhatsAppChannel = func(cfg config.WhatsAppChannelConfig) (channel.Channel, error) {
		if cfg.DeviceName != "smolbot-test" {
			t.Fatalf("unexpected whatsapp config %#v", cfg)
		}
		return &runtimeLoginStatusChannel{name: "whatsapp", status: channel.Status{State: "connected"}}, nil
	}
	newTelegramChannel = func(cfg config.TelegramChannelConfig) (channel.Channel, error) {
		if cfg.BotToken != "telegram-bot-token" {
			t.Fatalf("unexpected telegram config %#v", cfg)
		}
		return &runtimeLoginStatusChannel{name: "telegram", status: channel.Status{State: "connected"}}, nil
	}
	newDiscordChannel = func(cfg config.DiscordChannelConfig) (channel.Channel, error) {
		if cfg.BotToken != "discord-bot-token" {
			t.Fatalf("unexpected discord config %#v", cfg)
		}
		return &runtimeLoginStatusChannel{name: "discord", status: channel.Status{State: "connected"}}, nil
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Channels.Signal.Enabled = true
	cfg.Channels.Signal.Account = "+15551234567"
	cfg.Channels.WhatsApp.Enabled = true
	cfg.Channels.WhatsApp.DeviceName = "smolbot-test"
	cfg.Channels.Telegram.Enabled = true
	cfg.Channels.Telegram.BotToken = "telegram-bot-token"
	cfg.Channels.Discord.Enabled = true
	cfg.Channels.Discord.BotToken = "discord-bot-token"

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

	if got := strings.Join(app.channels.ChannelNames(), ","); got != "discord,signal,telegram,whatsapp" {
		t.Fatalf("registered channels = %q, want discord,signal,telegram,whatsapp", got)
	}
	statuses := app.channels.Statuses(context.Background())
	if statuses["signal"].State != "connected" || statuses["whatsapp"].State != "connected" || statuses["telegram"].State != "connected" || statuses["discord"].State != "connected" {
		t.Fatalf("unexpected channel statuses %#v", statuses)
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

func TestBuildRuntimeConstructsConfiguredTelegramChannel(t *testing.T) {
	orig := newTelegramChannel
	defer func() { newTelegramChannel = orig }()

	fakeTelegram := &runtimeLoginStatusChannel{name: "telegram", status: channel.Status{State: "connected"}}
	newTelegramChannel = func(cfg config.TelegramChannelConfig) (channel.Channel, error) {
		if cfg.BotToken != "telegram-bot-token" {
			t.Fatalf("unexpected telegram config %#v", cfg)
		}
		return fakeTelegram, nil
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Channels.Telegram.Enabled = true
	cfg.Channels.Telegram.BotToken = "telegram-bot-token"

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

	if got := strings.Join(app.channels.ChannelNames(), ","); got != "telegram" {
		t.Fatalf("registered channels = %q, want telegram", got)
	}
	if state := app.channels.Statuses(context.Background())["telegram"].State; state != "connected" {
		t.Fatalf("telegram status = %q, want connected", state)
	}
}

func TestBuildRuntimeConstructsConfiguredDiscordChannel(t *testing.T) {
	orig := newDiscordChannel
	defer func() { newDiscordChannel = orig }()

	fakeDiscord := &runtimeLoginStatusChannel{name: "discord", status: channel.Status{State: "connected"}}
	newDiscordChannel = func(cfg config.DiscordChannelConfig) (channel.Channel, error) {
		if cfg.BotToken != "discord-bot-token" {
			t.Fatalf("unexpected discord config %#v", cfg)
		}
		if got := strings.Join(cfg.AllowedChannelIDs, ","); got != "111,222" {
			t.Fatalf("unexpected discord allowlist %#v", cfg.AllowedChannelIDs)
		}
		return fakeDiscord, nil
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Channels.Discord.Enabled = true
	cfg.Channels.Discord.BotToken = "discord-bot-token"
	cfg.Channels.Discord.AllowedChannelIDs = []string{"111", "222"}

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

	if got := strings.Join(app.channels.ChannelNames(), ","); got != "discord" {
		t.Fatalf("registered channels = %q, want discord", got)
	}
	if state := app.channels.Statuses(context.Background())["discord"].State; state != "connected" {
		t.Fatalf("discord status = %q, want connected", state)
	}
}

func TestRunChannelLoginUsesConfiguredTelegramChannel(t *testing.T) {
	orig := newTelegramChannel
	defer func() { newTelegramChannel = orig }()

	fakeTelegram := &runtimeLoginStatusChannel{name: "telegram"}
	newTelegramChannel = func(cfg config.TelegramChannelConfig) (channel.Channel, error) {
		if cfg.BotToken != "telegram-bot-token" {
			t.Fatalf("unexpected telegram config %#v", cfg)
		}
		return fakeTelegram, nil
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Channels.Telegram.Enabled = true
	cfg.Channels.Telegram.BotToken = "telegram-bot-token"

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	if err := runChannelLoginImpl(context.Background(), rootOptions{configPath: cfgPath}, "telegram", io.Discard); err != nil {
		t.Fatalf("runChannelLoginImpl: %v", err)
	}
	if fakeTelegram.logins != 1 {
		t.Fatalf("telegram login calls = %d, want 1", fakeTelegram.logins)
	}
}

func TestRunChannelLoginUsesConfiguredDiscordChannel(t *testing.T) {
	orig := newDiscordChannel
	defer func() { newDiscordChannel = orig }()

	fakeDiscord := &runtimeDiscordLoginChannel{name: "discord"}
	newDiscordChannel = func(cfg config.DiscordChannelConfig) (channel.Channel, error) {
		if cfg.BotToken != "discord-bot-token" {
			t.Fatalf("unexpected discord config %#v", cfg)
		}
		return fakeDiscord, nil
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Channels.Discord.Enabled = true
	cfg.Channels.Discord.BotToken = "discord-bot-token"

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	var out strings.Builder
	if err := runChannelLoginImpl(context.Background(), rootOptions{configPath: cfgPath}, "discord", &out); err != nil {
		t.Fatalf("runChannelLoginImpl: %v", err)
	}
	if fakeDiscord.logins != 1 {
		t.Fatalf("discord login calls = %d, want 1", fakeDiscord.logins)
	}
	if got := out.String(); !strings.Contains(got, "connected: Bot: @smolbot") {
		t.Fatalf("expected discord login status in output %q", got)
	}
}

func TestRunChannelLoginReportsDiscordStatusWhenLoginFails(t *testing.T) {
	orig := newDiscordChannel
	defer func() { newDiscordChannel = orig }()

	wantErr := errors.New("invalid discord token")
	fakeDiscord := &runtimeDiscordLoginChannel{
		name:     "discord",
		status:   channel.Status{State: "auth-required", Detail: "Invalid token: invalid discord token"},
		loginErr: wantErr,
	}
	newDiscordChannel = func(cfg config.DiscordChannelConfig) (channel.Channel, error) {
		if cfg.BotToken != "discord-bot-token" {
			t.Fatalf("unexpected discord config %#v", cfg)
		}
		return fakeDiscord, nil
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Channels.Discord.Enabled = true
	cfg.Channels.Discord.BotToken = "discord-bot-token"

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	var out strings.Builder
	err := runChannelLoginImpl(context.Background(), rootOptions{configPath: cfgPath}, "discord", &out)
	if !errors.Is(err, wantErr) {
		t.Fatalf("runChannelLoginImpl error = %v, want %v", err, wantErr)
	}
	rendered := out.String()
	if !strings.Contains(rendered, "auth-required") || !strings.Contains(rendered, "Invalid token: invalid discord token") {
		t.Fatalf("expected discord auth-required status in output %q", rendered)
	}
	if fakeDiscord.logins != 1 {
		t.Fatalf("discord login calls = %d, want 1", fakeDiscord.logins)
	}
}

func TestRunChannelLoginUsesInteractiveTelegramLogin(t *testing.T) {
	orig := newTelegramChannel
	defer func() { newTelegramChannel = orig }()

	fakeTelegram := &runtimeInteractiveTelegramChannel{name: "telegram"}
	newTelegramChannel = func(cfg config.TelegramChannelConfig) (channel.Channel, error) {
		if cfg.BotToken != "telegram-bot-token" {
			t.Fatalf("unexpected telegram config %#v", cfg)
		}
		return fakeTelegram, nil
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Channels.Telegram.Enabled = true
	cfg.Channels.Telegram.BotToken = "telegram-bot-token"

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	var out strings.Builder
	if err := runChannelLoginImpl(context.Background(), rootOptions{configPath: cfgPath}, "telegram", &out); err != nil {
		t.Fatalf("runChannelLoginImpl: %v", err)
	}
	if fakeTelegram.interactiveLogins != 1 {
		t.Fatalf("telegram interactive login calls = %d, want 1", fakeTelegram.interactiveLogins)
	}
	if fakeTelegram.loginCalls != 0 {
		t.Fatalf("telegram fallback login calls = %d, want 0", fakeTelegram.loginCalls)
	}
	rendered := out.String()
	if !strings.Contains(rendered, "connecting: Validating bot token...") {
		t.Fatalf("expected interactive login updates in output %q", rendered)
	}
	if !strings.Contains(rendered, "connected: Bot: @smolbot") {
		t.Fatalf("expected connected status in output %q", rendered)
	}
}

func TestRunChannelLoginDoesNotReplayInteractiveTelegramFailureStatus(t *testing.T) {
	orig := newTelegramChannel
	defer func() { newTelegramChannel = orig }()

	wantErr := errors.New("telegram login failed")
	fakeTelegram := &runtimeInteractiveTelegramChannel{
		name:          "telegram",
		loginErr:      wantErr,
		terminalState: channel.Status{State: "auth-required", Detail: "Invalid token: telegram login failed"},
	}
	newTelegramChannel = func(cfg config.TelegramChannelConfig) (channel.Channel, error) {
		if cfg.BotToken != "telegram-bot-token" {
			t.Fatalf("unexpected telegram config %#v", cfg)
		}
		return fakeTelegram, nil
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Channels.Telegram.Enabled = true
	cfg.Channels.Telegram.BotToken = "telegram-bot-token"

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	var out strings.Builder
	err := runChannelLoginImpl(context.Background(), rootOptions{configPath: cfgPath}, "telegram", &out)
	if !errors.Is(err, wantErr) {
		t.Fatalf("runChannelLoginImpl error = %v, want %v", err, wantErr)
	}
	rendered := out.String()
	if got := strings.Count(rendered, "auth-required: Invalid token: telegram login failed"); got != 1 {
		t.Fatalf("expected terminal telegram failure status once, got %d in output %q", got, rendered)
	}
}

func TestRunChannelLoginReportsStatusOnlyTelegramLoginClearly(t *testing.T) {
	orig := newTelegramChannel
	defer func() { newTelegramChannel = orig }()

	newTelegramChannel = func(cfg config.TelegramChannelConfig) (channel.Channel, error) {
		if cfg.BotToken != "telegram-bot-token" {
			t.Fatalf("unexpected telegram config %#v", cfg)
		}
		return &runtimeStatusOnlyTelegramChannel{name: "telegram"}, nil
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Channels.Telegram.Enabled = true
	cfg.Channels.Telegram.BotToken = "telegram-bot-token"

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	err := runChannelLoginImpl(context.Background(), rootOptions{configPath: cfgPath}, "telegram", io.Discard)
	if err == nil {
		t.Fatal("expected telegram login to fail for status-only channel")
	}
	if got := err.Error(); !strings.Contains(got, "telegram") || !strings.Contains(got, "does not support login") {
		t.Fatalf("unexpected error %q", got)
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

func TestRunChannelLoginUsesConfiguredDiscordChannelWhenDisabled(t *testing.T) {
	orig := newDiscordChannel
	defer func() { newDiscordChannel = orig }()

	fakeDiscord := &runtimeDiscordLoginChannel{name: "discord"}
	newDiscordChannel = func(cfg config.DiscordChannelConfig) (channel.Channel, error) {
		if cfg.BotToken != "discord-bot-token" {
			t.Fatalf("unexpected discord config %#v", cfg)
		}
		return fakeDiscord, nil
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Channels.Discord.Enabled = false
	cfg.Channels.Discord.BotToken = "discord-bot-token"

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	if err := runChannelLoginImpl(context.Background(), rootOptions{configPath: cfgPath}, "discord", io.Discard); err != nil {
		t.Fatalf("runChannelLoginImpl: %v", err)
	}
	if fakeDiscord.logins != 1 {
		t.Fatalf("discord login calls = %d, want 1", fakeDiscord.logins)
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

func TestBuildRuntimeReturnsConfiguredTelegramChannelError(t *testing.T) {
	orig := newTelegramChannel
	defer func() { newTelegramChannel = orig }()

	wantErr := errors.New("telegram seam failed")
	newTelegramChannel = func(cfg config.TelegramChannelConfig) (channel.Channel, error) {
		if cfg.BotToken == "" {
			t.Fatalf("expected bot token in telegram config %#v", cfg)
		}
		return nil, wantErr
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Channels.Telegram.Enabled = true
	cfg.Channels.Telegram.BotToken = "telegram-bot-token"

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

func TestBuildRuntimeReturnsConfiguredDiscordChannelError(t *testing.T) {
	orig := newDiscordChannel
	defer func() { newDiscordChannel = orig }()

	wantErr := errors.New("discord seam failed")
	newDiscordChannel = func(cfg config.DiscordChannelConfig) (channel.Channel, error) {
		if cfg.BotToken == "" {
			t.Fatalf("expected bot token in discord config %#v", cfg)
		}
		return nil, wantErr
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Channels.Discord.Enabled = true
	cfg.Channels.Discord.BotToken = "discord-bot-token"

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

func TestBuildRuntimeWiresSkillsToGateway(t *testing.T) {
	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
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
		Provider: &fakeRuntimeProvider{},
	})
	if err != nil {
		t.Fatalf("buildRuntime: %v", err)
	}
	defer app.Close()

	httpServer := httptest.NewServer(app.gateway.Handler())
	defer httpServer.Close()

	cl := connectGatewayClient(t, httpServer.URL)
	defer cl.Close()

	skills, err := cl.Skills()
	if err != nil {
		t.Fatalf("Skills: %v", err)
	}
	if len(skills) == 0 {
		t.Fatal("skills.list returned empty list — Skills not wired to gateway in buildRuntime")
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

type runtimeInteractiveTelegramChannel struct {
	name              string
	status            channel.Status
	interactiveLogins int
	loginCalls        int
	loginErr          error
	terminalState     channel.Status
}

func (f *runtimeInteractiveTelegramChannel) Name() string { return f.name }

func (f *runtimeInteractiveTelegramChannel) Start(context.Context, channel.Handler) error { return nil }

func (f *runtimeInteractiveTelegramChannel) Stop(context.Context) error { return nil }

func (f *runtimeInteractiveTelegramChannel) Send(context.Context, channel.OutboundMessage) error {
	return nil
}

func (f *runtimeInteractiveTelegramChannel) Status(context.Context) (channel.Status, error) {
	return f.status, nil
}

func (f *runtimeInteractiveTelegramChannel) Login(context.Context) error {
	f.loginCalls++
	return nil
}

func (f *runtimeInteractiveTelegramChannel) LoginWithUpdates(ctx context.Context, report func(channel.Status) error) error {
	f.interactiveLogins++
	connecting := channel.Status{State: "connecting", Detail: "Validating bot token..."}
	f.status = connecting
	if report != nil {
		if err := report(connecting); err != nil {
			return err
		}
	}
	if f.loginErr != nil {
		terminal := f.terminalState
		if terminal.State == "" {
			terminal = channel.Status{State: "error", Detail: f.loginErr.Error()}
		}
		f.status = terminal
		if report != nil {
			if err := report(terminal); err != nil {
				return err
			}
		}
		return f.loginErr
	}
	connected := channel.Status{State: "connected", Detail: "Bot: @smolbot"}
	f.status = connected
	if report != nil {
		if err := report(connected); err != nil {
			return err
		}
	}
	return nil
}

type runtimeStatusOnlyTelegramChannel struct {
	name string
}

func (f *runtimeStatusOnlyTelegramChannel) Name() string { return f.name }

func (f *runtimeStatusOnlyTelegramChannel) Start(context.Context, channel.Handler) error { return nil }

func (f *runtimeStatusOnlyTelegramChannel) Stop(context.Context) error { return nil }

func (f *runtimeStatusOnlyTelegramChannel) Send(context.Context, channel.OutboundMessage) error {
	return nil
}

func (f *runtimeStatusOnlyTelegramChannel) Status(context.Context) (channel.Status, error) {
	return channel.Status{State: "registered", Detail: "status only"}, nil
}

type runtimeDiscordLoginChannel struct {
	name     string
	status   channel.Status
	logins   int
	loginErr error
}

func (f *runtimeDiscordLoginChannel) Name() string { return f.name }

func (f *runtimeDiscordLoginChannel) Start(context.Context, channel.Handler) error { return nil }

func (f *runtimeDiscordLoginChannel) Stop(context.Context) error { return nil }

func (f *runtimeDiscordLoginChannel) Send(context.Context, channel.OutboundMessage) error { return nil }

func (f *runtimeDiscordLoginChannel) Status(context.Context) (channel.Status, error) {
	return f.status, nil
}

func (f *runtimeDiscordLoginChannel) Login(context.Context) error {
	f.logins++
	if f.loginErr != nil {
		return f.loginErr
	}
	f.status = channel.Status{State: "connected", Detail: "Bot: @smolbot"}
	return nil
}
