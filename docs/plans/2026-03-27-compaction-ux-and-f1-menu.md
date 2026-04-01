# Compaction UX & F1 Menu Overhaul — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make context compaction visible and controllable in the TUI, and expand the F1 menu from a basic launcher into a proper command centre with compaction controls, skills, MCP servers, and provider management.

**Architecture:** Changes span three layers: (1) TUI components for new UI, (2) gateway protocol for new methods/events, (3) agent loop for manual compaction trigger. Tasks are grouped by dependency — compaction UX first, then F1 menu expansion.

**Tech Stack:** Go 1.26, charm.land/bubbletea v2.0.2, charm.land/lipgloss v2.0.2, charm.land/bubbles v2.0.0

---

## Current State

### Compaction
- **Backend**: Full compression engine in `pkg/agent/compression/` with conservative/default/aggressive modes. Auto-triggers at configurable threshold (default 60% of context window). Emits `EventContextCompressed` with original/compressed token counts and reduction %.
- **Gateway**: Forwards `context.compressed` event to TUI with `originalTokens`, `compressedTokens`, `reductionPercent`. Status response includes `contextWindow` in usage.
- **TUI**: Footer renders compression indicator (`↓ X%`) with color coding when compression has occurred. Token usage % shown in footer and header. **No user-triggered compaction. No animation. No warning at critical usage. No `/compact` command.**

### F1 Menu
- **Current structure**: 3-page hierarchy — Root → Themes, Root → Sessions. Root has 8 items: Close, Themes, Sessions, Models, Clear, Status, Help, Quit.
- **Missing**: Skills, MCP servers, providers, compaction controls, settings, keybindings reference.

---

## File Map

| File | Change |
|------|--------|
| `internal/tui/tui.go` | Add `/compact` command handler; add compaction animation state; add critical context warning; add new message types for skills/MCPs |
| `internal/tui/menu_dialog.go` | Expand root menu; add Skills, MCPs, Providers, Compaction, Keybindings pages |
| `internal/components/status/footer.go` | Add compaction progress animation; add critical usage warning indicator |
| `internal/components/dialog/skills.go` | **New** — Skills browser dialog |
| `internal/components/dialog/mcps.go` | **New** — MCP server status dialog |
| `internal/components/dialog/providers.go` | **New** — Provider selector dialog |
| `internal/components/dialog/keybindings.go` | **New** — Keybindings reference dialog |
| `internal/client/messages.go` | Add `Skills()`, `MCPServers()`, `CompactSession()` gateway calls |
| `internal/client/types.go` | Add `SkillInfo`, `MCPServerInfo`, `CompactResult` types |
| `internal/client/protocol.go` | Add payload types for new methods |
| `pkg/gateway/server.go` | Add `skills.list`, `mcps.list`, `compact` protocol methods; add `compact.start`/`compact.done` events |
| `pkg/agent/loop.go` | Add `CompactNow()` method for on-demand compaction |

---

## Part 1: Compaction UX

### Task 1: Add `/compact` Slash Command

**Files:**
- Modify: `internal/tui/tui.go` (command handler)
- Modify: `internal/client/messages.go` (gateway call)
- Modify: `internal/client/types.go` (result type)
- Modify: `pkg/gateway/server.go` (protocol method)
- Modify: `pkg/agent/loop.go` (public compact method)

The user should be able to type `/compact` to manually trigger context compaction at any time, regardless of the auto-compaction threshold.

- [ ] **Step 1: Add `CompactNow()` to agent loop**

In `pkg/agent/loop.go`, add a public method that runs compression on the current session regardless of threshold:

```go
// CompactNow forces context compression for the given session, returning
// the reduction stats. Returns (0, 0, 0, nil) if compression is disabled
// or the session has no history worth compressing.
func (a *AgentLoop) CompactNow(ctx context.Context, sessionKey string) (originalTokens, compressedTokens int, reductionPct float64, err error)
```

Implementation:
1. Load session history from store
2. If fewer than 4 messages, return zero (nothing to compact)
3. Run `compression.Compress()` with current config mode
4. Calculate stats via `TokenTracker`
5. Replace session history with compressed version
6. Emit `EventContextCompressed`
7. Return stats

- [ ] **Step 2: Add `compact` gateway method**

In `pkg/gateway/server.go`, add handler for `"compact"` method:

```go
case "compact":
    params := struct {
        Session string `json:"session"`
    }{}
    if err := json.Unmarshal(req.Params, &params); err != nil {
        return nil, fmt.Errorf("parse compact params: %w", err)
    }
    session := params.Session
    if session == "" {
        session = state.sessionKey
    }
    original, compressed, pct, err := s.agent.CompactNow(ctx, session)
    if err != nil {
        return nil, err
    }
    return map[string]any{
        "originalTokens":   original,
        "compressedTokens": compressed,
        "reductionPercent": pct,
    }, nil
```

