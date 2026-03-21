package main

import (
	"context"
	"strings"
	"testing"
)

func TestFetchChannelStatusesWithDetail(t *testing.T) {
	orig := fetchStatus
	defer func() { fetchStatus = orig }()

	fetchStatus = func(ctx context.Context, opts rootOptions) (*statusReport, error) {
		return &statusReport{
			Model:         "claude-sonnet",
			UptimeSeconds: 12,
			Channels:      []string{"signal", "whatsapp"},
			ChannelStates: map[string]map[string]string{
				"signal": {
					"state":  "connected",
					"detail": "signal-cli ready",
				},
				"whatsapp": {
					"state":  "auth_required",
					"detail": "device not linked",
				},
			},
			ConnectedClients: 1,
		}, nil
	}

	statuses, err := fetchChannelStatuses(context.Background(), rootOptions{})
	if err != nil {
		t.Fatalf("fetchChannelStatuses: %v", err)
	}
	if len(statuses) != 2 {
		t.Fatalf("expected 2 statuses, got %d", len(statuses))
	}

	for _, s := range statuses {
		if s.Name == "signal" {
			if s.State != "connected" {
				t.Errorf("signal: expected state=%q, got %q", "connected", s.State)
			}
			if s.Detail != "signal-cli ready" {
				t.Errorf("signal: expected detail=%q, got %q", "signal-cli ready", s.Detail)
			}
		}
		if s.Name == "whatsapp" {
			if s.State != "auth_required" {
				t.Errorf("whatsapp: expected state=%q, got %q", "auth_required", s.State)
			}
			if s.Detail != "device not linked" {
				t.Errorf("whatsapp: expected detail=%q, got %q", "device not linked", s.Detail)
			}
		}
	}
}

func TestChannelsStatusCommandRendersDetail(t *testing.T) {
	orig := fetchChannelStatuses
	defer func() { fetchChannelStatuses = orig }()

	fetchChannelStatuses = func(ctx context.Context, opts rootOptions) ([]channelStatus, error) {
		return []channelStatus{
			{Name: "signal", State: "connected", Detail: "signal-cli ready"},
			{Name: "whatsapp", State: "auth_required", Detail: "device not linked"},
		}, nil
	}

	cmd := newChannelsStatusCmd(&rootOptions{})
	var buf strings.Builder
	cmd.SetOut(&buf)
	cmd.SetArgs(nil)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	out := buf.String()
	for _, tc := range []struct {
		needle string
	}{
		{"signal"},
		{"connected"},
		{"signal-cli ready"},
		{"whatsapp"},
		{"auth_required"},
		{"device not linked"},
	} {
		if !strings.Contains(out, tc.needle) {
			t.Errorf("expected %q in channels status output %q", tc.needle, out)
		}
	}
}
