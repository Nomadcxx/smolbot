# TUI Polish (P0–P2) — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring smolbot's TUI to Crush-level polish with animated spinners, tool-specific renderers, expand/collapse for thinking+tool output, parsed tool parameters, a scrollbar, and clipboard support.

**Architecture:** Adopt Crush's `ToolRenderer` interface pattern for per-tool rendering. Add a simple tick-based spinner for running tools. Implement `Expandable` on thinking blocks and tool items. Add a scrollbar component to the chatlist viewport. All changes are in `internal/` — no backend changes.

**Tech Stack:** Go 1.26, Bubble Tea v2, Lipgloss v2, Glamour v2, Ultraviolet

**Reference:** Crush source at `/home/nomadx/crush/internal/ui/chat/` (tools.go, bash.go, assistant.go) and `/home/nomadx/crush/internal/ui/anim/anim.go`, `/home/nomadx/crush/internal/ui/common/scrollbar.go`

---

## File Map

| File | Responsibility | Tasks |
|------|---------------|-------|
| `internal/components/anim/spinner.go` | Create: tick-based animated spinner | 1 |
| `internal/components/chatlist/items.go` | Modify: add expand/collapse, tool renderer interface, parsed params | 2, 3, 4 |
| `internal/components/chatlist/tool_renderers.go` | Create: per-tool renderers (bash, file, generic) | 3 |
| `internal/components/chatlist/item.go` | Modify: add Expandable interface | 2 |
| `internal/components/chatlist/list.go` | Modify: add content size tracking for scrollbar | 6 |
| `internal/components/chat/messages.go` | Modify: wire spinner ticks, toggle expand | 1, 2 |
| `internal/components/chat/scrollbar.go` | Create: scrollbar rendering | 6 |
| `internal/tui/tui.go` | Modify: tick subscription, scrollbar in View, clipboard keybindings | 1, 5, 6 |

---

### Task 1: Animated Spinner for Running Tools

Crush uses a gradient color-cycling spinner (anim.go). We'll implement a simpler ellipsis spinner that ticks via Bubble Tea's `tea.Tick`.

**Files:**
- Create: `internal/components/anim/spinner.go`
- Modify: `internal/components/chatlist/items.go` (ToolItem.Render uses spinner)
- Modify: `internal/components/chat/messages.go` (tick handling)
- Modify: `internal/tui/tui.go` (subscribe to ticks while streaming)
- Test: `internal/components/anim/spinner_test.go`

- [ ] **Step 1: Write failing test for spinner**

Create `internal/components/anim/spinner_test.go`:

```go
package anim

import "testing"

func TestSpinnerFrames(t *testing.T) {
	s := NewSpinner()
	frames := []string{s.Frame(), s.Frame(), s.Frame(), s.Frame()}
	// Should cycle through frames
	if frames[0] == frames[1] || frames[3] == frames[0] {
		t.Fatal("spinner should cycle through different frames")
	}
}

func TestSpinnerReset(t *testing.T) {
	s := NewSpinner()
	s.Frame()
	s.Frame()
	s.Reset()
	f := s.Frame()
	if f != "⠋" {
		t.Fatalf("expected first frame after reset, got %q", f)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/components/anim/ -run TestSpinner -v`
Expected: FAIL — package doesn't exist

- [ ] **Step 3: Implement spinner**

Create `internal/components/anim/spinner.go`:

```go
package anim

import "time"

var frames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

const Interval = 80 * time.Millisecond

type Spinner struct {
	idx int
}

func NewSpinner() *Spinner {
	return &Spinner{}
}

func (s *Spinner) Frame() string {
	f := frames[s.idx%len(frames)]
	s.idx++
	return f
}

func (s *Spinner) Reset() {
	s.idx = 0
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/components/anim/ -run TestSpinner -v`
Expected: PASS

- [ ] **Step 5: Wire spinner into ToolItem rendering**

In `internal/components/chatlist/items.go`, update `ToolItem` to use spinner for running status:

```go
// Add import "github.com/Nomadcxx/smolbot/internal/components/anim"

// Add field to ToolItem:
// spinner *anim.Spinner

// In renderToolCall, replace static "●" for running:
// When status == "running", use spinner.Frame() instead of "●"
```

- [ ] **Step 6: Add tick message type and subscription in tui.go**

