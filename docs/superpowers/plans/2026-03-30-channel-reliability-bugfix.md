# Channel Reliability Bugfix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix six channel-reliability bugs: nil handler guard in the channel manager, WhatsApp status tracking on disconnect/reconnect, cron job isolation (continue-on-error + concurrent execution guard), Signal receive-loop reconnect, a periodic manager health-watch, and goroutine panic recovery for the inbound message handler.

**Architecture:** Each fix is contained to the file(s) listed in the task. Fixes follow TDD: write a failing test first, implement the minimum code to pass, commit. No cross-cutting architectural changes; the largest refactor is extracting a one-method `agentRunner` interface in `runtime.go` to enable M7 testing.

**Tech Stack:** Go standard library (`sync`, `context`, `log`, `time`), whatsmeow event system (`go.mau.fi/whatsmeow/types/events`), existing `pkg/channel`, `pkg/cron`, `cmd/smolbot` packages.

---

## Pre-flight: L4 re-assessment

**L4 (Discord channelEnabled gap) is already fixed.** `channelEnabled` at `cmd/smolbot/runtime.go:1009–1020` already handles `"discord"`. No task needed.

---

## File Map

| File | Change |
|------|--------|
| `pkg/channel/manager.go` | Task 1: nil handler guard; Task 5: `Watch` method |
| `pkg/channel/manager_test.go` | Task 1: new test + fix two lifecycle tests; Task 5: Watch test |
| `pkg/channel/manager_lifecycle_test.go` | Task 1: add `SetInboundHandler` calls to two existing tests |
| `pkg/channel/whatsapp/adapter.go` | Task 2: `clientSeam` interface + seam fields/handleEvent + Adapter.Start |
| `pkg/channel/whatsapp/adapter_test.go` | Task 2: new test + update `fakeSeam` |
| `pkg/cron/service.go` | Task 3: `runningJobs`, fix `RunDue`, fix `executeJob` |
| `pkg/cron/service_test.go` | Task 3: update `fakeCronProcessor`, add two sub-tests |
| `pkg/channel/signal/adapter.go` | Task 4: reconnect loop with configurable delay |
| `pkg/channel/signal/adapter_test.go` | Task 4: reconnect test |
| `cmd/smolbot/runtime.go` | Task 5: wire Watch; Task 6: `agentRunner` interface + `recover()` |
| `cmd/smolbot/runtime_services_test.go` | Task 6: `handleInbound` panic recovery test |

---

## Task 1: H1 — Nil inbound handler guard in Manager.Start

**Files:**
- Modify: `pkg/channel/manager.go:36–58`
- Modify: `pkg/channel/manager_test.go`
- Modify: `pkg/channel/manager_lifecycle_test.go`

`Manager.Start` at line 47 passes `m.inboundHandler` directly to each channel's `Start` without checking if it is nil. Channels that do not validate their handler will panic when the first inbound message arrives. The fix adds an explicit guard at the top of `Start`. Two existing lifecycle tests call `Start` without calling `SetInboundHandler`; they must be updated to pass a no-op handler.

- [ ] **Step 1: Write the failing test**

Add to `pkg/channel/manager_test.go` (inside the `import` block, add `"strings"`):

```go
func TestManagerStartReturnsErrorWhenNoInboundHandlerSet(t *testing.T) {
	manager := NewManager()
	manager.Register(&fakeChannel{name: "signal"})
	// intentionally do NOT call SetInboundHandler
	err := manager.Start(context.Background())
	if err == nil {
		t.Fatal("expected error when starting without an inbound handler")
	}
	if !strings.Contains(err.Error(), "inbound handler") {
		t.Fatalf("unexpected error message %q", err.Error())
	}
}
```

Add `"strings"` to the import in `pkg/channel/manager_test.go`:

```go
import (
	"context"
	"errors"
	"strings"
	"testing"
)
```

- [ ] **Step 2: Run failing test**

```
go test ./pkg/channel/ -run TestManagerStartReturnsErrorWhenNoInboundHandlerSet -v
```

Expected: FAIL — `Start` returns nil (no guard exists yet).

- [ ] **Step 3: Add guard to Manager.Start**

Replace the entire `Start` method in `pkg/channel/manager.go`:

```go
func (m *Manager) Start(ctx context.Context) error {
	m.mu.Lock()
	if m.inboundHandler == nil {
		m.mu.Unlock()
		return errors.New("channel manager: SetInboundHandler must be called before Start")
	}
	channels := make([]Channel, 0, len(m.channels))
	for _, ch := range m.channels {
		channels = append(channels, ch)
	}
	sort.Slice(channels, func(i, j int) bool { return channels[i].Name() < channels[j].Name() })
	m.mu.Unlock()

	var started []Channel
	for _, channel := range channels {
		if err := channel.Start(ctx, m.inboundHandler); err != nil {
			for _, ch := range started {
				_ = ch.Stop(context.Background())
			}
			return fmt.Errorf("start channel %s: %w", channel.Name(), err)
		}
		started = append(started, channel)
		m.mu.Lock()
		m.running[channel] = true
		m.mu.Unlock()
	}
	return nil
}
```

