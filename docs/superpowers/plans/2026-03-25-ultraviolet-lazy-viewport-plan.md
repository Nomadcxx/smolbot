# Implementation Plan: Ultraviolet + Lazy Viewport

**Spec:** `docs/superpowers/specs/2026-03-25-ultraviolet-lazy-viewport-design.md`
**Depends on:** `docs/superpowers/plans/2026-03-25-tui-ui-overhaul-plan.md` (complete first)
**Date:** 2026-03-25

## Phase 1: Create chatlist package (lazy list)

### Task 1: Port list scrolling core

**Files:** `internal/components/chatlist/list.go` (NEW)

Create the index-based lazy list, adapted from Crush's `list.List`:

```go
package chatlist

type List struct {
    width, height int
    items         []Item
    gap           int
    offsetIdx     int
    offsetLine    int
    follow        bool
}
```

Implement core methods:
- `NewList() *List`
- `SetSize(w, h int)`
- `SetGap(gap int)`
- `Len() int`
- `AtBottom() bool`
- `ScrollBy(lines int)` — scroll up/down by N lines, adjusting offsetIdx/offsetLine
- `ScrollToTop()`
- `ScrollToBottom()`
- `Render() string` — iterate from offsetIdx, render only visible items, assemble output

Key behavior: `Render()` calls `item.Render(width)` only for items that fall within the viewport window. Items above `offsetIdx` and items that would start below `offsetIdx + height` are not rendered.

**Validation:** Unit tests in Task 2.

---

### Task 2: Item interface + unit tests

**Files:**
- `internal/components/chatlist/item.go` (NEW)
- `internal/components/chatlist/list_test.go` (NEW)

Define the Item interface:
```go
type Item interface {
    Render(width int) string
}
```

Create a `TextItem` for testing:
```go
type TextItem struct {
    Content string
}
func (t *TextItem) Render(width int) string { return t.Content }
```

Write tests:
- `TestListRenderOnlyVisibleItems` — 20 items, viewport height 5, verify only ~2-3 items rendered
- `TestListScrollBy` — scroll down, verify offsetIdx advances
- `TestListScrollToBottom` — verify last item visible
- `TestListScrollToTop` — verify first item visible
- `TestListAtBottom` — returns true when last item is visible
- `TestListAppendWithFollow` — when follow=true, new items auto-scroll
- `TestListGap` — gap lines rendered between items
- `TestListPartialItem` — item taller than viewport, partial rendering via offsetLine

**Validation:** `go test ./internal/components/chatlist/...`

---

## Phase 2: Convert messages to chat items

### Task 3: Create concrete item types

**Files:** `internal/components/chatlist/items.go` (NEW)

Create item types that wrap existing rendering logic:

```go
// UserItem renders a user message with left-border accent.
type UserItem struct {
    Content string
    cached  string
    width   int
}

// AssistantItem renders an assistant message with markdown.
type AssistantItem struct {
    Content  string
    renderer *glamour.TermRenderer
    cached   string
    width    int
}

// ThinkingItem renders a finalized thinking block with duration.
type ThinkingItem struct {
    Content  string
    Duration time.Duration
    cached   string
    width    int
}

// ToolItem renders a compact inline tool call.
type ToolItem struct {
    ID     string
    Name   string
    Input  string
    Status string
    Output string
    cached string
    width  int
}

// ErrorItem renders an error message.
type ErrorItem struct {
    Content string
    cached  string
    width   int
}

// EphemeralItem renders in-progress content (thinking or streaming).
// Never cached — always re-renders.
type EphemeralItem struct {
    Label   string // "THINKING" or "ASSISTANT"
    Content string
}
```

Each `Render(width int)` method:
1. Check if `width` matches cached width and content unchanged → return cached
2. Otherwise, call the existing `renderRoleBlock()` / `renderToolCall()` / etc.
3. Cache result and width

Move rendering functions from `message.go` into this file or import them.

**Validation:** `go test ./internal/components/chatlist/...` — test each item type renders correctly.

---

### Task 4: Wire chatlist.List into MessagesModel

**Files:**
- `internal/components/chat/messages.go` — major rewrite of internals

Replace the internal data structures:

**Current internals:**
```go
messages       []ChatMessage
tools          []ToolCall
progress       string
thinking       string
viewport       viewport.Model
rendered       string
dirty          bool
```

**New internals:**
```go
list           *chatlist.List
ephThinking    *chatlist.EphemeralItem  // in-progress thinking (nil when inactive)
ephProgress    *chatlist.EphemeralItem  // streaming progress (nil when inactive)
activeTools    []*chatlist.ToolItem     // tools in current run (appended to list on done)
```

Rewrite public methods to manipulate items in the list:

- `AppendUser(content)` → `list.AppendItem(&UserItem{Content: content})`, clear ephemerals
- `AppendAssistant(content)` → remove ephemerals, `list.AppendItem(&AssistantItem{Content: content})`, clear activeTools into list
- `AppendThinking(content)` → `list.AppendItem(&ThinkingItem{Content: content, Duration: dur})`
- `SetThinking(content)` → update `ephThinking` content (or create if nil)
- `AppendProgress(content)` → append to `ephProgress` content
- `StartTool(id, name, input)` → append to `activeTools`
- `FinishTool(id, name, status, output)` → update matching tool in `activeTools`
- `HandleKey(key)` → `list.ScrollBy(±N)` or `list.ScrollToTop()`/`ScrollToBottom()`
- `View()` → `list.Render()`

The list always contains committed items. Ephemeral items (thinking, progress) and active tools are appended to the render output by a render callback or by temporarily appending them during `Render()`.

**Approach for ephemerals:** Register a render callback on the list that appends ephemeral items after the last committed item during rendering. Or simpler: before each `View()`, if ephemerals exist, temporarily append them, render, then remove. Crush uses a similar pattern.