Add `"compact"` to the methods list in `"hello"` response.

- [ ] **Step 3: Add TUI gateway client call**

In `internal/client/messages.go`:
```go
func (c *Client) Compact(session string) (*CompactResult, error)
```

In `internal/client/types.go`:
```go
type CompactResult struct {
    OriginalTokens   int     `json:"originalTokens"`
    CompressedTokens int     `json:"compressedTokens"`
    ReductionPercent float64 `json:"reductionPercent"`
}
```

- [ ] **Step 4: Add `/compact` command in TUI**

In `internal/tui/tui.go`, add to the slash command switch:

```go
case "compact", "compress":
    return m, m.compactCmd()
```

Add message types:
```go
type CompactStartMsg struct{}
type CompactDoneMsg struct {
    Original   int
    Compressed int
    Reduction  float64
}
type CompactErrorMsg struct{ Err error }
```

Handler in `Update`:
```go
case CompactDoneMsg:
    m.footer.SetCompaction(false) // stop animation
    m.messages.AppendSystem(fmt.Sprintf(
        "Context compacted: %s → %s (%.0f%% reduction)",
        formatTokens(msg.Original), formatTokens(msg.Compressed), msg.Reduction,
    ))
    return m, m.syncStatusCmd(false) // refresh usage display
```

- [ ] **Step 5: Add to commands dialog**

In `internal/components/dialog/commands.go`, add to command list:
```go
{Name: "/compact", Description: "Compress context to free tokens", Aliases: []string{"/compress"}}
```

- [ ] **Step 6: Write tests**

Test the `/compact` command triggers the gateway call and renders the result message. Test that `CompactNow` on the agent loop actually compresses history.

---

### Task 2: Compaction Progress Animation

**Files:**
- Modify: `internal/components/status/footer.go`
- Modify: `internal/tui/tui.go`

When compaction is running (either auto or manual), show a spinner animation in the footer where the compression indicator normally appears.

- [ ] **Step 1: Add compaction-in-progress state to footer**

In `footer.go`, add field:
```go
type FooterModel struct {
    // ... existing fields
    compacting bool
}

func (m *FooterModel) SetCompacting(v bool) { m.compacting = v }
```

In `renderCompression()`, when `m.compacting` is true, render a cycling spinner: `⣾ ⣽ ⣻ ⢿ ⡿ ⣟ ⣯ ⣷` with label "compacting...". Use a `tea.Tick` command at 80ms intervals to advance the frame.

- [ ] **Step 2: Wire compaction state in TUI**

Set `m.footer.SetCompacting(true)` when `/compact` is executed or when auto-compaction starts (on `context.compressed` event, briefly show the indicator then update with result).

For auto-compaction: emit a `compact.start` event from gateway before compression runs, and `compact.done` after. TUI handles both to show/hide the animation.

- [ ] **Step 3: Write tests**

Test that footer renders spinner text when `compacting=true` and normal indicator when false.

---

### Task 3: Critical Context Usage Warning

**Files:**
- Modify: `internal/components/status/footer.go`
- Modify: `internal/tui/tui.go`

When context usage hits 90%+, show a persistent blinking/pulsing warning in the footer. At 95%+, show a stronger warning suggesting `/compact`.

- [ ] **Step 1: Add warning rendering to footer**

In `renderUsage()`, after the percentage text, when `percentage >= 90`:
```go
if percentage >= 95 {
    warning = lipgloss.NewStyle().Foreground(t.TokenHighUsage).Bold(true).Blink(true).
        Render(" ⚠ /compact")
} else if percentage >= 90 {
    warning = lipgloss.NewStyle().Foreground(t.Warning).Bold(true).
        Render(" ⚠")
}
```

Note: `Blink` support depends on terminal. Fall back to bold red if blink isn't supported.

- [ ] **Step 2: Add system message at threshold**

In the `chat.usage` event handler in `tui.go`, when usage crosses 90% for the first time in a session:
```go
if pct >= 90 && !m.contextWarned {
    m.contextWarned = true
    m.messages.AppendSystem("Context is " + strconv.Itoa(pct) + "% full. Use /compact to free space.")
}
```

Reset `m.contextWarned` on session switch or `/compact`.

- [ ] **Step 3: Write tests**

Test that the warning renders at 90% and 95% thresholds. Test the system message appears once.

---

### Task 4: Compaction Result in Transcript

**Files:**
- Modify: `internal/components/chat/messages.go`
- Modify: `internal/components/chat/message.go`
- Modify: `internal/tui/tui.go`

