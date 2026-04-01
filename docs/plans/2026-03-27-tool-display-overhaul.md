# Tool Display & Diff Rendering Overhaul — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bring smolbot's tool use display up to the standard set by Claude Code and opencode — color-coded diffs for file edits, distinct visual blocks for each tool type, progress indicators, and clear differentiation between read/edit/command operations.

**Reference implementations:**
- **Claude Code** — React/Ink renderer. Dark green/red backgrounds for add/delete. Line number prefixes. Recently simplified tool summaries ("Read 3 files", "Edited 2 files"). 24-bit true color.
- **opencode** (`/home/nomadx/opencode/packages/tui/`) — Primary code reference. `ThickBorder()` left/right tool blocks. Color-coded by state (warning yellow for permissions, red for errors). Unified diff at <120 cols, side-by-side at ≥120. Intra-line highlighting via `diffmatchpatch`. Tool-specific renderers per tool type. Caches completed tool output.

**Architecture:** Changes are in the TUI layer only. Tool events already carry the data needed (tool name, input, output). The rendering needs to interpret this data and present it visually.

**Tech Stack:** Go 1.26, charm.land/bubbletea v2.0.2, charm.land/lipgloss v2.0.2, `github.com/sergi/go-diff` (for intra-line diff)

---

## Current State

The current tool display in smolbot is minimal:
- Tool start: shows tool name in a simple text block
- Tool done: shows tool name + brief output text
- No diff rendering for file edits
- No color coding for additions/deletions
- No distinction between read, edit, write, exec tool types
- No progress spinner while tool is running
- No line numbers in file content
- Tool blocks use the same visual style as regular messages

---

## File Map

| File | Change |
|------|--------|
| `internal/components/chat/message.go` | Rewrite tool block rendering; add per-tool-type renderers |
| `internal/components/chat/diff.go` | **New** — Unified diff parser and renderer with color coding |
| `internal/components/chat/toolblock.go` | **New** — Tool block container with border, title, state |
| `internal/components/chat/messages.go` | Wire new tool renderers; add tool progress spinner |
| `internal/theme/theme.go` | Add diff color fields to Theme struct |
| `internal/theme/themes/*.go` | Add diff colors to all 9 themes |
| `internal/tui/tui.go` | Forward tool events with full metadata to messages component |

---

## Theme Diff Colors

Each theme needs these additional fields:

```go
// In theme.go Theme struct:
DiffAdded          color.Color // Text color for added lines (green family)
DiffRemoved        color.Color // Text color for removed lines (red family)
DiffAddedBg        color.Color // Background for added lines (dark green)
DiffRemovedBg      color.Color // Background for removed lines (dark red)
DiffContext         color.Color // Text for unchanged context lines
DiffContextBg       color.Color // Background for context lines
DiffHighlightAdded  color.Color // Intra-line added char highlight
DiffHighlightRemoved color.Color // Intra-line removed char highlight
DiffLineNumber      color.Color // Line number color
```

