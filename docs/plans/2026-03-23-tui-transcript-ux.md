# TUI Transcript UX Fixes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix 9 TUI issues: thinking never visible, wrong transcript theme colors, no mouse wheel scroll, esc key conflict, "complete" sentinel bug, tool input not displayed, dead theme/overlay code, `/theme` command broken, and session context display missing.

**Architecture:** All changes are confined to the TUI layer — `internal/components/chat/`, `internal/tui/`, `internal/theme/`. The gateway and agent are unchanged. Tasks are independent and safe to implement in any order, though Task 3 (thinking persistence) should be done before Task 2 (theming) since it adds the rendering path the color fix targets.

**Tech Stack:** Go 1.26, charm.land/bubbletea v2.0.2, charm.land/lipgloss v2.0.2, charm.land/bubbles v2.0.0 (viewport)

---

## File Map

| File | Change |
|------|--------|
| `internal/tui/tui.go` | Remove `esc` from scroll keys; add `tea.MouseWheelMsg` handler; add `view.MouseMode`; call `AppendThinking` instead of `SetThinking`; fix `/theme` dirty+error; add context display |
| `internal/components/chat/messages.go` | Add `AppendThinking()`; fix `SetThinking` sentinel; use `TranscriptStreaming`/`TranscriptThinking` colors; forward `Update` for mouse |
| `internal/components/chat/message.go` | Add `Input` to `ToolCall`; render input in tool block; add THINKING to `transcriptRoleAccent` |
| `internal/theme/theme.go` | Remove unused `Surface` and `Subtle` fields |
| `internal/tui/tui_test.go` | Remove tests for dead `overlayView`/`overlayScrimView` functions |
| `internal/tui/tui.go` | Remove `overlayView`, `overlayScrimView`, `overlayScrimLine`, `overlayPanelView` dead functions |
| `internal/components/chat/messages_test.go` | Add/update tests for thinking role, theming, tool input |
| `internal/tui/tui_test.go` | Add mouse wheel and esc-key tests |

---

## Task 1: Fix esc Key — No Longer Scrolls Transcript to Bottom

**Files:**
- Modify: `internal/tui/tui.go:420`
- Modify: `internal/tui/tui_test.go` (add test)

The `esc` key is incorrectly wired to `GotoBottom()` on the transcript viewport. It should be reserved for closing dialogs, not navigation. Remove it from the scroll key list; `ctrl+l` and `end` already handle jump-to-bottom.

- [ ] **Step 1: Write the failing test**

Add to `internal/tui/tui_test.go` (also ensure `"strconv"` is in the imports):

```go
func TestEscKeyDoesNotScrollTranscript(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}
	model := New(app.Config{})
	model.width = 80
	model.height = 24
	model.messages.SetSize(78, 10)
	// Add enough messages to be scrollable
	for i := 0; i < 20; i++ {
		model.messages.AppendAssistant("line " + strconv.Itoa(i))
	}
	// Scroll up first
	model.messages.HandleKey("home")
	offsetBefore := model.messages.ViewportOffset()

	// Send esc — should NOT jump to bottom
	next, _ := model.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEsc}))
	got := next.(Model)
	offsetAfter := got.messages.ViewportOffset()

	if offsetAfter != offsetBefore {
		t.Fatalf("esc should not change scroll position: before=%d after=%d", offsetBefore, offsetAfter)
	}
}
```

- [ ] **Step 2: Add `ViewportOffset()` to `MessagesModel`** (needed by test)

In `internal/components/chat/messages.go`, add:

```go
func (m *MessagesModel) ViewportOffset() int {
	return m.viewport.YOffset()
}
```

- [ ] **Step 3: Run test to confirm it fails**

```bash
cd /home/nomadx/Documents/smolbot
go test ./internal/tui/... -run TestEscKeyDoesNotScrollTranscript -v
```

Expected: FAIL (esc currently calls GotoBottom)

- [ ] **Step 4: Remove `esc` from the scroll key case**

In `internal/tui/tui.go:420`, change:

```go
// old
case "pgup", "pgdown", "home", "end", "esc", "ctrl+l":
```

to:

```go
// new
case "pgup", "pgdown", "home", "end", "ctrl+l":
```

- [ ] **Step 5: Run test to confirm it passes**

```bash
go test ./internal/tui/... -run TestEscKeyDoesNotScrollTranscript -v
```

Expected: PASS

- [ ] **Step 6: Run full suite to check for regressions**

