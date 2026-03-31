package main

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type SignalLinker struct {
	cliPath string
	dataDir string

	mu      sync.Mutex
	qrURI   string
	status  string
	account string
	done    bool
	linkErr error
	started bool
	cmd     *exec.Cmd
	cancel  context.CancelFunc
}

func NewSignalLinker(cliPath, dataDir string) *SignalLinker {
	return &SignalLinker{
		cliPath: cliPath,
		dataDir: dataDir,
		status:  "Initializing...",
	}
}

func (l *SignalLinker) Start() error {
	l.mu.Lock()
	if l.started {
		l.mu.Unlock()
		return fmt.Errorf("already started")
	}
	l.started = true
	l.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	l.cancel = cancel

	go l.run(ctx)
	return nil
}

func (l *SignalLinker) run(ctx context.Context) {
	defer func() {
		l.mu.Lock()
		l.done = true
		l.mu.Unlock()
	}()

	args := []string{"--config", l.dataDir, "link", "-n", "smolbot"}
	l.cmd = exec.CommandContext(ctx, l.cliPath, args...)

	stdout, err := l.cmd.StdoutPipe()
	if err != nil {
		l.setError(fmt.Errorf("stdout pipe: %w", err))
		return
	}
	stderr, err := l.cmd.StderrPipe()
	if err != nil {
		l.setError(fmt.Errorf("stderr pipe: %w", err))
		return
	}

	if err := l.cmd.Start(); err != nil {
		l.setError(fmt.Errorf("start signal-cli: %w", err))
		return
	}

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "tsdevice://") {
			l.mu.Lock()
			l.qrURI = line
			l.status = "Scan QR with Signal"
			l.mu.Unlock()
			continue
		}

		if strings.HasPrefix(line, "+") {
			l.mu.Lock()
			l.account = line
			l.status = "Linked successfully!"
			l.mu.Unlock()
		}
	}

	errScanner := bufio.NewScanner(stderr)
	var errLines []string
	for errScanner.Scan() {
		errLines = append(errLines, errScanner.Text())
	}

	if err := l.cmd.Wait(); err != nil {
		l.mu.Lock()
		if l.account == "" {
			l.linkErr = fmt.Errorf("signal-cli link failed: %s", strings.Join(errLines, "; "))
		}
		l.mu.Unlock()
	}
}

func (l *SignalLinker) Poll() (qrURI string, status string, account string, done bool, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.qrURI, l.status, l.account, l.done, l.linkErr
}

func (l *SignalLinker) Cleanup() {
	if l.cancel != nil {
		l.cancel()
	}
}

func (l *SignalLinker) setError(err error) {
	l.mu.Lock()
	l.linkErr = err
	l.mu.Unlock()
}
