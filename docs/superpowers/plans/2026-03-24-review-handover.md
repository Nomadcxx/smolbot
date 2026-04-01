# P0/P1/P2 Implementation Review Handover

> **For:** Reviewing agent investigating why UI changes are not visible to user
> **Created:** 2026-03-24

---

## What Was Supposed to Be Implemented

Two plans were executed:

1. **`docs/superpowers/plans/2026-03-24-p0-p1-tui-fixes.md`** — 4 tasks (P0/P1)
2. **`docs/superpowers/plans/2026-03-24-p2-ui-polish.md`** — 5 tasks (P2)

### P0/P1 Tasks
| # | Task | Commit |
|---|------|--------|
| 1 | Fix CoerceArgs bool parse (LLMs send `"1"`/`"0"`/`"yes"`/`"no"`) | `2e9947b` |
| 2 | Add streaming thinking events to gateway → TUI | `cc0f543` |
| 3 | Fix tool duplicate rendering using tool call IDs | `6b1ad94` |
| 4 | Emit usage data events for context tracker | `c8c50bf` |

### P2 Tasks
| # | Task | Commit |
|---|------|--------|
| 1 | Redesign header as compact single-line with model/context%/path | `f0bd4e6` |
| 2 | Tool output truncation to 10 lines with hidden count hint | `f97f599` |
| 3 | Thinking block with "Thought for Xs" footer | `626e989` |
| 4 | Cap message content width at 120 characters | `07096fa` |
| 5 | Wire header to live app state, remove spacer in compact mode | `f0bd4e6` |

---

## User Reports None of the P2 UI Changes Are Visible

The user confirms:
- No thinking blocks shown
- No token usage in footer
- No context % in header
- Tool output still not truncated to 10 lines

**The user ran `/tmp/smolbot-tui` (freshly built binary).**

---

## Code Presence Check (What I Verified in Session)

### Tool Truncation — CODE IS PRESENT
```
$ grep -n maxToolOutputLines internal/components/chat/message.go
35:const maxToolOutputLines = 10
92:    if len(outputLines) > maxToolOutputLines {
93:        hidden := len(outputLines) - maxToolOutputLines
94:        bodyText = strings.Join(outputLines[:maxToolOutputLines], "\n")
```

### Thinking Block Renderer — CODE IS PRESENT
```
$ grep -n renderThinkingBlock internal/components/chat/message.go
221:func renderThinkingBlock(body string, dur time.Duration, accent color.Color, width int) string {
```

### Thinking Append — CODE IS PRESENT
```
$ grep -n AppendThinking internal/components/chat/messages.go
80:func (m *MessagesModel) AppendThinking(content string) {
```

### chat.usage handler — CODE IS PRESENT
```
$ grep -n "chat.usage" internal/tui/tui.go
407:    case "chat.usage":
```

### contextPct Setter — CODE IS PRESENT
```
$ grep -n SetContextPercent internal/components/header/header.go
37:func (m *Model) SetContextPercent(v int) {
```

### Binary Contains Expected Symbols
```
$ strings /tmp/smolbot-tui | grep -E "contextPct|SetContextPercent|renderCompact|maxToolOutputLines|renderThinkingBlock"
contextPct
renderCompact
SetContextPercent
github.com/Nomadcxx/smolbot/internal/components/header.(*Model).SetContextPercent
github.com/Nomadcxx/smolbot/internal/components/header.(*Model).renderCompact
```

**The code is there. The binary contains it. Yet the user sees nothing.**

---

## Root Cause Hypothesis

The thinking lifecycle in `messages.go` has a critical path:

1. `SetThinking(content)` — accumulates streaming chunks, sets `thinkingStart`
2. `AppendThinking(content)` — finalizes, calculates duration, appends to messages
3. `View()` — renders `m.thinking` as ephemeral, and `m.messages` (including role="thinking")

**Potential issue:** In `messages.go`, `AppendUser()` and `AppendAssistant()` CLEAR `m.thinking`:

