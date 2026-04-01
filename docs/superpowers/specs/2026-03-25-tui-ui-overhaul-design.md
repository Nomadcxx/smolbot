# TUI UI Overhaul — Scroll Performance + Compact Tool Rendering

**Date:** 2026-03-25
**Status:** Approved
**Reference:** Crush TUI (charmbracelet/crush) as design reference

## Problem

1. **Scroll lag** — `HandleKey()` calls `sync()` on every scroll event, triggering full re-render of all messages (glamour markdown + lipgloss styling). Additionally, `transcriptFrameView()` wraps the entire viewport in a `RoundedBorder()` on every frame, and `View()` applies expensive `.Width().Height().Render()` on the viewport output.

2. **Tool rendering** — Each tool call renders as a full bordered box with rounded corners, header bar, input section, body, and truncation hint. This consumes excessive vertical space and makes scrolling through tool-heavy transcripts painful.

3. **Header** — ASCII art is center-aligned, wastes space, doesn't show contextual info inline. Spacer line below header wastes a row.

4. **Transcript frame** — `RoundedBorder()` around entire viewport is visually heavy and computationally expensive.

## Design

### 1. Scroll Performance Fix

**File:** `internal/components/chat/messages.go`

Remove `sync(false)` call from `HandleKey()`. The viewport handles offset changes internally. Content only needs re-rendering when dirty (new messages, resize, theme change).

```go
func (m *MessagesModel) HandleKey(key string) {
    switch key {
    case "pgup":
        m.viewport.PageUp()
    case "pgdown":
        m.viewport.PageDown()
    case "home":
        m.viewport.GotoTop()
    case "end", "ctrl+l":
        m.viewport.GotoBottom()
    }
}
```

Remove the `.Width().Height().Render()` wrapper in `View()`:

```go
func (m *MessagesModel) View() string {
    if m.dirty {
        m.sync(m.viewport.AtBottom() || m.viewport.TotalLineCount() == 0)
    }
    if m.width <= 0 || m.height <= 0 {
        return m.rendered
    }
    if strings.TrimSpace(m.rendered) == "" {
        return strings.Repeat("\n", max(0, m.height-1))
    }
    return m.viewport.View()
}
```

### 2. Remove Transcript Frame Border

**File:** `internal/tui/tui.go`

Replace `transcriptFrameView()` with a simple pass-through. No border, no background styling on the viewport content. The role blocks with left-border accents provide enough visual structure.

```go
func transcriptFrameView(content string, width int, hasContentAbove bool) string {
    if !hasContentAbove {
        return content
    }
    t := theme.Current()
    if t == nil {
        return content
    }
    hint := lipgloss.NewStyle().
        Foreground(t.TextMuted).
        Width(max(0, width)).
        Align(lipgloss.Right).
        Render("scroll ↑↓")
    return hint + "\n" + content
}
```

### 3. Header — Left-aligned with diagonal fill

**File:** `internal/components/header/header.go`

Full-mode header: ASCII art left-aligned. Right side filled with diagonal pattern (`╱`) in muted color. Model + context % shown on the line below the art or to the right if terminal is wide enough.

```
 ▄▄▄▄▄▄▄ ▄▄    ▄▄  ▄▄▄▄▄▄  ▄▄       ▄▄▄▄▄▄▄    ▄▄▄▄▄▄  ▄▄▄▄▄▄▄▄ ╱╱╱╱╱╱╱╱╱
██▀▀▀▀▀▀ ███▄▄███ ██▀▀▀▀██ ██       ██▀▀▀▀██  ██▀▀▀▀██ ▀▀▀██▀▀▀ ╱╱╱╱╱╱╱╱╱
▀██████▄ ██▀██▀██ ██    ██ ██       ████████  ██    ██    ██    ╱╱╱╱╱╱╱╱╱
▄▄▄▄▄▄██ ██    ██ ██▄▄▄▄██ ██▄▄▄▄▄▄ ██▄▄▄▄██  ██▄▄▄▄██    ██    ╱╱╱╱╱╱╱╱╱
▀▀▀▀▀▀▀  ▀▀    ▀▀  ▀▀▀▀▀▀  ▀▀▀▀▀▀▀▀ ▀▀▀▀▀▀▀    ▀▀▀▀▀▀     ▀▀    ╱╱╱╱╱╱╱╱╱
 kimi-k2.5:cloud • 15% • ~/Documents/smolbot
```