```bash
go test ./... 2>&1 | tail -20
```

Expected: no new failures

- [ ] **Step 7: Commit**

```bash
git add internal/tui/tui.go internal/tui/tui_test.go internal/components/chat/messages.go
git commit -m "fix(tui): remove esc from viewport scroll keys — reserved for dialog close"
```

---

## Task 2: Persist Thinking in Conversation History

**Files:**
- Modify: `internal/components/chat/messages.go` — add `AppendThinking()`, update `renderContent`
- Modify: `internal/components/chat/message.go` — add THINKING to `transcriptRoleAccent`
- Modify: `internal/tui/tui.go:264-266` — call `AppendThinking` instead of `SetThinking`
- Modify: `internal/components/chat/messages_test.go` — add persistence test

**Root cause:** `ThinkingDoneMsg` calls `SetThinking()` which stores content in `m.thinking` (transient). Milliseconds later, `ChatDoneMsg` calls `AppendAssistant()` which clears `m.thinking`. The user never sees it. Fix: persist as a `ChatMessage{Role: "thinking"}`.

- [ ] **Step 1: Write the failing test**

Add to `internal/components/chat/messages_test.go`:

```go
func TestThinkingContentPersistsAfterAssistantResponse(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}
	model := NewMessages()
	model.SetSize(80, 40)
	model.AppendUser("what is 2+2?")
	model.AppendThinking("Let me reason through this: 2+2 equals 4.")
	model.AppendAssistant("The answer is 4.")

	view := model.View()
	if !strings.Contains(view, "THINKING") {
		t.Fatalf("expected THINKING label in view after assistant responded, got %q", view)
	}
	if !strings.Contains(view, "Let me reason through this") {
		t.Fatalf("expected thinking content to persist in view, got %q", view)
	}
	if !strings.Contains(view, "The answer is 4.") {
		t.Fatalf("expected assistant response also visible, got %q", view)
	}
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
go test ./internal/components/chat/... -run TestThinkingContentPersistsAfterAssistantResponse -v
```

Expected: FAIL — `AppendThinking` undefined

- [ ] **Step 3: Add `AppendThinking` to `MessagesModel`**

In `internal/components/chat/messages.go`, add after `AppendError`:

```go
func (m *MessagesModel) AppendThinking(content string) {
	if strings.TrimSpace(content) == "" {
		return
	}
	m.messages = append(m.messages, ChatMessage{Role: "thinking", Content: content})
	m.sync(m.viewport.AtBottom())
}
```

Also fix `SetThinking` to remove the "complete" sentinel (replace the method body):

```go
func (m *MessagesModel) SetThinking(content string) {
	m.thinking = content
	m.sync(m.viewport.AtBottom())
}
```

- [ ] **Step 4: Add THINKING rendering in `renderContent`**

In `internal/components/chat/messages.go`, in `renderContent()`, add a `case "thinking":` branch inside the message loop:

```go
for _, msg := range m.messages {
    switch msg.Role {
    case "user":
        lines = append(lines, renderRoleBlock("USER", msg.Content, t.Primary, m.width))
    case "assistant":
        lines = append(lines, renderRoleBlock("ASSISTANT", m.renderAssistant(msg.Content), t.Secondary, m.width))
    case "thinking":
        lines = append(lines, renderRoleBlock("THINKING", msg.Content, t.TranscriptThinking, m.width))
    case "error":
        lines = append(lines, renderMessageBlock("ERROR", msg.Content, t.Error, m.width))
    }
    lines = append(lines, "")
}
```

- [ ] **Step 5: Add THINKING to `transcriptRoleAccent`**

In `internal/components/chat/message.go`, update `transcriptRoleAccent`:

```go
func transcriptRoleAccent(label string, t *theme.Theme) color.Color {
	switch label {
	case "USER":
		return t.TranscriptUserAccent
	case "ASSISTANT":
		return t.TranscriptAssistantAccent
	case "THINKING":
		return t.TranscriptThinking
	default:
		return nil
	}
}
```

- [ ] **Step 6: Wire up in `tui.go` — call `AppendThinking` on `ThinkingDoneMsg`**

In `internal/tui/tui.go:264-266`, change:

```go
// old
case ThinkingDoneMsg:
    m.messages.SetThinking(msg.Content)
    return m, nil
```

to:

```go
// new
case ThinkingDoneMsg:
    m.messages.AppendThinking(msg.Content)
    return m, nil
```

