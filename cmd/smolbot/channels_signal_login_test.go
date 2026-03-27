package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/channel"
	"github.com/Nomadcxx/smolbot/pkg/config"
)

func TestChannelsLoginRoutesSignalToDedicatedFlow(t *testing.T) {
	origSignal := runSignalLogin
	origGeneric := runChannelLogin
	defer func() {
		runSignalLogin = origSignal
		runChannelLogin = origGeneric
	}()

	called := false
	runSignalLogin = func(context.Context, rootOptions, io.Writer) error {
		called = true
		return nil
	}
	runChannelLogin = func(context.Context, rootOptions, string, io.Writer) error {
		t.Fatal("signal login should not use the generic manager login path")
		return nil
	}

	cmd := NewRootCmd("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"channels", "login", "signal"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !called {
		t.Fatal("expected dedicated signal login flow to be called")
	}
}

func TestRunSignalLoginUsesInteractiveLoginAndRendersProvisioningURI(t *testing.T) {
	orig := newSignalChannel
	defer func() { newSignalChannel = orig }()

	fake := &signalInteractiveLoginChannel{name: "signal"}
	newSignalChannel = func(cfg config.SignalChannelConfig) channel.Channel {
		if cfg.Account != "+15551234567" {
			t.Fatalf("unexpected signal config %#v", cfg)
		}
		return fake
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Channels.Signal.Enabled = true
	cfg.Channels.Signal.Account = "+15551234567"

	cfgPath := writeSignalLoginConfig(t, cfg)

	var out bytes.Buffer
	if err := runSignalLogin(context.Background(), rootOptions{configPath: cfgPath}, &out); err != nil {
		t.Fatalf("runSignalLogin: %v", err)
	}
	if fake.loginCalls != 0 {
		t.Fatalf("fallback login calls = %d, want 0", fake.loginCalls)
	}
	if fake.interactiveCalls != 1 {
		t.Fatalf("interactive login calls = %d, want 1", fake.interactiveCalls)
	}
	rendered := out.String()
	if !strings.Contains(rendered, "connecting: Linking device...") {
		t.Fatalf("expected connecting update in output %q", rendered)
	}
	if !strings.Contains(rendered, "auth-required: signal://provisioning-token") {
		t.Fatalf("expected provisioning URI in output %q", rendered)
	}
}

func TestRunSignalLoginCancelsCleanly(t *testing.T) {
	orig := newSignalChannel
	defer func() { newSignalChannel = orig }()

	fake := &signalBlockingLoginChannel{name: "signal"}
	newSignalChannel = func(cfg config.SignalChannelConfig) channel.Channel {
		return fake
	}

	port := freePort(t)
	cfg := config.DefaultConfig()
	cfg.Agents.Defaults.Model = "gpt-test"
	cfg.Agents.Defaults.Workspace = t.TempDir()
	cfg.Gateway.Host = "127.0.0.1"
	cfg.Gateway.Port = port
	cfg.Channels.Signal.Enabled = true
	cfg.Channels.Signal.Account = "+15551234567"

	cfgPath := writeSignalLoginConfig(t, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var out bytes.Buffer
	err := runSignalLogin(ctx, rootOptions{configPath: cfgPath}, &out)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("runSignalLogin error = %v, want context.Canceled", err)
	}
	if out.Len() != 0 {
		t.Fatalf("expected no output on cancellation, got %q", out.String())
	}
	if fake.interactiveCalls != 0 {
		t.Fatalf("interactive login calls = %d, want 0", fake.interactiveCalls)
	}
}

func writeSignalLoginConfig(t *testing.T, cfg config.Config) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := writeConfigFile(path, &cfg); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}
	return path
}

type signalInteractiveLoginChannel struct {
	name             string
	loginCalls       int
	interactiveCalls  int
}

func (f *signalInteractiveLoginChannel) Name() string { return f.name }

func (f *signalInteractiveLoginChannel) Start(context.Context, channel.Handler) error { return nil }

func (f *signalInteractiveLoginChannel) Stop(context.Context) error { return nil }

func (f *signalInteractiveLoginChannel) Send(context.Context, channel.OutboundMessage) error { return nil }

func (f *signalInteractiveLoginChannel) Login(context.Context) error {
	f.loginCalls++
	return nil
}

func (f *signalInteractiveLoginChannel) LoginWithUpdates(ctx context.Context, report func(channel.Status) error) error {
	f.interactiveCalls++
	if report != nil {
		if err := report(channel.Status{State: "connecting", Detail: "Linking device..."}); err != nil {
			return err
		}
		if err := report(channel.Status{State: "auth-required", Detail: "signal://provisioning-token"}); err != nil {
			return err
		}
	}
	return nil
}

type signalBlockingLoginChannel struct {
	name            string
	interactiveCalls int
}

func (f *signalBlockingLoginChannel) Name() string { return f.name }

func (f *signalBlockingLoginChannel) Start(context.Context, channel.Handler) error { return nil }

func (f *signalBlockingLoginChannel) Stop(context.Context) error { return nil }

func (f *signalBlockingLoginChannel) Send(context.Context, channel.OutboundMessage) error { return nil }

func (f *signalBlockingLoginChannel) Login(context.Context) error {
	panic("fallback login should not be used for signal")
}

func (f *signalBlockingLoginChannel) LoginWithUpdates(ctx context.Context, report func(channel.Status) error) error {
	f.interactiveCalls++
	<-ctx.Done()
	return ctx.Err()
}
