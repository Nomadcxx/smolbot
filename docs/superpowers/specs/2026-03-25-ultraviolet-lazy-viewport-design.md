# Ultraviolet + Lazy Viewport Adoption

**Date:** 2026-03-25
**Status:** Approved
**Depends on:** `2026-03-25-tui-ui-overhaul-design.md` (must be completed first)
**Reference:** Crush's `internal/ui/list/list.go` and `charmbracelet/ultraviolet`

## Problem

After the UI overhaul (Plan 1), we'll have fixed the worst scroll performance issues (removing `sync()` from scroll, removing transcript border). But the fundamental architecture still has a ceiling:

1. **`bubbles/viewport` renders ALL content then slices** — The viewport receives a single giant string of all rendered messages, then shows a window into it. As conversations grow to 100+ messages, the initial render of all messages (markdown + lipgloss styling) becomes increasingly expensive.

2. **`lipgloss.JoinVertical` + string concatenation for layout** — The entire UI is composed by joining styled strings vertically. This means lipgloss must process ANSI sequences across the full height of the terminal on every frame.

3. **No item-level render caching** — When any single message changes (e.g., tool output updating), all messages are re-rendered.

## Solution

Replace the string-based rendering pipeline with:

1. **Index-based lazy list** — A list component (inspired by Crush's `list.List`) that only renders items visible in the viewport. Scroll operations adjust indices, not string offsets.

2. **Ultraviolet screen buffers** — Cell-based canvas rendering that replaces `lipgloss.JoinVertical` and `lipgloss.NewCanvas` for layout composition. Components draw into rectangle regions of a screen buffer.

3. **Item-level render caching** — Each chat item (message, tool call, thinking block) caches its rendered output. Only items whose content changes get re-rendered.

## Architecture

### Chat Item Interface

Each element in the transcript becomes a `list.Item`:

```go
package chatlist

// Item represents a renderable chat transcript element.
type Item interface {
    // Render returns the string representation for the given width.
    Render(width int) string
}

// CacheableItem optionally provides render caching.
type CacheableItem interface {
    Item
    // Dirty returns true if the item needs re-rendering.
    Dirty() bool
    // ClearDirty marks the item as clean after rendering.
    ClearDirty()
}
```

Concrete item types:
- `UserMessageItem` — user message, rendered via markdown
- `AssistantMessageItem` — assistant response, rendered via glamour
- `ThinkingItem` — finalized thinking block with duration
- `EphemeralThinkingItem` — in-progress thinking (always dirty)
- `EphemeralProgressItem` — streaming assistant content (always dirty)
- `ToolCallItem` — compact inline tool (icon + name + output)
- `ErrorItem` — error message

### Lazy List

Port of Crush's `list.List` pattern, simplified for smolbot's needs:

```go
package chatlist

type List struct {
    width, height int
    items         []Item
    gap           int        // lines between items (1)
    offsetIdx     int        // index of first visible item
    offsetLine    int        // lines scrolled within that item
    follow        bool       // auto-scroll to bottom on new items
}

// Key methods:
func (l *List) Render() string           // only renders visible items
func (l *List) ScrollBy(lines int)       // adjusts offsetIdx/offsetLine
func (l *List) ScrollToBottom()
func (l *List) AtBottom() bool
func (l *List) AppendItem(item Item)
func (l *List) UpdateItem(idx int, item Item)
func (l *List) SetSize(w, h int)
```

The `Render()` method iterates from `offsetIdx` forward, calling `item.Render(width)` only on visible items, and assembles exactly `height` lines of output via string splitting and concatenation. Items scrolled above or below the viewport are never rendered.

### Ultraviolet Screen Buffer

Replace the current `View()` pipeline:

**Current:**
```
header.View() → string
messages.View() → viewport.View() → .Width.Height.Render() → string
transcriptFrameView() → .Border.Render() → string
status.View() → string
editor.View() → string
footer.View() → string
lipgloss.JoinVertical(all...) → string
lipgloss.NewCanvas → compositor → string
tea.NewView(string)
```

**New:**
```
scr := uv.NewScreenBuffer(width, height)
header.Draw(scr, headerRect)
chatlist.Draw(scr, chatRect)     // list.Render() → StyledString.Draw()
status.Draw(scr, statusRect)
editor.Draw(scr, editorRect)
footer.Draw(scr, footerRect)
if dialog != nil {
    dialog.Draw(scr, dialogRect) // overlay
}
tea.NewView(scr.Render())
```

Each component receives a `uv.Screen` and a `uv.Rectangle` defining its region. Components draw their content into the screen buffer at the specified position. The buffer handles clipping automatically.

### Component Draw Interface

```go
type Drawable interface {
    Draw(scr uv.Screen, area uv.Rectangle)
}
```

Each component (header, chat list, status, editor, footer) implements `Drawable`. The main `View()` orchestrates layout by computing rectangles and calling `Draw()` on each component.

### Layout Computation

```go
type layout struct {
    header  uv.Rectangle
    chat    uv.Rectangle
    status  uv.Rectangle
    editor  uv.Rectangle
    footer  uv.Rectangle
}

func computeLayout(width, height int, compact bool, headerHeight, editorHeight int) layout {
    y := 0
    headerH := headerHeight
    statusH := 1
    footerH := 1
    editorH := editorHeight
    chatH := height - headerH - statusH - editorH - footerH

    return layout{
        header: uv.Rect(0, y, width, headerH),
        chat:   uv.Rect(0, y+headerH, width, chatH),
        status: uv.Rect(0, y+headerH+chatH, width, statusH),
        editor: uv.Rect(0, y+headerH+chatH+statusH, width, editorH),
        footer: uv.Rect(0, height-footerH, width, footerH),
    }
}
```

## Migration Strategy

This is a significant architectural change. Migrate incrementally:

### Phase 1: Introduce chatlist package
- Create `internal/components/chatlist/` with `Item` interface, `List` struct
- Port Crush's list.go scrolling logic (ScrollBy, AtBottom, Render)
- Write unit tests for scrolling, rendering, item management

### Phase 2: Convert messages to items
- Create item types (UserMessageItem, AssistantMessageItem, etc.)
- Each item wraps existing rendering logic (renderRoleBlock, renderToolCall, etc.)
- Item renders are cached — only re-render when content changes

### Phase 3: Replace MessagesModel internals
- Swap `bubbles/viewport` + `renderContent()` for `chatlist.List`
- Keep the same public API (`AppendUser`, `AppendAssistant`, `StartTool`, etc.)
- `View()` now calls `list.Render()` directly
- Scroll operations call `list.ScrollBy()` — no re-rendering

### Phase 4: Adopt Ultraviolet screen buffers
- Add `ultraviolet` dependency
- Create `Drawable` interface
- Convert header, status, editor, footer to `Draw()` method
- Replace `lipgloss.JoinVertical` layout with rectangle-based drawing
- Remove `lipgloss.NewCanvas` / compositor

### Phase 5: Dialog overlay via screen buffer
- Dialogs draw directly onto the screen buffer at computed position
- No need for separate compositor layer

## Files Changed

| File | Change |
|------|--------|
| `internal/components/chatlist/list.go` | **NEW** — Lazy list with index-based scrolling |
| `internal/components/chatlist/item.go` | **NEW** — Item interface and concrete types |
| `internal/components/chatlist/list_test.go` | **NEW** — Scroll, render, item management tests |
| `internal/components/chat/messages.go` | Replace viewport with chatlist.List |
| `internal/components/chat/message.go` | Move render functions into item types |
| `internal/components/header/header.go` | Add `Draw(scr, area)` method |
| `internal/components/status/*.go` | Add `Draw(scr, area)` methods |
| `internal/components/chat/editor.go` | Add `Draw(scr, area)` method |
| `internal/tui/tui.go` | Replace JoinVertical + Canvas with screen buffer layout |
| `go.mod` | Add `charmbracelet/ultraviolet` dependency |

## Dependencies

- `github.com/charmbracelet/ultraviolet` — Screen buffer, cell-based rendering
- Existing: `charm.land/bubbletea/v2`, `charm.land/lipgloss/v2`, `charm.land/glamour/v2`

Note: `bubbles/viewport` can be removed from go.mod after Phase 3 is complete.

## Success Criteria

- Scrolling through 200+ messages has constant-time performance (no degradation with transcript length)
- Adding a new message to a long transcript does not re-render existing messages
- Tool output updates only re-render the affected tool item
- Layout is pixel-identical to post-overhaul Plan 1 output
- All existing tests pass (updated as needed)
- No new dependencies beyond `ultraviolet`

## Risks

- **Ultraviolet is pre-1.0** (v0.0.0 commit hash) — API may change. Pin to specific commit.
- **Bubble Tea integration** — `tea.View` still expects a string. The screen buffer's `Render()` method produces this, but cursor positioning for the editor textarea requires care.
- **Glamour markdown rendering** — Items containing markdown still need glamour, which is the most expensive render step. Caching at the item level mitigates this.
- **Editor component** — The `bubbles/textarea` used by the editor returns a string from `View()`. We'll need a shim to draw that string into the screen buffer.
