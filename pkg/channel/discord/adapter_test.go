package discord

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/channel"
	"github.com/Nomadcxx/smolbot/pkg/config"
)

func TestDiscordAdapterImplementsChannelStatusAndLogin(t *testing.T) {
	var _ channel.Channel = (*Adapter)(nil)
	var _ channel.StatusReporter = (*Adapter)(nil)
	var _ channel.LoginHandler = (*Adapter)(nil)
}

func TestDiscordAdapterDoesNotExposeInteractiveLogin(t *testing.T) {
	adapter := NewAdapter(&fakeSeam{})
	if _, ok := any(adapter).(channel.InteractiveLoginHandler); ok {
		t.Fatal("discord login should not be interactive")
	}
}

func TestDiscordNewProductionAdapterRequiresToken(t *testing.T) {
	if _, err := NewProductionAdapter(config.DiscordChannelConfig{}); err == nil {
		t.Fatal("expected NewProductionAdapter to reject an empty discord token")
	}
}

func TestDiscordNewProductionAdapterLoadsTokenFromFile(t *testing.T) {
	origFactory := newDiscordSeamFactory
	defer func() { newDiscordSeamFactory = origFactory }()

	var gotCfg config.DiscordChannelConfig
	newDiscordSeamFactory = func(cfg config.DiscordChannelConfig) (clientSeam, error) {
		gotCfg = cfg
		return &fakeSeam{}, nil
	}

	tokenFile := writeTempFile(t, "  discord-token-from-file  \n")
	adapter, err := NewProductionAdapter(config.DiscordChannelConfig{TokenFile: tokenFile})
	if err != nil {
		t.Fatalf("NewProductionAdapter: %v", err)
	}
	if adapter == nil {
		t.Fatal("expected adapter")
	}
	if gotCfg.BotToken != "discord-token-from-file" {
		t.Fatalf("bot token passed to seam = %q, want %q", gotCfg.BotToken, "discord-token-from-file")
	}
}