- [ ] **Step 4: Fix the two existing lifecycle tests that call Start without a handler**

In `pkg/channel/manager_lifecycle_test.go`, add `SetInboundHandler` calls before each `Start`:

```go
func TestManagerStartRollbackUsesBoundedContext(t *testing.T) {
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
	if first.stops == 0 && second.stops == 0 {
		t.Fatal("expected at least one channel to be stopped on rollback")
	}
	if first.lastStopCtx == nil && second.lastStopCtx == nil {
		t.Fatal("expected stopped channels to receive non-nil context")
	}
}

func TestManagerStopDeliversBoundedContext(t *testing.T) {
	manager := NewManager()
	fake := &fakeChannel{name: "signal"}
	manager.Register(fake)
	manager.SetInboundHandler(func(context.Context, InboundMessage) {})

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	if err := manager.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := manager.Stop(ctx); err != nil {
		t.Fatalf("Stop: %v", err)
	}
}
```

- [ ] **Step 5: Run all channel package tests**

```
go test ./pkg/channel/... -v
```

Expected: all pass including `TestManagerStartReturnsErrorWhenNoInboundHandlerSet`.

- [ ] **Step 6: Commit**

```bash
git add pkg/channel/manager.go pkg/channel/manager_test.go pkg/channel/manager_lifecycle_test.go
git commit -m "fix(channel): guard against nil inbound handler in Manager.Start"
```

---

## Task 2: H2 — WhatsApp disconnect/reconnect status tracking

**Files:**
- Modify: `pkg/channel/whatsapp/adapter.go`
- Modify: `pkg/channel/whatsapp/adapter_test.go`

`whatsmeow` fires `*waEvents.Disconnected` and `*waEvents.Connected` events during its auto-reconnect cycle. The seam's `handleEvent` only handles `*waEvents.Message`, so the adapter's status never transitions away from "connected" during a disconnect/reconnect. The fix adds a `SetConnectionStateHandler` method to the `clientSeam` interface, stores the callbacks in `whatsmeowSeam`, and handles the two new event types.

- [ ] **Step 1: Write the failing test**

Add to `pkg/channel/whatsapp/adapter_test.go`:

```go
func TestAdapterStatusTransitionsOnDisconnectAndReconnect(t *testing.T) {
	seam := &fakeSeam{}
	adapter := NewAdapter(seam)

	if err := adapter.Start(context.Background(), func(context.Context, channel.InboundMessage) {}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// After Start the adapter should be "connected"
	if s, _ := adapter.Status(context.Background()); s.State != "connected" {
		t.Fatalf("expected connected after Start, got %s", s.State)
	}

	// Start must have registered a disconnect callback on the seam
	if seam.onDisconnect == nil {
		t.Fatal("expected Start to register a disconnect callback on the seam")
	}

	// Simulate whatsmeow firing a Disconnected event
	seam.onDisconnect()
	if s, _ := adapter.Status(context.Background()); s.State != "disconnected" {
		t.Fatalf("expected disconnected after disconnect event, got %s", s.State)
	}

	// Simulate whatsmeow firing a Connected event (auto-reconnect succeeded)
	seam.onReconnect()
	if s, _ := adapter.Status(context.Background()); s.State != "connected" {
		t.Fatalf("expected connected after reconnect event, got %s", s.State)
	}
}
```

- [ ] **Step 2: Run failing test**

```
go test ./pkg/channel/whatsapp/ -run TestAdapterStatusTransitionsOnDisconnectAndReconnect -v
```

Expected: FAIL — compile error: `seam.onDisconnect` undefined and `clientSeam` does not have `SetConnectionStateHandler`.

- [ ] **Step 3: Extend the clientSeam interface**

In `pkg/channel/whatsapp/adapter.go`, add `SetConnectionStateHandler` to the `clientSeam` interface:

```go
type clientSeam interface {
	Send(context.Context, string, string) error
	Start(context.Context, func(rawInboundMessage) error) error
	Stop(context.Context) error
	Login(context.Context, func(loginUpdate) error) error
	SetConnectionStateHandler(onDisconnect func(), onReconnect func())
}
```

- [ ] **Step 4: Add fields and method to whatsmeowSeam**

In `pkg/channel/whatsapp/adapter.go`, update the `whatsmeowSeam` struct to add two callback fields:

```go
type whatsmeowSeam struct {
	client *whatsmeow.Client

	mu             sync.Mutex
	started        bool
	handlerID      uint32
	onDisconnect   func()
	onReconnect    func()

	recentMessages map[string]time.Time
	recentMu       sync.Mutex
}
```