**Validation:** All existing `messages_test.go` and `render_test.go` tests pass with updated internals.

---

## Phase 3: Add Ultraviolet dependency + Drawable interface

### Task 5: Add ultraviolet, define Drawable

**Files:**
- `go.mod` — add dependency
- `internal/tui/drawable.go` (NEW) — Drawable interface

```bash
go get github.com/charmbracelet/ultraviolet@v0.0.0-20260205113103-524a6607adb8
```

Define:
```go
package tui

import uv "github.com/charmbracelet/ultraviolet"

type Drawable interface {
    Draw(scr uv.Screen, area uv.Rectangle)
}
```

**Validation:** `go build ./internal/tui/...`

---

### Task 6: Convert components to Drawable

**Files:**
- `internal/components/header/header.go` — add `Draw()` method
- `internal/components/status/status.go` — add `Draw()` method
- `internal/components/status/footer.go` — add `Draw()` method
- `internal/components/chat/editor.go` — add `Draw()` method
- `internal/components/chat/messages.go` — add `Draw()` method

Each component gets a `Draw(scr uv.Screen, area uv.Rectangle)` method that:
1. Renders to string via existing `View()` method
2. Draws string into screen buffer region via `uv.NewStyledString(view).Draw(scr, area)`

This is the shim approach — keeps existing View() logic working while enabling screen buffer composition. Over time, components can be migrated to draw directly to the buffer.

**Validation:** `go build ./internal/...`

---

### Task 7: Replace tui.View() with screen buffer layout

**Files:**
- `internal/tui/tui.go` — rewrite `View()` and add `computeLayout()`

Add layout computation:
```go
type uiLayout struct {
    header uv.Rectangle
    chat   uv.Rectangle
    status uv.Rectangle
    editor uv.Rectangle
    footer uv.Rectangle
}

func computeLayout(width, height int, headerH, editorH int) uiLayout { ... }
```

Rewrite `View()`:
```go
func (m Model) View() tea.View {
    t := theme.Current()
    if t == nil {
        return tea.NewView("Loading...")
    }

    lay := computeLayout(m.width, m.height, m.header.Height(), m.editor.Height())
    scr := uv.NewScreenBuffer(m.width, m.height)

    m.header.Draw(scr, lay.header)
    m.messages.Draw(scr, lay.chat)
    m.status.Draw(scr, lay.status)
    m.editor.Draw(scr, lay.editor)
    m.footer.Draw(scr, lay.footer)

    if m.dialog != nil {
        dialogView := m.dialog.View()
        dw := lipgloss.Width(dialogView)
        dh := lipgloss.Height(dialogView)
        dx := max(0, (m.width-dw)/2)
        dy := max(0, (m.height-dh)/2)
        uv.NewStyledString(dialogView).Draw(scr, uv.Rect(dx, dy, dw, dh))
    }

    content := scr.Render()
    view := tea.NewView(content)
    view.AltScreen = true
    view.MouseMode = tea.MouseModeCellMotion
    view.BackgroundColor = t.Background
    return view
}
```

Remove:
- `transcriptFrameView()` (already simplified in Plan 1, now replaced entirely)
- `transcriptSpacer()` (already removed in Plan 1)
- `lipgloss.JoinVertical` layout composition
- `lipgloss.NewCanvas` / `lipgloss.NewCompositor`

**Validation:** `go build ./internal/tui/...`, visual test in terminal.

---

### Task 8: Update tests

**Files:**
- `internal/components/chat/messages_test.go`
- `internal/tui/tui_test.go`

Update all tests that:
- Reference `viewport.Model` internals
- Assert on `RoundedBorder` in transcript output
- Check `lipgloss.NewCanvas` / compositor behavior
- Assert on exact rendered output format (may change slightly with screen buffer)

**Validation:** `go test ./internal/... ./pkg/...`

---

### Task 9: Remove bubbles/viewport dependency

**Files:**
- `go.mod` / `go.sum` — remove `charm.land/bubbles/v2/viewport`
- `internal/components/chat/messages.go` — remove viewport import

Run `go mod tidy` to clean up.

**Validation:** `go build ./...` (excluding scripts/), all tests pass.

---

### Task 10: Build, install, end-to-end verify

**Steps:**
1. `go mod tidy && go build ./cmd/...`
2. Stop gateway, install binaries, restart
3. Launch TUI and verify:
   - Scrolling through 50+ messages is instant (no perceptible delay)
   - New messages appear at bottom without re-rendering scroll position
   - Tool output updates only affect the tool item
   - Dialogs overlay correctly
   - Editor input works (cursor positioning, multiline)
   - Compact mode still functions
   - Theme switching works

---

## Execution Order

```
Phase 1:
  [1] List core  ──→ [2] Item + tests

Phase 2:
  [3] Item types ──→ [4] Wire into MessagesModel

Phase 3:
  [5] UV dep + Drawable ──→ [6] Convert components ──→ [7] Rewrite tui.View()

Finalize:
  [8] Update tests ──→ [9] Remove viewport dep ──→ [10] Build & verify
```

Phases 1 and 3-task-5 can run in parallel since they're independent packages.

## Estimated Complexity

| Task | Scope |
|------|-------|
| 1-2: chatlist package | ~300 lines new code + tests |
| 3: Item types | ~200 lines, mostly moving existing render code |
| 4: MessagesModel rewrite | ~150 lines changed (internals swap) |
| 5-6: Ultraviolet + Drawable | ~50 lines new, ~30 lines per component |
| 7: tui.View() rewrite | ~80 lines changed |
| 8-10: Tests + cleanup | Variable |

Total: ~800-1000 lines of code changes across ~12 files.
