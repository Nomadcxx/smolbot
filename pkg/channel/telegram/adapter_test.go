package telegram

import (
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Nomadcxx/smolbot/pkg/channel"
	"github.com/Nomadcxx/smolbot/pkg/config"
	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func TestNewProductionAdapterRequiresToken(t *testing.T) {
	if _, err := NewProductionAdapter(config.TelegramChannelConfig{}); err == nil {
		t.Fatal("expected NewProductionAdapter to reject an empty telegram token")
	}
}

func TestNewProductionAdapterLoadsTokenFromFile(t *testing.T) {
	origFactory := newTelegramSeam
	defer func() { newTelegramSeam = origFactory }()

	var gotToken string
	newTelegramSeam = func(token string) (clientSeam, error) {
		gotToken = token
		return &fakeSeam{}, nil
	}

	tokenFile := writeTempFile(t, "  token-from-file  \n")
	adapter, err := NewProductionAdapter(config.TelegramChannelConfig{TokenFile: tokenFile})
	if err != nil {
		t.Fatalf("NewProductionAdapter: %v", err)
	}
	if adapter == nil {
		t.Fatal("expected adapter")
	}
	if gotToken != "token-from-file" {
		t.Fatalf("token passed to seam = %q, want %q", gotToken, "token-from-file")
	}
}

func TestNewProductionAdapterTrimsAllowedChatIDs(t *testing.T) {
	origFactory := newTelegramSeam
	defer func() { newTelegramSeam = origFactory }()

	seam := &fakeSeam{
		startFn: func(_ context.Context, handle func(chatID int64, text string)) error {
			handle(111, "allowed")
			handle(222, "also allowed")
			handle(333, "blocked")
			return nil
		},
	}
	newTelegramSeam = func(token string) (clientSeam, error) {
		return seam, nil
	}

	adapter, err := NewProductionAdapter(config.TelegramChannelConfig{
		BotToken:       "token",
		AllowedChatIDs: []string{" 111 ", "\t222\n", "   "},
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

func TestAdapterLoginWithUpdatesNilReporterDoesNotPanic(t *testing.T) {
	adapter := NewAdapter(&fakeSeam{
		getMeFn: func(context.Context) (string, error) {
			return "smolbot", nil
		},
	})

	if err := adapter.LoginWithUpdates(context.Background(), nil); err != nil {
		t.Fatalf("LoginWithUpdates: %v", err)
	}

	status, err := adapter.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.State != "connected" || status.Detail != "Bot: @smolbot" {
		t.Fatalf("unexpected status %#v", status)
	}
}

func TestAdapterStartRejectsNilHandler(t *testing.T) {
	adapter := NewAdapter(&fakeSeam{})

	if err := adapter.Start(context.Background(), nil); err == nil {
		t.Fatal("expected Start to reject a nil handler")
	}
}

func TestAdapterStartAndStopUpdateStatus(t *testing.T) {
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

func TestAdapterSendUsesSharedChunkMessage(t *testing.T) {
	seam := &fakeSeam{}
	adapter := NewAdapter(seam)

	content := strings.Repeat("0123456789", 500)
	if err := adapter.Send(context.Background(), channel.OutboundMessage{
		Channel: "telegram",
		ChatID:  "12345",
		Content: content,
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	wantChunks := channel.ChunkMessage(content, 4096)
	if got := seam.sendCalls; !reflect.DeepEqual(got, wantChunksToSendCalls(wantChunks, 12345)) {
		t.Fatalf("unexpected send calls %#v, want %#v", got, wantChunksToSendCalls(wantChunks, 12345))
	}
}

func TestAdapterStartFiltersDisallowedChats(t *testing.T) {
	seam := &fakeSeam{
		startFn: func(_ context.Context, handle func(chatID int64, text string)) error {
			handle(111, "allowed")
			handle(222, "blocked")
			return nil
		},
	}
	adapter := NewAdapter(seam)
	adapter.enforce = true
	adapter.chatIDs = map[string]struct{}{
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

func TestAdapterStartSuppressesDuplicateInboundMessages(t *testing.T) {
	seam := &fakeSeam{
		startFn: func(_ context.Context, handle func(chatID int64, text string)) error {
			handle(111, "duplicate")
			handle(111, "duplicate")
			return nil
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
		ChatID:  "111",
		Content: "duplicate",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected inbound messages %#v, want %#v", got, want)
	}
}

func TestTelegramSeamStopClearsBotForRelogin(t *testing.T) {
	origFactory := newTelegramBot
	defer func() { newTelegramBot = origFactory }()

	firstBot := &fakeTelegramBot{user: &models.User{Username: "first"}}
	secondBot := &fakeTelegramBot{user: &models.User{Username: "second"}}
	calls := 0
	newTelegramBot = func(token string, opts ...bot.Option) (telegramBot, error) {
		calls++
		if calls == 1 {
			return firstBot, nil
		}
		return secondBot, nil
	}

	seam := &telegramSeam{token: "token"}

	name, err := seam.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe before Stop: %v", err)
	}
	if name != "first" {
		t.Fatalf("GetMe before Stop = %q, want %q", name, "first")
	}

	if err := seam.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if seam.bot != nil {
		t.Fatal("expected Stop to clear the cached bot")
	}

	name, err = seam.GetMe(context.Background())
	if err != nil {
		t.Fatalf("GetMe after Stop: %v", err)
	}
	if name != "second" {
		t.Fatalf("GetMe after Stop = %q, want %q", name, "second")
	}
	if calls != 2 {
		t.Fatalf("factory calls = %d, want 2", calls)
	}
	if !firstBot.closed {
		t.Fatal("expected Stop to close the first bot")
	}
}

func TestAdapterLoginPropagatesReporterError(t *testing.T) {
	wantErr := errors.New("report failed")
	adapter := NewAdapter(&fakeSeam{
		getMeFn: func(context.Context) (string, error) {
			return "smolbot", nil
		},
	})

	err := adapter.LoginWithUpdates(context.Background(), func(status channel.Status) error {
		if status.State == "connected" {
			return wantErr
		}
		return nil
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("LoginWithUpdates error = %v, want %v", err, wantErr)
	}
}

func TestChunkMessageIsRuneSafeAtBoundary(t *testing.T) {
	content := "aaaébbb"
	chunks := channel.ChunkMessage(content, 4)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %#v", chunks)
	}
	for i, chunk := range chunks {
		if !utf8.ValidString(chunk) {
			t.Fatalf("chunk %d is not valid UTF-8: %q", i, chunk)
		}
	}
	if got := strings.Join(chunks, ""); got != content {
		t.Fatalf("chunks rejoined = %q, want %q", got, content)
	}
}

type fakeSeam struct {
	startFn func(context.Context, func(chatID int64, text string)) error
	stopFn  func(context.Context) error
	sendFn  func(context.Context, int64, string) error
	getMeFn func(context.Context) (string, error)

	sendCalls []sendCall
}

type sendCall struct {
	ChatID  int64
	Content string
}

func (f *fakeSeam) Start(ctx context.Context, handler func(chatID int64, text string)) error {
	if f.startFn != nil {
		return f.startFn(ctx, handler)
	}
	return nil
}

func (f *fakeSeam) Stop(ctx context.Context) error {
	if f.stopFn != nil {
		return f.stopFn(ctx)
	}
	return nil
}

func (f *fakeSeam) Send(ctx context.Context, chatID int64, text string) error {
	f.sendCalls = append(f.sendCalls, sendCall{ChatID: chatID, Content: text})
	if f.sendFn != nil {
		return f.sendFn(ctx, chatID, text)
	}
	return nil
}

func (f *fakeSeam) GetMe(ctx context.Context) (string, error) {
	if f.getMeFn != nil {
		return f.getMeFn(ctx)
	}
	return "smolbot", nil
}

func wantChunksToSendCalls(chunks []string, chatID int64) []sendCall {
	out := make([]sendCall, 0, len(chunks))
	for _, chunk := range chunks {
		out = append(out, sendCall{ChatID: chatID, Content: chunk})
	}
	return out
}

type fakeTelegramBot struct {
	user     *models.User
	closed   bool
	closeErr error
}

func (f *fakeTelegramBot) Start(context.Context) {}

func (f *fakeTelegramBot) Close(context.Context) (bool, error) {
	f.closed = true
	return true, f.closeErr
}

func (f *fakeTelegramBot) SendMessage(context.Context, *bot.SendMessageParams) (*models.Message, error) {
	return &models.Message{}, nil
}

func (f *fakeTelegramBot) GetMe(context.Context) (*models.User, error) {
	return f.user, nil
}

func writeTempFile(t *testing.T, content string) string {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "telegram-token-*")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		f.Close()
		t.Fatalf("WriteString: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	return f.Name()
}