In `internal/tui/tui.go`:
- Add `type SpinnerTickMsg time.Time`
- In `Update`, when `m.streaming` is true and SpinnerTickMsg arrives, return the model (trigger re-render)
- Add a tick command when streaming starts: `tea.Tick(anim.Interval, func(t time.Time) tea.Msg { return SpinnerTickMsg(t) })`
- Re-subscribe on each tick while still streaming

- [ ] **Step 7: Run all TUI tests**

Run: `go test ./internal/tui/ ./internal/components/chat/ -v`
Expected: All PASS

- [ ] **Step 8: Commit**

```bash
git add internal/components/anim/ internal/components/chatlist/items.go internal/components/chat/messages.go internal/tui/tui.go
git commit -m "feat: add animated spinner for running tool calls"
```

---

### Task 2: Expand/Collapse for Thinking Blocks and Tool Output

Crush supports `Expandable` interface — items can toggle between collapsed (10 lines) and expanded (full) views. We'll add this to `ThinkingItem` and `ToolItem`.

**Files:**
- Modify: `internal/components/chatlist/item.go` (add Expandable interface)
- Modify: `internal/components/chatlist/items.go` (ThinkingItem + ToolItem implement Expandable)
- Modify: `internal/components/chat/messages.go` (ToggleToolExpand fills empty stub)
- Modify: `internal/tui/tui.go` (keybinding for expand/collapse)
- Test: `internal/components/chat/messages_test.go`

- [ ] **Step 1: Write failing test for expand/collapse**

In `internal/components/chat/messages_test.go`, add:

```go
func TestToolExpandCollapseToggles(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}
	model := NewMessages()
	model.SetSize(80, 40)
	model.AppendUser("run something")

	// Create tool with >10 lines output
	lines := make([]string, 15)
	for i := range lines {
		lines[i] = fmt.Sprintf("output line %d", i+1)
	}
	model.StartTool("t1", "bash", "ls")
	model.FinishTool("t1", "bash", "done", strings.Join(lines, "\n"))

	// Collapsed: should show truncation hint
	view := model.View()
	if !strings.Contains(view, "+5 lines") {
		t.Fatalf("expected truncation hint in collapsed view, got %q", view)
	}

	// Toggle expand
	model.ToggleToolExpand(0)

	// Expanded: should show all lines
	view = model.View()
	if strings.Contains(view, "+5 lines") {
		t.Fatalf("expected no truncation hint after expand, got %q", view)
	}
	if !strings.Contains(view, "output line 15") {
		t.Fatalf("expected last line visible after expand, got %q", view)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/components/chat/ -run TestToolExpandCollapseToggles -v`
Expected: FAIL — ToggleToolExpand is empty

- [ ] **Step 3: Add Expandable interface**

In `internal/components/chatlist/item.go`:

```go
type Item interface {
	Render(width int) string
}

type Expandable interface {
	IsExpanded() bool
	SetExpanded(expanded bool)
}
```

- [ ] **Step 4: Implement Expandable on ToolItem**

In `internal/components/chatlist/items.go`, add `expanded bool` field to `ToolItem`. Update `Render` to pass `expanded` to `renderToolCall`. Add methods:

```go
func (t *ToolItem) IsExpanded() bool { return t.expanded }
func (t *ToolItem) SetExpanded(v bool) { t.expanded = v; t.cached = "" }
```

Update `renderToolCall`: use the `expanded` parameter already in its signature.

- [ ] **Step 5: Implement Expandable on ThinkingItem**

Same pattern: add `expanded bool`, implement `IsExpanded`/`SetExpanded`, pass to `renderThinkingBlock`.

- [ ] **Step 6: Fill in ToggleToolExpand**

In `internal/components/chat/messages.go`:

```go
func (m *MessagesModel) ToggleToolExpand(index int) {
	// Iterate through list items, find the Nth expandable item, toggle it
	count := 0
	for i := 0; i < m.list.Len(); i++ {
		item := m.list.ItemAt(i)
		if exp, ok := item.(chatlist.Expandable); ok {
			if count == index {
				exp.SetExpanded(!exp.IsExpanded())
				return
			}
			count++
		}
	}
}
```

Also add `ItemAt(idx int) Item` method to `List` in `list.go`:

```go
func (l *List) ItemAt(idx int) Item {
	if idx < 0 || idx >= len(l.items) {
		return nil
	}
	return l.items[idx]
}
```

- [ ] **Step 7: Add keybinding in tui.go**

In the keyboard handler, add `e` key to toggle expand on most recent expandable item (when not in dialog and not editing):

