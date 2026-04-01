# Backend Event Pipeline Fixes — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix 6 broken/incomplete features in the agent→gateway→TUI event pipeline so compression indicators, tool outputs, and error handling work end-to-end.

**Architecture:** The agent loop (`pkg/agent/loop.go`) emits `Event` structs with `Type`, `Content`, and `Data` fields. The gateway (`pkg/gateway/server.go`) switches on `event.Type`, extracts fields from `event.Data`, and writes JSON event frames to the WebSocket client. The TUI client unmarshals these into typed payloads. Every fix follows the same pattern: ensure the emitting side populates the fields the receiving side reads.

**Tech Stack:** Go 1.26, WebSocket (gorilla/websocket), JSON marshaling

---

## File Map

| File | Responsibility | Tasks |
|------|---------------|-------|
| `pkg/agent/loop.go` | Emits events during agent processing | 1, 2, 3, 6 |
| `pkg/gateway/server.go` | Routes events from agent to WebSocket clients | 2, 4 |
| `pkg/gateway/server_test.go` | Gateway unit tests | 1, 2, 3 |
| `pkg/agent/loop_test.go` | Agent event emission tests | 1, 3 |
| `internal/client/protocol.go` | Client-side payload types | 5 |
| `internal/tui/tui.go` | TUI event handlers | 2 |

---

### Task 1: Fix EventContextCompressed — Populate Data Map

The agent emits `EventContextCompressed` with only a `Content` string, but the gateway reads `event.Data["originalTokens"]`, `event.Data["compressedTokens"]`, `event.Data["reductionPercent"]` — all nil. The TUI footer compression indicator silently shows 0%.

**Files:**
- Modify: `pkg/agent/loop.go:159-163`
- Modify: `pkg/agent/loop_test.go` (add test)
- Modify: `pkg/gateway/server_test.go` (add integration assertion)

- [ ] **Step 1: Write failing test for event Data fields**

In `pkg/agent/loop_test.go`, add a test that verifies `EventContextCompressed` events contain the Data map:

```go
func TestContextCompressedEventIncludesDataFields(t *testing.T) {
	// Setup: create agent loop with compression enabled and a tokenizer
	// that returns counts high enough to trigger compression.
	// Run a request, capture events, find EventContextCompressed,
	// assert event.Data["originalTokens"] is a non-zero int,
	// assert event.Data["compressedTokens"] is a non-zero int,
	// assert event.Data["reductionPercent"] is a non-zero float64.
}
```

If the existing test infrastructure doesn't support triggering compression easily, write a unit test that directly checks the emit call by extracting the compression event emission into a helper.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/agent/ -run TestContextCompressedEventIncludesDataFields -v`
Expected: FAIL — Data fields are nil

- [ ] **Step 3: Fix the emit call in loop.go**

Replace lines 159-163 in `pkg/agent/loop.go`:

```go
// BEFORE (broken):
emit(cb, Event{
    Type:    EventContextCompressed,
    Content: fmt.Sprintf("Context compressed: %d→%d tokens (%.0f%% reduction)",
        orig, comp, reduction),
})

// AFTER (fixed):
emit(cb, Event{
    Type:    EventContextCompressed,
    Content: fmt.Sprintf("Context compressed: %d→%d tokens (%.0f%% reduction)",
        orig, comp, reduction),
    Data: map[string]any{
        "originalTokens":   orig,
        "compressedTokens": comp,
        "reductionPercent": reduction,
        "mode":             string(compressionCfg.Mode),
    },
})
```

