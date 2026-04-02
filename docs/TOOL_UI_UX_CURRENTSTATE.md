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

## 7. Claude Code's Approach (Deep Reference)

Claude Code takes the opposite approach — **minimal by default, detail on demand**.
The system is implemented across ~1,600 lines in two major subsystems:
an aggregation engine and a rendering layer.

### 7.1 Architecture Overview

```
API Response (tool_use blocks)
    |
collapseReadSearch.ts -- classifies, accumulates, emits groups
    |
CollapsedReadSearchContent.tsx -- renders groups as natural-language
    |
Individual tool UIs (FileRead, FileWrite, Bash, etc.)
    |
ToolUseLoader.tsx -- 2-char animated status indicator
```

Two **separate** grouping systems exist:
1. **collapseReadSearch** (~1,109 lines): Groups consecutive collapsible tools into
   natural-language summaries. This is the primary system.
2. **groupToolUses** (separate file): Groups 2+ identical tools from the same API
   response — only for tools with a `renderGroupedToolUse` method (currently just AgentTool).

### 7.2 Tool Classification

Every tool use passes through `getToolSearchOrReadInfo()` which returns:

```typescript
type SearchOrReadResult = {
  isCollapsible: boolean       // Can this tool be absorbed into a group?
  isSearch: boolean            // grep, ripgrep, file search
  isRead: boolean              // file read, cat
  isList: boolean              // ls, directory listing
  isREPL: boolean              // bash/shell execution
  isMemoryWrite: boolean       // memory/context operations
  isAbsorbedSilently: boolean  // absorbed without incrementing display counts
}
```

Classification determines whether a tool joins the current group or breaks it.

### 7.3 GroupAccumulator Data Structure

```typescript
// collapseReadSearch.ts:581
type GroupAccumulator = {
  readCount: number        // file reads
  searchCount: number      // grep/search operations
  listCount: number        // directory listings
  bashCount: number        // shell/REPL executions
  mcpCount: number         // MCP tool calls
  memoryCount: number      // memory operations (subtracted from display later)
  filePaths: Set<string>   // unique files touched (for dedup)
  searchPatterns: string[] // patterns searched for
  toolUses: ToolUse[]      // all tool uses in this group
  isActive: boolean        // group still accumulating (tools running)
}
```

Key design: `filePaths` is a `Set<string>` — when rendering "Read N files", N comes
from `filePaths.size` (unique files), not raw `readCount`. This prevents inflation
when the same file is read multiple times.

### 7.4 Collapse Algorithm Flow

```
for each block in API response:
    if block is assistant text:
        -> flush current GroupAccumulator (emit as collapsed group)
        -> emit text block as-is
        -> start new GroupAccumulator

    if block is tool_use:
        classify via getToolSearchOrReadInfo()
        if isCollapsible:
            -> add to current GroupAccumulator
            -> increment relevant count (readCount, searchCount, etc.)
            -> add filePath to Set (if applicable)
        else:
            -> flush current GroupAccumulator
            -> emit tool_use as standalone block (full rendering)
            -> start new GroupAccumulator

    if block is tool_result:
        -> attach to corresponding tool_use (already classified)

end: flush final GroupAccumulator
```

Non-collapsible tools (write_file, edit_file, complex bash) **always break the group**
and render individually with full detail. This is crucial — only "read-like" operations
get collapsed.

### 7.5 Natural-Language Rendering

`CollapsedReadSearchContent.tsx` (483 lines) renders each group:

```typescript
// pushPart() builds comma-separated parts
const parts: string[] = []

if (searchCount > 0) pushPart(`searched for ${searchCount} pattern${s}`)
if (readCount > 0) pushPart(`read ${readCount} file${s}`)
if (listCount > 0) pushPart(`listed ${listCount} director${ies}`)
if (bashCount > 0) pushPart(`ran ${bashCount} bash command${s}`)
if (mcpCount > 0) pushPart(`used ${mcpCount} MCP tool${s}`)

// Memory operations subtracted AFTER accumulation
readCount -= memoryReadCount
searchCount -= memorySearchCount
```

Result: `"Read 3 files, searched 2 patterns, ran 1 bash command"`

#### Verb Tense as Status

| Group State | Tense | Example |
|-------------|-------|---------|
| Active (tools running) | Present participle | "Reading 3 files, searching 2 patterns..." |
| Complete (all done) | Past tense | "Read 3 files, searched 2 patterns" |
| Error | Past tense, red indicator | "Read 3 files" (with red dot) |

### 7.6 Anti-Jitter Pattern (Max-Ref Tracking)

During streaming, tool counts can fluctuate as the API response arrives.
Claude Code prevents display jitter with `useRef`:

