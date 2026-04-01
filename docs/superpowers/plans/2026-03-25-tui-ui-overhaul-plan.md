# Implementation Plan: TUI UI Overhaul

**Spec:** `docs/superpowers/specs/2026-03-25-tui-ui-overhaul-design.md`
**Date:** 2026-03-25

## Task 1: Replace ASCII header asset

**Files:**
- `internal/assets/SMOLBOT.txt` — replace content
- `internal/assets/header.go` — no change needed (embed stays)

**Steps:**
1. Copy `/home/nomadx/SMOLBOT2.txt` content into `internal/assets/SMOLBOT.txt`
2. Verify the embed still works: `go build ./internal/assets/...`

**New header content (4 lines, integrated diagonal fill):**
```
██▀▀▀▀ █▄   ▄█ ██▀▀██ ██     ██▀▀██  ██▀▀██ ▀▀██▀▀   ▄▄   ▄▄   ▄▄   ▄▄
██▄▄▄▄ ███▄███ ██  ██ ██     ██▄▄█▀  ██  ██   ██    ▄█▀  ▄█▀  ▄█▀  ▄█▀
    ██ ██ █ ██ ██  ██ ██     ██  ██  ██  ██   ██   ▄█▀  ▄█▀  ▄█▀  ▄█▀
▀▀▀▀▀▀ ▀▀   ▀▀ ▀▀▀▀▀▀ ▀▀▀▀▀▀ ▀▀▀▀▀▀  ▀▀▀▀▀▀   ▀▀   ▀▀   ▀▀   ▀▀   ▀▀
```

**Validation:** `go build ./internal/...`

---

## Task 2: Left-align header + info line + remove spacer

**Files:**
- `internal/components/header/header.go` — rewrite `View()` for full mode
- `internal/tui/tui.go` — remove spacer in `View()`

**Steps:**

### 2a. Rewrite header `View()` full mode

Replace the center-aligned rendering with left-aligned. Add an info line below the ASCII art showing model, context %, and workdir:

```go
func (m *Model) View() string {
    t := theme.Current()
    if t == nil {
        return "smolbot"
    }
    if m.cached != "" && m.theme == t.Name {
        return m.cached
    }
    m.theme = t.Name

    if m.compact {
        m.cached = m.renderCompact(t)
        return m.cached
    }

    // Left-aligned ASCII art, primary color
    lines := strings.Split(strings.TrimRight(assets.Header, "\n"), "\n")
    var out strings.Builder
    artStyle := lipgloss.NewStyle().Foreground(t.Primary)
    for i, line := range lines {
        out.WriteString(artStyle.Render(line))
        if i < len(lines)-1 {
            out.WriteByte('\n')
        }
    }

    // Info line: model • context% • ~/path
    info := m.renderInfoLine(t)
    if info != "" {
        out.WriteByte('\n')
        out.WriteString(info)
    }

    m.cached = out.String()
    return m.cached
}
```

Add `renderInfoLine` method:

```go
func (m *Model) renderInfoLine(t *theme.Theme) string {
    var parts []string
    sep := lipgloss.NewStyle().Foreground(t.TextMuted).Render(" • ")

    if m.model != "" {
        parts = append(parts, lipgloss.NewStyle().Foreground(t.Secondary).Render(m.model))
    }
    if m.contextPct > 0 {
        pctStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
        if m.contextPct >= 90 {
            pctStyle = lipgloss.NewStyle().Foreground(t.Error).Bold(true)
        } else if m.contextPct >= 60 {
            pctStyle = lipgloss.NewStyle().Foreground(t.Warning)
        }
        parts = append(parts, pctStyle.Render(fmt.Sprintf("%d%%", m.contextPct)))
    }
    if m.workDir != "" {
        parts = append(parts, lipgloss.NewStyle().Foreground(t.TextMuted).Render(trimWorkDir(m.workDir, 4)))
    }
    if len(parts) == 0 {
        return ""
    }
    return " " + strings.Join(parts, sep)
}
```