func TestDiscordNewProductionAdapterTrimsAllowedChannelIDs(t *testing.T) {
	origFactory := newDiscordSeamFactory
	defer func() { newDiscordSeamFactory = origFactory }()

	seam := &fakeSeam{
		startFn: func(_ context.Context, handle func(rawInboundMessage) error) error {
			if err := handle(rawInboundMessage{ChannelID: "111", Content: "allowed"}); err != nil {
				return err
			}
			if err := handle(rawInboundMessage{ChannelID: "222", Content: "also allowed"}); err != nil {
				return err
			}
			return handle(rawInboundMessage{ChannelID: "333", Content: "blocked"})
		},
	}
	newDiscordSeamFactory = func(cfg config.DiscordChannelConfig) (clientSeam, error) {
		return seam, nil
	}

	adapter, err := NewProductionAdapter(config.DiscordChannelConfig{
		BotToken:          "token",
		AllowedChannelIDs: []string{" 111 ", "\t222\n", "   "},
	})
	if err != nil {
		t.Fatalf("NewProductionAdapter: %v", err)
	}

	var got []channel.InboundMessage
	if err := adapter.Start(context.Background(), func(_ context.Context, msg channel.InboundMessage) {
		got = append(got, msg)
	}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	want := []channel.InboundMessage{
		{Channel: channelName, ChatID: "111", Content: "allowed"},
		{Channel: channelName, ChatID: "222", Content: "also allowed"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected inbound messages %#v, want %#v", got, want)
	}
}

func TestDiscordAdapterStartRejectsNilHandler(t *testing.T) {
	adapter := NewAdapter(&fakeSeam{})
	if err := adapter.Start(context.Background(), nil); err == nil {
		t.Fatal("expected Start to reject a nil handler")
	}
}

func TestDiscordAdapterStartAndStopUpdateStatus(t *testing.T) {
	seam := &fakeSeam{}
	adapter := NewAdapter(seam)

	status, err := adapter.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.State != "disconnected" {
		t.Fatalf("initial status = %#v, want disconnected", status)
	}

	if err := adapter.Start(context.Background(), func(context.Context, channel.InboundMessage) {}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	status, err = adapter.Status(context.Background())
	if err != nil {
		t.Fatalf("Status after Start: %v", err)
	}
	if status.State != "connected" {
		t.Fatalf("status after Start = %#v, want connected", status)
	}

	if err := adapter.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	status, err = adapter.Status(context.Background())
	if err != nil {
		t.Fatalf("Status after Stop: %v", err)
	}
	if status.State != "disconnected" {
		t.Fatalf("status after Stop = %#v, want disconnected", status)
	}
}

func TestDiscordAdapterSendUsesSharedChunkMessage(t *testing.T) {
	seam := &fakeSeam{}
	adapter := NewAdapter(seam)

	content := strings.Repeat("0123456789", 500)
	if err := adapter.Send(context.Background(), channel.OutboundMessage{
		Channel: "discord",
		ChatID:  "12345",
		Content: content,
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	wantChunks := channel.ChunkMessage(content, 2000)
	if got := seam.sendCalls; !reflect.DeepEqual(got, wantChunksToSendCalls(wantChunks, "12345")) {
		t.Fatalf("unexpected send calls %#v, want %#v", got, wantChunksToSendCalls(wantChunks, "12345"))
	}
}

func TestDiscordAdapterStartFiltersDisallowedChannels(t *testing.T) {
	seam := &fakeSeam{
		startFn: func(_ context.Context, handle func(rawInboundMessage) error) error {
			if err := handle(rawInboundMessage{ChannelID: "111", Content: "allowed"}); err != nil {
				return err
			}
			return handle(rawInboundMessage{ChannelID: "222", Content: "blocked"})
		},
	}
	adapter := NewAdapter(seam)
	adapter.enforce = true
	adapter.channelIDs = map[string]struct{}{
		"111": {},
	}

	var got []channel.InboundMessage
	if err := adapter.Start(context.Background(), func(_ context.Context, msg channel.InboundMessage) {
		got = append(got, msg)
	}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	want := []channel.InboundMessage{{
		Channel: channelName,
		ChatID:  "111",
		Content: "allowed",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected inbound messages %#v, want %#v", got, want)
	}
}

func TestDiscordAdapterStartNormalizesInboundMessages(t *testing.T) {
	seam := &fakeSeam{
		startFn: func(_ context.Context, handle func(rawInboundMessage) error) error {
			return handle(rawInboundMessage{
				ChannelID: "  999  ",
				Content:   "  hello from discord  ",
			})
		},
	}
	adapter := NewAdapter(seam)

	var got []channel.InboundMessage
	if err := adapter.Start(context.Background(), func(_ context.Context, msg channel.InboundMessage) {
		got = append(got, msg)
	}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	want := []channel.InboundMessage{{
		Channel: channelName,
		ChatID:  "999",
		Content: "hello from discord",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected inbound messages %#v, want %#v", got, want)
	}
}

func TestDiscordAdapterLoginReportsConnectedAfterValidation(t *testing.T) {
	seam := &fakeSeam{
		identifyFn: func(context.Context) (string, error) {
			return "smolbot", nil
		},
	}
	adapter := NewAdapter(seam)

	if err := adapter.Login(context.Background()); err != nil {
		t.Fatalf("Login: %v", err)
	}

	status, err := adapter.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.State != "connected" || status.Detail != "Bot: @smolbot" {
		t.Fatalf("unexpected status %#v", status)
	}
}

func TestDiscordAdapterLoginMarksAuthRequiredWhenValidationFails(t *testing.T) {
	seam := &fakeSeam{
		identifyFn: func(context.Context) (string, error) {
			return "", errors.New("invalid token")
		},
	}
	adapter := NewAdapter(seam)

	if err := adapter.Login(context.Background()); err == nil {
		t.Fatal("expected Login to fail for an invalid token")
	}

	status, err := adapter.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.State != "auth-required" || !strings.Contains(status.Detail, "invalid token") {
		t.Fatalf("unexpected status %#v", status)
	}
}

type fakeSeam struct {
	startFn    func(context.Context, func(rawInboundMessage) error) error
	identifyFn func(context.Context) (string, error)
	sendCalls  []sendCall
	stopped    bool
}

type sendCall struct {
	ChannelID string
	Content   string
}

func (f *fakeSeam) Start(ctx context.Context, handle func(rawInboundMessage) error) error {
	if f.startFn != nil {
		return f.startFn(ctx, handle)
	}
	return nil
}

func (f *fakeSeam) Stop(context.Context) error {
	f.stopped = true
	return nil
}

func (f *fakeSeam) Send(_ context.Context, channelID, content string) error {
	f.sendCalls = append(f.sendCalls, sendCall{ChannelID: channelID, Content: content})
	return nil
}

func (f *fakeSeam) Identify(ctx context.Context) (string, error) {
	if f.identifyFn != nil {
		return f.identifyFn(ctx)
	}
	return "smolbot", nil
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "discord-token-*.txt")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("WriteString: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return f.Name()
}

func wantChunksToSendCalls(chunks []string, channelID string) []sendCall {
	got := make([]sendCall, 0, len(chunks))
	for _, chunk := range chunks {
		got = append(got, sendCall{ChannelID: channelID, Content: chunk})
	}
	return got
}
