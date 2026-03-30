package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestReadlineMultilineInputBackslashContinuation(t *testing.T) {
	origReadline := newReadlineSession
	defer func() { newReadlineSession = origReadline }()

	newReadlineSession = func(in *chatIO, out io.Writer) (readlineSession, error) {
		return &fakeReadline{
			lines: []string{
				"line one \\",
				"line two \\",
				"line three",
			},
		}, nil
	}

	origRun := runChatMessage
	defer func() { runChatMessage = origRun }()

	runChatMessage = func(context.Context, chatRequest) (string, error) {
		return "ok", nil
	}

	input := strings.NewReader("")
	var out bytes.Buffer
	err := runInteractiveChatSession(context.Background(), chatRequest{}, &chatIO{In: input, Out: &out}, chatSessionDeps{})
	if err != nil {
		t.Fatalf("runInteractiveChatSession: %v", err)
	}
}

func TestReadlineMultilineInputEmptyLineContinuation(t *testing.T) {
	orig := newReadlineSession
	defer func() { newReadlineSession = orig }()

	newReadlineSession = func(in *chatIO, out io.Writer) (readlineSession, error) {
		return &fakeReadline{
			lines: []string{"first", "", "continued", ""},
		}, nil
	}

	origRun := runChatMessage
	defer func() { runChatMessage = origRun }()

	runChatMessage = func(context.Context, chatRequest) (string, error) {
		return "ok", nil
	}

	input := strings.NewReader("")
	var out bytes.Buffer
	err := runInteractiveChatSession(context.Background(), chatRequest{}, &chatIO{In: input, Out: &out}, chatSessionDeps{})
	if err != nil {
		t.Fatalf("runInteractiveChatSession: %v", err)
	}
}

func TestReadlineHistoryRecallUpArrow(t *testing.T) {
	orig := newReadlineSession
	defer func() { newReadlineSession = orig }()

	histIdx := 0
	newReadlineSession = func(in *chatIO, out io.Writer) (readlineSession, error) {
		return &fakeReadline{
			history: []string{"first command", "second command", "third command"},
			histIdx: &histIdx,
		}, nil
	}

	origRun := runChatMessage
	defer func() { runChatMessage = origRun }()

	runChatMessage = func(context.Context, chatRequest) (string, error) {
		return "ok", nil
	}

	input := strings.NewReader("")
	var out bytes.Buffer
	err := runInteractiveChatSession(context.Background(), chatRequest{}, &chatIO{In: input, Out: &out}, chatSessionDeps{})
	if err != nil {
		t.Fatalf("runInteractiveChatSession: %v", err)
	}
}

func TestReadlineHistoryRecallDownArrow(t *testing.T) {
	orig := newReadlineSession
	defer func() { newReadlineSession = orig }()

	histIdx := 0
	newReadlineSession = func(in *chatIO, out io.Writer) (readlineSession, error) {
		return &fakeReadline{
			history: []string{"first", "second", "third"},
			histIdx: &histIdx,
		}, nil
	}

	origRun := runChatMessage
	defer func() { runChatMessage = origRun }()

	runChatMessage = func(context.Context, chatRequest) (string, error) {
		return "ok", nil
	}

	input := strings.NewReader("")
	var out bytes.Buffer
	err := runInteractiveChatSession(context.Background(), chatRequest{}, &chatIO{In: input, Out: &out}, chatSessionDeps{})
	if err != nil {
		t.Fatalf("runInteractiveChatSession: %v", err)
	}
}