Update `Height()` to account for info line:

```go
func (m Model) Height() int {
    if m.compact {
        return 1
    }
    artLines := strings.Count(strings.TrimRight(assets.Header, "\n"), "\n") + 1
    return artLines + 1 // +1 for info line
}
```

### 2b. Remove spacer from tui.go View()

In `View()`, remove the `transcriptSpacer` call. The header's info line provides the visual break.

Change:
```go
if !m.header.IsCompact() {
    content = lipgloss.JoinVertical(lipgloss.Left, content, transcriptSpacer(m.width))
}
```
To: remove this entire `if` block.

Delete the `transcriptSpacer` function.

**Validation:** `go build ./internal/...` and `go test ./internal/components/header/...`

---

## Task 3: Remove transcript border + skip compositor when no dialog

**Files:**
- `internal/tui/tui.go` — simplify `transcriptFrameView`, optimize `View()`

**Steps:**

### 3a. Simplify transcriptFrameView

Replace the bordered frame with a minimal pass-through that only adds a scroll hint when needed:

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

### 3b. Skip canvas compositor when no dialog

In `View()`, only use the canvas/compositor when a dialog needs to be overlaid:

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

**Validation:** `go build ./internal/tui/...` and `go test ./internal/tui/...`

---

## Task 4: Fix scroll performance

**Files:**
- `internal/components/chat/messages.go` — fix `HandleKey()` and `View()`

**Steps:**

### 4a. Remove sync() from HandleKey

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

### 4b. Simplify View() — remove expensive Width/Height wrapper

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

**Validation:** `go test ./internal/components/chat/...` — scroll-related tests may need updating.

---

## Task 5: Compact inline tool rendering

**Files:**
- `internal/components/chat/message.go` — rewrite `renderToolCall()`

**Steps:**

Replace the entire `renderToolCall` function with a compact inline format:

```go
func renderToolCall(tc ToolCall, width int, expanded bool) string {
    t := theme.Current()
    if t == nil {
        return tc.Name
    }

    innerWidth := cappedWidth(width)
    indent := "    "

    // Status icon
    icon, iconColor := toolIcon(tc.Status, t)
    iconStr := lipgloss.NewStyle().Foreground(iconColor).Bold(true).Render(icon)

    // Tool name
    nameStr := lipgloss.NewStyle().Foreground(t.ToolName).Bold(true).Render(tc.Name)

    // Truncated params
    params := ""
    if strings.TrimSpace(tc.Input) != "" {
        params = tc.Input
        maxParams := innerWidth - lipgloss.Width(icon) - lipgloss.Width(tc.Name) - 6
        if maxParams > 0 && len(params) > maxParams {
            params = params[:maxParams] + "…"
        }
    }
    paramsStr := ""
    if params != "" {
        paramsStr = "  " + lipgloss.NewStyle().Foreground(t.TextMuted).Render(params)
    }

    header := "  " + iconStr + " " + nameStr + paramsStr

    // Output body (indented, truncated, muted)
    bodyText := tc.Output
    if strings.TrimSpace(bodyText) == "" {
        if tc.Status == "running" {
            bodyText = "running…"
        } else {
            return header
        }
    }

    truncHint := ""
    if !expanded {
        outputLines := strings.Split(bodyText, "\n")
        if len(outputLines) > maxToolOutputLines {
            hidden := len(outputLines) - maxToolOutputLines
            bodyText = strings.Join(outputLines[:maxToolOutputLines], "\n")
            truncHint = fmt.Sprintf("… (%d lines hidden)", hidden)
        }
    }

    bodyStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
    var lines []string
    lines = append(lines, header)
    for _, line := range strings.Split(bodyText, "\n") {
        lines = append(lines, bodyStyle.Render(indent+line))
    }
    if truncHint != "" {
        lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).Italic(true).Render(indent+truncHint))
    }
    return strings.Join(lines, "\n")
}
```