```go
case "e":
    m.messages.ToggleToolExpand(0) // Toggle the last expandable item
    return m, nil
```

- [ ] **Step 8: Run test to verify it passes**

Run: `go test ./internal/components/chat/ -run TestToolExpandCollapseToggles -v`
Expected: PASS

- [ ] **Step 9: Commit**

```bash
git add internal/components/chatlist/item.go internal/components/chatlist/items.go internal/components/chatlist/list.go internal/components/chat/messages.go internal/components/chat/messages_test.go internal/tui/tui.go
git commit -m "feat: add expand/collapse for thinking blocks and tool output"
```

---

### Task 3: Tool Renderer Interface + Bash/Generic Renderers

Crush has 20+ per-tool renderers. We'll start with the pattern: a `ToolRenderer` interface, a bash renderer that parses the command and shows it in the header, and a generic fallback.

**Files:**
- Create: `internal/components/chatlist/tool_renderers.go`
- Modify: `internal/components/chatlist/items.go` (ToolItem uses renderer)
- Test: `internal/components/chatlist/tool_renderers_test.go`

- [ ] **Step 1: Write failing tests for tool renderers**

Create `internal/components/chatlist/tool_renderers_test.go`:

```go
package chatlist

import (
	"strings"
	"testing"

	"github.com/Nomadcxx/smolbot/internal/theme"
	_ "github.com/Nomadcxx/smolbot/internal/theme/themes"
)

func TestBashRendererShowsCommand(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}
	item := &ToolItem{Name: "Bash", Input: `{"command": "go test ./..."}`, Status: "done", Output: "PASS"}
	view := item.Render(80)
	if !strings.Contains(view, "go test") {
		t.Fatalf("expected bash command in header, got %q", view)
	}
}

func TestGenericRendererShowsName(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}
	item := &ToolItem{Name: "custom_tool", Input: `{"key": "value"}`, Status: "done", Output: "result"}
	view := item.Render(80)
	if !strings.Contains(view, "custom_tool") {
		t.Fatalf("expected tool name, got %q", view)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/components/chatlist/ -run TestBashRenderer -v`
Expected: FAIL or existing behavior doesn't parse command

- [ ] **Step 3: Create tool_renderers.go**

```go
package chatlist

import "encoding/json"

// ToolRenderer renders a specific tool type.
type ToolRenderer interface {
	RenderHeader(tc ToolCall, width int) string
}

// BashRenderer extracts the command from input JSON.
type BashRenderer struct{}

func (b BashRenderer) RenderHeader(tc ToolCall, width int) string {
	var params struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal([]byte(tc.Input), &params); err == nil && params.Command != "" {
		cmd := params.Command
		if len(cmd) > 80 {
			cmd = cmd[:77] + "..."
		}
		return cmd
	}
	return tc.Input
}

// ReadRenderer shows file path.
type ReadRenderer struct{}

func (r ReadRenderer) RenderHeader(tc ToolCall, width int) string {
	var params struct {
		Path     string `json:"file_path"`
		FilePath string `json:"path"`
	}
	if err := json.Unmarshal([]byte(tc.Input), &params); err == nil {
		p := params.Path
		if p == "" {
			p = params.FilePath
		}
		if p != "" {
			return p
		}
	}
	return tc.Input
}

// EditRenderer shows file path.
type EditRenderer struct{}

func (e EditRenderer) RenderHeader(tc ToolCall, width int) string {
	var params struct {
		Path     string `json:"file_path"`
		FilePath string `json:"path"`
	}
	if err := json.Unmarshal([]byte(tc.Input), &params); err == nil {
		p := params.Path
		if p == "" {
			p = params.FilePath
		}
		if p != "" {
			return p
		}
	}
	return tc.Input
}

// GenericRenderer falls back to truncated raw input.
type GenericRenderer struct{}

func (g GenericRenderer) RenderHeader(tc ToolCall, width int) string {
	return formatParamsKeyValue(tc.Input, 80)
}

// formatParamsKeyValue parses JSON input and formats as key=value pairs.
func formatParamsKeyValue(input string, maxLen int) string {
	var raw map[string]any
	if err := json.Unmarshal([]byte(input), &raw); err != nil {
		if len(input) > maxLen {
			return input[:maxLen-3] + "..."
		}
		return input
	}
	var parts []string
	for k, v := range raw {
		s, _ := json.Marshal(v)
		parts = append(parts, k+"="+string(s))
	}
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += ", "
		}
		result += p
	}
	if len(result) > maxLen {
		result = result[:maxLen-3] + "..."
	}
	return result
}

// rendererForTool returns the appropriate renderer for a tool name.
func rendererForTool(name string) ToolRenderer {
	switch name {
	case "Bash", "bash":
		return BashRenderer{}
	case "Read", "read", "read_file":
		return ReadRenderer{}
	case "Edit", "edit", "Write", "write":
		return EditRenderer{}
	default:
		return GenericRenderer{}
	}
}
```

