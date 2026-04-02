# Tool UI/UX Implementation Plan 3: Integration Gaps

**Created**: 2026-04-02  
**Status**: Draft  
**Depends On**: Plans 1 & 2 (both complete)

---

## Overview

Plans 1 and 2 built the **infrastructure** for Claude Code tool UI/UX parity. This plan focuses on **integration gaps**—components that were implemented but not wired up, and features that are missing entirely.

### Gap Summary

| Component | Status | Priority | Effort |
|-----------|--------|----------|--------|
| Viewport Culling | ❌ Missing | High | Medium |
| Streaming Shimmer | ❌ Missing | Medium | Low |
| MinDisplayValue | Built, unused | Medium | Low |
| ScrollState | Built, unused | Low | Low |
| BufferedWriter | Built, unused | Low | Low |

---

## Phase 18: Viewport Culling

**Problem**: All messages are rendered every update, even those scrolled off-screen. With 100+ messages, this causes unnecessary CPU usage.

**Solution**: Skip rendering for messages outside the visible viewport range.

### 18.1 Add Visibility Tracking to MessagesModel

```go
// internal/components/chat/messages.go

type MessagesModel struct {
    // ... existing fields ...
    
    // Viewport culling
    lineOffsets []int  // Starting line number for each message
    lastOffset  int    // Current scroll offset for cache invalidation
}
```

### 18.2 Compute Line Offsets After Render

```go
// After rendering all messages, compute cumulative line counts
func (m *MessagesModel) computeLineOffsets() {
    m.lineOffsets = make([]int, len(m.messages)+1)
    offset := 0
    for i, msg := range m.messages {
        m.lineOffsets[i] = offset
        offset += strings.Count(m.renderMessage(msg), "\n") + 1
    }
    m.lineOffsets[len(m.messages)] = offset // sentinel
}
```

### 18.3 Implement Culled Rendering

```go
func (m *MessagesModel) renderTranscriptCulled() string {
    if len(m.messages) == 0 {
        return ""
    }
    
    viewStart := m.viewport.YOffset
    viewEnd := viewStart + m.viewport.Height
    
    var visible []string
    for i, msg := range m.messages {
        msgStart := m.lineOffsets[i]
        msgEnd := m.lineOffsets[i+1]
        
        // Skip if entirely above or below viewport
        if msgEnd < viewStart || msgStart > viewEnd {
            // Add placeholder for correct line numbering
            visible = append(visible, strings.Repeat("\n", msgEnd-msgStart-1))
            continue
        }
        
        visible = append(visible, m.renderMessage(msg))
    }
    
    return strings.Join(visible, "\n")
}
```

### 18.4 Integration

- Call `computeLineOffsets()` after message list changes
- Use `renderTranscriptCulled()` in `View()` instead of `renderTranscript()`
- Add `CullingEnabled bool` flag for toggle/debug
- Benchmark improvement with 100+ message conversations

**Files**: `internal/components/chat/messages.go`

---

## Phase 19: Streaming Shimmer Effect

**Problem**: No visual indication during streaming beyond spinner. Claude Code has a subtle shimmer/highlight sweep.

**Solution**: Add animated highlight to most recent assistant content chunk during streaming.

### 19.1 Theme Shimmer Colors

```go
// internal/theme/theme.go - add to existing theme struct

type Theme struct {
    // ... existing fields ...
    
    // Shimmer animation (for streaming effect)
    ShimmerStart lipgloss.AdaptiveColor
    ShimmerEnd   lipgloss.AdaptiveColor
}
```

### 19.2 Shimmer Frame Calculation

```go
// internal/components/chat/shimmer.go (NEW FILE)
package chat

import "github.com/charmbracelet/lipgloss"

const ShimmerFrameCount = 8

// GetShimmerStyle returns a style for the given frame (0-7)
// Creates a subtle wave effect across text
func GetShimmerStyle(frame int, theme *theme.Theme) lipgloss.Style {
    // Interpolate between base text and highlight
    intensity := float64(frame) / float64(ShimmerFrameCount-1)
    // ... color interpolation logic ...
    return lipgloss.NewStyle().Foreground(color)
}

// ApplyShimmer wraps recent text with animated highlight
func ApplyShimmer(text string, frame int, theme *theme.Theme) string {
    if len(text) == 0 {
        return text
    }
    
    style := GetShimmerStyle(frame, theme)
    // Apply to last N characters (shimmer travels)
    shimmerLen := min(20, len(text))
    offset := (frame * len(text) / ShimmerFrameCount) % len(text)
    
    // ... apply style to text[offset:offset+shimmerLen] ...
}
```

### 19.3 Integration in Message Rendering

```go
// In renderAssistantContent(), during streaming:
if m.isStreaming && isLastChunk {
    content = ApplyShimmer(content, m.shimmerFrame, m.theme)
}
```

### 19.4 Animation Tick

- Reuse existing spinner tick (100ms) for shimmer frame advance
- Add `shimmerFrame` counter to MessagesModel (0-7 cycle)
- Only animate when `isStreaming == true`

**Files**: 
- `internal/components/chat/shimmer.go` (new)
- `internal/components/chat/messages.go`
- `internal/theme/theme.go`

---

## Phase 20: MinDisplayValue Integration

**Problem**: `min_display.go` was built for anti-flicker but never used. Rapid status updates can cause visual noise.

**Solution**: Use MinDisplayValue for status bar and metadata displays.

### 20.1 Wrap Footer Status Text