```tsx
const maxReadCountRef = useRef(0)
maxReadCountRef.current = Math.max(maxReadCountRef.current, rawReadCount)
// Display uses maxReadCountRef.current, never rawReadCount
```

This ensures counts only ever go **up** during streaming. Once "Read 3 files" appears,
it never flickers back to "Read 2 files" even if intermediate renders have fewer tools.

### 7.7 ToolUseLoader — Status Indicator

A 2-character wide component replaces entire tool blocks:

```tsx
// ToolUseLoader.tsx (41 lines)
<Box minWidth={2}>
  {isLoading ? <Text><Blink>●</Blink></Text> :   // running (animated)
   isSuccess ? <Text color="green">●</Text> :     // success
   <Text color="red">●</Text>}                    // error
</Box>
```

`minWidth={2}` prevents layout shift when the blinking animation toggles.

### 7.8 Hint Line System

Below collapsed groups, a hint line shows the current operation:

```
● Reading 3 files...
  ⎿ Reading src/index.ts (3s · 124 lines)
```

The `⎿` prefix indicates a detail line under the group summary.

**Anti-flicker**: `MIN_HINT_DISPLAY_MS = 700ms` — fast operations that complete
in <700ms don't show a hint line at all, preventing rapid flickering.

### 7.9 Progressive Disclosure — 4 Levels

| Level | Trigger | What's Shown |
|-------|---------|-------------|
| **Minimal** | Default (single active tool) | `● Reading...` (2-char indicator + gerund) |
| **Standard** | Default (group complete) | `● Read 3 files, searched 2 patterns` |
| **Expanded** | `Ctrl+O` | Per-tool detail within group (file paths, line counts) |
| **Verbose** | `--verbose` flag / `Ctrl+O` global | Every tool separately with full input/output |

`Ctrl+O` toggles a global `verbose` state. All tool groups re-render — collapsed groups
expand to show individual tools, individual tools show full I/O.

Sub-agent contexts receive `style: 'condensed'` hint, making their tool rendering
even more compact than the main conversation.

### 7.10 Individual Tool UIs

Even at expanded/verbose level, each tool type has a **purpose-built compact UI**:

| Tool | Compact Display | Detail Display |
|------|----------------|----------------|
| **FileRead** | `Read src/main.go` | `Read 124 lines from src/main.go` |
| **FileWrite** | `Wrote src/main.go` | First 10 lines + `... +N lines` |
| **Bash** | `Ran command` | `go test ./... (5s · 42 lines)` with exit code |
| **Grep** | `Searched for "TODO"` | `Found 12 matches in 4 files` |
| **Agent** | `Agent completed task` | Sub-agent transcript (condensed) |

Key: even "expanded" tools show **summary metadata** (line counts, durations, file counts),
not raw output. Raw output only appears in verbose mode.

### 7.11 Dedup Pattern

File reads deduplicate by path:
```typescript
const uniqueFiles = new Set(toolUses.map(t => t.filePath))
const displayCount = uniqueFiles.size  // not toolUses.length
```

If the agent reads `main.go` 3 times, it shows "Read 1 file" not "Read 3 files".

### 7.12 Memory Subtraction

Memory/context operations (e.g., reading from memory bank) are tracked in the
GroupAccumulator but **subtracted from display counts**:

```typescript
const displayReadCount = totalReadCount - memoryReadCount
```

This prevents user confusion — "Read 5 files" means 5 real files, not 3 files + 2 memory reads.

---

## 8. Gap Summary: smolbot vs Claude Code

| Aspect | smolbot (current) | Claude Code | Gap |
|--------|-------------------|-------------|-----|
| Default rendering | Full block per tool | Grouped single-line | **Critical** |
| Same-type grouping | None | Auto-groups consecutive collapsible tools | **Critical** |
| Footer tool counts | None | Natural-language aggregate in-line | **Critical** |
| Vertical space (8 reads) | ~64 lines | ~2 lines | **Critical** |
| Tool classification | None — all tools rendered identically | `SearchOrReadResult` classifies each tool | **Critical** |
| GroupAccumulator | None | Stateful accumulator with counts + file dedup | **Critical** |
| Progressive disclosure | Toggle expand per-tool only | 4 levels (minimal to verbose) | **High** |
| Verb tense status | Static labels (INPUT/OUTPUT) | Active participle / past tense | **High** |
| Individual tool UIs | Same bordered block for all tools | Purpose-built compact UI per tool type | **High** |
| Anti-jitter (streaming) | N/A (no aggregation) | `useRef` max-tracking | **Medium** |
| Expansion keybind | None documented | `Ctrl+O` global toggle | **Medium** |
| Hint line (current op) | None | `⎿` prefix with operation detail, 700ms anti-flicker | **Medium** |
| Metadata in tool display | None (no line counts, durations) | Line counts, durations, file counts | **Medium** |
| Condensed sub-agent tools | Not supported | `style: 'condensed'` context hint | **Medium** |
| File read dedup | N/A | `Set<filePath>.size` for unique count | **Medium** |
| Memory subtraction | N/A | Memory ops subtracted from display counts | **Low** |