Add the implementation immediately after the struct definition:

```go
func (s *whatsmeowSeam) SetConnectionStateHandler(onDisconnect func(), onReconnect func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onDisconnect = onDisconnect
	s.onReconnect = onReconnect
}
```

- [ ] **Step 5: Handle Disconnected and Connected events in handleEvent**

In `pkg/channel/whatsapp/adapter.go`, extend the `handleEvent` switch to cover the two new event types (add after the `case *waEvents.Message:` block):

```go
func (s *whatsmeowSeam) handleEvent(evt any, handle func(rawInboundMessage) error) {
	switch typed := evt.(type) {
	case *waEvents.Message:
		if typed == nil {
			return
		}
		// ... existing message handling unchanged — do not modify ...
	case *waEvents.Disconnected:
		if typed == nil {
			return
		}
		s.mu.Lock()
		cb := s.onDisconnect
		s.mu.Unlock()
		if cb != nil {
			cb()
		}
	case *waEvents.Connected:
		if typed == nil {
			return
		}
		s.mu.Lock()
		cb := s.onReconnect
		s.mu.Unlock()
		if cb != nil {
			cb()
		}
	}
}
```

- [ ] **Step 6: Call SetConnectionStateHandler in Adapter.Start**

In `pkg/channel/whatsapp/adapter.go`, add the `SetConnectionStateHandler` call in `Adapter.Start` **before** the `a.seam.Start(...)` call. Insert after the allowlist log line:

```go
func (a *Adapter) Start(ctx context.Context, handler channel.Handler) error {
	if handler == nil {
		return errors.New("whatsapp handler is required")
	}
	if a.seam == nil {
		return errors.New("whatsapp client seam is required")
	}
	if a.enforceAllowlist && len(a.allowedChatIDs) == 0 {
		log.Printf("[whatsapp] inbound allowlist empty; all inbound WhatsApp messages will be ignored")
	}
	a.seam.SetConnectionStateHandler(
		func() { a.updateStatus("disconnected", "auto-reconnect in progress") },
		func() { a.updateStatus("connected", "") },
	)
	a.updateStatus("connecting", "")
	log.Printf("[whatsapp] adapter starting...")
	err := a.seam.Start(ctx, func(raw rawInboundMessage) error {
		msg := raw.normalize()
		if !a.isAllowedChat(msg.ChatID) {
			log.Printf("[whatsapp] dropping inbound from disallowed chat %q", msg.ChatID)
			return nil
		}
		log.Printf("[whatsapp] raw inbound: chatID=%q content=%q", msg.ChatID, msg.Content)
		handler(ctx, msg)
		return nil
	})
	if err != nil {
		log.Printf("[whatsapp] adapter start failed: %v", err)
		a.updateStatus("error", strings.TrimSpace(err.Error()))
		return err
	}
	log.Printf("[whatsapp] adapter started successfully")
	a.updateStatus("connected", "")
	return nil
}
```

- [ ] **Step 7: Update fakeSeam in the test file**

In `pkg/channel/whatsapp/adapter_test.go`, add `onDisconnect` and `onReconnect` fields to `fakeSeam` and implement the new method:

```go
type fakeSeam struct {
	sendCalls    []sendCall
	startFn      func(context.Context, func(rawInboundMessage) error) error
	loginFn      func(context.Context, func(loginUpdate) error) error
	stopped      bool
	loginUpdates []loginUpdate
	onDisconnect func()
	onReconnect  func()
}

func (f *fakeSeam) SetConnectionStateHandler(onDisconnect, onReconnect func()) {
	f.onDisconnect = onDisconnect
	f.onReconnect = onReconnect
}
```

Leave all existing methods on `fakeSeam` unchanged.

- [ ] **Step 8: Run all whatsapp package tests**

```
go test ./pkg/channel/whatsapp/... -v
```

Expected: all pass including `TestAdapterStatusTransitionsOnDisconnectAndReconnect`.

- [ ] **Step 9: Commit**

```bash
git add pkg/channel/whatsapp/adapter.go pkg/channel/whatsapp/adapter_test.go
git commit -m "fix(whatsapp): track disconnect/reconnect events to keep adapter status accurate"
```

---

## Task 3: C15 + M11 — Cron: continue-on-error and concurrent job guard

**Files:**
- Modify: `pkg/cron/service.go`
- Modify: `pkg/cron/service_test.go`

Two bugs in `RunDue`:

- **C15**: `return err` on the first failing job stops all remaining jobs from executing this cycle.
- **M11**: The mutex is released before `executeJob` runs, so if the scheduler fires again while a job is still executing, the same job runs concurrently.

Fix: (a) change `RunDue` to collect the first error but continue running all due jobs; (b) add a `runningJobs map[string]bool` field to `Service`, mark jobs as running before executing them, and clear the mark inside `executeJob` when the mutex is re-acquired.

