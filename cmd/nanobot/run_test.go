package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestRunCommandUsesFlagsAndLauncher(t *testing.T) {
	original := launchDaemon
	defer func() { launchDaemon = original }()

	called := false
	var got daemonLaunchOptions
	launchDaemon = func(ctx context.Context, opts daemonLaunchOptions) error {
		called = true
		got = opts
		return nil
	}

	cmd := NewRootCmd("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"run",
		"--port", "19000",
		"--workspace", "/tmp/work",
		"--config", "/tmp/config.json",
		"--verbose",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !called {
		t.Fatal("expected launcher to be called")
	}
	if got.Port != 19000 || got.Workspace != "/tmp/work" || got.ConfigPath != "/tmp/config.json" || !got.Verbose {
		t.Fatalf("unexpected launch options %#v", got)
	}
}

func TestStatusCommandPrintsGatewayStatus(t *testing.T) {
	orig := fetchStatus
	defer func() { fetchStatus = orig }()

	fetchStatus = func(context.Context, rootOptions) (*statusReport, error) {
		return &statusReport{
			Model:            "claude-sonnet",
			UptimeSeconds:    12,
			Channels:         []string{"slack", "discord"},
			ConnectedClients: 3,
		}, nil
	}

	cmd := NewRootCmd("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"status"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	rendered := out.String()
	for _, needle := range []string{"claude-sonnet", "12", "slack", "discord", "3"} {
		if !strings.Contains(rendered, needle) {
			t.Fatalf("expected %q in output %q", needle, rendered)
		}
	}
}

func TestChannelsCommands(t *testing.T) {
	statusOrig := fetchChannelStatuses
	loginOrig := runChannelLogin
	defer func() {
		fetchChannelStatuses = statusOrig
		runChannelLogin = loginOrig
	}()

	fetchChannelStatuses = func(context.Context, rootOptions) ([]channelStatus, error) {
		return []channelStatus{
			{Name: "slack", State: "connected"},
			{Name: "discord", State: "disconnected"},
		}, nil
	}

	var loginCalled string
	runChannelLogin = func(_ context.Context, _ rootOptions, channelName string, out io.Writer) error {
		loginCalled = channelName
		_, _ = fmt.Fprintln(out, "qr: code-1")
		return nil
	}

	cmd := NewRootCmd("test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"channels", "status"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute status: %v", err)
	}
	if got := out.String(); !strings.Contains(got, "slack") || !strings.Contains(got, "discord") {
		t.Fatalf("unexpected channels status output %q", got)
	}

	out.Reset()
	cmd = NewRootCmd("test")
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"channels", "login", "slack"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute login: %v", err)
	}
	if loginCalled != "slack" {
		t.Fatalf("expected login flow for slack, got %q", loginCalled)
	}
	if got := out.String(); !strings.Contains(got, "qr: code-1") {
		t.Fatalf("expected login updates in output %q", got)
	}
	if got := out.String(); strings.Contains(got, "complete") {
		t.Fatalf("expected no misleading completion line in output %q", got)
	}
}

func TestChannelsLoginUsesCommandContext(t *testing.T) {
	loginOrig := runChannelLogin
	defer func() { runChannelLogin = loginOrig }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	runChannelLogin = func(ctx context.Context, _ rootOptions, _ string, _ io.Writer) error {
		if err := ctx.Err(); err == nil {
			t.Fatal("expected canceled command context")
		}
		return ctx.Err()
	}

	cmd := NewRootCmd("test")
	cmd.SetContext(ctx)
	cmd.SetArgs([]string{"channels", "login", "signal"})

	if err := cmd.Execute(); err == nil {
		t.Fatal("expected Execute to fail with canceled context")
	}
}