```go
// Line 61, 68, 76, 117 — m.thinking = ""
```

If `ThinkingDoneMsg` is not handled (or handled AFTER `AppendAssistant`/`AppendUser`), the thinking text is wiped.

**Another potential issue:** The `chat.thinking.done` event is mapped to `ThinkingDoneMsg`, but what if thinking chunks arrive as `chat.progress` or some other event type, and `chat.thinking.done` never fires?

**Another potential issue:** The thinking content may not be reaching the TUI at all if the gateway isn't emitting `chat.thinking` events.

---

## Files to Investigate (Full Traces Required)

### Thinking Trace
| Step | File | What to check |
|------|------|---------------|
| 1 | `pkg/agent/loop.go` | Does `EventThinking` case emit to callback? (grep: `EventThinking`) |
| 2 | `pkg/gateway/server.go` | Does `executeRun` relay `chat.thinking` events? (grep: `chat.thinking`) |
| 3 | `internal/tui/tui.go` | Does `EventMsg` switch have `"chat.thinking"` case? (grep: `chat.thinking`) |
| 4 | `internal/components/chat/messages.go` | Does `SetThinking` work? Does `View()` render thinking role? (grep: `thinking`) |
| 5 | `internal/components/chat/message.go` | Is `renderThinkingBlock` at line 221? (grep: `renderThinkingBlock`) |

### Usage Trace
| Step | File | What to check |
|------|------|---------------|
| 1 | `pkg/agent/loop.go` | Does `EventUsage` exist and emit after `consumeStream`? (grep: `EventUsage`) |
| 2 | `pkg/gateway/server.go` | Does `executeRun` relay `chat.usage`? (grep: `chat.usage`) |
| 3 | `internal/tui/tui.go` | Does `EventMsg` handle `chat.usage`? (grep: `chat.usage`) |
| 4 | `internal/components/status/footer.go` | Does `SetUsage` store and `renderUsage` display? (grep: `renderUsage`) |

### Header Trace
| Step | File | What to check |
|------|------|---------------|
| 1 | `internal/tui/tui.go` | Does `StatusLoadedMsg` call `m.header.SetContextPercent`? |
| 2 | `internal/tui/tui.go` | Does `chat.usage` handler call `m.header.SetContextPercent`? |
| 3 | `internal/components/header/header.go` | Does `renderCompact` use `contextPct`? (grep: `contextPct`) |

### Tool Truncation Trace
| Step | File | What to check |
|------|------|---------------|
| 1 | `internal/components/chat/message.go` | Is `maxToolOutputLines = 10` at line 35? |
| 2 | `internal/components/chat/messages.go` | Does `View()` pass `expanded` flag to `renderToolCall`? |

---

## Key File Locations

### Plans (Spec)
| File | Description |
|------|-------------|
| `docs/superpowers/plans/2026-03-24-p0-p1-tui-fixes.md` | P0/P1 spec (4 tasks) |
| `docs/superpowers/plans/2026-03-24-p2-ui-polish.md` | P2 spec (5 tasks) |

### Core Implementation Files
| File | Purpose |
|------|---------|
| `internal/components/header/header.go` | Header rendering — compact vs full mode |
| `internal/tui/tui.go` | Main TUI — layout, event handling, View() |
| `internal/components/chat/message.go` | Message/tool/thinking render functions |
| `internal/components/chat/messages.go` | MessagesModel — state management |
| `internal/components/status/footer.go` | Footer — usage display |
| `internal/tui/menu_dialog.go` | Menu dialog rendering |
| `pkg/agent/loop.go` | Agent loop — event emission |
| `pkg/agent/types.go` | Event type constants |
| `pkg/gateway/server.go` | Gateway — event relay |
| `internal/client/protocol.go` | Client event payload types |

### Test Files
| File | Purpose |
|------|---------|
| `internal/tui/tui_test.go` | TUI integration tests |
| `internal/components/chat/messages_test.go` | Messages model tests |
| `internal/components/chat/message_test.go` | Message render tests |
| `pkg/tool/coerce_test.go` | CoerceArgs tests |