- [ ] **Step 7: Run tests**

```bash
go test ./internal/components/chat/... -run TestThinkingContentPersistsAfterAssistantResponse -v
go test ./internal/components/chat/... -v 2>&1 | tail -30
```

Expected: new test PASS, no regressions

- [ ] **Step 8: Commit**

```bash
git add internal/components/chat/messages.go internal/components/chat/message.go internal/tui/tui.go internal/components/chat/messages_test.go
git commit -m "fix(tui): persist thinking content in transcript history instead of transient state"
```

---

## Task 3: Fix Transcript Theming — Use TranscriptStreaming and TranscriptThinking Colors

**Files:**
- Modify: `internal/components/chat/messages.go:162-167`
- Modify: `internal/components/chat/messages_test.go` (add color assertion test)

The `STREAM` block uses `t.Info` and `THINKING` (transient, not the persisted one) uses `t.TextMuted`. Both have dedicated semantic colors in the `Theme` struct that are unused. Wire them in.

Note: After Task 2, the persisted THINKING messages are rendered via `renderRoleBlock` which uses `t.TranscriptThinking` (already wired in Task 2's `transcriptRoleAccent`). This task fixes the **transient** `m.thinking` and `m.progress` labels.

- [ ] **Step 1: Write the failing test**

Add to `internal/components/chat/messages_test.go`:

```go
func TestProgressAndThinkingBlocksUseSemanticTranscriptColors(t *testing.T) {
	const themeName = "transcript-color-test"
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}
	base := *theme.Current()
	base.Name = themeName
	base.TranscriptStreaming = lipgloss.Color("#FF0099")
	base.TranscriptThinking = lipgloss.Color("#00FF88")
	theme.Register(&base)
	if !theme.Set(themeName) {
		t.Fatalf("could not set test theme %q", themeName)
	}
	t.Cleanup(func() { theme.Set("nord") })

	model := NewMessages()
	model.SetSize(80, 20)
	model.AppendUser("go")
	model.SetProgress("streaming text...")
	model.SetThinking("reasoning...")

	view := model.View()
	if !strings.Contains(view, ansiFg("#FF0099")) {
		t.Fatalf("STREAM block should use TranscriptStreaming color #FF0099, got %q", view)
	}
	if !strings.Contains(view, ansiFg("#00FF88")) {
		t.Fatalf("THINKING block should use TranscriptThinking color #00FF88, got %q", view)
	}
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
go test ./internal/components/chat/... -run TestProgressAndThinkingBlocksUseSemanticTranscriptColors -v
```

Expected: FAIL

- [ ] **Step 3: Fix the colors in `renderContent()`**

In `internal/components/chat/messages.go`, change:

```go
// old
if m.progress != "" {
    lines = append(lines, renderMessageBlock("STREAM", m.progress, t.Info, m.width))
}
if m.thinking != "" {
    lines = append(lines, renderMessageBlock("THINKING", m.thinking, t.TextMuted, m.width))
}
```

to:

```go
// new
if m.progress != "" {
    lines = append(lines, renderMessageBlock("STREAM", m.progress, t.TranscriptStreaming, m.width))
}
if m.thinking != "" {
    lines = append(lines, renderMessageBlock("THINKING", m.thinking, t.TranscriptThinking, m.width))
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/components/chat/... -run TestProgressAndThinkingBlocksUseSemanticTranscriptColors -v
go test ./internal/components/chat/... -v 2>&1 | tail -30
```

Expected: new test PASS, no regressions

- [ ] **Step 5: Commit**

```bash
git add internal/components/chat/messages.go internal/components/chat/messages_test.go
git commit -m "fix(tui): use TranscriptStreaming/TranscriptThinking theme colors for STREAM and THINKING blocks"
```

---

## Task 4: Enable Mouse Wheel Scrolling

**Files:**
- Modify: `internal/tui/tui.go` — add `tea.MouseWheelMsg` case in `Update`, add `MouseMode` in `View`
- Modify: `internal/tui/tui_test.go` — add mouse wheel test

BubbleTea v2 dispatches `tea.MouseWheelMsg` but mouse mode must be enabled per-view via `view.MouseMode`. The `tui.go:View()` already returns a `tea.View` — just add `view.MouseMode = tea.MouseModeCellMotion`. Then handle the wheel events in `Update`.

- [ ] **Step 1: Write the failing test**

Add to `internal/tui/tui_test.go` (ensure `"strconv"` is in the imports):

```go
func TestMouseWheelScrollsTranscript(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}
	model := New(app.Config{})
	model.width = 80
	model.height = 24
	model.messages.SetSize(78, 10)
	for i := 0; i < 30; i++ {
		model.messages.AppendAssistant("message line " + strconv.Itoa(i))
	}
	// Start at bottom (default after appends)
	model.messages.HandleKey("end")
	bottomOffset := model.messages.ViewportOffset()

	// Wheel up should scroll toward top
	next, _ := model.Update(tea.MouseWheelMsg{Button: tea.MouseWheelUp})
	got := next.(Model)
	if got.messages.ViewportOffset() >= bottomOffset {
		t.Fatalf("mouse wheel up should reduce scroll offset: before=%d after=%d", bottomOffset, got.messages.ViewportOffset())
	}

	// Wheel down should scroll back toward bottom
	next2, _ := got.Update(tea.MouseWheelMsg{Button: tea.MouseWheelDown})
	got2 := next2.(Model)
	if got2.messages.ViewportOffset() <= got.messages.ViewportOffset() {
		t.Fatalf("mouse wheel down should increase scroll offset: before=%d after=%d", got.messages.ViewportOffset(), got2.messages.ViewportOffset())
	}
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
go test ./internal/tui/... -run TestMouseWheelScrollsTranscript -v
```

Expected: FAIL — mouse wheel not handled

- [ ] **Step 3: Add `tea.MouseWheelMsg` handler in `tui.go:Update()`**

In `internal/tui/tui.go`, in the `Update` switch, add BEFORE the `tea.KeyMsg` case:

```go
case tea.MouseWheelMsg:
    switch msg.Button {
    case tea.MouseWheelUp:
        m.messages.HandleKey("pgup")
    case tea.MouseWheelDown:
        m.messages.HandleKey("pgdown")
    }
    return m, nil
```

- [ ] **Step 4: Enable mouse mode in `View()`**

In `internal/tui/tui.go:View()`, after `view.AltScreen = true`, add:

```go
view.MouseMode = tea.MouseModeCellMotion
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/tui/... -run TestMouseWheelScrollsTranscript -v
go test ./internal/tui/... -v 2>&1 | tail -30
```

Expected: new test PASS, no regressions

- [ ] **Step 6: Commit**

```bash
git add internal/tui/tui.go internal/tui/tui_test.go
git commit -m "feat(tui): enable mouse wheel scrolling for transcript viewport"
```

---

## Task 5: Show Tool Input in Tool Call Blocks

**Files:**
- Modify: `internal/components/chat/message.go` — add `Input` to `ToolCall`, render it
- Modify: `internal/components/chat/messages.go` — store input in `StartTool`
- Modify: `internal/components/chat/messages_test.go` — add test

Currently `StartTool(name, _ string)` discards the input argument. Users can't see what parameters were passed to a tool.

- [ ] **Step 1: Write the failing test**

Add to `internal/components/chat/messages_test.go`:

```go
func TestToolInputIsDisplayedInToolBlock(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}
	model := NewMessages()
	model.SetSize(80, 30)
	model.AppendUser("read the config")
	model.StartTool("read_file", `{"path": "/etc/smolbot.yaml"}`)
	model.FinishTool("read_file", "done", "config loaded")

	view := model.View()
	if !strings.Contains(view, `"path"`) {
		t.Fatalf("expected tool input to appear in view, got %q", view)
	}
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
go test ./internal/components/chat/... -run TestToolInputIsDisplayedInToolBlock -v
```

Expected: FAIL

- [ ] **Step 3: Add `Input` field to `ToolCall`**

In `internal/components/chat/message.go`, update the struct:

```go
type ToolCall struct {
	Name   string
	Input  string
	Status string
	Output string
}
```

- [ ] **Step 4: Update `StartTool` to store input**

In `internal/components/chat/messages.go:96`, change:

```go
// old
func (m *MessagesModel) StartTool(name, _ string) {
    m.tools = append(m.tools, ToolCall{Name: name, Status: "running"})
```

to:

```go
// new
func (m *MessagesModel) StartTool(name, input string) {
    m.tools = append(m.tools, ToolCall{Name: name, Input: input, Status: "running"})
```

- [ ] **Step 5: Render input in `renderToolCall`**

In `internal/components/chat/message.go`, update `renderToolCall`. Add input rendering between `header` and `body`:

```go
// After the header is built, before body:
var rows []string
rows = append(rows, header)
if strings.TrimSpace(tc.Input) != "" {
    inputText := lipgloss.NewStyle().
        Background(t.ToolArtifactBody).
        Foreground(t.TextMuted).
        Width(innerWidth).
        Padding(0, 1).
        Render(tc.Input)
    rows = append(rows, inputText)
}
rows = append(rows, body)
content := lipgloss.JoinVertical(lipgloss.Left, rows...)
```

(Replace the existing `content := lipgloss.JoinVertical(lipgloss.Left, header, body)` line.)

- [ ] **Step 6: Run tests**

```bash
go test ./internal/components/chat/... -run TestToolInputIsDisplayedInToolBlock -v
go test ./internal/components/chat/... -v 2>&1 | tail -30
```

Expected: new test PASS, no regressions

- [ ] **Step 7: Commit**

```bash
git add internal/components/chat/message.go internal/components/chat/messages.go internal/components/chat/messages_test.go
git commit -m "feat(tui): display tool input arguments in tool call blocks"
```

---

## Task 6: Remove Dead Code — Surface/Subtle Fields and Overlay Functions

**Files:**
- Modify: `internal/theme/theme.go` — remove `Surface` and `Subtle` fields
- Modify: `internal/tui/tui.go` — remove `overlayView`, `overlayScrimView`, `overlayScrimLine`, `overlayPanelView`
- Modify: `internal/tui/tui_test.go` — remove tests for dead overlay functions

These fields/functions are defined but never used in the live render path. Removing them reduces confusion about how the UI is built.

**Before touching anything, verify nothing uses Surface or Subtle:**

```bash
grep -rn "\.Surface\b\|\.Subtle\b" /home/nomadx/Documents/smolbot/internal/ --include="*.go" | grep -v "_test.go"
```

Expected: no matches outside of the theme.go definition itself.

**Verify overlay functions are not called in production code:**

```bash
grep -rn "overlayView\|overlayScrimView\|overlayPanelView\|overlayScrimLine" \
  /home/nomadx/Documents/smolbot/internal/ --include="*.go" | grep -v "_test.go"
```

Expected: matches only in `tui.go` (definitions) — no callers.

- [ ] **Step 1: Remove `Surface` and `Subtle` from `theme.go`**

In `internal/theme/theme.go`, delete these two lines:

```go
Surface     color.Color   // DELETE
Subtle      color.Color   // DELETE
```

- [ ] **Step 2: Remove overlay dead functions from `tui.go`**

In `internal/tui/tui.go`, delete the following four functions entirely (lines ~631-687):

- `overlayPanelView(content string) string`
- `overlayScrimView(width, height int) string`
- `overlayView(content string, maxWidth, maxHeight int) string`
- `overlayScrimLine(width int) string`

- [ ] **Step 3: Remove the dead-code tests in `tui_test.go`**

In `internal/tui/tui_test.go`, delete only:
- `TestOverlayUsesPanelSurface` (line ~911, calls `overlayPanelView` — now deleted)
- `TestOverlayAddsScrimWithoutForcingRootBackground` (line ~928, calls `overlayScrimView` — now deleted)

**Do NOT delete `TestEditorUsesThemeSurface`** — despite its name it tests editor background rendering (`"48;2;0;0;0"`), which is independent of the `Surface` field. It remains valid.

- [ ] **Step 4: Compile check**

```bash
go build ./...
```

Expected: compiles with no errors

- [ ] **Step 5: Run full tests**

```bash
go test ./... 2>&1 | tail -20
```

Expected: no failures (previously passing tests still pass)

- [ ] **Step 6: Commit**

```bash
git add internal/theme/theme.go internal/tui/tui.go internal/tui/tui_test.go
git commit -m "chore(tui): remove unused Surface/Subtle theme fields and dead overlay functions"
```

---

## Task 7: Add Scroll Position Indicator in Transcript

**Files:**
- Modify: `internal/components/chat/messages.go` — expose `HasContentAbove() bool`
- Modify: `internal/tui/tui.go:transcriptFrameView` — add scroll hint in border title

When the user has scrolled up (not at bottom), show a subtle hint that there is content above and below. This makes scrollability discoverable.

- [ ] **Step 1: Write failing test**

Add to `internal/components/chat/messages_test.go`:

```go
func TestHasContentAboveReflectsScrollPosition(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}
	model := NewMessages()
	model.SetSize(80, 10)
	for i := 0; i < 30; i++ {
		model.AppendAssistant("message " + strconv.Itoa(i))
	}
	// At bottom (default after appends) — no content above that we haven't seen
	// After pgup, there IS content above
	model.HandleKey("home")
	if !model.HasContentAbove() {
		t.Fatal("expected HasContentAbove() = true after scrolling to top")
	}
	model.HandleKey("end")
	if model.HasContentAbove() {
		t.Fatal("expected HasContentAbove() = false at bottom")
	}
}
```

- [ ] **Step 2: Run to confirm failure**

```bash
go test ./internal/components/chat/... -run TestHasContentAboveReflectsScrollPosition -v
```

Expected: FAIL — `HasContentAbove` undefined

- [ ] **Step 3: Add `HasContentAbove` to `MessagesModel`**

In `internal/components/chat/messages.go`, add:

```go
func (m *MessagesModel) HasContentAbove() bool {
	return m.viewport.YOffset() > 0
}
```

- [ ] **Step 4: Show scroll hint in transcript frame**

In `internal/tui/tui.go`, update `transcriptFrameView` to accept a `hasContentAbove bool` param and add a hint when true.

**Important:** The first line of `rendered` contains ANSI escape codes. Do NOT byte-slice it using visual-width indices — that will corrupt the escape sequences. Instead, render the hint as a separate text line above the frame, or use `ansi.Truncate` from `github.com/charmbracelet/x/ansi` (already a transitive dep via lipgloss).

The simplest correct approach is to prepend a hint line above the frame rather than modifying the border:

```go
func transcriptFrameView(content string, width int, hasContentAbove bool) string {
	t := theme.Current()
	if t == nil {
		return content
	}
	style := lipgloss.NewStyle().
		Background(t.Panel).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Border)
	if width > 2 {
		style = style.Width(width - 2)
	}
	frame := style.Render(content)
	if !hasContentAbove {
		return frame
	}
	hint := lipgloss.NewStyle().
		Foreground(t.TextMuted).
		Width(max(0, width-2)).
		Align(lipgloss.Right).
		Render("↑ PgUp/PgDn")
	return lipgloss.JoinVertical(lipgloss.Left, hint, frame)
}
```

**Note:** This prepends one extra line above the frame. Adjust the height budget in `tui.go:WindowSizeMsg` accordingly — subtract 1 more from `chatH` when `hasContentAbove` is true, OR accept the minor layout shift (the hint line displaces one row of chat content). The simplest approach for the plan is to accept the shift; the space math can be tightened in a follow-up.

**Also add import:** `ansi "github.com/charmbracelet/x/ansi"` is available if a future implementation needs ANSI-aware string manipulation in this file.

Update the call site in `tui.go:View()`:

```go
// old
transcriptFrameView(m.messages.View(), m.width),
// new
transcriptFrameView(m.messages.View(), m.width, m.messages.HasContentAbove()),
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/components/chat/... -run TestHasContentAboveReflectsScrollPosition -v
go test ./... 2>&1 | tail -20
```

Expected: new test PASS, no regressions

- [ ] **Step 6: Commit**

```bash
git add internal/components/chat/messages.go internal/tui/tui.go internal/components/chat/messages_test.go
git commit -m "feat(tui): show scroll position hint when transcript has content above viewport"
```

---

## Task 8: Fix /theme Slash Command — Silent Failure and Stale Render

**Files:**
- Modify: `internal/tui/tui.go:510-519` — add error on unknown theme, set messages dirty on success

Two bugs: (1) unknown theme name fails silently, (2) existing transcript doesn't re-render with new theme colors because `m.messages.dirty` is never set.

- [ ] **Step 1: Add `InvalidateTheme()` to `MessagesModel`**

In `internal/components/chat/messages.go`, add:

```go
func (m *MessagesModel) InvalidateTheme() {
	m.renderer = nil // force renderer cache bust
	m.dirty = true
}
```

- [ ] **Step 2: Fix the `/theme` case in `tui.go`**

```go
// old
case "/theme":
    if args == "" {
        m.dialog = newThemesMenuDialog()
        return m, nil
    }
    if theme.Set(args) {
        m.app.Theme = args
        m.header = header.New()
        return m, m.persistStateCmd()
    }

// new
case "/theme":
    if args == "" {
        m.dialog = newThemesMenuDialog()
        return m, nil
    }
    if theme.Set(args) {
        m.app.Theme = args
        m.header = header.New()
        m.messages.InvalidateTheme()
        return m, m.persistStateCmd()
    }
    m.messages.AppendError("Unknown theme: " + args + ". Available: " + strings.Join(theme.List(), ", "))
```

- [ ] **Step 3: Write a test**

Add to `internal/tui/tui_test.go`:

```go
func TestThemeCommandShowsErrorOnUnknownTheme(t *testing.T) {
	model := New(app.Config{})
	model.width = 80
	model.height = 24
	got, _ := model.handleSlashCommand("/theme totally-not-a-real-theme")
	view := got.(Model).messages.View()
	if !strings.Contains(view, "Unknown theme") {
		t.Fatalf("expected error message for unknown theme, got %q", view)
	}
}

func TestThemeCommandInvalidatesMessageRender(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}
	model := New(app.Config{})
	model.width = 80
	model.height = 24
	model.messages.SetSize(78, 10)
	model.messages.AppendAssistant("hello")
	// Theme change should set dirty = true on messages
	got, _ := model.handleSlashCommand("/theme dracula")
	if !got.(Model).messages.IsDirty() {
		t.Fatal("expected messages to be dirty after theme change")
	}
}
```

Also add `IsDirty()` to `MessagesModel` in `messages.go`:

```go
func (m *MessagesModel) IsDirty() bool {
	return m.dirty
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/tui/... -run TestThemeCommand -v
go test ./... 2>&1 | tail -20
```

Expected: both tests PASS, no regressions

- [ ] **Step 5: Commit**

```bash
git add internal/tui/tui.go internal/tui/tui_test.go internal/components/chat/messages.go
git commit -m "fix(tui): /theme command now invalidates render cache and shows error on unknown theme"
```

---

## Task 9: Show Session Context in Footer / Transcript

**Files:**
- Modify: `internal/components/status/footer.go` — verify context display is visible
- Modify: `internal/tui/tui.go` — ensure `syncStatusCmd` is called after chat completes

The footer already has `SetUsage()` and renders `X% (tokens/context)`. The question is whether it's visible and populated correctly. This task is investigative first.

- [ ] **Step 1: Verify footer renders usage**

Read `internal/components/status/footer.go` and check the layout renders token usage on the right side. Run:

```bash
go test ./internal/components/status/... -v 2>&1 | tail -30
```

- [ ] **Step 2: Check `syncStatusCmd` is called after `ChatDoneMsg`**

In `internal/tui/tui.go:255-260`, `ChatDoneMsg` handler calls `m.syncStatusCmd(false)`. This refreshes token usage after each response. Verify this is wired correctly.

- [ ] **Step 3: If footer isn't showing, check width constraints**

In `internal/components/status/footer.go`, the right side (token usage) is only rendered when `width` is large enough. Add a test:

```bash
go test ./internal/components/status/... -run TestFooter -v
```

If usage is missing from narrow terminals, the footer needs a min-width fallback or the token count should be shown separately.

- [ ] **Step 4: Add a "context" display to the header if footer is not sufficient**

If the footer token display is confirmed working, no further action needed. If users want a more prominent context indicator, consider showing it in the header or as a system message after each response (e.g., `[context: 12% | 24K/200K tokens]`).

- [ ] **Step 5: Commit any changes made**

```bash
git add <changed files>
git commit -m "fix(tui): ensure session context/token usage is visible in footer"
```

---

## Final Verification

After all tasks are complete, run the full test suite and build:

```bash
cd /home/nomadx/Documents/smolbot
go build ./...
go test ./... -v 2>&1 | grep -E "^(ok|FAIL|---)" | head -40
```

Expected: all packages `ok`, zero `FAIL` lines.

Manual smoke-test checklist (using Ollama — no Anthropic provider configured):
- [ ] Send a message and trigger a tool — THINKING block appears in transcript (Ollama models that support thinking, e.g. qwen3:8b with extended thinking, or any model where the gateway emits `chat.thinking.done`)
- [ ] Scroll up in a long conversation with mouse wheel
- [ ] Verify `esc` no longer jumps to bottom of transcript (does nothing when no dialog open)
- [ ] Open a tool-using session — tool input visible in tool block
- [ ] Scroll up in transcript — `↑ PgUp/PgDn` hint appears above frame border
- [ ] `/theme dracula` then `/theme nord` — THINKING and STREAM blocks change colors with theme