- [ ] **Step 4: Wire renderer into renderToolCall**

In `internal/components/chatlist/items.go`, update `renderToolCall` to use `rendererForTool`:

```go
// Replace the inputSummary logic with:
renderer := rendererForTool(tc.Name)
inputSummary := renderer.RenderHeader(tc, width)
```

- [ ] **Step 5: Run tests**

Run: `go test ./internal/components/chatlist/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/components/chatlist/tool_renderers.go internal/components/chatlist/tool_renderers_test.go internal/components/chatlist/items.go
git commit -m "feat: add ToolRenderer interface with bash/read/edit/generic renderers"
```

---

### Task 4: Tool Status States (Running/Done/Error/Canceled)

Add a canceled state for aborted tool calls, and show a meaningful running indicator instead of just an icon.

**Files:**
- Modify: `internal/components/chatlist/items.go` (add "canceled" status + running text)
- Modify: `internal/components/chat/messages.go` (cancel active tools on abort)
- Test: `internal/components/chat/messages_test.go`

- [ ] **Step 1: Write failing test**

```go
func TestToolCancelStatusRendered(t *testing.T) {
	if !theme.Set("nord") {
		t.Fatal("expected nord theme")
	}
	item := &chatlist.ToolItem{Name: "bash", Status: "canceled"}
	view := item.Render(80)
	if !strings.Contains(view, "⊘") {
		t.Fatalf("expected canceled icon, got %q", view)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

- [ ] **Step 3: Add canceled status to toolIcon**

In `items.go`, update `toolIcon`:

```go
case "canceled":
    return "⊘", t.TextMuted
```

- [ ] **Step 4: Add CancelActiveTools method to messages.go**

```go
func (m *MessagesModel) CancelActiveTools() {
	for _, tool := range m.activeTools {
		if tool.Status == "running" {
			tool.Status = "canceled"
		}
	}
}
```

- [ ] **Step 5: Call CancelActiveTools in handleCtrlC**

In `tui.go`, in `handleCtrlC()`, after abort call:

```go
m.messages.CancelActiveTools()
```

- [ ] **Step 6: Run tests**

Run: `go test ./internal/components/chat/ ./internal/tui/ -v`
Expected: All PASS

- [ ] **Step 7: Commit**

```bash
git add internal/components/chatlist/items.go internal/components/chat/messages.go internal/tui/tui.go internal/components/chat/messages_test.go
git commit -m "feat: add canceled tool status and cancel-on-abort behavior"
```

---

### Task 5: Copy-to-Clipboard Support

Crush uses OSC 52 escape sequences + native clipboard. We'll start with OSC 52 which works across most modern terminals.

**Files:**
- Create: `internal/components/common/clipboard.go`
- Modify: `internal/tui/tui.go` (add `c` keybinding)
- Test: `internal/components/common/clipboard_test.go`

- [ ] **Step 1: Create clipboard.go**

```go
package common

import (
	"encoding/base64"
	"fmt"
	"io"
)

// WriteOSC52 writes the OSC 52 escape sequence to copy text to clipboard.
func WriteOSC52(w io.Writer, text string) {
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	fmt.Fprintf(w, "\033]52;c;%s\a", encoded)
}
```

- [ ] **Step 2: Write test**

```go
package common

import (
	"bytes"
	"strings"
	"testing"
)

