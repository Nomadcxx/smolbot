package signal

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/Nomadcxx/smolbot/pkg/channel"
	"github.com/Nomadcxx/smolbot/pkg/config"
)

const channelName = "signal"

type commandRunner interface {
	Run(context.Context, string, ...string) (string, error)
	Receive(context.Context, string, []string, func(rawInboundMessage) error) error
}

type Adapter struct {
	cfg    config.SignalChannelConfig
	runner commandRunner

	mu              sync.RWMutex
	provisioningURI string
	connected       bool
	receiveCancel   context.CancelFunc
	receiveDone     chan struct{}
}

const receiveStartupGrace = 50 * time.Millisecond

type rawInboundMessage struct {
	Source  string `json:"source,omitempty"`
	ChatID  string `json:"chatId,omitempty"`
	Content string `json:"content,omitempty"`
}

func NewAdapter(cfg config.SignalChannelConfig, runner commandRunner) *Adapter {
	if runner == nil {
		runner = execRunner{}
	}
	return &Adapter{cfg: cfg, runner: runner}
}

func (a *Adapter) Name() string {
	return channelName
}

func (a *Adapter) Start(ctx context.Context, handler channel.Handler) error {
	if handler == nil {
		return errors.New("signal handler is required")
	}
	args := a.receiveArgs()
	receiveCtx, cancel := context.WithCancel(ctx)
	resultCh := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		resultCh <- a.runner.Receive(receiveCtx, a.cliPath(), args, func(raw rawInboundMessage) error {
			handler(receiveCtx, raw.normalize())
			return nil
		})
	}()

	select {
	case err := <-resultCh:
		cancel()
		if err == nil {
			return errors.New("signal receive loop exited during startup")
		}
		return err
	case <-ctx.Done():
		cancel()
		<-done
		return ctx.Err()
	case <-time.After(receiveStartupGrace):
	}

	a.mu.Lock()
	a.connected = true
	a.provisioningURI = ""
	a.receiveCancel = cancel
	a.receiveDone = done
	a.mu.Unlock()

	go func() {
		err := <-resultCh
		a.mu.Lock()
		defer a.mu.Unlock()
		a.connected = false
		a.receiveCancel = nil
		a.receiveDone = nil
		if err != nil && receiveCtx.Err() == nil {
			a.provisioningURI = ""
		}
	}()

	return nil
}

func (a *Adapter) Stop(ctx context.Context) error {
	a.mu.Lock()
	cancel := a.receiveCancel
	done := a.receiveDone
	a.connected = false
	a.receiveCancel = nil
	a.receiveDone = nil
	a.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if done != nil {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (a *Adapter) Send(ctx context.Context, msg channel.OutboundMessage) error {
	args := a.baseArgs("send", "-m", msg.Content, msg.ChatID)
	if _, err := a.runner.Run(ctx, a.cliPath(), args...); err != nil {
		return fmt.Errorf("signal send: %w", err)
	}
	return nil
}

func (a *Adapter) Status(context.Context) (channel.Status, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.connected {
		return channel.Status{State: "connected"}, nil
	}
	if a.provisioningURI != "" {
		return channel.Status{State: "auth-required", Detail: a.provisioningURI}, nil
	}
	return channel.Status{State: "disconnected"}, nil
}

func (a *Adapter) Login(ctx context.Context) error {
	return a.LoginWithUpdates(ctx, nil)
}

func (a *Adapter) LoginWithUpdates(ctx context.Context, report func(channel.Status) error) error {
	out, err := a.runner.Run(ctx, a.cliPath(), a.baseArgs("link", "-n", "smolbot")...)
	if err != nil {
		return fmt.Errorf("signal login: %w", err)
	}

	a.mu.Lock()
	a.provisioningURI = strings.TrimSpace(out)
	a.mu.Unlock()

	if report != nil {
		status, err := a.Status(ctx)
		if err != nil {
			return err
		}
		if err := report(status); err != nil {
			return err
		}
	}

	return nil
}

func (a *Adapter) cliPath() string {
	if a.cfg.CLIPath != "" {
		return a.cfg.CLIPath
	}
	return "signal-cli"
}

func (a *Adapter) account() string {
	return strings.TrimSpace(a.cfg.Account)
}

func (a *Adapter) receiveArgs() []string {
	return a.baseArgs("--output", "json", "receive")
}

func (a *Adapter) baseArgs(args ...string) []string {
	out := make([]string, 0, len(args)+4)
	if dir := strings.TrimSpace(a.cfg.DataDir); dir != "" {
		out = append(out, "--config", dir)
	}
	if account := a.account(); account != "" {
		out = append(out, "-a", account)
	}
	return append(out, args...)
}

func (m rawInboundMessage) normalize() channel.InboundMessage {
	chatID := strings.TrimSpace(m.ChatID)
	if chatID == "" {
		chatID = strings.TrimSpace(m.Source)
	}
	return channel.InboundMessage{
		Channel: channelName,
		ChatID:  chatID,
		Content: strings.TrimSpace(m.Content),
	}
}

type execRunner struct{}

func (execRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return strings.TrimSpace(string(out)), nil
}

func (execRunner) Receive(ctx context.Context, name string, args []string, handle func(rawInboundMessage) error) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("%s %s: stdout pipe: %w", name, strings.Join(args, " "), err)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("%s %s: start: %w", name, strings.Join(args, " "), err)
	}

	if err := scanInboundJSONLines(stdout, handle); err != nil {
		cancel()
		_ = cmd.Wait()
		return fmt.Errorf("%s %s: receive: %w", name, strings.Join(args, " "), err)
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return nil
}

func decodeInboundMessageLine(out []byte, handle func(rawInboundMessage) error) error {
	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		return nil
	}

	var msg rawInboundMessage
	if err := json.Unmarshal(out, &msg); err != nil {
		return errors.New("signal receive output was not recognized as json")
	}
	return handle(msg)
}

func scanInboundJSONLines(r io.Reader, handle func(rawInboundMessage) error) error {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if err := decodeInboundMessageLine([]byte(line), handle); err != nil {
			return err
		}
	}
	return scanner.Err()
}
