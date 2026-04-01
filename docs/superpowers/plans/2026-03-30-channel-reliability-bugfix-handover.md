# Handover: Channel Reliability Bugfix Plan

**Assigned issues:** H1, H2, C15, M11, M5, M6, M7 (7 bugs across 6 tasks)
**Plan file:** `docs/superpowers/plans/2026-03-30-channel-reliability-bugfix.md`
**Branch:** Create a new worktree branch (e.g., `fix/channel-reliability-bugfix`)
**Worktree location:** `.worktrees/channel-reliability-bugfix`

---

## What You Are Taking Over

This is a continuation of bugfix work. The plan at `docs/superpowers/plans/2026-03-30-channel-reliability-bugfix.md` has been reviewed and the key references verified. The issues you are responsible for are:

| Bug ID | Issue | Task |
|--------|-------|------|
| H1 | Nil handler guard — `Manager.Start` doesn't check if `inboundHandler` is nil before passing to channels | Task 1 |
| H2 | WhatsApp disconnect status — adapter only handles `*waEvents.Message`, never transitions away from "connected" on disconnect | Task 2 |
| C15 | Cron continue-on-error — `RunDue` returns on first error, skipping remaining jobs | Task 3 |
| M11 | Cron concurrent job guard — mutex released before `executeJob` runs, allowing same job to run twice | Task 3 |
| M5 | Signal reconnect loop — receive goroutine exits on crash but never retries; no reconnect logic | Task 4 |
| M6 | Manager health-watch — no background loop to surface dead channels to operators | Task 5 |
| M7 | Inbound goroutine panic recovery — `handleInbound` goroutine has no `recover()`, panic kills the goroutine silently | Task 6 |

**Already fixed:** L4 (Discord `channelEnabled` gap) — confirmed fixed at `runtime.go:1009–1020`.

---

## Professional Standards I Hold Myself To

**Be thorough, not fast.** Speed without accuracy creates rework. If something in the plan seems wrong or the codebase doesn't match the plan's line references, investigate before implementing. It is always better to report a discrepancy and ask than to silently do the wrong thing.

**Verify before claiming.** Every test you write should fail before the fix and pass after. If a test doesn't fail, the bug it was meant to catch doesn't exist — figure out why before proceeding. Every commit should have passing tests.

**Keep scope tight.** The plan describes specific bugs with specific fixes. Do not refactor adjacent code, rename things for style, or add functionality not in the plan. If you find a related bug that isn't in the plan, mention it in your report but don't fix it unless explicitly asked.

**Communicate clearly.** If you encounter something unexpected — a referenced line doesn't exist, a function signature is different, a test helper doesn't exist — stop and report it. You will never be penalized for asking. You will be questioned if you silently do something different from what was planned.

**Own your commits.** Each task should be its own commit with a clear message following conventional commit format (`fix(scope): description`). Review what you've changed before committing.

---

## How To Approach This Work

### Before You Start

1. **Set up a fresh worktree** — do not work on `main` or an existing branch:
   ```bash
   cd /home/nomadx/Documents/smolbot
   git worktree add .worktrees/channel-reliability-bugfix -b fix/channel-reliability-bugfix
   cd .worktrees/channel-reliability-bugfix
   go mod download
   go test ./... -count=1 2>&1 | tee /tmp/baseline-tests.txt
   ```
   Confirm all tests pass on the baseline before making any changes.

2. **Read the plan** — `docs/superpowers/plans/2026-03-30-channel-reliability-bugfix.md` in full. Understand the root cause of each issue, not just the fix.

3. **Verify line references** — the plan was written before the current state of the code. Lines may have shifted. Verify each referenced location before starting a task.

### Per-Task Workflow

For each task, follow this sequence:

1. **Write the failing test first** — as specified in the plan. Do not implement the fix until the test fails with the expected error.

2. **Run the test** — confirm it fails with the expected symptom (compile error, nil pointer, unexpected behavior, etc.).

