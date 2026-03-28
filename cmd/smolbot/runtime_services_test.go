package main

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/channel"
	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/Nomadcxx/smolbot/pkg/provider"
)

func TestLaunchDaemonStartsAndStopsRegisteredChannels(t *testing.T) {
	orig := launchRuntimeDeps
	defer func() { launchRuntimeDeps = orig }()

	fakeChannel := &runtimeLifecycleChannel{name: "slack"}
	launchRuntimeDeps = func() runtimeDeps {
		return runtimeDeps{
			Channels: []channel.Channel{fakeChannel},
		}
	}

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
			t.Fatalf("launchDaemon: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("launchDaemon did not stop")
	}

	if fakeChannel.starts != 1 {
		t.Fatalf("expected channel start, got %d", fakeChannel.starts)
	}
	if fakeChannel.stops != 1 {
		t.Fatalf("expected channel stop, got %d", fakeChannel.stops)
	}
}

func TestBuildRuntimeRegistersCronTool(t *testing.T) {
	port := freePort(t)
	cfgPath := writeTestConfig(t, port)

	app, err := buildRuntime(daemonLaunchOptions{
		ConfigPath: cfgPath,
		Port:       port,
	}, runtimeDeps{})
	if err != nil {
		t.Fatalf("buildRuntime: %v", err)
	}
	defer app.Close()

	names := make([]string, 0, len(app.tools.Definitions()))
	for _, def := range app.tools.Definitions() {
		names = append(names, def.Name)
	}
	if !slices.Contains(names, "cron") {
		t.Fatalf("expected cron tool in runtime definitions, got %#v", names)
	}
}

func TestBuildRuntimeConfiguresOllamaQuotaRunner(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "ollama/llama3.2"
	cfg.Agents.Defaults.Provider = "ollama"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = freePort(t)

	cfgPath := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(cfgPath, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}

	app, err := buildRuntime(daemonLaunchOptions{
		ConfigPath: cfgPath,
		Port:       cfg.Gateway.Port,
	}, runtimeDeps{})
	if err != nil {
		t.Fatalf("buildRuntime: %v", err)
	}
	defer app.Close()

	if app.runQuota == nil {
		t.Fatal("expected Ollama runtime to configure quota runner")
	}
	if app.quotaEvery != time.Hour {
		t.Fatalf("quotaEvery = %s, want %s", app.quotaEvery, time.Hour)
	}
}

func TestLaunchDaemonRunsCronAndHeartbeatLoops(t *testing.T) {
	orig := launchRuntimeDeps
	defer func() { launchRuntimeDeps = orig }()

	var cronCalls atomic.Int32
	var heartbeatCalls atomic.Int32
	var quotaCalls atomic.Int32
	launchRuntimeDeps = func() runtimeDeps {
		return runtimeDeps{
			CronRun: func(context.Context, time.Time) error {
				cronCalls.Add(1)
				return nil
			},
			CronInterval: 10 * time.Millisecond,
			HeartbeatRun: func(context.Context) error {
				heartbeatCalls.Add(1)
				return nil
			},
			HeartbeatInterval: 10 * time.Millisecond,
			HeartbeatEnabled:  true,
			QuotaRun: func(context.Context) error {
				quotaCalls.Add(1)
				return nil
			},
			QuotaInterval: 10 * time.Millisecond,
		}
	}

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
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if cronCalls.Load() > 0 && heartbeatCalls.Load() > 0 && quotaCalls.Load() > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	if err := <-errCh; err != nil {
		t.Fatalf("launchDaemon: %v", err)
	}

	if cronCalls.Load() == 0 {
		t.Fatal("expected cron loop to run")
	}
	if heartbeatCalls.Load() == 0 {
		t.Fatal("expected heartbeat loop to run")
	}
	if quotaCalls.Load() == 0 {
		t.Fatal("expected quota loop to run")
	}
}