- [ ] **Step 1: Add errFn to fakeCronProcessor for configurable errors**

In `pkg/cron/service_test.go`, replace the existing `fakeCronProcessor` definition with:

```go
type fakeCronProcessor struct {
	calls     int
	lastReq   agent.Request
	result    string
	cbEvent   *agent.Event
	errOnCall int   // if > 0, return callErr on the nth call
	callErr   error
}

func (f *fakeCronProcessor) ProcessDirect(ctx context.Context, req agent.Request, cb agent.EventCallback) (string, error) {
	f.calls++
	f.lastReq = req
	if f.cbEvent != nil && cb != nil {
		cb(*f.cbEvent)
	}
	if f.errOnCall > 0 && f.calls == f.errOnCall {
		return "", f.callErr
	}
	return f.result, nil
}
```

- [ ] **Step 2: Run existing tests to confirm they still pass**

```
go test ./pkg/cron/ -v
```

Expected: all pass (the new fields have zero values, so existing behavior is unchanged).

- [ ] **Step 3: Write the failing test for C15 (continue on error)**

Add the following sub-test at the end of `TestService` in `pkg/cron/service_test.go`, inside the outer `t.Run(...)` body (before the closing `}`). The sub-test also needs `"errors"` added to the import block.

```go
t.Run("RunDue continues executing remaining jobs after one fails", func(t *testing.T) {
	now2 := time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC)
	errProcessor := &fakeCronProcessor{
		result:    "ok",
		errOnCall: 1,
		callErr:   errors.New("job-a exploded"),
	}
	svc2 := NewService(ServiceDeps{
		JobsFile:  filepath.Join(t.TempDir(), "jobs2.json"),
		Processor: errProcessor,
		Router:    &fakeCronRouter{},
		Evaluator: &fakeCronEvaluator{deliver: false},
		Now:       func() time.Time { return now2 },
	})
	for _, name := range []string{"job-a", "job-b"} {
		if _, err := svc2.Handle(context.Background(), toolpkg.CronRequest{
			Action:   "create",
			Name:     name,
			Schedule: now2.Add(-time.Minute).Format(time.RFC3339),
			Timezone: "UTC",
			Reminder: "ping",
			Enabled:  true,
		}); err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
	}
	_ = svc2.RunDue(context.Background(), now2)
	if errProcessor.calls != 2 {
		t.Fatalf("expected 2 processor calls (both jobs run despite first error), got %d", errProcessor.calls)
	}
})
```

Add `"errors"` to the import block in `service_test.go` if not already present.

- [ ] **Step 4: Write the failing test for M11 (concurrent job guard)**

Add a second sub-test after the C15 test. Also add `blockingCronProcessor` as a file-level type at the bottom of `service_test.go`:

```go
t.Run("RunDue skips a job that is already running from a prior cycle", func(t *testing.T) {
	now3 := time.Date(2026, 3, 20, 11, 0, 0, 0, time.UTC)
	started := make(chan struct{}, 1)
	unblock := make(chan struct{})
	blockP := &blockingCronProcessor{started: started, wait: unblock}
	svc3 := NewService(ServiceDeps{
		JobsFile:  filepath.Join(t.TempDir(), "jobs3.json"),
		Processor: blockP,
		Router:    &fakeCronRouter{},
		Evaluator: &fakeCronEvaluator{deliver: false},
		Now:       func() time.Time { return now3 },
	})
	if _, err := svc3.Handle(context.Background(), toolpkg.CronRequest{
		Action:   "create",
		Name:     "slow-job",
		Schedule: now3.Add(-time.Minute).Format(time.RFC3339),
		Timezone: "UTC",
		Reminder: "slow",
		Enabled:  true,
	}); err != nil {
		t.Fatalf("create: %v", err)
	}

	// First RunDue — job starts but blocks inside processor
	done1 := make(chan error, 1)
	go func() { done1 <- svc3.RunDue(context.Background(), now3) }()
	<-started // wait until processor is executing

	// Second RunDue — job should be skipped (already running)
	if err := svc3.RunDue(context.Background(), now3); err != nil {
		t.Fatalf("second RunDue: %v", err)
	}
	if blockP.calls != 1 {
		t.Fatalf("expected 1 processor call after second RunDue, got %d", blockP.calls)
	}

	// Unblock first job and wait for it to finish
	close(unblock)
	if err := <-done1; err != nil {
		t.Fatalf("first RunDue: %v", err)
	}
})
```

Add `blockingCronProcessor` at the bottom of `service_test.go`:

```go
type blockingCronProcessor struct {
	mu      sync.Mutex
	calls   int
	started chan struct{}
	wait    chan struct{}
}

func (b *blockingCronProcessor) ProcessDirect(_ context.Context, _ agent.Request, _ agent.EventCallback) (string, error) {
	b.mu.Lock()
	b.calls++
	b.mu.Unlock()
	b.started <- struct{}{}
	<-b.wait
	return "done", nil
}
```