func TestWriteOSC52(t *testing.T) {
	var buf bytes.Buffer
	WriteOSC52(&buf, "hello")
	if !strings.HasPrefix(buf.String(), "\033]52;c;") {
		t.Fatalf("expected OSC 52 prefix, got %q", buf.String())
	}
}
```

- [ ] **Step 3: Run tests**

Run: `go test ./internal/components/common/ -v`
Expected: PASS

- [ ] **Step 4: Wire into TUI**

In `tui.go`, add a `c` keybinding (when not in editor focus and dialog is nil) that copies the last assistant message content to clipboard via OSC 52. Use `tea.Printf` or direct stdout write.

- [ ] **Step 5: Commit**

```bash
git add internal/components/common/ internal/tui/tui.go
git commit -m "feat: add copy-to-clipboard via OSC 52 escape sequence"
```

---

### Task 6: Scrollbar Indicator

Crush has a simple proportional scrollbar (scrollbar.go, 46 lines). We'll port it.

**Files:**
- Create: `internal/components/chat/scrollbar.go`
- Modify: `internal/tui/tui.go` (render scrollbar alongside messages)
- Test: `internal/components/chat/scrollbar_test.go`

- [ ] **Step 1: Write failing test for scrollbar**

Create `internal/components/chat/scrollbar_test.go`:

```go
package chat

import (
	"strings"
	"testing"
)

func TestScrollbarRendersProportionalThumb(t *testing.T) {
	bar := RenderScrollbar(10, 100, 10, 0)
	lines := strings.Split(bar, "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(lines))
	}
	// First line should be thumb (at offset 0)
	if !strings.Contains(lines[0], "█") {
		t.Fatalf("expected thumb at top, got %q", lines[0])
	}
}

func TestScrollbarEmptyWhenContentFits(t *testing.T) {
	bar := RenderScrollbar(10, 5, 10, 0)
	if bar != "" {
		t.Fatalf("expected empty scrollbar when content fits, got %q", bar)
	}
}
```

- [ ] **Step 2: Implement scrollbar.go**

Create `internal/components/chat/scrollbar.go`:

```go
package chat

import (
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/Nomadcxx/smolbot/internal/theme"
)

const (
	scrollThumb = "█"
	scrollTrack = "░"
)

// RenderScrollbar draws a vertical scrollbar.
func RenderScrollbar(height, contentSize, viewportSize, offset int) string {
	if height <= 0 || contentSize <= viewportSize {
		return ""
	}

	thumbSize := max(1, height*viewportSize/contentSize)
	maxOffset := contentSize - viewportSize
	if maxOffset <= 0 {
		return ""
	}

	trackSpace := height - thumbSize
	thumbPos := 0
	if trackSpace > 0 && maxOffset > 0 {
		thumbPos = min(trackSpace, offset*trackSpace/maxOffset)
	}

	t := theme.Current()
	thumbStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
	trackStyle := lipgloss.NewStyle().Foreground(t.Panel)

	var sb strings.Builder
	for i := range height {
		if i > 0 {
			sb.WriteString("\n")
		}
		if i >= thumbPos && i < thumbPos+thumbSize {
			sb.WriteString(thumbStyle.Render(scrollThumb))
		} else {
			sb.WriteString(trackStyle.Render(scrollTrack))
		}
	}
	return sb.String()
}
```

- [ ] **Step 3: Run test**

Run: `go test ./internal/components/chat/ -run TestScrollbar -v`
Expected: PASS

- [ ] **Step 4: Render scrollbar in TUI View**

In `internal/tui/tui.go`, in the `View()` method, after drawing messages, draw the scrollbar on the right edge if content is scrolled:

```go
if m.messages.HasContentAbove() || !m.messages.IsAtBottom() {
    bar := chat.RenderScrollbar(chatH, totalContentLines, chatH, m.messages.ViewportOffset())
    if bar != "" {
        uv.NewStyledString(bar).Draw(scr, uv.Rect(m.width-1, chatY, 1, chatH))
    }
}
```

This requires exposing total content height from the chatlist. Add a `TotalContentHeight()` method to `MessagesModel` that returns `list.totalContentHeight()`.

- [ ] **Step 5: Run all tests**

Run: `go test ./internal/tui/ ./internal/components/chat/ -v`
Expected: All PASS

- [ ] **Step 6: Commit**

```bash
git add internal/components/chat/scrollbar.go internal/components/chat/scrollbar_test.go internal/tui/tui.go internal/components/chat/messages.go internal/components/chatlist/list.go
git commit -m "feat: add scrollbar indicator to chat viewport"
```

---

## Final Verification

- [ ] **Full build:** `go build ./cmd/... ./pkg/... ./internal/...`
- [ ] **Full test suite:** `go test ./internal/... -v`
- [ ] **Build binaries:** `go build -o smolbot-tui ./cmd/smolbot-tui && go build -o smolbot ./cmd/smolbot`
- [ ] **Manual test:** Run `./smolbot-tui`, send a message, verify spinner animates during tool execution, tool output is collapsed with expand hint, scrollbar appears when scrolled