When auto-compaction fires (via `context.compressed` event), show a compact system message in the transcript instead of silently updating the footer.

- [ ] **Step 1: Add `AppendSystem()` method to messages model**

If it doesn't already exist, add a method to render a system-level notification in the transcript. Use a distinct style — muted text, no role block header, centered or left-aligned with a `─── system ───` divider.

```go
func (m *MessagesModel) AppendSystem(text string) {
    // Render as a subtle, non-role message block
}
```

- [ ] **Step 2: Handle `context.compressed` event with transcript message**

In `tui.go`, the existing handler:
```go
case "context.compressed":
    var p client.CompressionInfo
    _ = json.Unmarshal(msg.Event.Payload, &p)
    m.footer.SetCompression(&p)
```

Add after `SetCompression`:
```go
m.messages.AppendSystem(fmt.Sprintf(
    "Context compacted: %s → %s (%.0f%% reduction)",
    formatTokens(p.OriginalTokens), formatTokens(p.CompressedTokens), p.ReductionPercent,
))
```

- [ ] **Step 3: Write tests**

Test that `context.compressed` events produce a system message in the transcript.

---

## Part 2: F1 Menu Expansion

### Task 5: Add Compaction Controls to F1 Menu

**Files:**
- Modify: `internal/tui/menu_dialog.go`

Add a "Context" submenu to the F1 root page.

- [ ] **Step 1: Add Context page**

```go
menuPageContext menuPage = "context"
```

Items:
- ← Back
- Compact Now → executes `/compact`
- Mode: Conservative → sets compression mode (future: needs gateway method)
- Mode: Default → sets compression mode
- Mode: Aggressive → sets compression mode

Initially only "Compact Now" is functional. Mode switching is a stretch goal requiring a new `compression.setMode` gateway method.

- [ ] **Step 2: Add to root menu**

Insert between "Models" and "Clear Transcript":
```go
{label: "Context & Compaction", page: menuPageContext}
```

- [ ] **Step 3: Write tests**

Test menu navigation to the Context page and that selecting "Compact Now" emits the correct command.

---

### Task 6: Add Skills Browser

**Files:**
- New: `internal/components/dialog/skills.go`
- Modify: `internal/tui/menu_dialog.go`
- Modify: `internal/tui/tui.go`
- Modify: `internal/client/messages.go`
- Modify: `internal/client/types.go`
- Modify: `pkg/gateway/server.go`

Add a filterable dialog showing available agent skills, accessible from F1 menu.

- [ ] **Step 1: Add `skills.list` gateway method**

In `pkg/gateway/server.go`, add handler that queries the skill registry:

```go
case "skills.list":
    skills := s.skillRegistry.List()
    result := make([]map[string]any, len(skills))
    for i, sk := range skills {
        result[i] = map[string]any{
            "name":   sk.Name,
            "status": sk.Status, // "available", "always", "loaded"
        }
    }
    return map[string]any{"skills": result}, nil
```

Add `"skills.list"` to hello methods. This requires the gateway server to hold a reference to the skill registry (or expose it via the agent).

- [ ] **Step 2: Add client types and call**

```go
// internal/client/types.go
type SkillInfo struct {
    Name   string `json:"name"`
    Status string `json:"status"`
}

// internal/client/messages.go
func (c *Client) Skills() ([]SkillInfo, error)
```

- [ ] **Step 3: Build skills dialog**

`internal/components/dialog/skills.go` — follows the same pattern as `sessions.go`/`models.go`:
- Filterable list of skills
- Shows status badge: `[active]` green, `[available]` muted
- Enter on a skill shows its description or loads it (stretch goal)
- For now, read-only browser

- [ ] **Step 4: Wire to F1 menu**

Add "Skills" item to root menu, opening a dialog that loads skills from gateway.

- [ ] **Step 5: Write tests**

Test skills dialog renders, filters, and handles empty state.

---

### Task 7: Add MCP Servers Browser

**Files:**
- New: `internal/components/dialog/mcps.go`
- Modify: `internal/tui/menu_dialog.go`
- Modify: `internal/tui/tui.go`
- Modify: `internal/client/messages.go`
- Modify: `internal/client/types.go`
- Modify: `pkg/gateway/server.go`

Add a dialog showing configured MCP servers and their status.

- [ ] **Step 1: Add `mcps.list` gateway method**

Query the tool registry or config for MCP server definitions:

```go
case "mcps.list":
    servers := s.config.Tools.MCPServers
    result := make([]map[string]any, 0)
    for name, cfg := range servers {
        result = append(result, map[string]any{
            "name":    name,
            "command": cfg.Command,
            "status":  "configured", // or query actual connection status
        })
    }
    return map[string]any{"servers": result}, nil
```