Add `"sync"` to the import block in `service_test.go` if not already present.

- [ ] **Step 5: Run failing tests**

```
go test ./pkg/cron/ -run "TestService/RunDue_continues|TestService/RunDue_skips" -v
```

Expected: both FAIL.

- [ ] **Step 6: Add runningJobs field to Service**

In `pkg/cron/service.go`, update the `Service` struct:

```go
type Service struct {
	mu          sync.Mutex
	jobsFile    string
	processor   Processor
	evaluator   Evaluator
	router      Router
	now         func() time.Time
	jobs        []Job
	runningJobs map[string]bool
}
```

Update `NewService` to initialize `runningJobs`:

```go
func NewService(deps ServiceDeps) *Service {
	now := deps.Now
	if now == nil {
		now = time.Now
	}
	s := &Service{
		jobsFile:    deps.JobsFile,
		processor:   deps.Processor,
		evaluator:   deps.Evaluator,
		router:      deps.Router,
		now:         now,
		runningJobs: make(map[string]bool),
	}
	_ = s.load()
	return s
}
```

- [ ] **Step 7: Fix RunDue**

Replace the `RunDue` method in `pkg/cron/service.go`:

```go
func (s *Service) RunDue(ctx context.Context, now time.Time) error {
	s.mu.Lock()
	jobs := make([]Job, 0, len(s.jobs))
	for _, job := range s.jobs {
		if !job.Enabled || job.NextRun.After(now) {
			continue
		}
		if s.runningJobs[job.ID] {
			continue
		}
		s.runningJobs[job.ID] = true
		jobs = append(jobs, job)
	}
	s.mu.Unlock()

	var firstErr error
	for _, job := range jobs {
		if err := s.executeJob(ctx, job, now); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			log.Printf("[cron] job %q failed: %v", job.Name, err)
		}
	}
	return firstErr
}
```

Add `"log"` to the import block in `service.go` if not already present.

- [ ] **Step 8: Fix executeJob to clear the running flag**

In `pkg/cron/service.go`, update `executeJob` to add `delete(s.runningJobs, job.ID)` in the two places where the mutex is taken:

The function currently starts:
```go
func (s *Service) executeJob(ctx context.Context, job Job, now time.Time) error {
	if s.processor == nil {
		return nil
	}
```

Replace with:
```go
func (s *Service) executeJob(ctx context.Context, job Job, now time.Time) error {
	if s.processor == nil {
		s.mu.Lock()
		delete(s.runningJobs, job.ID)
		s.mu.Unlock()
		return nil
	}
```

Further down, the function takes the mutex before updating job state. Add the delete call immediately after `s.mu.Lock()`:

```go
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.runningJobs, job.ID)
	stored := s.jobByIDLocked(job.ID)
	if stored == nil {
		return nil
	}
	// ... rest of executeJob unchanged ...
```

- [ ] **Step 9: Run all cron tests**

```
go test ./pkg/cron/... -v
```

Expected: all pass including the two new sub-tests.

- [ ] **Step 10: Commit**

```bash
git add pkg/cron/service.go pkg/cron/service_test.go
git commit -m "fix(cron): continue-on-error in RunDue and prevent concurrent job re-entry"
```

---

## Task 4: M5 — Signal receive-loop reconnect

**Files:**
- Modify: `pkg/channel/signal/adapter.go`
- Modify: `pkg/channel/signal/adapter_test.go`

The Signal adapter spawns a `signal-cli receive` goroutine at `Start`. If signal-cli crashes, the goroutine exits and sets `connected = false`, but the adapter never retries. The fix replaces the one-shot monitoring goroutine with a reconnect loop that waits with exponential backoff and re-launches the receive goroutine whenever it exits unexpectedly (i.e., when the context is still active).

- [ ] **Step 1: Add a configurable reconnect delay field to Adapter**

In `pkg/channel/signal/adapter.go`, add `testReconnectDelay time.Duration` to the `Adapter` struct (the field is package-private and only set in tests — in production it stays zero, meaning use the default):

```go
type Adapter struct {
	cfg    config.SignalChannelConfig
	runner commandRunner

	testReconnectDelay time.Duration // zero → use default 5s; set in tests to speed up

	mu              sync.RWMutex
	provisioningURI string
	connected       bool
	receiveCancel   context.CancelFunc
	receiveDone     chan struct{}
}
```

Add a package-level constant for the production default (add after the `receiveStartupGrace` constant):

```go
const (
	receiveStartupGrace      = 50 * time.Millisecond
	signalReconnectInitial   = 5 * time.Second
	signalReconnectMax       = 5 * time.Minute
)
```