---

## 9. Architectural Mapping: React/Ink to Go/Bubbletea

Key differences to account for when reimplementing:

### 9.1 State Management

| Claude Code (React/Ink) | smolbot (Go/Bubbletea) |
|-------------------------|------------------------|
| `useRef` for max-tracking | Field on model struct (`maxReadCount int`) |
| `useState` + re-render | `tea.Msg` dispatched to `Update()` |
| Component-local state | Model struct fields |
| `useMemo`/`useCallback` | Computed in `View()` or cached in model |

### 9.2 Proposed Go Types

```go
// Tool classification (equivalent of SearchOrReadResult)
type ToolClass int
const (
    ToolClassCollapsible ToolClass = iota  // read, search, list
    ToolClassStandalone                     // write, edit, complex bash
)

type ToolKind int
const (
    ToolKindRead ToolKind = iota
    ToolKindSearch
    ToolKindList
    ToolKindBash
    ToolKindMCP
    ToolKindMemory
    ToolKindOther
)

// Equivalent of GroupAccumulator
type ToolGroup struct {
    ReadCount     int
    SearchCount   int
    ListCount     int
    BashCount     int
    MCPCount      int
    MemoryCount   int
    FilePaths     map[string]bool  // Set equivalent for dedup
    SearchQueries []string
    Tools         []ToolCall       // all tools in group
    IsActive      bool             // still accumulating
    MaxReadCount  int              // anti-jitter: only increases
    MaxSearchCount int
}

// Classification function
func classifyTool(name string, input string) (ToolClass, ToolKind)

// Collapse engine
func collapseToolGroups(tools []ToolCall, assistantBreaks []int) []ToolGroup

// Natural language rendering
func (g ToolGroup) Summary(active bool) string
// active=true:  "Reading 3 files, searching 2 patterns..."
// active=false: "Read 3 files, searched 2 patterns"
```

### 9.3 Integration Points

| Component | Change Required |
|-----------|----------------|
| `messages.go` | Replace per-tool rendering with group-based rendering |
| `toolblock.go` | New compact group renderer alongside existing block renderer |
| `message.go` | Add `classifyTool()`, per-tool compact summaries |
| `footer.go` | Add tool activity counters (running/done/error counts) |
| `tui.go` | Handle `Ctrl+O` toggle, pass verbose state to renderers |
| `messages.go` | New `collapseToolGroups()` in `renderTranscript()` pipeline |

### 9.4 Rendering Strategy

```
renderTranscript() [messages.go]
    |
    +-- for each message: render as before
    |
    +-- for tools: collapseToolGroups(m.tools, breakpoints)
            |
            +-- CollapsibleGroup -> renderToolGroup() [NEW]
            |       |
            |       +-- verbose=false: "● Read 3 files, searched 2 patterns"
            |       +-- verbose=true:  individual tool blocks (existing)
            |
            +-- StandaloneBlock -> renderToolCall() [existing, enhanced]
                    |
                    +-- write_file: show first 10 lines + "... +N lines"
                    +-- edit_file: show diff (existing)
                    +-- exec: show "5s · 42 lines" metadata
```

---

## Appendix A: Key Source Files — smolbot

| File | Role |
|------|------|
| `internal/components/chat/message.go` | Tool rendering, title generation, input/output formatting |
| `internal/components/chat/toolblock.go` | Bordered block container, status styling, spinner |
| `internal/components/chat/messages.go` | Message model, tool lifecycle, transcript assembly |
| `internal/components/chat/diff.go` | Unified diff rendering for edit_file |
| `internal/components/status/footer.go` | Footer/status bar rendering |
| `internal/tui/tui.go` | Event dispatch, tool start/done handlers |

## Appendix B: Key Source Files — Claude Code

| File | Role |
|------|------|
| `src/utils/collapseReadSearch.ts` (~1,109 lines) | Aggregation engine: classify, accumulate, emit groups |
| `src/components/messages/CollapsedReadSearchContent.tsx` (483 lines) | Render groups as natural-language sentences |
| `src/components/ToolUseLoader.tsx` (41 lines) | 2-char animated status indicator |
| `src/utils/groupToolUses.ts` | Secondary grouping for identical tools (AgentTool) |
| `src/components/tools/FileReadTool.tsx` | Individual FileRead compact UI |
| `src/components/tools/FileWriteTool.tsx` | Individual FileWrite compact UI |
| `src/components/tools/BashTool.tsx` | Individual Bash compact UI with duration |
| `src/components/tools/GrepTool.tsx` | Individual Grep compact UI with match counts |
