# Tool UI/UX — Current State Analysis

> **Purpose**: Document exactly how smolbot renders tool calls and results in the TUI today,
> then compare against Claude Code's compact approach to identify improvements.

---

## 1. Current Tool Rendering Architecture

### 1.1 Data Model

```go
// internal/components/chat/message.go:32-38
type ToolCall struct {
    ID     string  // Unique execution ID
    Name   string  // Tool name (read_file, exec, edit_file, etc.)
    Input  string  // JSON-encoded arguments
    Status string  // "running" | "done" | "error"
    Output string  // Execution result
}
```

### 1.2 Rendering Pipeline

```
Server event (chat.tool.start / chat.tool.done)
    ↓
tui.go:725-742 — dispatches to messages component
    ↓
messages.go:186-210 — StartTool() / FinishTool() updates m.tools[]
    ↓
messages.go:386-429 — renderTranscript() iterates messages + tools
    ↓
message.go:43-62 — renderToolCall() builds the visual block
    ↓
toolblock.go:42-96 — RenderToolBlock() applies styling + border
    ↓
viewport — scrolls to bottom if following
```

### 1.3 Expand/Collapse

- `messages.go` maintains `expandedTools map[string]bool`
- Tools are collapsed by default; user can toggle
- Collapsed: shows header + truncated output
- Expanded: shows full output

---

## 2. Visual Structure of a Tool Block

Every tool call renders as a **bordered box** with a thick left accent border:

```
┃ [STATUS] TITLE
┃ INPUT_LABEL
┃ key: value
┃ key: value
┃ OUTPUT_LABEL
┃ line 1
┃ line 2
┃ … (N lines hidden)
```

### 2.1 Status Glyphs

| State | Glyph | Border Colour |
|-------|-------|---------------|
| Running | `◐` `◓` `◑` `◒` (animated) | Yellow/Warning |
| Done | `✓` | Green/Success |
| Error | `✗` | Red/Error |

Source: `toolblock.go:113-153`

### 2.2 Title Generation

| Tool | Title Format | Example |
|------|-------------|---------|
| `read_file` | `Read <filename>` | `Read main.go` |
| `write_file` | `Write <filename>` | `Write config.json` |
| `edit_file` | `Edit <filename>` | `Edit handler.go` |
| `list_dir` | `List <dirname>` | `List src/` |
| `exec` | `Shell` | `Shell` |
| `web_search` | `Search <query>` | `Search golang channels` |
| `web_fetch` | `Fetch <url>` | `Fetch https://...` |
| `message` | `Message` | `Message` |
| `cron` | `Cron` | `Cron` |

Source: `message.go:90-125`

### 2.3 Input Fields Rendered Per Tool

| Tool | Fields Shown |
|------|-------------|
| `read_file` | path, offset, limit, extraAllowedDirs |
| `write_file` | path, content (truncated at 160 chars) |
| `edit_file` | path, replace_all |
| `exec` | command, timeout |
| `web_search` | query, maxResults |
| `web_fetch` | url |
| `message` | channel, chat_id, content |
| `cron` | action, id, name, schedule, timezone, reminder, channel, chat_id, isEnabled |

Source: `message.go:139-223`

### 2.4 Output Rendering

- **Line limit**: 10 lines max (`maxToolOutputLines`)
- **Byte limit**: 4,096 bytes max (`maxToolOutputBytes`)
- Truncation indicator: `… (N lines hidden)` or `… (N bytes hidden)`
- Output labels vary by tool and status:

| Tool (done) | Label | Tool (error) | Label |
|-------------|-------|--------------|-------|
| `read_file` | CONTENT | any | ERROR |
| `write_file` | RESULT | | |
| `edit_file` | RESULT (+ unified diff) | | |
| `exec` | OUTPUT | | |
| `web_search` | RESULTS | | |
| `message` | DELIVERY | | |
| default | OUTPUT | | |

Source: `message.go:225-275`

### 2.5 Special: Edit File Diff

When `edit_file` completes successfully with `old_string`/`new_string`, renders a unified diff:
```
--- a/path/to/file
+++ b/path/to/file
@@
-old line
+new line
```
Source: `message.go:162-188`, `diff.go`

---

## 3. Vertical Space Consumption

This is the core problem. Each tool block consumes:

| Component | Lines |
|-----------|-------|
| Status + Title header | 1 |
| Input label | 1 |
| Input fields (1-6 lines) | 1–6 |
| Output label | 1 |
| Output content (up to 10) | 1–10 |
| Truncation indicator | 0–1 |
| Blank separator between blocks | 1 |
| **Total per tool block** | **6–20 lines** |

### 3.1 Worst Case: Agent reads 8 files

If the agent calls `read_file` 8 times:
```
┃ ✓ Read file1.go          ← 1
┃ PATH                     ← 2
┃ Path: src/file1.go       ← 3
┃ CONTENT                  ← 4
┃ package main             ← 5
┃ import "fmt"             ← 6
┃ func main() {            ← 7
┃ … (45 lines hidden)      ← 8
                            ← 9 (separator)
┃ ✓ Read file2.go          ← 10
┃ PATH                     ← ...
┃ Path: src/file2.go
┃ CONTENT
┃ ...
... (repeat 6 more times)
```

**8 reads × ~8 lines each = ~64 lines consumed**. On an 80×40 terminal, that's 1.6 screens
of tool blocks before the user sees any assistant response.

### 3.2 Worst Case: exec with long output

```
┃ ✓ Shell                  ← 1
┃ COMMAND                  ← 2
┃ Command: go test ./...   ← 3
┃ OUTPUT                   ← 4
┃ line 1                   ← 5
┃ line 2                   ← 6
┃ ...                      ← ...
┃ line 10                  ← 14
┃ … (58 lines hidden)      ← 15
```