Remove the spacer line between header and transcript.

Compact mode stays as-is: `smolbot ╱╱╱ model • 45% • ~/path`

### 4. Compact Inline Tool Rendering

**File:** `internal/components/chat/message.go`

Replace the bordered box tool rendering with Crush-style inline format:

```
  ✓ list_dir  path="./src"
    file1.go  file2.go  file3.go

  ● exec  command="go test ./..."
    PASS ok  github.com/smolbot/... 0.342s
    … (15 lines hidden)

  ✗ read_file  path="./missing.go"
    file not found
```

Structure:
- **Line 1:** `  {icon} {name}  {truncated_params}` — icon colored by status, name bold, params muted
- **Lines 2+:** `    {output}` — 4-space indent, muted color, capped at 10 lines
- **Truncation:** `    … (N lines hidden)` in italic muted
- **No borders, no boxes, no background colors**
- **Status icons:** `✓` done (green), `●` running (yellow/animated), `✗` error (red)

### 5. Lighten Message Role Blocks

**File:** `internal/components/chat/message.go`

Keep the left-border accent and badge header for USER, ASSISTANT, THINKING, ERROR blocks. Remove `Background(t.Panel)` from the message body so content renders against the terminal background. Keep the `subtleWash` header background — it's subtle enough to look intentional without being heavy.

### 6. Remove Canvas Compositor (when no dialog)

**File:** `internal/tui/tui.go`

The `lipgloss.NewCanvas` + `lipgloss.NewCompositor` is only needed when a dialog overlay is present. When no dialog, skip the compositor and return the `JoinVertical` content directly:

```go
if m.width > 0 && m.height > 0 {
    if m.dialog != nil {
        canvas := lipgloss.NewCanvas(m.width, m.height)
        layers := []*lipgloss.Layer{
            lipgloss.NewLayer(content).X(0).Y(0),
        }
        dialogView := m.dialog.View()
        layers = append(layers, lipgloss.NewLayer(dialogView).
            X(max(0, (m.width-lipgloss.Width(dialogView))/2)).
            Y(max(0, (m.height-lipgloss.Height(dialogView))/2)))
        canvas.Compose(lipgloss.NewCompositor(layers...))
        content = canvas.Render()
    }
}
```

## Files Changed

| File | Changes |
|------|---------|
| `internal/components/chat/messages.go` | Remove `sync()` from `HandleKey`, simplify `View()` |
| `internal/components/chat/message.go` | Replace `renderToolCall` with compact inline, lighten role blocks |
| `internal/tui/tui.go` | Remove transcript border, skip compositor when no dialog, remove spacer |
| `internal/components/header/header.go` | Left-align ASCII art, add diagonal fill, info line below |
| `internal/components/chat/messages_test.go` | Update tests for new tool format and removed border |
| `internal/tui/tui_test.go` | Update layout tests |

## Success Criteria

- Scrolling up/down through 50+ messages has no perceptible lag
- Tool calls display as compact inline items (2-3 lines typical, not 8+)
- ASCII header is left-aligned with diagonal fill to right edge
- No bordered frame around transcript area
- Existing role blocks (USER/ASSISTANT/THINKING/ERROR) retain left-border accent
- All existing tests pass (updated as needed)

## Future: Ultraviolet Adoption (Separate Plan)

Replace `bubbles/viewport` with index-based list (like Crush's `list.go`) for true lazy rendering — only render visible items. Adopt Ultraviolet screen buffers for layout composition. This is a larger architectural change to be planned and executed separately after this overhaul is stable.