- [ ] **Step 2: Write the failing test**

Add to `pkg/channel/signal/adapter_test.go`:

```go
func TestAdapterStartReconnectsReceiveLoopAfterCrash(t *testing.T) {
	calls := 0
	runner := &fakeRunner{
		receiveFn: func(ctx context.Context, _ string, _ []string, _ func(rawInboundMessage) error) error {
			calls++
			if calls == 1 {
				// Block past startup grace (50ms), then simulate crash
				select {
				case <-time.After(100 * time.Millisecond):
					return errors.New("signal-cli crashed")
				case <-ctx.Done():
					return nil
				}
			}
			// Second call: block until context cancelled (normal operation)
			<-ctx.Done()
			return nil
		},
	}
	adapter := NewAdapter(config.SignalChannelConfig{
		Account: "+15551234567",
		CLIPath: "signal-cli",
		DataDir: "/tmp/signal",
	}, runner)
	adapter.testReconnectDelay = 10 * time.Millisecond // speed up for test

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := adapter.Start(ctx, func(context.Context, channel.InboundMessage) {}); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Wait for the reconnect to happen (two receive calls means reconnect fired)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		status, _ := adapter.Status(context.Background())
		if status.State == "connected" && calls >= 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	if calls < 2 {
		t.Fatalf("expected at least 2 receive-loop calls (initial + reconnect), got %d", calls)
	}
	status, _ := adapter.Status(context.Background())
	if status.State != "connected" {
		t.Fatalf("expected connected after reconnect, got %s", status.State)
	}
}
```

- [ ] **Step 3: Run failing test**

```
go test ./pkg/channel/signal/ -run TestAdapterStartReconnectsReceiveLoopAfterCrash -v
```

Expected: FAIL — only 1 receive call happens; no reconnect loop.

- [ ] **Step 4: Replace the monitoring goroutine with a reconnect loop**

In `pkg/channel/signal/adapter.go`, replace the monitoring goroutine (the `go func() { err := <-resultCh ... }()` block that starts at around line 94) with the reconnect loop below. The rest of `Start` is unchanged.

The monitoring goroutine to remove:
```go
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
```

Replace it with (this goes in the same location in `Start`, after the grace-period select):

```go
go func() {
    currentResultCh := resultCh
    backoff := signalReconnectInitial
    if a.testReconnectDelay != 0 {
        backoff = a.testReconnectDelay
    }
    for {
        err := <-currentResultCh

        a.mu.Lock()
        a.connected = false
        if receiveCtx.Err() != nil {
            // Intentional stop (Stop() called or parent ctx cancelled)
            a.receiveCancel = nil
            a.receiveDone = nil
            a.mu.Unlock()
            return
        }
        a.mu.Unlock()

        log.Printf("[signal] receive loop exited (%v); reconnecting in %s", err, backoff)
        select {
        case <-receiveCtx.Done():
            a.mu.Lock()
            a.receiveCancel = nil
            a.receiveDone = nil
            a.mu.Unlock()
            return
        case <-time.After(backoff):
        }
        if a.testReconnectDelay == 0 && backoff < signalReconnectMax {
            backoff *= 2
            if backoff > signalReconnectMax {
                backoff = signalReconnectMax
            }
        }

        // Restart the receive loop
        newResultCh := make(chan error, 1)
        newDone := make(chan struct{})
        go func() {
            defer close(newDone)
            newResultCh <- a.runner.Receive(receiveCtx, a.cliPath(), args, func(raw rawInboundMessage) error {
                handler(receiveCtx, raw.normalize())
                return nil
            })
        }()

        a.mu.Lock()
        a.connected = true
        a.receiveDone = newDone
        a.mu.Unlock()

        currentResultCh = newResultCh
    }
}()
```

Note: `args`, `receiveCtx`, and `handler` are already in scope from the enclosing `Start` function. `log` must be in the import block (add `"log"` if not already present).

- [ ] **Step 5: Run all signal package tests**

```
go test ./pkg/channel/signal/... -v
```

Expected: all pass including `TestAdapterStartReconnectsReceiveLoopAfterCrash`.

- [ ] **Step 6: Commit**

```bash
git add pkg/channel/signal/adapter.go pkg/channel/signal/adapter_test.go
git commit -m "fix(signal): reconnect receive loop with exponential backoff on crash"
```

---

## Task 5: M6 — Manager periodic health-watch

**Files:**
- Modify: `pkg/channel/manager.go`
- Modify: `pkg/channel/manager_test.go`
- Modify: `cmd/smolbot/runtime.go`

The `Manager` has no background loop to surface dead channels. After the H2 and M5 fixes, channel status is accurately reported, but operators have no visibility into dead channels at runtime. Adding `Watch` fills this gap: it ticks on the given interval, calls `Statuses`, and fires a callback for any non-connected channel. `runtime.go` wires it up to log warnings.

