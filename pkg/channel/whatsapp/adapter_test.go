package whatsapp

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/Nomadcxx/smolbot/pkg/channel"
	"github.com/Nomadcxx/smolbot/pkg/config"
)

func TestAdapterImplementsChannelStatusReporterAndLoginHandler(t *testing.T) {
	var _ channel.Channel = (*Adapter)(nil)
	var _ channel.StatusReporter = (*Adapter)(nil)
	var _ channel.LoginHandler = (*Adapter)(nil)
	var _ channel.InteractiveLoginHandler = (*Adapter)(nil)
}

func TestAdapterSendUsesClientSeam(t *testing.T) {
	seam := &fakeSeam{}
	adapter := NewAdapter(seam)

	if err := adapter.Send(context.Background(), channel.OutboundMessage{
		Channel: "whatsapp",
		ChatID:  "15551234567",
		Content: " hello there ",
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	want := []sendCall{{
		ChatID:  "15551234567",
		Content: " hello there ",
	}}
	if !reflect.DeepEqual(seam.sendCalls, want) {
		t.Fatalf("unexpected send calls %#v, want %#v", seam.sendCalls, want)
	}
}

func TestAdapterStartNormalizesInboundMessages(t *testing.T) {
	seam := &fakeSeam{
		startFn: func(_ context.Context, handle func(rawInboundMessage) error) error {
			return handle(rawInboundMessage{
				ChatID:  "15557654321",
				Content: "  hello from whatsapp  ",
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
		Channel: "whatsapp",
		ChatID:  "15557654321",
		Content: "hello from whatsapp",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected inbound messages %#v, want %#v", got, want)
	}
}

func TestAdapterLoginReportsQRAndDeviceLinkStateThroughSeam(t *testing.T) {
	seam := &fakeSeam{
		loginFn: func(_ context.Context, report func(loginUpdate) error) error {
			if err := report(loginUpdate{State: "qr", Detail: "code-1"}); err != nil {
				return err
			}
			if err := report(loginUpdate{State: "device-link", Detail: "link-1"}); err != nil {
				return err
			}
			return report(loginUpdate{State: "connected"})
		},
	}
	adapter := NewAdapter(seam)

	if status, err := adapter.Status(context.Background()); err != nil || status.State != "disconnected" {
		t.Fatalf("initial Status = %#v, %v", status, err)
	}

	var gotStatuses []channel.Status
	if err := adapter.LoginWithUpdates(context.Background(), func(status channel.Status) error {
		gotStatuses = append(gotStatuses, status)
		return nil
	}); err != nil {
		t.Fatalf("Login: %v", err)
	}

	wantUpdates := []loginUpdate{
		{State: "qr", Detail: "code-1"},
		{State: "device-link", Detail: "link-1"},
		{State: "connected"},
	}
	if !reflect.DeepEqual(seam.loginUpdates, wantUpdates) {
		t.Fatalf("unexpected login updates %#v, want %#v", seam.loginUpdates, wantUpdates)
	}
	wantStatuses := []channel.Status{
		{State: "qr", Detail: "code-1"},
		{State: "device-link", Detail: "link-1"},
		{State: "connected"},
	}
	if !reflect.DeepEqual(gotStatuses, wantStatuses) {
		t.Fatalf("unexpected reported statuses %#v, want %#v", gotStatuses, wantStatuses)
	}

	status, err := adapter.Status(context.Background())
	if err != nil {
		t.Fatalf("Status after login: %v", err)
	}
	if status.State != "connected" || status.Detail != "" {
		t.Fatalf("unexpected final status %#v", status)
	}
}

func TestAdapterStopUsesClientSeam(t *testing.T) {
	seam := &fakeSeam{}
	adapter := NewAdapter(seam)

	if err := adapter.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if !seam.stopped {
		t.Fatal("expected Stop to reach the seam")
	}
}

func TestNewProductionAdapterRejectsMissingStorePath(t *testing.T) {
	if _, err := NewProductionAdapter(config.WhatsAppChannelConfig{DeviceName: "smolbot"}); err == nil {
		t.Fatal("expected NewProductionAdapter to reject an empty store path")
	}
}

func TestNewProductionAdapterUsesFactory(t *testing.T) {
	origFactory := newWhatsAppSeamFactory
	defer func() { newWhatsAppSeamFactory = origFactory }()

	fakeSeam := &fakeSeam{}
	var gotCfg config.WhatsAppChannelConfig
	newWhatsAppSeamFactory = func(cfg config.WhatsAppChannelConfig) (clientSeam, error) {
		gotCfg = cfg
		return fakeSeam, nil
	}

	adapter, err := NewProductionAdapter(config.WhatsAppChannelConfig{
		DeviceName: "smolbot-test",
		StorePath:  "/tmp/nanobot-whatsapp.db",
	})
	if err != nil {
		t.Fatalf("NewProductionAdapter: %v", err)
	}

	if gotCfg.DeviceName != "smolbot-test" || gotCfg.StorePath != "/tmp/nanobot-whatsapp.db" {
		t.Fatalf("factory received %#v, want the supplied config", gotCfg)
	}
	if err := adapter.Send(context.Background(), channel.OutboundMessage{ChatID: "15551234567", Content: "hello"}); err != nil {
		t.Fatalf("Send through production adapter: %v", err)
	}
	if len(fakeSeam.sendCalls) != 1 {
		t.Fatalf("send calls = %d, want 1", len(fakeSeam.sendCalls))
	}
}

func TestAdapterStartReturnsHandlerError(t *testing.T) {
	wantErr := errors.New("boom")
	seam := &fakeSeam{
		startFn: func(_ context.Context, handle func(rawInboundMessage) error) error {
			if err := handle(rawInboundMessage{ChatID: "chat", Content: "msg"}); err != nil {
				return err
			}
			return wantErr
		},
	}
	adapter := NewAdapter(seam)

	err := adapter.Start(context.Background(), func(context.Context, channel.InboundMessage) {
	})
	if err == nil || err.Error() != wantErr.Error() {
		t.Fatalf("expected handler error %v, got %v", wantErr, err)
	}
}

type fakeSeam struct {
	sendCalls    []sendCall
	startFn      func(context.Context, func(rawInboundMessage) error) error
	loginFn      func(context.Context, func(loginUpdate) error) error
	stopped      bool
	loginUpdates []loginUpdate
}

type sendCall struct {
	ChatID  string
	Content string
}

func (f *fakeSeam) Send(_ context.Context, chatID, content string) error {
	f.sendCalls = append(f.sendCalls, sendCall{ChatID: chatID, Content: content})
	return nil
}

func (f *fakeSeam) Start(ctx context.Context, handle func(rawInboundMessage) error) error {
	if f.startFn == nil {
		return nil
	}
	return f.startFn(ctx, handle)
}

func (f *fakeSeam) Stop(context.Context) error {
	f.stopped = true
	return nil
}

func (f *fakeSeam) Login(ctx context.Context, report func(loginUpdate) error) error {
	if f.loginFn == nil {
		return nil
	}
	wrapped := func(update loginUpdate) error {
		f.loginUpdates = append(f.loginUpdates, update)
		return report(update)
	}
	return f.loginFn(ctx, wrapped)
}
