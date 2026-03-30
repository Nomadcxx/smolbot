package channel

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestManager(t *testing.T) {
	manager := NewManager()
	fake := &fakeChannel{name: "slack"}
	manager.Register(fake)

	if got := manager.ChannelNames(); len(got) != 1 || got[0] != "slack" {
		t.Fatalf("unexpected channel names %#v", got)
	}

	inbound := make(chan InboundMessage, 1)
	manager.SetInboundHandler(func(_ context.Context, msg InboundMessage) {
		inbound <- msg
	})

	if err := manager.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if fake.starts != 1 {
		t.Fatalf("expected start call, got %d", fake.starts)
	}

	if err := manager.Route(context.Background(), "slack", "C123", "hello"); err != nil {
		t.Fatalf("Route: %v", err)
	}
	if fake.lastOutbound.Channel != "slack" || fake.lastOutbound.ChatID != "C123" || fake.lastOutbound.Content != "hello" {
		t.Fatalf("unexpected outbound %#v", fake.lastOutbound)
	}

	fake.emit(InboundMessage{Channel: "slack", ChatID: "C123", Content: "incoming"})
	got := <-inbound
	if got.Content != "incoming" || got.ChatID != "C123" {
		t.Fatalf("unexpected inbound %#v", got)
	}

	if err := manager.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	if fake.stops != 1 {
		t.Fatalf("expected stop call, got %d", fake.stops)
	}
}

func TestManagerStatusAndLogin(t *testing.T) {
	manager := NewManager()
	fake := &fakeChannel{name: "slack", status: Status{State: "connected"}}
	manager.Register(fake)

	statuses := manager.Statuses(context.Background())
	if statuses["slack"].State != "connected" {
		t.Fatalf("unexpected channel status %#v", statuses)
	}

	if err := manager.Login(context.Background(), "slack"); err != nil {
		t.Fatalf("Login: %v", err)
	}
	if fake.logins != 1 {
		t.Fatalf("expected one login call, got %d", fake.logins)
	}
}

func TestManagerLoginWithUpdatesUsesInteractiveHandler(t *testing.T) {
	manager := NewManager()
	fake := &fakeChannel{
		name: "whatsapp",
		loginStatuses: []Status{
			{State: "qr", Detail: "code-1"},
			{State: "connected"},
		},
	}
	manager.Register(fake)

	var got []Status
	if err := manager.LoginWithUpdates(context.Background(), "whatsapp", func(status Status) error {
		got = append(got, status)
		return nil
	}); err != nil {
		t.Fatalf("LoginWithUpdates: %v", err)
	}

	if len(got) != 2 || got[0].State != "qr" || got[1].State != "connected" {
		t.Fatalf("unexpected login statuses %#v", got)
	}
	if fake.logins != 1 {
		t.Fatalf("expected one login call, got %d", fake.logins)
	}
}

func TestManagerStartStopsAlreadyStartedChannelsOnFailure(t *testing.T) {
	manager := NewManager()
	first := &fakeChannel{name: "signal"}
	second := &fakeChannel{name: "whatsapp", startErr: errors.New("boom")}
	manager.Register(first)
	manager.Register(second)
	manager.SetInboundHandler(func(context.Context, InboundMessage) {})

	err := manager.Start(context.Background())
	if err == nil {
		t.Fatal("expected Start to fail")
	}
	rollbacks := first.starts
	if rollbacks < 1 {
		t.Fatalf("expected first channel to be started at least once before failure, got %d", rollbacks)
	}
	if first.stops < 1 {
		t.Fatalf("expected first channel to be stopped at least once on rollback, got %d", first.stops)
	}
}

type fakeChannel struct {
	name           string
	starts         int
	stops          int
	logins         int
	status         Status
	loginStatuses  []Status
	lastOutbound   OutboundMessage
	handler        func(context.Context, InboundMessage)
	startErr       error
	stopBlockCh    chan struct{}
	lastStopCtx    context.Context
	lastLoginCtx   context.Context
}

func (f *fakeChannel) Name() string {
	return f.name
}

func (f *fakeChannel) Start(_ context.Context, handler Handler) error {
	f.starts++
	f.handler = handler
	return f.startErr
}

func (f *fakeChannel) Stop(ctx context.Context) error {
	f.stops++
	f.lastStopCtx = ctx
	if f.stopBlockCh != nil {
		<-f.stopBlockCh
	}
	return nil
}

func (f *fakeChannel) Send(_ context.Context, msg OutboundMessage) error {
	f.lastOutbound = msg
	return nil
}

func (f *fakeChannel) Status(context.Context) (Status, error) {
	return f.status, nil
}

func (f *fakeChannel) Login(ctx context.Context) error {
	f.logins++
	f.lastLoginCtx = ctx
	return nil
}

func (f *fakeChannel) LoginWithUpdates(ctx context.Context, report func(Status) error) error {
	f.logins++
	f.lastLoginCtx = ctx
	for _, status := range f.loginStatuses {
		if err := report(status); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeChannel) emit(msg InboundMessage) {
	if f.handler != nil {
		f.handler(context.Background(), msg)
	}
}

func TestManagerStartReturnsErrorWhenNoInboundHandlerSet(t *testing.T) {
	manager := NewManager()
	manager.Register(&fakeChannel{name: "signal"})
	err := manager.Start(context.Background())
	if err == nil {
		t.Fatal("expected error when starting without an inbound handler")
	}
	if !strings.Contains(err.Error(), "SetInboundHandler") {
		t.Fatalf("unexpected error message %q", err.Error())
	}
}
