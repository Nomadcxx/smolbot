# Chat Interruption And Queueing Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Eliminate `session "... already active"` from ordinary TUI use by adding proper message queueing and clear run-state behavior while preserving explicit abort semantics.

**Architecture:** Treat this as a run-orchestration problem, not a cosmetic TUI bug. The gateway and agent loop currently enforce one active run per session; the TUI still submits while streaming. The fix is to introduce queued pending input at the gateway/session layer, surface queue state through events, and teach the TUI to show and manage queued messages cleanly.

**Tech Stack:** Go, Bubble Tea, gateway websocket protocol, agent loop session lifecycle.

---

## Execution Workflow

Implement one task at a time in a dedicated worktree. After each task:

1. Run the task’s focused verification.
2. Run a spec/completeness review for that slice.
3. Run a code-quality review for that slice.
4. Fix findings before moving on.
5. Create a checkpoint commit.

## Reference Notes

- Current smolbot behavior:
  - `pkg/gateway/server.go` rejects duplicate `chat.send` for an active session.
  - `pkg/gateway/concurrency_test.go` explicitly expects the rejection.
  - `pkg/agent/loop.go` permits only one active task per session.
  - `internal/tui/tui.go` still submits user input while streaming.
- Desired user-facing behavior:
  - ordinary follow-up input during a run should queue, not hard-fail
  - explicit abort should still interrupt the active run
  - queued work should be visible in the UI

## Non-Negotiable Invariants

- Only one active run may execute per session at a time.
- Additional sends for that session must queue rather than error.
- Abort must cancel the active run without corrupting queued work.
- The TUI must clearly show when input is queued.
- Queue behavior must be covered with real gateway tests.

## Task 1: Capture The Current Failure As A Regression

**Files:**
- Modify: `pkg/gateway/concurrency_test.go`
- Modify: `internal/tui/tui_test.go`

**Intent:**
Convert the observed bug into explicit tests before changing behavior.

**Steps:**
1. Add a TUI-level regression showing that submit-during-streaming currently reaches the backend and produces `already active`.
2. Add a gateway-level characterization test for current duplicate-send behavior.
3. Mark clearly which test expectations will change after queueing is implemented.

**Verification:**
- `go test ./pkg/gateway ./internal/tui -run 'Test.*Concurrency|Test.*Queue|Test.*Streaming'`

**Gate:**
- Spec review: the current bug is reproduced in tests.
- Code-quality review: characterization tests do not overfit incidental details.

**Checkpoint Commit:**
- `test(chat): capture duplicate-send regression`

## Task 2: Add Gateway Session Queueing

**Files:**
- Modify: `pkg/gateway/server.go`
- Modify: `pkg/gateway/concurrency_test.go`
- Create: `pkg/gateway/queue_test.go`

**Intent:**
Replace duplicate-send rejection with per-session pending queue behavior.

**Steps:**
1. Add per-session queued request storage in the gateway.
2. On `chat.send`:
   - start immediately if no active run exists
   - enqueue if a run is active for the same session
3. When the active run completes, automatically start the next queued request.
4. Preserve current behavior across different sessions; only same-session sends should queue behind each other.
5. Add queue events for the TUI:
   - queued
   - dequeued/started
   - queue drained

**Verification:**
- `go test ./pkg/gateway -run 'Test.*Queue|Test.*Concurrency'`

**Gate:**
- Spec review: same-session sends now queue instead of erroring.
- Code-quality review: queue ownership/lifecycle is explicit and cleanup-safe.

**Checkpoint Commit:**
- `feat(gateway): queue same-session chat sends`

## Task 3: Integrate Queueing With Abort And Disconnect Semantics

**Files:**
- Modify: `pkg/gateway/server.go`
- Modify: `pkg/gateway/concurrency_test.go`
- Modify: `pkg/agent/loop_test.go`

**Intent:**
Make sure abort, disconnect, and shutdown semantics remain coherent once queued work exists.

**Steps:**
1. Decide and document queue semantics for abort:
   - abort active only, leaving queue intact
   - or abort active plus queued messages for the same session