3. **Implement the fix** — exactly as described in the plan. If the plan's approach doesn't fit the actual code, stop and report the discrepancy.

4. **Run the test again** — confirm it passes.

5. **Run the full test suite for the affected package(s)** — confirm no regressions.

6. **Commit** — with a conventional commit message.

### Issue-Specific Notes

**Task 1 (H1 — Nil handler guard):** Straightforward nil check at the top of `Manager.Start`. The plan also fixes two existing lifecycle tests in `manager_lifecycle_test.go` that call `Start` without `SetInboundHandler` — those tests need the handler added.

**Task 2 (H2 — WhatsApp disconnect tracking):** This introduces a `clientSeam` interface extension with `SetConnectionStateHandler`. The test uses a `fakeSeam` that needs new `onDisconnect`/`onReconnect` fields and the new method. The production implementation stores callbacks in `whatsmeowSeam` and fires them from `handleEvent` when `*waEvents.Disconnected` or `*waEvents.Connected` events arrive.

**Task 3 (C15 + M11 — Cron):** Both bugs are in `RunDue` and `executeJob`. C15 changes the error handling in `RunDue` to continue after the first error. M11 adds a `runningJobs map[string]bool` to `Service` to prevent concurrent execution of the same job. The plan provides two sub-tests: one for continue-on-error, one for the concurrent guard. The `blockingCronProcessor` helper uses a `sync.Mutex` pattern that is safe for concurrent test access.

**Task 4 (M5 — Signal reconnect):** The most complex fix. Replaces a one-shot monitoring goroutine with a reconnect loop. The loop uses exponential backoff (5s initial, max 5min) and re-launches the `receive` goroutine when it exits unexpectedly. The plan's test sets `adapter.testReconnectDelay = 10ms` to speed up the test.

**Task 5 (M6 — Manager.Watch):** Simple periodic health-check loop. `Watch` takes a `context.Context`, an interval, and a callback. The runtime wires it to log warnings for dead channels every 60 seconds.

**Task 6 (M7 — Panic recovery):** Extracts an `agentRunner` interface to enable testing without a real `*agent.AgentLoop`. The `recover()` defer is added inside the goroutine spawned by `handleInbound`. The test uses a `panicOnProcessAgent` type that panics when `ProcessDirect` is called.

---

## What To Report When You're Done

When you complete your assigned tasks, report:

- Which tasks you completed and their commit SHAs
- Any tasks you could not complete and why
- Any discrepancies you found between the plan and the actual code
- Any new issues you discovered that are related but outside scope

---

## Contacts and Context

This project is `smolbot` at `/home/nomadx/Documents/smolbot`. The codebase is Go with:
- Channel manager: `pkg/channel/manager.go`
- WhatsApp adapter: `pkg/channel/whatsapp/adapter.go`
- Signal adapter: `pkg/channel/signal/adapter.go`
- Cron service: `pkg/cron/service.go`
- CLI runtime: `cmd/smolbot/runtime.go`

Tests use the standard `testing` package and `go test`. Run the full suite with `go test ./...`. Run with `-race` for race detection: `go test -race ./pkg/channel/... ./pkg/cron/... ./cmd/smolbot/...`.

---

## A Note From Me

The plan is well-structured with comprehensive tests. A few things to watch:

1. **Task 3 (Cron)** has two sub-tests within one `TestService` test function. Follow the plan exactly when inserting them — the C15 sub-test must be placed before the M11 sub-test, and both go inside the `t.Run` closure body.

2. **Task 4 (Signal reconnect)** — the reconnect loop in the plan uses `log.Printf`. Verify `log` is imported in the adapter file before implementing.

3. **Task 6 (Panic recovery)** — changing `runtimeApp.agent` from `*agent.AgentLoop` to `agentRunner` interface is safe because `*agent.AgentLoop` satisfies the interface. But verify this compiles before committing.

The worktree `.worktrees/channel-reliability-bugfix` is available for your use.

Good luck.