---

## Build & Test Commands

```bash
cd /home/nomadx/Documents/smolbot

# Build binary
go build -o /tmp/smolbot-tui ./cmd/smolbot-tui

# Run all relevant tests
go test ./internal/tui/ ./internal/components/chat/ ./internal/components/header/ ./pkg/tool/ ./pkg/gateway/ ./pkg/agent/ -count=1

# Run specific test
go test ./internal/tui/ -run TestF1MenuRendersCenteredAwayFromTopLeft -v

# Check binary symbols
strings /tmp/smolbot-tui | grep -E "renderThinkingBlock|maxToolOutputLines|contextPct|SetContextPercent"
```

---

## Pre-Existing Test Failures (NOT caused by this work)

| Test | Reason |
|------|--------|
| `TestF1MenuRendersCenteredAwayFromTopLeft` | lipgloss canvas + plain() ANSI strip issue — pre-existing |
| `TestChatHistoryResponseShape` | Gateway protocol JSON unmarshal — pre-existing |

Confirmed by: `git stash && go test ./pkg/gateway/ -run TestChatHistoryResponseShape` (fails without changes)

---

## Git Log

```
f0bd4e6 feat(tui): wire header to live app state and tighten compact layout
07096fa feat(tui): cap message content width at 120 characters
626e989 feat(tui): add 'Thought for Xs' footer to thinking blocks
f97f599 feat(tui): truncate tool output to 10 lines with hidden count hint
c8c50bf feat(gateway): emit token usage events for context tracker
6b1ad94 fix(tui): match tool calls by unique ID instead of name
cc0f543 feat(gateway): stream thinking chunks to TUI in real time
2e9947b fix(tool): CoerceArgs uses target struct types for bool/int coercion
```

---

## Critical Instruction for Reviewing Agent

**The user sees NONE of the P2 UI changes despite:**
1. Code being committed and present in source files
2. Binary rebuilt and containing expected symbols
3. Binary replaced at `/tmp/smolbot-tui`
4. Tests passing (except 2 pre-existing failures)

**Do NOT assume the code is correct. Trace EVERYTHING from gateway to render. Find the actual bug. Fix it. Rebuild. Verify with tests.**

Possible root causes to investigate:
1. Event type mismatch — TUI looking for wrong event name
2. Thinking cleared before render — `AppendAssistant`/`AppendUser` wiping `m.thinking`
3. `ThinkingDoneMsg` mapped to wrong handler
4. `chat.thinking` event never arrives from gateway (gateway not emitting)
5. `resp.Usage` is zero/not populated by provider (usage never > 0)
6. Context window token config is zero (`ContextWindowTokens` in config)
7. Binary not actually being run (user running different binary)
8. Cached header render — `m.cached` not invalidated when `contextPct` changes

---

## What the Spec Says (P2 Plan Summary)

### Task 1: Compact Header
- Shows `smolbot ╱╱╱ model-name • 45% • ~/path` when terminal ≤30 rows
- Threshold changed from ≤16 to ≤30 in `tui.go:222`

### Task 2: Tool Truncation
- `maxToolOutputLines = 10` at `message.go:35`
- Hint format: `… (N lines hidden)`

### Task 3: Thinking Duration
- `thinkingStart` set when first chunk arrives via `SetThinking`
- `AppendThinking` calculates `time.Since(thinkingStart)`
- Footer: `"Thought for Xs"` or `"Thought for Xms"`
- `renderThinkingBlock` at `message.go:221`

### Task 4: Width Cap
- `maxTextWidth = 120` — `cappedWidth()` helper limits content width

### Task 5: Wiring
- `ConnectedMsg` → `m.header.SetWorkDir(cwd)`
- `StatusLoadedMsg` → `m.header.SetModel()` + `SetContextPercent()`
- `chat.usage` → `m.header.SetContextPercent()`
- Spacer removed when `m.header.IsCompact()`