- [ ] **Step 1: Write the failing test**

Add to `pkg/channel/manager_test.go`:

```go
func TestManagerWatchFiresCallbackForDeadChannels(t *testing.T) {
	manager := NewManager()
	dead := &fakeChannel{name: "signal", status: Status{State: "disconnected", Detail: "offline"}}
	live := &fakeChannel{name: "whatsapp", status: Status{State: "connected"}}
	manager.Register(dead)
	manager.Register(live)

	notified := make(chan string, 4)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go manager.Watch(ctx, 20*time.Millisecond, func(name string, _ Status) {
		notified <- name
		cancel() // stop after first batch
	})

	select {
	case got := <-notified:
		if got != "signal" {
			t.Fatalf("expected dead channel %q to be reported, got %q", "signal", got)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Watch did not fire callback within 2 seconds")
	}
}

func TestManagerWatchDoesNotFireForConnectedChannels(t *testing.T) {
	manager := NewManager()
	manager.Register(&fakeChannel{name: "discord", status: Status{State: "connected"}})

	fired := make(chan struct{}, 1)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	go manager.Watch(ctx, 20*time.Millisecond, func(string, Status) {
		fired <- struct{}{}
	})

	<-ctx.Done()
	select {
	case <-fired:
		t.Fatal("Watch should not fire for connected channels")
	default:
	}
}
```

- [ ] **Step 2: Run failing tests**

```
go test ./pkg/channel/ -run "TestManagerWatchFires|TestManagerWatchDoesNot" -v
```

Expected: FAIL — `manager.Watch` is undefined.

- [ ] **Step 3: Implement Watch on Manager**

Add the following method to `pkg/channel/manager.go` (after `ChannelNames`):

```go
// Watch starts a background health-check loop. On each tick it calls Statuses
// and invokes onDead for every channel not in "connected" state. Watch blocks
// until ctx is cancelled; call it in a goroutine.
func (m *Manager) Watch(ctx context.Context, interval time.Duration, onDead func(name string, status Status)) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for name, status := range m.Statuses(ctx) {
				if status.State != "connected" {
					onDead(name, status)
				}
			}
		}
	}
}
```

- [ ] **Step 4: Run all channel package tests**

```
go test ./pkg/channel/... -v
```

Expected: all pass.

- [ ] **Step 5: Wire Watch into runtime.go**

In `cmd/smolbot/runtime.go`, inside `launchDaemon`, add the `Watch` call right after the `app.channels.Start(ctx)` block:

```go
if err := app.channels.Start(ctx); err != nil {
    return err
}
defer func() {
    _ = app.channels.Stop(context.Background())
}()
go app.channels.Watch(ctx, 60*time.Second, func(name string, status channel.Status) {
    log.Printf("[channel] health-check: %s is %s (%s)", name, status.State, status.Detail)
})
```

- [ ] **Step 6: Build to confirm no compile errors**

```
go build ./cmd/smolbot/...
```

Expected: no errors.

- [ ] **Step 7: Commit**

```bash
git add pkg/channel/manager.go pkg/channel/manager_test.go cmd/smolbot/runtime.go
git commit -m "fix(channel): add periodic health-watch to Manager, wire into daemon"
```

---

## Task 6: M7 — Inbound goroutine panic recovery

**Files:**
- Modify: `cmd/smolbot/runtime.go`
- Modify: `cmd/smolbot/runtime_services_test.go`

The goroutine spawned by `handleInbound` (line 1140) has no `recover()`. A panic inside `agent.ProcessDirect` (e.g., from a tool or provider bug) silently kills the goroutine, leaving the channel unable to process further messages for that session without a daemon restart. The fix adds a `recover` defer and extracts a minimal `agentRunner` interface so the behavior is testable without a real `agent.AgentLoop`.

- [ ] **Step 1: Write the failing test**

Add to `cmd/smolbot/runtime_services_test.go`:

```go
func TestHandleInboundGoroutineRecoversPanic(t *testing.T) {
	mgr := channel.NewManager()
	mgr.SetInboundHandler(func(context.Context, channel.InboundMessage) {})

	panicked := make(chan struct{})
	app := &runtimeApp{
		channels: mgr,
		agent:    &panicOnProcessAgent{done: panicked},
	}

	app.handleInbound(context.Background(), channel.InboundMessage{
		Channel: "signal",
		ChatID:  "+15551234567",
		Content: "trigger panic",
	})

	select {
	case <-panicked:
		// panic was triggered; if it wasn't recovered the test binary would have crashed
	case <-time.After(2 * time.Second):
		t.Fatal("agent was not called within 2 seconds")
	}
}

type panicOnProcessAgent struct {
	done chan struct{}
}

func (a *panicOnProcessAgent) ProcessDirect(context.Context, agent.Request, agent.EventCallback) (string, error) {
	close(a.done) // signal that we were called
	panic("deliberate test panic in agent")
}
```