Replace `toolStateTokens` with simpler `toolIcon`:

```go
func toolIcon(status string, t *theme.Theme) (string, color.Color) {
    switch status {
    case "running":
        return "●", t.ToolStateRunning
    case "done":
        return "✓", t.ToolStateDone
    case "error":
        return "✗", t.ToolStateError
    default:
        return "•", t.TextMuted
    }
}
```

Remove the old `toolStateTokens` function.

**Validation:** `go test ./internal/components/chat/...`

---

## Task 6: Lighten message role blocks

**Files:**
- `internal/components/chat/message.go` — modify `renderRoleBlock()`

**Steps:**

Remove `Background(t.Panel)` from the content body in `renderRoleBlock`. Keep the left-border accent and badge header with subtle wash:

```go
func renderRoleBlock(label, body string, accent color.Color, width int) string {
    t := theme.Current()
    if t == nil {
        return label + "\n" + body
    }
    if semanticAccent := transcriptRoleAccent(label, t); semanticAccent != nil {
        accent = semanticAccent
    }
    innerWidth := cappedWidth(width)
    badge := lipgloss.NewStyle().
        Background(accent).
        Foreground(t.Background).
        Bold(true).
        Padding(0, 1).
        Render(label)

    header := lipgloss.NewStyle().
        Background(subtleWash(accent)).
        Width(innerWidth).
        Padding(0, 1).
        Render(badge)
    contentBody := lipgloss.NewStyle().
        Foreground(t.Text).
        Width(innerWidth).
        Padding(0, 1).
        Render(body)
    content := lipgloss.JoinVertical(lipgloss.Left, header, contentBody)
    style := lipgloss.NewStyle().
        Border(lipgloss.ThickBorder(), false, false, false, true).
        BorderForeground(accent).
        Padding(0, 0)
    if width > 4 {
        style = style.Width(width - 2)
    }
    return style.Render(content)
}
```

Key change: removed `Background(t.Panel)` from both `contentBody` and the outer `style`.

Similarly update `renderThinkingBlock` — remove `Background(t.Panel)` from content body.

Similarly update `renderMessageBlock` (used for ERROR) — remove `Background(t.Panel)`.

**Validation:** `go test ./internal/components/chat/...`

---

## Task 7: Update tests

**Files:**
- `internal/components/chat/messages_test.go`
- `internal/components/chat/render_test.go`
- `internal/tui/tui_test.go`
- `internal/components/header/header_test.go`

**Steps:**

1. Update tool rendering tests — assertions now look for inline format (no "TOOL" label, no borders, check for `✓`/`●`/`✗` icons)
2. Update header height tests — 4 art lines + 1 info line = 5 total (was 5 art + 1 spacer = 6)
3. Update transcript layout tests — no border around viewport, adjusted heights
4. Update progress/thinking test — verify no `STREAM` label, ASSISTANT label used for progress
5. Run full test suite: `go test ./internal/... ./pkg/...`

**Validation:** All tests pass.

---

## Task 8: Build, install, verify

**Steps:**
1. `go mod tidy`
2. `go build ./cmd/smolbot && go build ./cmd/smolbot-tui`
3. Stop gateway, install binaries, restart gateway
4. Launch TUI and verify:
   - ASCII header left-aligned with diagonal fill
   - Info line shows model, context %, workdir
   - No border around transcript
   - Tools render as compact inline (icon + name + params, indented output)
   - Scrolling through 20+ messages is smooth with no lag
   - Compact mode (height ≤ 30) still works
   - Dialogs (F1 menu, model picker) still overlay correctly

---

## Execution Order

Tasks 1-6 are independent of each other and can be parallelized. Task 7 depends on 1-6. Task 8 depends on all.

```
[1] Header asset  ─┐
[2] Header code   ─┤
[3] Transcript    ─┤── [7] Update tests ── [8] Build & verify
[4] Scroll perf   ─┤
[5] Tool render   ─┤
[6] Role blocks   ─┘
```