15 lines for a single exec call, even truncated.

---

## 4. Footer / Status Bar

### 4.1 Current Layout

```
model gpt-4o | session tui:main | metadata | 45% (58.5K/131.1K)
```

Source: `internal/components/status/footer.go:145-210`

**Left section**: model name, session key, optional metadata, compression indicator
**Right section**: token usage percentage and counts

### 4.2 What's Missing

- **No tool activity tracking** — the footer has zero awareness of tools
- **No tool call counts** — user cannot see "5 tools ran" at a glance
- **No aggregate summary** — every tool is shown individually in the transcript
- **No compact alternative** to the full tool blocks

### 4.3 Compression Indicator

The footer does show a compression indicator when context compaction occurs:
- While compacting: spinner + "compacting..."
- After: `↓NN%` with colour based on reduction

This is the only dynamic status the footer currently tracks.

---

## 5. Streaming vs Final State

### 5.1 While Running

- Tool block appears immediately with `"running"` status
- Animated spinner cycles: `◐ → ◓ → ◑ → ◒`
- Border colour: yellow/warning
- Output area: empty or "running..."

### 5.2 After Completion

- Spinner replaced with `✓` or `✗`
- Border colour: green (done) or red (error)
- Output populated with truncated result

### 5.3 Thinking / Progress

- `m.progress`: streaming assistant text (transient, cleared on completion)
- `m.thinking`: thinking blocks with duration ("Thought for 2.5s")
- Both rendered as separate blocks in the transcript

---

## 6. Summary of Current Problems

| Problem | Impact |
|---------|--------|
| **Every tool gets its own full block** | 8 reads = 64+ lines of tool blocks |
| **No grouping of same-type tools** | 5 `read_file` calls shown as 5 separate boxes |
| **No aggregation in footer** | User must scroll through blocks to understand what happened |
| **Input fields always shown** | PATH label + path value = 2 extra lines per read |
| **Output always shown (up to 10 lines)** | Even successful reads dump content into transcript |
| **No progressive disclosure** | No way to start collapsed and expand on demand |
| **Labels add vertical noise** | INPUT, COMMAND, OUTPUT, CONTENT labels each take a full line |
| **No natural-language summaries** | Can't say "Read 3 files, ran 2 commands" |
| **Max width 120 chars** | Wide tool blocks on narrow terminals wrap and consume even more space |

---

## 7. Claude Code's Approach (Reference)

Claude Code takes the opposite approach — **minimal by default, detail on demand**.

### 7.1 Tool Grouping & Aggregation

Multiple tools of the same type from the same API response are automatically grouped:
```
● Read 3 files, searched 2 patterns…
  ⎿ Searching src/ for "TODO"
```

Instead of 5 separate blocks, one natural-language line.

Source: `CollapsedReadSearchContent.tsx:294-482`

### 7.2 Information Hierarchy

| Level | What's Shown | When |
|-------|-------------|------|
| **Minimal** | `● Reading…` | Single active tool |
| **Standard** | `● Read 3 files, searched 2 patterns…` | Multiple completed tools |
| **Hint line** | `⎿ Searching src/ for "TODO"` | Current operation detail |
| **Expanded** | Full tool inputs + outputs | User presses Ctrl+O |
| **Verbose** | Every tool separately with full detail | `--verbose` flag |

### 7.3 Verb Tense as Status

- Active: "Reading 3 files…" (present participle)
- Complete: "Read 3 files" (past tense)
- Error: Same wording, red indicator

### 7.4 Footer/Inline Summary

NOT a rigid `✓ Read ×8 | Write ×6` — instead a natural-language sentence:
```
Read 3 files, searched 2 patterns, ran 1 bash command
```

Counts built dynamically using max-ref tracking (prevents jitter during streaming):
```tsx
maxReadCountRef.current = Math.max(maxReadCountRef.current, rawReadCount);
```

### 7.5 Status Indicator

2-character wide indicator replaces the entire tool block:
- `●` (blinking) — running
- `✓` (green) — success
- `✕` (red) — error

### 7.6 Expansion: Ctrl+O

Collapsed groups show a `Ctrl+O to expand` hint. Pressing it reveals full details for that group.

---

## 8. Gap Summary: smolbot vs Claude Code

| Aspect | smolbot (current) | Claude Code | Gap |
|--------|-------------------|-------------|-----|
| Default rendering | Full block per tool | Grouped single-line | **Critical** |
| Same-type grouping | None | Auto-groups 2+ | **Critical** |
| Footer tool counts | None | Natural-language aggregate | **Critical** |
| Vertical space (8 reads) | ~64 lines | ~2 lines | **Critical** |
| Progressive disclosure | Toggle expand only | 4 levels (min→verbose) | **High** |
| Verb tense status | Static labels | Active/complete tense | **Medium** |
| Max-ref jitter prevention | N/A (no aggregation) | useRef max-tracking | **Medium** |
| Expansion keybind | (toggle via cursor?) | Ctrl+O | **Medium** |
| Condensed tool results | Not supported | `style: 'condensed'` | **Medium** |
| AI-powered summaries | None | Haiku 1-liners (optional) | **Low** |

---

## Appendix: Key Source Files

| File | Role |
|------|------|
| `internal/components/chat/message.go` | Tool rendering, title generation, input/output formatting |
| `internal/components/chat/toolblock.go` | Bordered block container, status styling, spinner |
| `internal/components/chat/messages.go` | Message model, tool lifecycle, transcript assembly |
| `internal/components/chat/diff.go` | Unified diff rendering for edit_file |
| `internal/components/status/footer.go` | Footer/status bar rendering |
| `internal/tui/tui.go` | Event dispatch, tool start/done handlers |