Note: `compressionCfg` is the compression config variable already in scope at this point in the function. Check the exact variable name — it may be `a.config.Agents.Defaults.Compression`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/agent/ -run TestContextCompressedEventIncludesDataFields -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/agent/loop.go pkg/agent/loop_test.go
git commit -m "fix: populate Data map in EventContextCompressed so gateway gets real token counts"
```

---

### Task 2: Handle EventToolHint in Gateway

The agent emits `EventToolHint` before each tool call, but the gateway has no case for it — the event is silently dropped. The TUI should receive a `chat.tool.hint` event so it can show which tool is about to run.

**Files:**
- Modify: `pkg/gateway/server.go:504-558` (add case)
- Modify: `pkg/gateway/server.go:190` (add to hello events list)
- Modify: `internal/client/protocol.go` (add payload type)
- Modify: `internal/tui/tui.go` (add event handler)
- Test: `pkg/gateway/protocol_test.go`

- [ ] **Step 1: Write failing test for tool hint event**

In `pkg/gateway/protocol_test.go`, add:

```go
func TestToolHintEventIsSentBeforeToolStart(t *testing.T) {
	// Setup: create test server with fake agent that emits EventToolHint then EventToolStart
	// Send chat.send, read events from WS
	// Assert: receive "chat.tool.hint" event with {"name": "tool_name"} before "chat.tool.start"
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/gateway/ -run TestToolHintEventIsSentBeforeToolStart -v`
Expected: FAIL — no `chat.tool.hint` event received

- [ ] **Step 3: Add case to gateway event switch**

In `pkg/gateway/server.go`, inside the `switch event.Type` block (after `EventProgress` case), add:

```go
case agent.EventToolHint:
    _ = s.writeEvent(state.owner, "chat.tool.hint", map[string]any{
        "name": event.Content,
    })
```

- [ ] **Step 4: Add `chat.tool.hint` to hello events list**

In `pkg/gateway/server.go:190`, add `"chat.tool.hint"` to the events slice:

```go
"events": []string{"chat.progress", "chat.done", "chat.error", "chat.tool.hint", "chat.tool.start", "chat.tool.done", "chat.thinking", "chat.thinking.done", "chat.usage", "context.compressed"},
```

- [ ] **Step 5: Add client-side payload type**

In `internal/client/protocol.go`, add:

```go
type ToolHintPayload struct {
    Name string `json:"name"`
}
```

- [ ] **Step 6: Handle in TUI event dispatcher**

In `internal/tui/tui.go`, inside the `switch msg.Event.Event` block, add:

```go
case "chat.tool.hint":
    // Tool hint is informational only for now; tool.start handles the actual display
```

- [ ] **Step 7: Run test to verify it passes**

Run: `go test ./pkg/gateway/ -run TestToolHintEventIsSentBeforeToolStart -v`
Expected: PASS

- [ ] **Step 8: Commit**

```bash
git add pkg/gateway/server.go internal/client/protocol.go internal/tui/tui.go pkg/gateway/protocol_test.go
git commit -m "fix: handle EventToolHint in gateway, send chat.tool.hint to clients"
```

---

### Task 3: Populate Tool Output/Error in EventToolDone

The agent emits `EventToolDone` with only `deliveredToRequestTarget` and `id` in Data. The gateway reads `event.Data["output"]` and `event.Data["error"]` — both are nil. Tool output displayed in the TUI comes only from tool events that have these fields.

**Files:**
- Modify: `pkg/agent/loop.go:258-265`
- Test: `pkg/agent/loop_test.go`

- [ ] **Step 1: Write failing test for tool done output**

In `pkg/agent/loop_test.go`, add/modify a test:

```go
func TestToolDoneEventIncludesOutput(t *testing.T) {
	// Setup: create agent loop with a tool that returns known output
	// Run a request, capture events, find EventToolDone
	// Assert: event.Data["output"] == expected output string
	// Assert: event.Data["id"] == tool call ID
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/agent/ -run TestToolDoneEventIncludesOutput -v`
Expected: FAIL — Data["output"] is nil

- [ ] **Step 3: Add output and error to EventToolDone emission**

In `pkg/agent/loop.go`, replace lines 258-265:

```go
// BEFORE:
emit(cb, Event{
    Type:    EventToolDone,
    Content: toolCall.Function.Name,
    Data: map[string]any{
        "deliveredToRequestTarget": delivered,
        "id":                      toolCall.ID,
    },
})

// AFTER:
toolOutput := truncateString(content, 4000)
toolError := ""
if err != nil {
    toolError = err.Error()
}
emit(cb, Event{
    Type:    EventToolDone,
    Content: toolCall.Function.Name,
    Data: map[string]any{
        "deliveredToRequestTarget": delivered,
        "id":                      toolCall.ID,
        "output":                  toolOutput,
        "error":                   toolError,
    },
})
```

**Important:** The variable `content` is already computed on line 241 (`content := truncateString(...)`). The `err` at this point refers to tool execution error from line 237-239. However, note the early return on error at line 238 — if the tool errors, the function returns before reaching EventToolDone. So `toolError` will always be empty here. That's fine — the gateway's errStr extraction will just get "". For tools that succeed but return partial errors in their output, those are already in `content`.

Actually, re-check: the tool error path at line 237-239 does `return "", err` — so EventToolDone is only emitted for successful tools. The error field is for the gateway's benefit if we later change that behavior. For now, just pass `""` for error and the output:

```go
emit(cb, Event{
    Type:    EventToolDone,
    Content: toolCall.Function.Name,
    Data: map[string]any{
        "deliveredToRequestTarget": delivered,
        "id":                      toolCall.ID,
        "output":                  truncateString(content, 4000),
        "error":                   "",
    },
})
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/agent/ -run TestToolDoneEventIncludesOutput -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/agent/loop.go pkg/agent/loop_test.go
git commit -m "fix: include tool output in EventToolDone Data for gateway to forward to clients"
```

---

### Task 4: Log writeEvent Errors Instead of Swallowing

All `writeEvent` calls in the gateway use `_ = s.writeEvent(...)`, silently discarding errors. If a WebSocket connection breaks mid-stream, events are lost with no indication.

**Files:**
- Modify: `pkg/gateway/server.go:503-578` (executeRun method)

- [ ] **Step 1: Write failing test for error logging**

In `pkg/gateway/server_test.go`, add a test that verifies write errors trigger logging (or at minimum, that a broken connection is detected). This may require a mock client or a connection that returns errors.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/gateway/ -run TestWriteEventErrorsAreLogged -v`
Expected: FAIL

- [ ] **Step 3: Replace silent discards with logged errors**

In `pkg/gateway/server.go`, add a `log` import if not present, then create a helper:

```go
func (s *Server) emitEvent(client *clientState, name string, payload map[string]any) {
	if err := s.writeEvent(client, name, payload); err != nil {
		log.Printf("[gateway] write %s event failed: %v", name, err)
	}
}
```

Then replace all `_ = s.writeEvent(state.owner, ...)` calls in `executeRun` with `s.emitEvent(state.owner, ...)`. This covers lines 507, 509, 513, 525, 533, 539, 552, 562, 565, 567.

Do NOT change the error handling behavior — just log. No retries, no disconnection logic. Keep it simple.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./pkg/gateway/ -run TestWriteEventErrorsAreLogged -v`
Expected: PASS

- [ ] **Step 5: Run all gateway tests to verify no regressions**

Run: `go test ./pkg/gateway/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add pkg/gateway/server.go pkg/gateway/server_test.go
git commit -m "fix: log writeEvent errors instead of silently discarding them"
```

---

### Task 5: Populate CompressionInfo Mode and LastRun

The gateway sends `context.compressed` events but never includes `mode` or timestamp. The TUI `CompressionInfo` struct has `Mode` and `LastRun` fields that stay empty.

**Files:**
- Modify: `pkg/gateway/server.go:535-544`
- Modify: `pkg/agent/loop.go:159-163` (already touched in Task 1 — add mode field)

- [ ] **Step 1: Write test for mode and timestamp in compression event**

In `pkg/gateway/server_test.go` or `pkg/gateway/protocol_test.go`:

```go
func TestCompressionEventIncludesModeAndTimestamp(t *testing.T) {
	// Verify the context.compressed event payload includes "mode" and "lastRun" fields
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/gateway/ -run TestCompressionEventIncludesModeAndTimestamp -v`
Expected: FAIL

- [ ] **Step 3: Add mode to agent emission (if not done in Task 1)**

Verify `event.Data["mode"]` is populated from Task 1. If not, add it now.

- [ ] **Step 4: Add mode and timestamp to gateway event**

In `pkg/gateway/server.go`, update the `EventContextCompressed` case:

```go
case agent.EventContextCompressed:
    originalTokens, _ := event.Data["originalTokens"].(int)
    compressedTokens, _ := event.Data["compressedTokens"].(int)
    reductionPercent, _ := event.Data["reductionPercent"].(float64)
    mode, _ := event.Data["mode"].(string)
    _ = s.writeEvent(state.owner, "context.compressed", map[string]any{
        "enabled":          true,
        "originalTokens":   originalTokens,
        "compressedTokens": compressedTokens,
        "reductionPercent": reductionPercent,
        "mode":             mode,
        "lastRun":          time.Now().UTC().Format(time.RFC3339),
    })
```

Add `"time"` to imports if not present.

- [ ] **Step 5: Run test to verify it passes**

Run: `go test ./pkg/gateway/ -run TestCompressionEventIncludesModeAndTimestamp -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add pkg/gateway/server.go pkg/agent/loop.go pkg/gateway/protocol_test.go
git commit -m "feat: include compression mode and timestamp in context.compressed events"
```

---

### Task 6: Log Memory Consolidation Errors

The background memory consolidation goroutine swallows all errors with `_ = a.memory.MaybeConsolidate(...)`. If consolidation repeatedly fails, the session's context grows unbounded.

**Files:**
- Modify: `pkg/agent/loop.go:288-294`

- [ ] **Step 1: Add logging to the background goroutine**

Replace lines 289-293:

```go
// BEFORE:
a.bgTasks.Add(1)
go func() {
    defer a.bgTasks.Done()
    _ = a.memory.MaybeConsolidate(context.Background(), req.SessionKey)
}()

// AFTER:
a.bgTasks.Add(1)
go func() {
    defer a.bgTasks.Done()
    if err := a.memory.MaybeConsolidate(context.Background(), req.SessionKey); err != nil {
        log.Printf("[agent] memory consolidation failed for session %s: %v", req.SessionKey, err)
    }
}()
```

Add `"log"` to imports if not present.

- [ ] **Step 2: Build to verify compilation**

Run: `go build ./pkg/agent/...`
Expected: Clean build

- [ ] **Step 3: Run all agent tests**

Run: `go test ./pkg/agent/ -v`
Expected: All PASS (or pre-existing failures only)

- [ ] **Step 4: Commit**

```bash
git add pkg/agent/loop.go
git commit -m "fix: log memory consolidation errors instead of silently swallowing them"
```

---

## Final Verification

- [ ] **Full build:** `go build ./cmd/... ./pkg/... ./internal/...`
- [ ] **Full test suite:** `go test ./pkg/gateway/ ./pkg/agent/ ./internal/tui/ ./internal/components/chat/ -v`
- [ ] **Build binaries:** `go build -o smolbot-tui ./cmd/smolbot-tui && go build -o smolbot ./cmd/smolbot`