```go
// internal/components/status/footer.go

type Footer struct {
    // ... existing fields ...
    
    // Anti-flicker wrappers
    modelDisplay    *utils.MinDisplayValue[string]
    tokenDisplay    *utils.MinDisplayValue[string]
    activityDisplay *utils.MinDisplayValue[string]
}

func NewFooter() *Footer {
    return &Footer{
        modelDisplay:    utils.NewMinDisplayValue[string](500 * time.Millisecond),
        tokenDisplay:    utils.NewMinDisplayValue[string](300 * time.Millisecond),
        activityDisplay: utils.NewMinDisplayValue[string](700 * time.Millisecond),
    }
}
```

### 20.2 Update Through MinDisplayValue

```go
func (f *Footer) SetModel(name string) {
    f.modelDisplay.Set(name)
}

func (f *Footer) View() string {
    model := f.modelDisplay.Get()  // Only changes if held for 500ms
    // ... rest of render ...
}
```

### 20.3 Key Locations

| Display | Min Duration | Rationale |
|---------|--------------|-----------|
| Model name | 500ms | Model switches are intentional, brief flicker OK |
| Token count | 300ms | Updates frequently during streaming |
| Tool activity | 700ms | Matches existing hint line hold |
| Metadata text | 500ms | General status area |

**Files**: `internal/components/status/footer.go`

---

## Phase 21: ScrollState Integration

**Problem**: `scroll.go` provides sticky scroll logic but isn't used. Direct viewport manipulation scattered across codebase.

**Solution**: Centralize scroll management through ScrollState.

### 21.1 Add ScrollState to MessagesModel

```go
// internal/components/chat/messages.go

import "github.com/Nomadcxx/smolbot/internal/components/scroll"

type MessagesModel struct {
    // ... existing fields ...
    
    scrollState *scroll.ScrollState
}

func NewMessagesModel() *MessagesModel {
    return &MessagesModel{
        scrollState: scroll.New(24), // Default height, updated on resize
    }
}
```

### 21.2 Wire Up Content Updates

```go
func (m *MessagesModel) afterRender() {
    totalLines := strings.Count(m.rendered, "\n")
    m.scrollState.SetContent(totalLines)
    
    // Sync bubbles viewport with our scroll state
    m.viewport.SetYOffset(m.scrollState.Offset)
}
```

### 21.3 Handle Scroll Events

```go
func (m *MessagesModel) Update(msg tea.Msg) (*MessagesModel, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "up", "k":
            m.scrollState.ScrollBy(-1)
        case "down", "j":
            m.scrollState.ScrollBy(1)
        case "pgup":
            m.scrollState.PageUp()
        case "pgdown":
            m.scrollState.PageDown()
        case "home", "g":
            m.scrollState.ScrollToTop()
        case "end", "G":
            m.scrollState.ScrollToBottom()
        }
        m.viewport.SetYOffset(m.scrollState.Offset)
    }
    // ...
}
```

### 21.4 Benefits

- **Sticky scroll**: Auto-follows new content, breaks when user scrolls up
- **Centralized**: All scroll logic in one place
- **Testable**: ScrollState has comprehensive unit tests

**Files**: `internal/components/chat/messages.go`

---

## Phase 22: BufferedWriter Integration

**Problem**: `debounce.go` BufferedWriter built for event batching but unused. Events processed one-by-one causing frequent redraws.

**Solution**: Batch progress events through BufferedWriter.

### 22.1 Replace Ad-hoc Progress Buffer

```go
// internal/tui/tui.go

type Model struct {
    // ... existing fields ...
    
    // Replace progressBuffer []byte + last flush
    progressWriter *utils.BufferedWriter[ChatProgressMsg]
}

func initialModel() Model {
    return Model{
        progressWriter: utils.NewBufferedWriter(func(batch []ChatProgressMsg) {
            // Merge batch into single update
            combined := mergeProgressBatch(batch)
            // Update messages component once
        }),
    }
}
```

### 22.2 Process Progress Through Buffer

```go
case ChatProgressMsg:
    // Instead of immediate processing:
    // m.messages.AppendContent(msg.Content)
    
    // Buffer and batch:
    m.progressWriter.Write(msg)
    return m, nil  // No immediate redraw

// On timer tick (already exists):
case spinnerTickMsg:
    m.progressWriter.Flush()  // Flush accumulated progress
    // ... spinner advance ...
```

### 22.3 Benefits

- Reduces redraw frequency during fast streaming
- Groups multiple content chunks into single update
- 1000ms max delay matches Claude Code defaults

**Files**: `internal/tui/tui.go`

---

## Implementation Order

```
Phase 18 ─┐
          ├─→ Phase 19 ─→ Phase 22
Phase 20 ─┤
          └─→ Phase 21
```

**Recommended Sequence**:
1. **Phase 20** (MinDisplayValue) - Quick win, low risk
2. **Phase 21** (ScrollState) - Low risk, improves code organization
3. **Phase 18** (Viewport Culling) - Biggest performance impact
4. **Phase 19** (Shimmer) - Visual polish, depends on phase 18 patterns
5. **Phase 22** (BufferedWriter) - Depends on understanding message flow

---

## Success Criteria

| Phase | Metric | Target |
|-------|--------|--------|
| 18 | CPU usage with 100 messages | < 50% of current |
| 19 | Shimmer visible during streaming | Subjective |
| 20 | No flicker on rapid status changes | Visual test |
| 21 | Scroll tests pass, sticky works | Unit tests |
| 22 | Fewer redraws during streaming | Profiler comparison |

---

## Risks

- **Phase 18**: Line offset calculation must stay in sync with actual renders
- **Phase 19**: Shimmer may look janky if frame rate too low
- **Phase 21**: Must keep bubbles viewport and ScrollState in sync
- **Phase 22**: Must not introduce visible lag in content display

---

## Non-Goals

- Full viewport virtualization (would require significant refactor)
- Complex shimmer gradients (keep it subtle)
- Multiple buffer strategies (one size fits all for now)