2. Implement that chosen behavior consistently.
3. Ensure websocket disconnect cleans up its owned active and queued work safely.
4. Ensure server shutdown cancels active runs and discards queued work safely.

**Verification:**
- `go test ./pkg/gateway ./pkg/agent -run 'Test.*Abort|Test.*Disconnect|Test.*Shutdown|Test.*Queue'`

**Gate:**
- Spec review: queue behavior during abort/disconnect is clear and unsurprising.
- Code-quality review: no orphaned goroutines or stale queue state remain.

**Checkpoint Commit:**
- `fix(gateway): align queueing with abort and shutdown`

## Task 4: Surface Queue State In The TUI

**Files:**
- Modify: `internal/client/protocol.go`
- Modify: `internal/tui/tui.go`
- Modify: `internal/tui/tui_test.go`
- Modify: `internal/components/chat/messages.go`
- Modify: `internal/components/chat/message_test.go`

**Intent:**
Show the user that their input was queued rather than dropped.

**Steps:**
1. Add protocol payloads for queue events.
2. Show a compact system or transcript artifact when a message is queued.
3. When the queued message begins running, update the UI so it is no longer ambiguous.
4. Keep the interaction lightweight; this should not become a deep orchestration UI.

**Verification:**
- `go test ./internal/tui ./internal/components/chat -run 'Test.*Queue|Test.*Streaming'`

**Gate:**
- Spec review: the TUI explains blocked/queued state clearly.
- Code-quality review: queue state does not pollute unrelated transcript rendering.

**Checkpoint Commit:**
- `feat(tui): display queued chat input state`

## Task 5: Tighten Input Handling During Streaming

**Files:**
- Modify: `internal/tui/tui.go`
- Modify: `internal/tui/tui_test.go`

**Intent:**
Make the TUI cooperate cleanly with queued runs and explicit interrupt behavior.

**Steps:**
1. Preserve normal typing while streaming.
2. On submit during streaming, enqueue rather than calling a path that can still error noisily.
3. Keep `Ctrl+C` / abort behavior explicit and separate from queueing.
4. Add tests for:
   - submit while streaming
   - submit multiple queued messages
   - abort active run with queued work present

**Verification:**
- `go test ./internal/tui -run 'Test.*Queue|Test.*Abort|Test.*Streaming'`

**Gate:**
- Spec review: follow-up typing during a run is usable.
- Code-quality review: TUI state machine stays understandable.

**Checkpoint Commit:**
- `feat(tui): queue follow-up input during streaming`

## Task 6: Final End-To-End Coverage

**Files:**
- Modify: `pkg/gateway/concurrency_test.go`
- Modify: `internal/tui/tui_test.go`
- Create: `pkg/gateway/queue_integration_test.go`

**Intent:**
Lock in the new semantics so the old `already active` behavior does not return accidentally.

**Steps:**
1. Replace old rejection assertions with queue assertions where appropriate.
2. Add a full-path test:
   - send first message
   - send second while first active
   - verify queued event
   - finish first
   - verify second begins automatically
3. Keep a separate test proving concurrent different-session sends still work.

**Verification:**
- `go test ./pkg/gateway ./internal/tui -run 'Test.*Queue|Test.*Concurrency'`

**Gate:**
- Spec review: the user-visible bug is gone.
- Code-quality review: tests prove the intended semantics instead of only one happy path.

**Checkpoint Commit:**
- `test(chat): add queueing integration coverage`

## Final Gate

Before push or merge:

1. Run:
   - `go test ./pkg/gateway ./pkg/agent ./internal/client ./internal/components/chat ./internal/tui ./cmd/smolbot`
   - `go build ./cmd/smolbot ./cmd/smolbot-tui`
2. Run a final spec/completeness review over the whole queueing change.
3. Run a final code-quality review over the whole queueing change.
4. Push only when:
   - submit-during-streaming no longer yields `already active`
   - queue behavior is visible and understandable
   - abort/disconnect behavior remains safe

