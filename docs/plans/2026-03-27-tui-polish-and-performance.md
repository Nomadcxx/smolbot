# TUI Polish & Performance — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix UX gaps that prevent smolbot from feeling like a polished terminal tool: clipboard support, input/scroll performance, skills visibility, usage/cost tracking, and the incorrect RAMA theme.

**Architecture:** All changes are TUI-layer or provider-layer. No gateway protocol changes needed except for a minor `skills.list` enhancement.

**Tech Stack:** Go 1.26, charm.land/bubbletea v2.0.2, charm.land/lipgloss v2.0.2

---

## Current State Summary

| Area | Status | Problem |
|------|--------|---------|
| Clipboard | Not implemented | Can't copy/paste in TUI at all |
| Performance | Functional but sluggish | Full content rebuild on every dirty frame; `lipgloss.Width()` in hot paths; no incremental rendering |
| Skills | 8 builtin skills exist | TUI has a skills dialog component but it's unclear if the gateway's `skills.list` method is wired |
| Usage/Cost | Tokens tracked, cost not | No per-request cost, no session totals, no model pricing awareness |
| RAMA Theme | Wrong palette | Uses Mediterranean/earthy colors (#F4A261 orange, #2A9D8F teal) instead of canonical RAMA (Space Cadet #2b2d42, Red Pantone #ef233c, Anti-flash White #edf2f4) |

---

## File Map

| File | Change |
|------|--------|
| `internal/components/common/clipboard.go` | **New** — OSC 52 clipboard writer |
| `internal/tui/tui.go` | Add clipboard keybinding; optimize Update path; wire skills dialog |
| `internal/components/chat/messages.go` | Incremental rendering; cache per-message renders |
| `internal/components/chat/editor.go` | Paste support via OSC 52 or bracketed paste |
| `internal/components/status/footer.go` | Cache `lipgloss.Width()` results; add cost display |
| `internal/theme/themes/rama.go` | Rewrite with canonical RAMA palette from syscgo |
| `pkg/provider/types.go` | Add cost fields to Usage struct |
| `pkg/provider/openai.go` | Parse cost from response metadata if available |
| `internal/client/types.go` | Add cost fields to UsageInfo/UsagePayload |
| `pkg/gateway/server.go` | Wire `skills.list` method if missing; add cost to usage events |

---

## Task 1: Fix RAMA Theme

**Files:**
- Modify: `internal/theme/themes/rama.go`

The RAMA theme currently uses a Mediterranean palette that doesn't match the canonical RAMA definition from syscgo. The real RAMA palette is a French Flag-inspired scheme: deep navy backgrounds, bold reds, cool grays, anti-flash white text.

**Reference palette from `/home/nomadx/sysc-Go/animations/palettes.go`:**

| Name | Hex | Usage |
|------|-----|-------|
| Space Cadet | `#2b2d42` | Background, dark base |
| Cool Gray | `#8d99ae` | Muted text, borders, comments |
| Fire Engine Red | `#d90429` | Strong accent, errors, danger |
| Red Pantone | `#ef233c` | Primary color, highlights, active |
| Anti-flash White | `#edf2f4` | Text, foreground |

Additional derived colors from syscgo:
- Gradient stops: `#ef233c` → `#d90429` → `#edf2f4`
- Beam colors: `#ffffff` → `#ef233c` → `#d90429`
- Burn progression includes: `#ffd700`, `#ff8c00`, `#ff4500`, `#dc143c`, `#8b0000`

- [ ] **Step 1: Rewrite rama.go**

```go
func init() {
	register("rama", [15]string{
		"#2b2d42", // [0]  Background - Space Cadet
		"#2b2d42", // [1]  Panel - same as bg
		"#2b2d42", // [2]  Element - same as bg
		"#8d99ae", // [3]  Border - Cool Gray
		"#ef233c", // [4]  BorderFocus - Red Pantone
		"#ef233c", // [5]  Primary - Red Pantone
		"#8d99ae", // [6]  Secondary - Cool Gray
		"#d90429", // [7]  Accent - Fire Engine Red
		"#edf2f4", // [8]  Text - Anti-flash White
		"#8d99ae", // [9]  TextMuted - Cool Gray
		"#d90429", // [10] Error - Fire Engine Red
		"#ffd700", // [11] Warning - Gold (from burn palette)
		"#8d99ae", // [12] Success - Cool Gray (subdued)
		"#edf2f4", // [13] Info - Anti-flash White
		"#8d99ae", // [14] ToolBorder - Cool Gray
	}, func(t *theme.Theme) {
		t.ToolName = lipgloss.Color("#ef233c")
		t.TranscriptUserAccent = lipgloss.Color("#ef233c")
		t.TranscriptAssistantAccent = lipgloss.Color("#8d99ae")
		t.TranscriptThinking = lipgloss.Color("#8d99ae")
		t.TranscriptStreaming = lipgloss.Color("#edf2f4")
		t.TranscriptError = lipgloss.Color("#d90429")
		t.MarkdownHeading = lipgloss.Color("#ef233c")
		t.MarkdownLink = lipgloss.Color("#d90429")
		t.MarkdownCode = lipgloss.Color("#edf2f4")
		t.SyntaxKeyword = lipgloss.Color("#ef233c")
		t.SyntaxString = lipgloss.Color("#edf2f4")
		t.SyntaxComment = lipgloss.Color("#8d99ae")
		t.ToolStateRunning = lipgloss.Color("#ffd700")
		t.ToolStateDone = lipgloss.Color("#8d99ae")
		t.ToolStateError = lipgloss.Color("#d90429")
		t.ToolArtifactHeader = lipgloss.Color("#1a1c2a")
		t.ToolArtifactBody = lipgloss.Color("#2b2d42")
	})
}
```

The theme should feel: dark navy base, bold red accents that pop, clean white text, gray for secondary elements. No orange, no teal, no earth tones.

- [ ] **Step 2: Verify in TUI**

Build and confirm the theme looks correct with all transcript elements (user, assistant, thinking, streaming, tool blocks).

---

## Task 2: Copy-to-Clipboard (OSC 52)

**Files:**
- New: `internal/components/common/clipboard.go`
- Modify: `internal/tui/tui.go`

OSC 52 is the standard terminal escape sequence for clipboard access. Supported by most modern terminals (kitty, iTerm2, wezterm, alacritty, foot).

- [ ] **Step 1: Implement OSC 52 writer**

```go
// internal/components/common/clipboard.go
package common

import (
	"encoding/base64"
	"fmt"
	"io"
)

// WriteOSC52 writes text to the system clipboard via OSC 52 escape sequence.
func WriteOSC52(w io.Writer, text string) {
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	fmt.Fprintf(w, "\033]52;c;%s\a", encoded)
}
```

- [ ] **Step 2: Add copy keybinding**

In `tui.go`, when not in editor focus and no dialog open, `c` copies the last assistant message:

```go
case "c":
	if m.dialog == nil && !m.editor.Focused() {
		if content := m.messages.LastAssistantContent(); content != "" {
			common.WriteOSC52(os.Stdout, content)
			m.status.SetFlash("Copied to clipboard")
		}
		return m, nil
	}
```

Also add `y` as alias (vim-style yank).

- [ ] **Step 3: Add paste support via bracketed paste**

Bubbletea v2 supports bracketed paste mode natively. Verify it's enabled and that `tea.PasteMsg` events are forwarded to the editor's textarea. If not, add handling in the editor:

```go
case tea.PasteMsg:
	// Insert pasted text into textarea
	m.textarea.InsertText(string(msg))
```

- [ ] **Step 4: Add flash/notification for clipboard feedback**

Add a brief status line flash message that shows "Copied to clipboard" for 2 seconds, then clears.

- [ ] **Step 5: Write tests**

Test OSC 52 output format. Test keybinding triggers copy. Test paste event forwarding.

---

## Task 3: Scroll & Render Performance

**Files:**
- Modify: `internal/components/chat/messages.go`
- Modify: `internal/components/status/footer.go`
- Modify: `internal/tui/tui.go`

The core performance issue: `renderContent()` iterates ALL messages and re-renders markdown for every one when dirty. This is O(n) on every content change. The footer calls `lipgloss.Width()` multiple times per frame.

- [ ] **Step 1: Per-message render cache**

Instead of re-rendering all messages when dirty, cache each rendered message individually:

```go
type messageCache struct {
	role     string
	content  string
	rendered string
	width    int
}

type MessagesModel struct {
	// ... existing
	cache []messageCache
}
```

When a new message arrives or content changes, only re-render the affected message. Append it to the cached output string.

In `renderContent()`:
```go
func (m *MessagesModel) renderContent() string {
	var blocks []string
	for i, msg := range m.history {
		if i < len(m.cache) && m.cache[i].content == msg.content && m.cache[i].width == m.width {
			blocks = append(blocks, m.cache[i].rendered)
			continue
		}
		rendered := m.renderMessage(msg)
		m.cache = append(m.cache[:i], messageCache{...})
		blocks = append(blocks, rendered)
	}
	// ... append progress/thinking
}
```

- [ ] **Step 2: Cache lipgloss.Width() in footer**

In `footer.go`, precompute widths on `SetWidth()` and reuse:

```go
func (m *FooterModel) SetWidth(w int) {
	m.width = w
	// Precompute any fixed-width elements
	m.cachedModelWidth = lipgloss.Width(m.model)
	m.cachedSessionWidth = lipgloss.Width(m.session)
}
```

In `View()`, use cached values instead of recalculating.

- [ ] **Step 3: Debounce rapid progress events**

During streaming, `chat.progress` events arrive every few milliseconds. Each one triggers a re-render. Batch these:

```go
// In event handler, accumulate progress into buffer
case "chat.progress":
	m.progressBuffer.WriteString(content)
	if !m.progressFlushPending {
		m.progressFlushPending = true
		return m, tea.Tick(16*time.Millisecond, func(time.Time) tea.Msg {
			return flushProgressMsg{}
		})
	}

case flushProgressMsg:
	m.progressFlushPending = false
	m.messages.SetProgress(m.messages.GetProgress() + m.progressBuffer.String())
	m.progressBuffer.Reset()
```

This caps rendering at ~60fps during streaming instead of per-token.

- [ ] **Step 4: Profile and verify**

Use `go test -bench` or manual observation to verify scroll and input responsiveness improve.

- [ ] **Step 5: Write tests**

Test message cache invalidation on width change. Test progress batching coalesces multiple deltas.

---

## Task 4: Skills Visibility & Gateway Wiring

**Files:**
- Modify: `pkg/gateway/server.go` (verify `skills.list` method)
- Modify: `internal/tui/tui.go` (wire skills to F1 menu)
- Modify: `internal/tui/menu_dialog.go` (add Skills menu item)

8 builtin skills exist (`clawhub`, `cron`, `github`, `memory`, `skill-creator`, `summarize`, `tmux`, `weather`). The skills dialog component exists in `internal/components/dialog/skills.go`. Need to verify the full pipeline works.

- [ ] **Step 1: Verify `skills.list` gateway method**

Check if `pkg/gateway/server.go` has a `skills.list` handler. If not, add one:

```go
case "skills.list":
	names := s.skillRegistry.Names()
	skills := make([]map[string]any, 0, len(names))
	for _, name := range names {
		sk, _ := s.skillRegistry.Get(name)
		status := "available"
		if !sk.Available { status = "unavailable" }
		if sk.AlwaysOn { status = "always" }
		skills = append(skills, map[string]any{
			"name":        sk.Name,
			"description": sk.Description,
			"status":      status,
		})
	}
	return map[string]any{"skills": skills}, nil
```

- [ ] **Step 2: Wire skills dialog to F1 menu**

In `menu_dialog.go`, add "Skills" item to root menu. When selected, load skills from gateway and show the skills dialog.

- [ ] **Step 3: Add `/skills` slash command**

```go
case "skills":
	return m, m.loadSkillsCmd()
```

- [ ] **Step 4: Write tests**

Test skills list renders with available/unavailable/always states.

---

## Task 5: Usage Cost Tracking

**Files:**
- Modify: `pkg/provider/types.go`
- Modify: `pkg/provider/openai.go`
- Modify: `pkg/gateway/server.go`
- Modify: `internal/client/types.go`
- Modify: `internal/components/status/footer.go`

Ollama doesn't charge per-token, but cloud APIs (Anthropic, OpenAI, etc.) do. We should track and display cost when pricing data is available.

- [ ] **Step 1: Add cost fields to provider types**

```go
// pkg/provider/types.go
type Usage struct {
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	Cost             float64 `json:"cost,omitempty"` // USD, if known
}
```

- [ ] **Step 2: Add model pricing lookup**

Create a simple pricing map for known models:

```go
// pkg/provider/pricing.go
var modelPricing = map[string]struct {
	PromptPerMillion  float64
	OutputPerMillion  float64
}{
	"claude-sonnet-4-5-20250514": {3.0, 15.0},
	"claude-opus-4-6":            {15.0, 75.0},
	"claude-haiku-4-5-20251001":  {0.80, 4.0},
	"gpt-4o":                     {2.50, 10.0},
	"gpt-4o-mini":                {0.15, 0.60},
}

func EstimateCost(model string, prompt, completion int) float64 {
	p, ok := modelPricing[model]
	if !ok { return 0 }
	return (float64(prompt)/1_000_000)*p.PromptPerMillion +
		(float64(completion)/1_000_000)*p.OutputPerMillion
}
```

- [ ] **Step 3: Add cost to gateway usage events**

In `server.go`, when emitting `chat.usage`, include estimated cost:

```go
cost := provider.EstimateCost(s.currentModel(), pt, ct)
s.emitEvent(state.owner, "chat.usage", map[string]any{
	// ... existing fields
	"cost": cost,
})
```

Also track cumulative session cost in `lastUsage`.

- [ ] **Step 4: Display cost in footer**

In `renderUsage()`, if cost > 0, append:

```go
if m.usage.Cost > 0 {
	costStr := fmt.Sprintf("$%.4f", m.usage.Cost)
	// Render after token counts
}
```

Format: `78% (99.8K/128K) $0.0042`

For Ollama/local models where cost is 0, don't show cost.

- [ ] **Step 5: Add cumulative session cost**

Track total cost across all requests in a session. Display in sidebar context section.

- [ ] **Step 6: Write tests**

Test pricing lookup for known and unknown models. Test cost display formatting. Test zero-cost suppression for local models.

---

## Priority Order

| Priority | Task | Rationale |
|----------|------|-----------|
| P0 | Task 1: Fix RAMA theme | Quick fix, immediately visible improvement |
| P0 | Task 2: Clipboard support | Fundamental UX — can't copy/paste is a blocker |
| P1 | Task 3: Performance | Directly impacts perceived quality; lag = frustration |
| P1 | Task 4: Skills visibility | Features exist but are invisible — low effort to wire |
| P2 | Task 5: Cost tracking | Nice to have, not critical for Ollama users |

---

## Testing Strategy

- Theme: visual verification + unit test that RAMA colors match expected hex values
- Clipboard: unit test for OSC 52 encoding; integration test for keybinding
- Performance: benchmark `renderContent()` with 50+ messages before/after cache
- Skills: integration test for `skills.list` gateway → TUI dialog
- Cost: unit test for pricing calculation; test display formatting

## Risks

1. **OSC 52 terminal support**: Not all terminals support OSC 52. May need a fallback to `xclip`/`wl-copy`. Detect via `$TERM` or attempt OSC 52 and fall back.
2. **Per-message cache invalidation**: Must invalidate on width change, theme change, and content change. Over-caching causes stale renders.
3. **Progress batching**: 16ms batch window introduces slight visual delay. May need tuning (8ms, 32ms). Should be configurable.
4. **Model pricing staleness**: Hardcoded prices go stale. Consider loading from config or a prices.json file.