- [ ] **Step 2: Add client types and call**

```go
type MCPServerInfo struct {
    Name    string `json:"name"`
    Command string `json:"command"`
    Status  string `json:"status"`
}

func (c *Client) MCPServers() ([]MCPServerInfo, error)
```

- [ ] **Step 3: Build MCP dialog**

`internal/components/dialog/mcps.go`:
- List of configured servers with status badges
- Status: `[connected]` green, `[configured]` yellow, `[error]` red
- Read-only for now — future: enable/disable, view tools

- [ ] **Step 4: Wire to F1 menu**

Add "MCP Servers" item to root menu.

- [ ] **Step 5: Write tests**

Test dialog renders server list, handles no servers configured.

---

### Task 8: Add Provider/Model Info to F1 Menu

**Files:**
- Modify: `internal/tui/menu_dialog.go`
- Modify: `internal/components/dialog/models.go` (enhance)

The existing models dialog shows model names but not provider context. Enhance it and add a "Providers" submenu.

- [ ] **Step 1: Enhance models dialog**

In `internal/components/dialog/models.go`:
- Group models by provider in the display
- Show provider name as a section header or badge: `[ollama] kimi-k2.5:cloud`
- Highlight current provider

- [ ] **Step 2: Add Providers info page to menu**

A simple info page (not a dialog) showing:
- Current provider: name, API base URL
- Available providers from config
- Current model + context window size

This can be a static render within the menu dialog system rather than a full interactive dialog.

- [ ] **Step 3: Write tests**

Test model grouping and provider display.

---

### Task 9: Add Keybindings Reference

**Files:**
- New: `internal/components/dialog/keybindings.go`
- Modify: `internal/tui/menu_dialog.go`

A read-only dialog showing all available keyboard shortcuts, accessible from F1 → Keybindings.

- [ ] **Step 1: Build keybindings reference**

Static content dialog showing:
```
Global
  F1 / Ctrl+M    Open menu
  Ctrl+C          Stop / Quit
  PgUp/PgDn       Scroll transcript
  Home/End         Top/Bottom of transcript
  Ctrl+L           Jump to latest

Editor
  Enter            Send message
  Shift+Enter      New line
  Up/Down          Input history

Dialogs
  Esc              Close / Back
  ↑/↓ or j/k       Navigate
  Enter             Select
  Type to filter

Commands
  /compact          Compress context
  /session          Switch session
  /model            Change model
  /theme            Change theme
  /status           Show status
  /clear            Clear transcript
  /help             Show help
  /quit             Exit
```

- [ ] **Step 2: Wire to F1 menu**

Add "Keybindings" item to root menu between "Help" and "Quit".

- [ ] **Step 3: Write tests**

Test the dialog renders and can be closed.

---

## Revised F1 Menu Structure

After all tasks, the root menu becomes:

```
┌─── Menu ───────────────────┐
│  Close Menu                │
│  Context & Compaction    → │
│  Skills                  → │
│  MCP Servers             → │
│  ─────────────────────── ─ │
│  Sessions                → │
│  Models & Providers      → │
│  Themes                  → │
│  ─────────────────────── ─ │
│  Clear Transcript          │
│  Status                    │
│  Keybindings             → │
│  Help                      │
│  Quit                      │
└────────────────────────────┘
```

Items with `→` open subpages or dialogs. Separators are visual grouping (not selectable items).

---

## Priority Order

| Priority | Task | Rationale |
|----------|------|-----------|
| P0 | Task 1: `/compact` command | Core functionality, unblocks everything else |
| P0 | Task 4: Compaction transcript message | Users need to see when auto-compaction fires |
| P1 | Task 3: Critical usage warning | Safety — prevents silent context overflow |
| P1 | Task 5: F1 Context submenu | Discoverability for `/compact` |
| P2 | Task 2: Compaction animation | Polish |
| P2 | Task 9: Keybindings reference | Discoverability |
| P2 | Task 6: Skills browser | Informational |
| P2 | Task 7: MCP servers browser | Informational |
| P3 | Task 8: Provider info | Nice to have |

---

## Testing Strategy

- Unit tests for each new component (dialog renders, command parsing)
- Integration test for `/compact` → gateway → agent → response flow
- Footer rendering tests for animation frames and warning thresholds
- Menu navigation tests for new pages (existing pattern in `menu_dialog_test.go`)

## Risks

1. **MCP/Skills gateway methods** require the gateway server to hold references to registries it may not currently have. May need dependency injection changes.
2. **Compression mode switching at runtime** requires the agent loop config to be mutable. Currently config is loaded once at startup. Consider this a stretch goal.
3. **Blink ANSI support** varies by terminal. Use bold red as fallback for warnings.