Add `"github.com/Nomadcxx/smolbot/pkg/agent"` to the import block if not already present.

- [ ] **Step 2: Run failing test**

```
go test ./cmd/smolbot/ -run TestHandleInboundGoroutineRecoversPanic -v
```

Expected: FAIL — compile error: `runtimeApp.agent` is `*agent.AgentLoop`, not an interface; `panicOnProcessAgent` does not satisfy it.

- [ ] **Step 3: Extract agentRunner interface in runtime.go**

In `cmd/smolbot/runtime.go`, add the interface definition above `runtimeApp`:

```go
// agentRunner is the minimal interface required by runtimeApp.
// *agent.AgentLoop satisfies this interface.
type agentRunner interface {
	ProcessDirect(ctx context.Context, req agent.Request, cb agent.EventCallback) (string, error)
}
```

Change the `agent` field in `runtimeApp` from `*agent.AgentLoop` to `agentRunner`:

```go
type runtimeApp struct {
	config           *config.Config
	paths            *config.Paths
	sessions         *session.Store
	usage            *usage.Store
	mcpCleanup       func()
	channels         *channel.Manager
	tools            *tool.Registry
	agent            agentRunner
	providerRegistry *provider.Registry
	cron             *cron.Service
	heartbeat        *heartbeat.Service
	runCron          func(context.Context, time.Time) error
	runBeat          func(context.Context) error
	runQuota         func(context.Context) error
	cronEvery        time.Duration
	beatEvery        time.Duration
	quotaEvery       time.Duration
	beatOn           bool
	gateway          *gateway.Server
}
```

The assignment `app.agent = loop` (where `loop` is `*agent.AgentLoop`) in `buildRuntime` continues to work because `*agent.AgentLoop` satisfies `agentRunner`.

- [ ] **Step 4: Add recover() to the inbound goroutine**

In `cmd/smolbot/runtime.go`, inside `handleInbound`, add a `recover` defer as the first statement inside the goroutine body:

```go
go func() {
    defer func() {
        if r := recover(); r != nil {
            log.Printf("[channel] panic in inbound handler for %s/%s: %v", msg.Channel, msg.ChatID, r)
        }
    }()
    sessionKey := firstNonEmpty(msg.Channel+":"+msg.ChatID, msg.ChatID, "channel:unknown")
    cb := a.channelEventCallback(msg.Channel, msg.ChatID)
    response, err := a.agent.ProcessDirect(ctx, agent.Request{
        Content:    msg.Content,
        SessionKey: sessionKey,
        Channel:    msg.Channel,
        ChatID:     msg.ChatID,
    }, cb)
    // ... rest of goroutine unchanged ...
}()
```

- [ ] **Step 5: Run all cmd/smolbot tests**

```
go test ./cmd/smolbot/... -v
```

Expected: all pass including `TestHandleInboundGoroutineRecoversPanic`.

- [ ] **Step 6: Commit**

```bash
git add cmd/smolbot/runtime.go cmd/smolbot/runtime_services_test.go
git commit -m "fix(runtime): recover panics in inbound handler goroutine"
```

---

## Final verification

- [ ] **Run the full test suite**

```
go test ./... -count=1
```

Expected: all pass with no race conditions. If you see flaky tests, re-run with `-race`:

```
go test ./pkg/channel/... ./pkg/cron/... ./cmd/smolbot/... -race -count=1
```

---

## Self-Review

**Spec coverage check:**

| Bug | Task | Covered? |
|-----|------|----------|
| H1 — nil handler guard | Task 1 | ✅ |
| H2 — WhatsApp disconnect status | Task 2 | ✅ |
| C15 — cron continue-on-error | Task 3 | ✅ |
| M11 — concurrent job guard | Task 3 | ✅ |
| M5 — Signal reconnect loop | Task 4 | ✅ |
| M6 — manager health-watch | Task 5 | ✅ |
| M7 — inbound goroutine recover | Task 6 | ✅ |
| L4 — Discord channelEnabled | N/A | ✅ already fixed |

**Placeholder scan:** No TBD, TODO, or vague steps found. All code blocks are complete.

**Type consistency:**
- `agentRunner` interface defined in Task 6 Step 3 and used in `runtimeApp.agent` in same step.
- `Watch(ctx context.Context, interval time.Duration, onDead func(name string, status Status))` signature defined in Task 5 Step 3 and called in Task 5 Step 5.
- `blockingCronProcessor.calls` incremented in `ProcessDirect` and read in test — consistent.
- `signalReconnectInitial`, `signalReconnectMax` constants added in Task 4 Step 1 and used in Task 4 Step 4.
- `runningJobs map[string]bool` added to `Service` in Task 3 Step 6, initialized in `NewService`, cleared in `executeJob` Step 8 — consistent.
