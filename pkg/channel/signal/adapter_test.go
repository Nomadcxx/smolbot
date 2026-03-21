package signal

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/channel"
	"github.com/Nomadcxx/smolbot/pkg/config"
)

func TestAdapterImplementsChannelStatusReporterAndLoginHandler(t *testing.T) {
	var _ channel.Channel = (*Adapter)(nil)
	var _ channel.StatusReporter = (*Adapter)(nil)
	var _ channel.LoginHandler = (*Adapter)(nil)
	var _ channel.InteractiveLoginHandler = (*Adapter)(nil)
}

func TestAdapterSendBuildsExpectedSignalCLIInvocation(t *testing.T) {
	runner := &fakeRunner{}
	adapter := NewAdapter(config.SignalChannelConfig{
		Account: "+15551234567",
		CLIPath: "signal-cli",
		DataDir: "/tmp/signal",
	}, runner)

	if err := adapter.Send(context.Background(), channel.OutboundMessage{
		Channel: "signal",
		ChatID:  "+15557654321",
		Content: "hello world",
	}); err != nil {
		t.Fatalf("Send: %v", err)
	}

	got := runner.calls
	want := []commandCall{{
		Name: "signal-cli",
		Args: []string{"--config", "/tmp/signal", "-a", "+15551234567", "send", "-m", "hello world", "+15557654321"},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected calls %#v, want %#v", got, want)
	}
}

func TestAdapterStartNormalizesInboundMessagesThroughSeam(t *testing.T) {
	runner := &fakeRunner{
		receiveFn: func(ctx context.Context, name string, args []string, handle func(rawInboundMessage) error) error {
			if name != "signal-cli" {
				t.Fatalf("unexpected command name %q", name)
			}
			if !reflect.DeepEqual(args, []string{"--config", "/tmp/signal", "-a", "+15551234567", "--output", "json", "receive"}) {
				t.Fatalf("unexpected receive args %#v", args)
			}
			if err := handle(rawInboundMessage{
				Source:  "+15557654321",
				ChatID:  "",
				Content: "  hello from signal  \n",
			}); err != nil {
				return err
			}
			<-ctx.Done()
			return nil
		},
	}
	adapter := NewAdapter(config.SignalChannelConfig{
		Account: "+15551234567",
		CLIPath: "signal-cli",
		DataDir: "/tmp/signal",
	}, runner)

	inbound := make(chan channel.InboundMessage, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := adapter.Start(ctx, func(_ context.Context, msg channel.InboundMessage) {
		inbound <- msg
	}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	got := <-inbound
	if got.Channel != "signal" || got.ChatID != "+15557654321" || got.Content != "hello from signal" {
		t.Fatalf("unexpected inbound %#v", got)
	}
}

func TestAdapterStartReceivesMultipleInboundMessagesFromStream(t *testing.T) {
	runner := &fakeRunner{
		receiveFn: func(ctx context.Context, _ string, args []string, handle func(rawInboundMessage) error) error {
			if !reflect.DeepEqual(args, []string{"--config", "/tmp/signal", "-a", "+15551234567", "--output", "json", "receive"}) {
				t.Fatalf("unexpected receive args %#v", args)
			}
			lines := []rawInboundMessage{
				{Source: "+15557654321", Content: "first"},
				{Source: "+15557654321", Content: "second"},
			}
			for _, line := range lines {
				if err := handle(line); err != nil {
					return err
				}
			}
			<-ctx.Done()
			return nil
		},
	}
	adapter := NewAdapter(config.SignalChannelConfig{
		Account: "+15551234567",
		CLIPath: "signal-cli",
		DataDir: "/tmp/signal",
	}, runner)

	inbound := make(chan channel.InboundMessage, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := adapter.Start(ctx, func(_ context.Context, msg channel.InboundMessage) {
		inbound <- msg
	}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	first := <-inbound
	second := <-inbound
	if first.Content != "first" || second.Content != "second" {
		t.Fatalf("unexpected inbound stream %#v %#v", first, second)
	}
}

func TestAdapterStatusAndLogin(t *testing.T) {
	runner := &fakeRunner{runOutput: "tsdevice:/?uuid=abc123"}
	adapter := NewAdapter(config.SignalChannelConfig{
		Account: "+15551234567",
		CLIPath: "signal-cli",
		DataDir: "/tmp/signal",
	}, runner)

	status, err := adapter.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.State != "disconnected" {
		t.Fatalf("unexpected initial status %#v", status)
	}

	var updates []channel.Status
	if err := adapter.LoginWithUpdates(context.Background(), func(status channel.Status) error {
		updates = append(updates, status)
		return nil
	}); err != nil {
		t.Fatalf("Login: %v", err)
	}

	wantCalls := []commandCall{{
		Name: "signal-cli",
		Args: []string{"--config", "/tmp/signal", "-a", "+15551234567", "link", "-n", "nanobot-go"},
	}}
	if !reflect.DeepEqual(runner.calls, wantCalls) {
		t.Fatalf("unexpected calls %#v, want %#v", runner.calls, wantCalls)
	}
	if len(updates) != 1 || updates[0].State != "auth-required" || !strings.Contains(updates[0].Detail, "tsdevice:/?uuid=abc123") {
		t.Fatalf("unexpected login updates %#v", updates)
	}

	status, err = adapter.Status(context.Background())
	if err != nil {
		t.Fatalf("Status after login: %v", err)
	}
	if status.State != "auth-required" {
		t.Fatalf("unexpected login status %#v", status)
	}
	if !strings.Contains(status.Detail, "tsdevice:/?uuid=abc123") {
		t.Fatalf("expected provisioning URI in status detail, got %#v", status)
	}
}

func TestAdapterStartReturnsWithoutWaitingForReceiveLoop(t *testing.T) {
	released := make(chan struct{})
	runner := &fakeRunner{
		receiveFn: func(ctx context.Context, _ string, _ []string, _ func(rawInboundMessage) error) error {
			<-ctx.Done()
			close(released)
			return nil
		},
	}
	adapter := NewAdapter(config.SignalChannelConfig{
		Account: "+15551234567",
		CLIPath: "signal-cli",
		DataDir: "/tmp/signal",
	}, runner)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- adapter.Start(ctx, func(context.Context, channel.InboundMessage) {})
	}()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Start blocked on the receive loop")
	}

	status, err := adapter.Status(context.Background())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if status.State != "connected" {
		t.Fatalf("status = %#v, want connected", status)
	}

	cancel()
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("receive loop did not stop after context cancellation")
	}
}

func TestAdapterStartReturnsReceiveStartupError(t *testing.T) {
	runner := &fakeRunner{
		receiveFn: func(context.Context, string, []string, func(rawInboundMessage) error) error {
			return errors.New("signal-cli missing")
		},
	}
	adapter := NewAdapter(config.SignalChannelConfig{
		Account: "+15551234567",
		CLIPath: "signal-cli",
		DataDir: "/tmp/signal",
	}, runner)

	err := adapter.Start(context.Background(), func(context.Context, channel.InboundMessage) {})
	if err == nil || !strings.Contains(err.Error(), "signal-cli missing") {
		t.Fatalf("expected startup error, got %v", err)
	}
}

func TestAdapterStopCancelsReceiveLoop(t *testing.T) {
	released := make(chan struct{})
	runner := &fakeRunner{
		receiveFn: func(ctx context.Context, _ string, _ []string, _ func(rawInboundMessage) error) error {
			<-ctx.Done()
			close(released)
			return nil
		},
	}
	adapter := NewAdapter(config.SignalChannelConfig{
		Account: "+15551234567",
		CLIPath: "signal-cli",
		DataDir: "/tmp/signal",
	}, runner)

	if err := adapter.Start(context.Background(), func(context.Context, channel.InboundMessage) {}); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := adapter.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}

	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("receive loop did not stop after Stop")
	}
}