func TestLaunchDaemonShutsDownServerOnBackgroundLoopError(t *testing.T) {
	orig := launchRuntimeDeps
	defer func() { launchRuntimeDeps = orig }()

	var calls atomic.Int32
	launchRuntimeDeps = func() runtimeDeps {
		return runtimeDeps{
			CronRun: func(context.Context, time.Time) error {
				if calls.Add(1) == 1 {
					return nil
				}
				return errors.New("cron boom")
			},
			CronInterval: 25 * time.Millisecond,
		}
	}

	port := freePort(t)
	cfgPath := writeTestConfig(t, port)
	ctx := context.Background()
	errCh := make(chan error, 1)
	go func() {
		errCh <- launchDaemon(ctx, daemonLaunchOptions{
			ConfigPath: cfgPath,
			Port:       port,
		})
	}()

	waitForHealth(t, port)
	err := <-errCh
	if err == nil || !strings.Contains(err.Error(), "cron boom") {
		t.Fatalf("launchDaemon error = %v, want cron boom", err)
	}

	client := &http.Client{Timeout: 200 * time.Millisecond}
	resp, reqErr := client.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/health")
	if reqErr == nil {
		_ = resp.Body.Close()
		t.Fatal("expected health server to be shut down after background loop failure")
	}
}

func TestLaunchDaemonRoutesInboundChannelMessages(t *testing.T) {
	for _, channelName := range []string{"signal", "whatsapp"} {
		t.Run(channelName, func(t *testing.T) {
			orig := launchRuntimeDeps
			defer func() { launchRuntimeDeps = orig }()

			fakeChannel := &runtimeInboundChannel{
				name: channelName,
				inbound: channel.InboundMessage{
					Channel: channelName,
					ChatID:  "C42",
					Content: "hello from channel",
				},
			}
			launchRuntimeDeps = func() runtimeDeps {
				return runtimeDeps{
					Channels: []channel.Channel{fakeChannel},
					Provider: &fakeRuntimeProvider{
						deltas: []*provider.StreamDelta{
							{Content: "reply to channel"},
							{FinishReason: stringPtr("stop")},
						},
					},
				}
			}

			port := freePort(t)
			cfg := config.DefaultConfig()
			cfg.Agents.Defaults.Model = "gpt-test"
			cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
			cfg.Gateway.Host = "127.0.0.1"
			cfg.Gateway.Port = port
			switch channelName {
			case "signal":
				cfg.Channels.Signal.Enabled = true
			case "whatsapp":
				cfg.Channels.WhatsApp.Enabled = true
			}
			cfgPath := filepath.Join(t.TempDir(), "config.json")
			if err := writeConfigFile(cfgPath, &cfg); err != nil {
				t.Fatalf("writeConfigFile: %v", err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			errCh := make(chan error, 1)
			go func() {
				errCh <- launchDaemon(ctx, daemonLaunchOptions{
					ConfigPath: cfgPath,
					Port:       port,
				})
			}()

			waitForHealth(t, port)
			deadline := time.Now().Add(500 * time.Millisecond)
			for time.Now().Before(deadline) {
				if fakeChannel.sent.Load() > 0 {
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
			cancel()
			if err := <-errCh; err != nil {
				t.Fatalf("launchDaemon: %v", err)
			}

			if fakeChannel.sent.Load() == 0 {
				t.Fatal("expected inbound channel message to produce an outbound reply")
			}
			last := fakeChannel.Last()
			if last.ChatID != "C42" || last.Content != "reply to channel" {
				t.Fatalf("unexpected outbound reply %#v", last)
			}
		})
	}
}

type runtimeFakeChannel struct {
	name  string
	last  channel.OutboundMessage
	calls int
}

func (f *runtimeFakeChannel) Name() string { return f.name }

func (f *runtimeFakeChannel) Start(context.Context, channel.Handler) error { return nil }

func (f *runtimeFakeChannel) Stop(context.Context) error { return nil }

func (f *runtimeFakeChannel) Send(_ context.Context, msg channel.OutboundMessage) error {
	f.last = msg
	f.calls++
	return nil
}

func TestBuildRuntimeHeartbeatUsesDecisionAndEvaluator(t *testing.T) {
	port := freePort(t)
	cfgPath := writeHeartbeatConfig(t, port, true, "slack")

	fakeChannel := &runtimeFakeChannel{name: "slack"}
	fakeProvider := &fakeRuntimeProvider{
		chatResponses: []*provider.Response{
			{Content: "run"},
			{Content: "deliver=true"},
		},
		deltas: []*provider.StreamDelta{
			{Content: "heartbeat output"},
			{FinishReason: stringPtr("stop")},
		},
	}

	app, err := buildRuntime(daemonLaunchOptions{
		ConfigPath: cfgPath,
		Port:       port,
	}, runtimeDeps{
		Provider: fakeProvider,
		Channels: []channel.Channel{fakeChannel},
	})
	if err != nil {
		t.Fatalf("buildRuntime: %v", err)
	}
	defer app.Close()

	if err := app.heartbeat.RunOnce(context.Background()); err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if len(fakeProvider.chatRequests) != 2 {
		t.Fatalf("expected decider and evaluator chat requests, got %d", len(fakeProvider.chatRequests))
	}
	if len(fakeProvider.requests) == 0 {
		t.Fatal("expected heartbeat execution to reach streaming agent path")
	}
	if fakeChannel.last.ChatID != "slack" || fakeChannel.last.Content != "heartbeat output" {
		t.Fatalf("unexpected heartbeat outbound %#v", fakeChannel.last)
	}
}

func TestBuildRuntimeHeartbeatIntervalUsesMinutes(t *testing.T) {
	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Gateway.Heartbeat.Enabled = true
	cfg.Gateway.Heartbeat.Interval = 2

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

	if app.beatEvery != 2*time.Minute {
		t.Fatalf("heartbeat interval = %v, want %v", app.beatEvery, 2*time.Minute)
	}
}

func TestHandleInboundUsesRequestContextForOutboundRouting(t *testing.T) {
	port := freePort(t)
	cfgPath := writeTestConfig(t, port)

	blocking := &runtimeBlockingSendChannel{name: "slack", sendErr: make(chan error, 1)}
	app, err := buildRuntime(daemonLaunchOptions{
		ConfigPath: cfgPath,
		Port:       port,
	}, runtimeDeps{
		Provider: &fakeRuntimeProvider{
			deltas: []*provider.StreamDelta{
				{Content: "reply to channel"},
				{FinishReason: stringPtr("stop")},
			},
		},
		Channels: []channel.Channel{blocking},
	})
	if err != nil {
		t.Fatalf("buildRuntime: %v", err)
	}
	defer app.Close()

	ctx, cancel := context.WithCancel(context.Background())
	app.handleInbound(ctx, channel.InboundMessage{
		Channel: "slack",
		ChatID:  "C42",
		Content: "hello from channel",
	})
	cancel()

	select {
	case err := <-blocking.sendErr:
		if err == nil {
			t.Fatal("expected send to observe context cancellation")
		}
	case <-time.After(time.Second):
		t.Fatal("expected send to exit on context cancellation")
	}
}

type runtimeLifecycleChannel struct {
	name   string
	starts int
	stops  int
}

func (f *runtimeLifecycleChannel) Name() string { return f.name }

func (f *runtimeLifecycleChannel) Start(context.Context, channel.Handler) error {
	f.starts++
	return nil
}

func (f *runtimeLifecycleChannel) Stop(context.Context) error {
	f.stops++
	return nil
}

func (f *runtimeLifecycleChannel) Send(context.Context, channel.OutboundMessage) error {
	return nil
}

type runtimeInboundChannel struct {
	name    string
	inbound channel.InboundMessage
	sent    atomic.Int32
	mu      sync.Mutex
	last    channel.OutboundMessage
}

func (f *runtimeInboundChannel) Name() string { return f.name }

func (f *runtimeInboundChannel) Start(ctx context.Context, handler channel.Handler) error {
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Millisecond):
			handler(ctx, f.inbound)
		}
	}()
	return nil
}

func (f *runtimeInboundChannel) Stop(context.Context) error { return nil }

func (f *runtimeInboundChannel) Send(_ context.Context, msg channel.OutboundMessage) error {
	f.mu.Lock()
	f.last = msg
	f.mu.Unlock()
	f.sent.Add(1)
	return nil
}

func (f *runtimeInboundChannel) Last() channel.OutboundMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.last
}

type runtimeBlockingSendChannel struct {
	name    string
	sendErr chan error
}

func (f *runtimeBlockingSendChannel) Name() string { return f.name }

func (f *runtimeBlockingSendChannel) Start(context.Context, channel.Handler) error { return nil }

func (f *runtimeBlockingSendChannel) Stop(context.Context) error { return nil }

func (f *runtimeBlockingSendChannel) Send(ctx context.Context, _ channel.OutboundMessage) error {
	if f.sendErr == nil {
		f.sendErr = make(chan error, 1)
	}
	<-ctx.Done()
	f.sendErr <- ctx.Err()
	return ctx.Err()
}

func writeHeartbeatConfig(t *testing.T, port int, enabled bool, channelName string) string {
	t.Helper()
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Provider = "openai"
	cfg.Agents.Defaults.Workspace = filepath.Join(t.TempDir(), "workspace")
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Gateway.Heartbeat.Enabled = enabled
	cfg.Gateway.Heartbeat.Interval = 1
	cfg.Gateway.Heartbeat.Channel = channelName

	path := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(path, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}
	return path
}