**Per-theme diff colors (from opencode's theme system):**

| Theme | DiffAdded | DiffRemoved | DiffAddedBg | DiffRemovedBg |
|-------|-----------|-------------|-------------|---------------|
| Dracula | `#50fa7b` | `#ff5555` | `#1a3a1a` | `#3a1a1a` |
| Nord | `#a3be8c` | `#bf616a` | `#1a2a1a` | `#2a1a1a` |
| Catppuccin | `#a6e3a1` | `#f38ba8` | `#1a2e1a` | `#2e1a1e` |
| Tokyo Night | `#9ece6a` | `#f7768e` | `#1a2a1a` | `#2a1a1e` |
| Gruvbox | `#b8bb26` | `#fb4934` | `#1a2a1a` | `#2a1a1a` |
| Material | `#c3e88d` | `#f07178` | `#1a2a1a` | `#2a1a1a` |
| RAMA | `#edf2f4` | `#ef233c` | `#1a2b1a` | `#2b1a1a` |
| Solarized | `#859900` | `#dc322f` | `#1a2a1a` | `#2a1a1a` |
| Monochrome | `#b8b8b8` | `#999999` | `#2a2a2a` | `#1a1a1a` |

---

## Task 1: Tool Block Container Component

**Files:**
- New: `internal/components/chat/toolblock.go`
- Modify: `internal/theme/theme.go`

A reusable container for tool display blocks. Follows opencode's pattern: thick left border, colored by state, tool name in title, content area.

- [ ] **Step 1: Build tool block component**

```go
// internal/components/chat/toolblock.go
package chat

type ToolBlockState int

const (
	ToolBlockRunning ToolBlockState = iota
	ToolBlockDone
	ToolBlockError
)

type ToolBlockOpts struct {
	Title   string         // e.g. "Edit internal/tui/tui.go"
	State   ToolBlockState
	Content string         // rendered content inside the block
	Width   int
}

func renderToolBlock(opts ToolBlockOpts, t *theme.Theme) string {
	// Border color based on state
	var borderColor color.Color
	switch opts.State {
	case ToolBlockRunning:
		borderColor = t.ToolStateRunning // yellow
	case ToolBlockDone:
		borderColor = t.ToolStateDone // green
	case ToolBlockError:
		borderColor = t.ToolStateError // red
	}

	// Title line: tool name, right-aligned state indicator
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(t.ToolName)

	// Container: thick left border, padding
	container := lipgloss.NewStyle().
		BorderStyle(lipgloss.ThickBorder()).
		BorderLeft(true).
		BorderRight(false).
		BorderTop(false).
		BorderBottom(false).
		BorderForeground(borderColor).
		PaddingLeft(1).
		Width(opts.Width - 2)

	header := titleStyle.Render(opts.Title)
	if opts.Content == "" {
		return container.Render(header)
	}
	return container.Render(header + "\n" + opts.Content)
}
```

- [ ] **Step 2: Add spinner for running state**

When `ToolBlockRunning`, show a spinner character cycling at tool-specific cadence:
```
▍ Running exec...
```

Use the same `⣾ ⣽ ⣻ ⢿ ⡿ ⣟ ⣯ ⣷` braille spinner from the compaction plan, or a simpler `▍▌▋█▊▉` bar cycle.

- [ ] **Step 3: Write tests**

Test block rendering in each state. Test border colors match theme. Test width constraints.

---

## Task 2: Diff Parser & Renderer

**Files:**
- New: `internal/components/chat/diff.go`
- Modify: `go.mod` (add `github.com/sergi/go-diff` for intra-line diff)

Parse unified diff format and render with color-coded additions/deletions, following opencode's approach.

- [ ] **Step 1: Parse unified diff**

```go
// internal/components/chat/diff.go
package chat

type DiffLineType int

const (
	DiffContext DiffLineType = iota
	DiffAdded
	DiffRemoved
)

type DiffLine struct {
	Type    DiffLineType
	Content string
	OldLine int // line number in old file (0 if added)
	NewLine int // line number in new file (0 if removed)
}

// ParseUnifiedDiff parses a unified diff string into structured lines.
func ParseUnifiedDiff(patch string) []DiffLine {
	// Parse @@ -start,count +start,count @@ headers
	// Classify lines by prefix: +, -, space
	// Track line numbers for both old and new files
}
```

- [ ] **Step 2: Render unified diff with colors**

```go
// RenderDiff renders a parsed diff with theme colors.
// At width < 120, unified format. At ≥ 120, side-by-side.
func RenderDiff(lines []DiffLine, width int, t *theme.Theme) string {
	if width >= 120 {
		return renderSideBySideDiff(lines, width, t)
	}
	return renderUnifiedDiff(lines, width, t)
}

func renderUnifiedDiff(lines []DiffLine, width int, t *theme.Theme) string {
	var b strings.Builder
	for _, line := range lines {
		lineNum := formatLineNumbers(line, t)

		var style lipgloss.Style
		var prefix string
		switch line.Type {
		case DiffAdded:
			prefix = "+"
			style = lipgloss.NewStyle().
				Foreground(t.DiffAdded).
				Background(t.DiffAddedBg).
				Width(width - lineNumWidth)
		case DiffRemoved:
			prefix = "-"
			style = lipgloss.NewStyle().
				Foreground(t.DiffRemoved).
				Background(t.DiffRemovedBg).
				Width(width - lineNumWidth)
		case DiffContext:
			prefix = " "
			style = lipgloss.NewStyle().
				Foreground(t.DiffContext).
				Background(t.DiffContextBg).
				Width(width - lineNumWidth)
		}

		b.WriteString(lineNum + style.Render(prefix + line.Content) + "\n")
	}
	return b.String()
}
```

- [ ] **Step 3: Add intra-line highlighting (stretch goal)**

Using `go-diff` for character-level change detection between adjacent removed/added line pairs:

```go
func highlightIntralineChanges(removed, added string) ([]Segment, []Segment) {
	diffs := diffmatchpatch.New().DiffMain(removed, added, false)
	// Build segment lists marking which characters changed
	// Apply DiffHighlightAdded/Removed colors to changed segments
}
```

This is a stretch goal because it adds a dependency and complexity. Start with line-level coloring.

- [ ] **Step 4: Write tests**

Test unified diff parsing with real diffs. Test line number tracking. Test color application. Test width-responsive format switching.

---

## Task 3: Per-Tool-Type Renderers

**Files:**
- Modify: `internal/components/chat/message.go`

Each tool type gets a specialized renderer following opencode's pattern. Currently all tools render the same way.

- [ ] **Step 1: Tool type dispatch**

```go
func (m *MessagesModel) renderToolOutput(name string, input, output string, state ToolBlockState, width int) string {
	t := theme.Current()
	switch name {
	case "read_file":
		return m.renderReadTool(input, output, state, width, t)
	case "edit_file":
		return m.renderEditTool(input, output, state, width, t)
	case "write_file":
		return m.renderWriteTool(input, output, state, width, t)
	case "exec":
		return m.renderExecTool(input, output, state, width, t)
	case "web_search", "web_fetch":
		return m.renderWebTool(name, input, output, state, width, t)
	case "glob", "grep":
		return m.renderSearchTool(name, input, output, state, width, t)
	default:
		return m.renderGenericTool(name, input, output, state, width, t)
	}
}
```

- [ ] **Step 2: Read file renderer**

Shows filename as title, file content with line numbers, truncated to ~6 visible lines:

```go
func (m *MessagesModel) renderReadTool(input, output string, state ToolBlockState, width int, t *theme.Theme) string {
	var params struct{ Path string }
	json.Unmarshal([]byte(input), &params)

	title := "Read " + filepath.Base(params.Path)
	content := ""
	if state == ToolBlockDone && output != "" {
		content = renderFilePreview(output, 6, width-4, t)
	}
	return renderToolBlock(ToolBlockOpts{Title: title, State: state, Content: content, Width: width}, t)
}

func renderFilePreview(content string, maxLines, width int, t *theme.Theme) string {
	lines := strings.Split(content, "\n")
	if len(lines) > maxLines {
		lines = lines[:maxLines]
		// Add "... N more lines" indicator
	}
	// Render with line numbers
	var b strings.Builder
	lineNumStyle := lipgloss.NewStyle().Foreground(t.DiffLineNumber)
	for i, line := range lines {
		num := lineNumStyle.Render(fmt.Sprintf("%4d ", i+1))
		b.WriteString(num + line + "\n")
	}
	return b.String()
}
```

- [ ] **Step 3: Edit file renderer (with diff)**

Shows filename, then renders the diff with color coding:

```go
func (m *MessagesModel) renderEditTool(input, output string, state ToolBlockState, width int, t *theme.Theme) string {
	var params struct {
		Path      string `json:"file_path"`
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	}
	json.Unmarshal([]byte(input), &params)

	title := "Edit " + filepath.Base(params.Path)
	content := ""
	if state == ToolBlockDone {
		// Generate a simple diff from old_string → new_string
		diff := generateSimpleDiff(params.OldString, params.NewString)
		content = RenderDiff(diff, width-4, t)
	} else if state == ToolBlockRunning {
		content = lipgloss.NewStyle().Foreground(t.TextMuted).Render("Preparing edit...")
	}
	return renderToolBlock(ToolBlockOpts{Title: title, State: state, Content: content, Width: width}, t)
}
```

- [ ] **Step 4: Exec (shell command) renderer**

Shows command with `$` prefix, output in a monospace block:

```go
func (m *MessagesModel) renderExecTool(input, output string, state ToolBlockState, width int, t *theme.Theme) string {
	var params struct {
		Command string `json:"command"`
	}
	json.Unmarshal([]byte(input), &params)

	title := "Shell"
	content := ""
	if params.Command != "" {
		cmdStyle := lipgloss.NewStyle().Bold(true).Foreground(t.TextPrimary)
		content = cmdStyle.Render("$ " + params.Command)
	}
	if state == ToolBlockDone && output != "" {
		outputStyle := lipgloss.NewStyle().Foreground(t.TextMuted)
		truncated := truncateLines(output, 10)
		content += "\n" + outputStyle.Render(truncated)
	}
	return renderToolBlock(ToolBlockOpts{Title: title, State: state, Content: content, Width: width}, t)
}
```

- [ ] **Step 5: Write file renderer**

Shows filename, content preview (truncated):

```go
func (m *MessagesModel) renderWriteTool(input, output string, state ToolBlockState, width int, t *theme.Theme) string {
	var params struct{ Path string `json:"file_path"` }
	json.Unmarshal([]byte(input), &params)
	title := "Write " + filepath.Base(params.Path)
	// Show brief content preview on done
	return renderToolBlock(ToolBlockOpts{Title: title, State: state, Content: content, Width: width}, t)
}
```

- [ ] **Step 6: Search tool renderer (glob/grep)**

Shows pattern and result count:

```go
func (m *MessagesModel) renderSearchTool(name, input, output string, state ToolBlockState, width int, t *theme.Theme) string {
	var params struct{ Pattern string `json:"pattern"` }
	json.Unmarshal([]byte(input), &params)
	title := strings.Title(name) + " " + params.Pattern
	if state == ToolBlockDone {
		lines := strings.Split(strings.TrimSpace(output), "\n")
		content = fmt.Sprintf("%d results", len(lines))
	}
	return renderToolBlock(ToolBlockOpts{Title: title, State: state, Content: content, Width: width}, t)
}
```

- [ ] **Step 7: Write tests**

Test each tool type renders correctly. Test with empty output. Test running vs done states. Test truncation.

---

## Task 4: Add Diff Colors to All Themes

**Files:**
- Modify: `internal/theme/theme.go` (add fields)
- Modify: `internal/theme/themes/*.go` (all 9 theme files)

- [ ] **Step 1: Add diff fields to Theme struct**

```go
// In theme.go
type Theme struct {
	// ... existing fields

	// Diff rendering
	DiffAdded            color.Color
	DiffRemoved          color.Color
	DiffAddedBg          color.Color
	DiffRemovedBg        color.Color
	DiffContext          color.Color
	DiffContextBg        color.Color
	DiffHighlightAdded   color.Color
	DiffHighlightRemoved color.Color
	DiffLineNumber       color.Color
}
```

- [ ] **Step 2: Set defaults in `register()` function**

Derive diff colors from existing theme colors when not explicitly set:

```go
// In register(), after setting base colors:
if t.DiffAdded == nil { t.DiffAdded = t.Success }
if t.DiffRemoved == nil { t.DiffRemoved = t.Error }
if t.DiffAddedBg == nil { t.DiffAddedBg = darken(t.Success, 0.85) }
if t.DiffRemovedBg == nil { t.DiffRemovedBg = darken(t.Error, 0.85) }
if t.DiffContext == nil { t.DiffContext = t.TextMuted }
if t.DiffContextBg == nil { t.DiffContextBg = t.Panel }
if t.DiffHighlightAdded == nil { t.DiffHighlightAdded = t.Success }
if t.DiffHighlightRemoved == nil { t.DiffHighlightRemoved = t.Error }
if t.DiffLineNumber == nil { t.DiffLineNumber = t.TextMuted }
```

This gives reasonable defaults for all themes. Themes can override individual colors in their `func(t *theme.Theme)` callback for finer control.

- [ ] **Step 3: Add explicit diff colors to key themes**

For themes with well-known diff palettes (from opencode's theme definitions):

**Dracula:**
```go
t.DiffAdded = lipgloss.Color("#50fa7b")
t.DiffRemoved = lipgloss.Color("#ff5555")
t.DiffAddedBg = lipgloss.Color("#1a3a1a")
t.DiffRemovedBg = lipgloss.Color("#3a1a1a")
```

**Nord:**
```go
t.DiffAdded = lipgloss.Color("#a3be8c")
t.DiffRemoved = lipgloss.Color("#bf616a")
t.DiffAddedBg = lipgloss.Color("#1a2a1a")
t.DiffRemovedBg = lipgloss.Color("#2a1a1a")
```

**RAMA:**
```go
t.DiffAdded = lipgloss.Color("#edf2f4")
t.DiffRemoved = lipgloss.Color("#ef233c")
t.DiffAddedBg = lipgloss.Color("#1a2b1a")
t.DiffRemovedBg = lipgloss.Color("#2b1a1a")
```

- [ ] **Step 4: Write tests**

Test default derivation works for themes without explicit diff colors. Test explicit overrides take precedence.

---

## Task 5: Tool Event Data Enhancement

**Files:**
- Modify: `internal/tui/tui.go`
- Modify: `internal/components/chat/messages.go`

Currently tool events may not carry enough data (input params) for per-tool rendering. Ensure the full tool input is available.

- [ ] **Step 1: Verify tool event payloads**

Check that `chat.tool.start` events include `input` (the raw JSON arguments). Check that `chat.tool.done` events include `output` (the tool result text).

Currently in `server.go`:
```go
case agent.EventToolStart:
	input, _ := event.Data["input"].(string)
	s.emitEvent(cs, "chat.tool.start", map[string]any{
		"name":  event.Content,
		"input": input,
		"id":    toolID,
	})
```

This should be sufficient. Verify the TUI receives and stores both input and output.

- [ ] **Step 2: Store tool input in messages model**

The messages model needs to track tool input alongside output:

```go
type ToolCallInfo struct {
	ID     string
	Name   string
	Input  string // raw JSON input
	Output string
	Error  string
	State  ToolBlockState
}
```

When `chat.tool.start` arrives, create the entry with input. When `chat.tool.done` arrives, update with output.

- [ ] **Step 3: Wire new renderers**

Replace the current tool rendering in `renderContent()` with calls to `renderToolOutput()` dispatch.

- [ ] **Step 4: Write tests**

Test tool lifecycle: start → running render → done → done render. Test that input is preserved from start to done.

---

## Task 6: Tool Output Truncation & Expansion (Stretch)

**Files:**
- Modify: `internal/components/chat/messages.go`

For long tool outputs (e.g., large file reads, verbose exec output), truncate by default and allow expansion.

- [ ] **Step 1: Truncate long outputs**

```go
func truncateLines(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	truncated := strings.Join(lines[:maxLines], "\n")
	remaining := len(lines) - maxLines
	return truncated + fmt.Sprintf("\n... %d more lines", remaining)
}
```

Per-tool defaults:
- `read_file`: 6 lines
- `exec`: 10 lines
- `web_fetch`: 10 lines
- `glob`/`grep`: show result count only
- `edit_file`: show full diff (diffs are typically short)

- [ ] **Step 2: (Stretch) Add expand/collapse toggle**

This would require tracking expanded state per tool block and a keybinding to toggle. Deferred — truncation with "N more lines" is sufficient for MVP.

---

## Priority Order

| Priority | Task | Rationale |
|----------|------|-----------|
| P0 | Task 1: Tool block container | Foundation for all tool rendering |
| P0 | Task 4: Diff colors in themes | Required before diff rendering works |
| P0 | Task 5: Tool event data | Required before per-tool renderers work |
| P0 | Task 3: Per-tool renderers | Core deliverable — distinct rendering per tool type |
| P1 | Task 2: Diff parser & renderer | Key visual improvement for edit tools |
| P2 | Task 6: Truncation | Polish |

---

## Dependencies

| This Plan | Depends On |
|-----------|------------|
| Task 2 (diff renderer) | Task 4 (diff colors in themes) |
| Task 3 (per-tool renderers) | Task 1 (tool block container) + Task 5 (event data) |
| None | `2026-03-27-tui-polish-and-performance.md` (independent plans) |

---

## Testing Strategy

- Unit tests for diff parsing (known diffs → expected DiffLine arrays)
- Visual snapshot tests for tool blocks in each state
- Per-theme diff color verification
- Tool lifecycle integration tests (start → progress → done rendering)
- Width-responsive tests (unified vs side-by-side at 80 vs 120 cols)

## Risks

1. **`go-diff` dependency**: Adding an external dep. If unacceptable, implement simple line-level diff without intra-line highlighting.
2. **Edit tool input format**: Tool input must contain `old_string`/`new_string` for diff generation. If the agent uses a different format, the diff renderer won't work. Need to verify tool schema.
3. **Markdown rendering interaction**: Tool blocks rendered inside the viewport may conflict with glamour markdown rendering. Tool blocks should be rendered raw (no markdown pass) and injected as pre-rendered content.
4. **ANSI in tool output**: Shell command output may contain ANSI escape codes. Need to strip them before applying our own styling, or pass them through carefully.