func TestDecodeInboundMessageLine(t *testing.T) {
	var got []channel.InboundMessage
	if err := decodeInboundMessageLine([]byte(`{"source":"+15557654321","content":"hello"}`), func(msg rawInboundMessage) error {
		got = append(got, msg.normalize())
		return nil
	}); err != nil {
		t.Fatalf("decodeInboundMessageLine: %v", err)
	}
	if len(got) != 1 || got[0].ChatID != "+15557654321" || got[0].Content != "hello" {
		t.Fatalf("unexpected decoded message %#v", got)
	}
}

func TestScanInboundJSONLinesHandlesLongTokens(t *testing.T) {
	longContent := strings.Repeat("x", 80*1024)
	input := fmt.Sprintf("%s\n%s\n", `{"source":"+15557654321","content":"`+longContent+`"}`, `{"source":"+15557654321","content":"done"}`)

	var got []string
	if err := scanInboundJSONLines(strings.NewReader(input), func(msg rawInboundMessage) error {
		got = append(got, msg.Content)
		return nil
	}); err != nil {
		t.Fatalf("scanInboundJSONLines: %v", err)
	}
	if len(got) != 2 || got[0] != longContent || got[1] != "done" {
		t.Fatalf("unexpected scanned contents len=%d first=%d second=%q", len(got), len(got[0]), got[1])
	}
}

func TestExecRunnerReceiveCancelsBeforeWaitOnDecodeError(t *testing.T) {
	t.Parallel()

	runner := execRunner{}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	err := runner.Receive(ctx, "sh", []string{"-c", `printf '%s\n' 'not-json'; sleep 5`}, func(rawInboundMessage) error {
		return nil
	})
	if err == nil {
		t.Fatal("expected receive error")
	}
	if time.Since(start) > time.Second {
		t.Fatalf("receive took too long, likely waited for child termination: %v", time.Since(start))
	}
	if !strings.Contains(err.Error(), "signal receive output was not recognized as json") {
		t.Fatalf("unexpected error %v", err)
	}
}

type fakeRunner struct {
	calls     []commandCall
	runOutput string
	runErr    error
	receiveFn func(context.Context, string, []string, func(rawInboundMessage) error) error
}

type commandCall struct {
	Name string
	Args []string
}

func (f *fakeRunner) Run(_ context.Context, name string, args ...string) (string, error) {
	f.calls = append(f.calls, commandCall{Name: name, Args: append([]string(nil), args...)})
	if f.runErr != nil {
		return "", f.runErr
	}
	return f.runOutput, nil
}

func (f *fakeRunner) Receive(ctx context.Context, name string, args []string, handle func(rawInboundMessage) error) error {
	f.calls = append(f.calls, commandCall{Name: name, Args: append([]string(nil), args...)})
	if f.receiveFn == nil {
		return nil
	}
	if len(args) == 0 {
		return errors.New("missing receive args")
	}
	return f.receiveFn(ctx, name, args, handle)
}