func TestReadlineSIGINTCancelsTurnWithoutKillingSession(t *testing.T) {
	orig := runChatMessage
	defer func() { runChatMessage = orig }()

	var mu sync.Mutex
	var calls []string
	turnStarted := make(chan struct{})
	turnDone := make(chan struct{})
	signalCh := make(chan os.Signal, 1)
	histIdx := 0

	runChatMessage = func(ctx context.Context, req chatRequest) (string, error) {
		mu.Lock()
		calls = append(calls, req.Message)
		callNum := len(calls)
		mu.Unlock()

		if callNum == 1 {
			close(turnStarted)
			<-ctx.Done()
			close(turnDone)
			return "", ctx.Err()
		}
		return "second reply", nil
	}

	origReadline := newReadlineSession
	defer func() { newReadlineSession = origReadline }()

	newReadlineSession = func(in *chatIO, out io.Writer) (readlineSession, error) {
		return &fakeReadline{
			lines:   []string{"first", "second"},
			signals: []os.Signal{os.Interrupt, nil},
			history: []string{"first", "second"},
			histIdx: &histIdx,
		}, nil
	}

	var out bytes.Buffer
	errCh := make(chan error, 1)
	go func() {
		errCh <- runInteractiveChatSession(context.Background(), chatRequest{}, &chatIO{In: strings.NewReader(""), Out: &out}, chatSessionDeps{
			Signals: signalCh,
		})
	}()

	select {
	case <-turnStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("expected turn to start")
	}

	signalCh <- os.Interrupt

	select {
	case <-turnDone:
	case <-time.After(2 * time.Second):
		t.Fatal("expected interrupted turn to cancel")
	}

	if err := <-errCh; err != nil {
		t.Fatalf("runInteractiveChatSession: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(calls) != 2 {
		t.Fatalf("expected 2 turns, got %d", len(calls))
	}
	if calls[0] != "first" || calls[1] != "second" {
		t.Fatalf("unexpected calls: %v", calls)
	}
}

func TestReadlineBracketedPasteStillStripped(t *testing.T) {
	orig := newReadlineSession
	defer func() { newReadlineSession = orig }()

	var gotInput string
	newReadlineSession = func(in *chatIO, out io.Writer) (readlineSession, error) {
		return &fakeReadline{
			pasteMode: true,
			lines:     []string{"\x1b[200~pasted line\x1b[201~"},
		}, nil
	}

	origRun := runChatMessage
	defer func() { runChatMessage = origRun }()

	runChatMessage = func(_ context.Context, req chatRequest) (string, error) {
		gotInput = req.Message
		return "ok", nil
	}

	input := strings.NewReader("")
	var out bytes.Buffer
	err := runInteractiveChatSession(context.Background(), chatRequest{}, &chatIO{In: input, Out: &out}, chatSessionDeps{})
	if err != nil {
		t.Fatalf("runInteractiveChatSession: %v", err)
	}

	if gotInput == "" || strings.Contains(gotInput, "\x1b[200~") {
		t.Fatalf("bracketed paste markers should be stripped, got: %q", gotInput)
	}
}

func TestReadlineSessionHistoryFileWritten(t *testing.T) {
	orig := newReadlineSession
	defer func() { newReadlineSession = orig }()

	newReadlineSession = func(in *chatIO, out io.Writer) (readlineSession, error) {
		return &fakeReadline{
			lines: []string{"command1", "command2"},
		}, nil
	}

	origRun := runChatMessage
	defer func() { runChatMessage = origRun }()

	runChatMessage = func(context.Context, chatRequest) (string, error) {
		return "reply", nil
	}

	home := t.TempDir()
	historyPath := home + "/.nanobot/chat_history"

	input := strings.NewReader("")
	var out bytes.Buffer
	err := runInteractiveChatSession(context.Background(), chatRequest{}, &chatIO{In: input, Out: &out}, chatSessionDeps{
		HistoryPath: historyPath,
	})
	if err != nil {
		t.Fatalf("runInteractiveChatSession: %v", err)
	}

	data, err := os.ReadFile(historyPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), "command1") {
		t.Fatalf("expected history to contain command1, got: %q", string(data))
	}
}

func TestReadlineTerminalStateRestoreOnExit(t *testing.T) {
	orig := newReadlineSession
	defer func() { newReadlineSession = orig }()

	var restoreCalled bool
	newReadlineSession = func(in *chatIO, out io.Writer) (readlineSession, error) {
		return &fakeReadline{
			restoreCalled: &restoreCalled,
			lines:         []string{"/exit"},
		}, nil
	}

	origRun := runChatMessage
	defer func() { runChatMessage = origRun }()

	runChatMessage = func(context.Context, chatRequest) (string, error) {
		return "", nil
	}

	input := strings.NewReader("")
	var out bytes.Buffer
	err := runInteractiveChatSession(context.Background(), chatRequest{}, &chatIO{In: input, Out: &out}, chatSessionDeps{})
	if err != nil {
		t.Fatalf("runInteractiveChatSession: %v", err)
	}

	if !restoreCalled {
		t.Fatal("expected terminal state to be restored on exit")
	}
}

type fakeReadline struct {
	lines         []string
	lineIdx       int
	pasteMode     bool
	history       []string
	histIdx       *int
	signals       []os.Signal
	sigIdx        int
	restoreCalled *bool
	width         int
	height        int
}

func (f *fakeReadline) ReadLine() (string, error) {
	if f.lineIdx >= len(f.lines) {
		return "", io.EOF
	}
	line := f.lines[f.lineIdx]
	f.lineIdx++
	return line, nil
}

func (f *fakeReadline) AddToHistory(line string) {
	f.history = append(f.history, line)
}

func (f *fakeReadline) Close() error {
	if f.restoreCalled != nil {
		*f.restoreCalled = true
	}
	return nil
}

func (f *fakeReadline) GetTerminalSize() (int, int, error) {
	return f.width, f.height, nil
}

var _ readlineSession = (*fakeReadline)(nil)

func TestNavigateHistoryReturnsSelectedEntry(t *testing.T) {
	r := &bubbleteaReadline{
		history: []string{"first", "second", "third"},
		histIdx: -1,
	}

	entry := r.navigateHistory(-1)
	if entry != "third" {
		t.Fatalf("expected 'third', got %q", entry)
	}

	entry = r.navigateHistory(-1)
	if entry != "second" {
		t.Fatalf("expected 'second', got %q", entry)
	}

	entry = r.navigateHistory(1)
	if entry != "third" {
		t.Fatalf("expected 'third', got %q", entry)
	}

	entry = r.navigateHistory(1)
	if entry != "" {
		t.Fatalf("expected empty string at end of history, got %q", entry)
	}
}
