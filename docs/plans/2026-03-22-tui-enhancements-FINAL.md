# TUI Enhancements - FINAL Revised Implementation Plan

**Date:** March 22, 2026
**Status:** FINAL REVISED - Ready for Agent Handoff
**Focus Area:** Area 3 - TUI Enhancements
**Repository:** `/home/nomadx/Documents/smolbot`

## Revision History

| Version | Date | Changes |
|---------|------|---------|
| REVISED | 2026-03-22 | Initial revision with gate structure |
| FINAL | 2026-03-22 | Added crush-inspired patterns, improved footer design |

## Reference Projects

- **Crush** (`/home/nomadx/crush`): Go-based AI coding assistant by Charm
  - Key insight: Tool status states (awaiting, running, success, error, canceled)
  - Key insight: Compact vs expanded rendering for tools
  - Key insight: Animation/spinner for running tools
- **Nanocoder** (`/home/nomadx/nanocoder`): TypeScript/React CLI
  - Key insight: Token usage display with percentage
  - Key insight: Compression indicator in footer
  - Key insight: Color-coded usage levels (green/yellow/red)

---

## ARCHITECTURE OVERVIEW

```
internal/
├── client/
│   ├── protocol.go       # MODIFY - Add CompressionInfo type
│   └── messages.go       # MODIFY - Add CompressionStatus method
├── components/status/
│   └── footer.go        # MODIFY - Add compression indicator
├── tui/
│   └── tui.go           # MODIFY - Add CompressionStatusMsg handler
└── theme/
    ├── theme.go          # READ - Check compression colors
    └── themes/          # READ - Theme color assignments
```

**Key Design Decisions (from crush analysis):**

1. **Footer compression indicator**: Simple `↓XX%` format inspired by nanocoder
2. **Color-coded by intensity**: Green (light), Amber (moderate), Red (heavy)
3. **Tool status states**: Use crush-style states for future tool display
4. **Compact mode support**: Footer works in compact and expanded modes

---

## GATE 0: Foundation - Types and Protocol (BLOCKING)

**Purpose:** Define compression types that both backend (Area 2) and frontend share.

**Entry Gate:** Area 2 compression types exist
**Exit Gate:** Types compile, client can represent compression info

### Task 0.1: Add CompressionInfo Type

**File:** `internal/client/protocol.go` (MODIFY)

**Add at end of file:**

```go
// CompressionInfo contains context compression state for UI display
type CompressionInfo struct {
    Enabled           bool    `json:"enabled"`
    Mode             string  `json:"mode"`               // conservative, default, aggressive
    LastRun          string  `json:"lastRun,omitempty"`  // ISO timestamp
    OriginalTokens    int     `json:"originalTokens"`
    CompressedTokens int     `json:"compressedTokens"`
    ReductionPercent float64 `json:"reductionPercent"`   // 0-100
}

// UsageLevel categorizes token usage for color coding
type UsageLevel int

const (
    UsageLevelLow UsageLevel = iota    // < 60%
    UsageLevelMedium                   // 60-80%
    UsageLevelHigh                    // 80-90%
    UsageLevelCritical               // > 90%
)
```

**Verification:** `cd /home/nomadx/Documents/smolbot && go build ./internal/client/...`
**Expected:** SUCCESS

**Commit:** `git add internal/client/protocol.go && git commit -m "feat(client): add CompressionInfo type"`

---

### Task 0.2: Check Theme Compression Colors

**File:** `internal/theme/theme.go` (READ + MODIFY if needed)

**Check if these fields exist:**
```go
CompressionActive  color.Color
CompressionSuccess color.Color
CompressionWarning color.Color
TokenHighUsage     color.Color
```

**If NOT present, add:**
```go
// Compression indicator colors (inspired by nanocoder)
CompressionActive  color.Color // Light compression (20-40%)
CompressionSuccess color.Color // Moderate compression (40-60%)  
CompressionWarning color.Color // High compression (>60%)
TokenHighUsage     color.Color // Token usage critical (>90%)
```

**Check themes in `internal/theme/themes/` - ensure each theme sets these colors.**

**Verification:** `cd /home/nomadx/Documents/smolbot && go build ./internal/theme/...`
**Expected:** SUCCESS

**Commit:** `git add internal/theme/theme.go internal/theme/themes/ && git commit -m "feat(theme): add compression indicator colors"`

---

## GATE 1: Footer Enhancement (PARALLEL - No Dependencies)

**Purpose:** Extend the existing footer to display compression status and enhanced token usage.

**Entry Gate:** Gate 0 complete
**Exit Gate:** `go test ./internal/components/status/... -v` passes

### Task 1.1: Add Compression to FooterModel

**File:** `internal/components/status/footer.go` (MODIFY)

**Add to FooterModel struct:**
```go
type FooterModel struct {
    // ... existing fields ...
    compression *client.CompressionInfo
}
```

**Add setter method:**
```go
func (m *FooterModel) SetCompression(info *client.CompressionInfo) {
    m.compression = info
}
```

---

### Task 1.2: Add Compression Rendering Method

**File:** `internal/components/status/footer.go` (MODIFY)

**Add new method:**

```go
func (m FooterModel) renderCompression(t *theme.Theme) string {
    if m.compression == nil || !m.compression.Enabled {
        return ""
    }
    
    pct := m.compression.ReductionPercent
    
    // Choose style based on reduction percentage (inspired by nanocoder)
    var style lipgloss.Style
    indicator := "↓" // Down arrow indicates compression
    
    switch {
    case pct >= 60: // Heavy compression
        style = lipgloss.NewStyle().
            Foreground(t.CompressionWarning).
            Bold(true)
        indicator += fmt.Sprintf("%.0f%%", pct)
    case pct >= 30: // Moderate compression
        style = lipgloss.NewStyle().
            Foreground(t.CompressionSuccess)
        indicator += fmt.Sprintf("%.0f%%", pct)
    default: // Light compression
        style = lipgloss.NewStyle().
            Foreground(t.CompressionActive)
        indicator += fmt.Sprintf("%.0f%%", pct)
    }
    
    return style.Render(indicator)
}
```

---

### Task 1.3: Update View() to Include Compression

**File:** `internal/components/status/footer.go` (MODIFY)

**Find the `parts := []string{` section in View() and add compression:**

```go
parts := []string{
    "model " + footerValue(m.app.Model, "connecting..."),
    "session " + footerValue(m.app.Session, "none"),
}

// NEW: Add compression indicator if available
if comp := m.renderCompression(t); comp != "" {
    parts = append(parts, comp)
}
```

---

### Task 1.4: Enhance Token Usage Display

**File:** `internal/components/status/footer.go` (MODIFY)

**Update `renderUsage()` to use color coding:**

```go
func (m FooterModel) renderUsage(t *theme.Theme, compact bool) string {
    if m.usage.ContextWindow <= 0 || m.usage.TotalTokens <= 0 {
        return ""
    }

    percentage := (float64(m.usage.TotalTokens) / float64(m.usage.ContextWindow)) * 100
    
    // Color coding inspired by nanocoder (green → yellow → red)
    var percentStyle lipgloss.Style
    if percentage >= 90 {
        percentStyle = lipgloss.NewStyle().Foreground(t.TokenHighUsage).Bold(true)
    } else if percentage >= 80 {
        percentStyle = lipgloss.NewStyle().Foreground(t.Warning).Bold(true)
    } else if percentage >= 60 {
        percentStyle = lipgloss.NewStyle().Foreground(t.CompressionWarning)
    } else {
        percentStyle = lipgloss.NewStyle().Foreground(t.TextMuted)
    }

    percentText := percentStyle.Render(fmt.Sprintf("%d%%", int(percentage+0.5)))
    
    if compact {
        return percentText + " " + lipgloss.NewStyle().
            Background(t.Panel).
            Foreground(t.TextMuted).
            Render("(" + formatUsageTokens(m.usage.TotalTokens) + ")")
    }

    return percentText + " " + lipgloss.NewStyle().
        Background(t.Panel).
        Foreground(t.TextMuted).
        Render(fmt.Sprintf("(%s/%s)", formatUsageTokens(m.usage.TotalTokens), formatUsageTokens(m.usage.ContextWindow)))
}
```

---

### Task 1.5: Write Footer Tests

**File:** `internal/components/status/footer_test.go` (MODIFY)

**Add tests:**

```go
func TestFooterCompressionIndicator(t *testing.T) {
    a := &mockApp{}
    f := NewFooter(a)
    f.SetWidth(100)
    
    f.SetCompression(&client.CompressionInfo{
        Enabled:          true,
        ReductionPercent: 35.0,
    })
    
    view := f.View()
    
    require.Contains(t, view, "↓35%")
}

func TestFooterCompressionHighReduction(t *testing.T) {
    a := &mockApp{}
    f := NewFooter(a)
    f.SetWidth(100)
    
    f.SetCompression(&client.CompressionInfo{
        Enabled:          true,
        ReductionPercent: 65.0, // High - should use warning color
    })
    
    view := f.View()
    require.Contains(t, view, "↓65%")
}

func TestFooterCompressionDisabled(t *testing.T) {
    a := &mockApp{}
    f := NewFooter(a)
    f.SetWidth(100)
    
    f.SetCompression(&client.CompressionInfo{Enabled: false})
    view := f.View()
    
    // Should not show compression indicator
    require.NotContains(t, view, "↓")
}

func TestFooterTokenUsageColorCoding(t *testing.T) {
    a := &mockApp{}
    f := NewFooter(a)
    f.SetWidth(100)
    
    // Test high usage (>90%)
    f.SetUsage(client.UsageInfo{
        TotalTokens:     9000,
        ContextWindow:   10000,
    })
    view := f.View()
    require.Contains(t, view, "90%")
    
    // Test medium usage (60-80%)
    f.SetUsage(client.UsageInfo{
        TotalTokens:     7000,
        ContextWindow:   10000,
    })
    view = f.View()
    require.Contains(t, view, "70%")
}
```

**Run:** `cd /home/nomadx/Documents/smolbot && go test ./internal/components/status/... -v`
**Expected:** All tests PASS

**Commit:** `git add internal/components/status/footer.go internal/components/status/footer_test.go && git commit -m "feat(tui): add compression indicator and token color coding to footer"`

---

## GATE 2: Client Protocol (PARALLEL with Gate 1)

**Purpose:** Add compression status method to client for fetching stats from backend.

**Entry Gate:** Gate 0 complete
**Exit Gate:** `go build ./internal/client/...` succeeds

### Task 2.1: Review Backend API Structure

**Files:** `pkg/gateway/server.go` or `cmd/smolbot/main.go` (READ)

**Understand how routes are registered.**

---

### Task 2.2: Add CompressionStatus Client Method

**File:** `internal/client/messages.go` (MODIFY)

**Add method:**

```go
// CompressionStatus fetches compression statistics for a session
func (c *Client) CompressionStatus(session string) (*CompressionInfo, error) {
    if session == "" {
        return &CompressionInfo{Enabled: false}, nil
    }
    
    resp, err := c.Get("/api/compression/" + session)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode == http.StatusNotFound {
        return &CompressionInfo{Enabled: false}, nil
    }
    
    var info CompressionInfo
    if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
        return nil, err
    }
    return &info, nil
}
```

**Add import if not present:**
```go
import (
    "encoding/json"
    "net/http"
    // ... existing imports
)
```

**Verification:** `cd /home/nomadx/Documents/smolbot && go build ./internal/client/...`
**Expected:** SUCCESS

**Commit:** `git add internal/client/messages.go && git commit -m "feat(client): add CompressionStatus method"`

---

## GATE 3: TUI Integration (BLOCKING - Depends on Gates 1 & 2)

**Purpose:** Wire compression status into the TUI message loop.

**Entry Gate:** Gates 1 and 2 complete
**Exit Gate:** `go build ./internal/tui/...` succeeds

### Task 3.1: Review TUI Message Handling

**File:** `internal/tui/tui.go` (READ)

**Find:**
1. Where message types are defined
2. Where Update() handles incoming messages
3. Where status sync happens

---

### Task 3.2: Add CompressionStatusMsg Type

**File:** `internal/tui/tui.go` (MODIFY)

**Add to message types section:**

```go
// CompressionStatusMsg indicates compression stats received from backend
type CompressionStatusMsg struct {
    Info client.CompressionInfo
}
```

---

### Task 3.3: Handle CompressionStatusMsg in Update

**File:** `internal/tui/tui.go` (MODIFY)

**Find the Update() method's switch statement and add:**

```go
case CompressionStatusMsg:
    m.footer.SetCompression(&msg.Info)
    return m, nil
```

---

### Task 3.4: Create Compression Sync Command

**File:** `internal/tui/compression_sync.go` (CREATE)

```go
package tui

import tea "github.com/charmbracelet/bubbletea"

// syncCompressionCmd fetches compression status from backend
func (m Model) syncCompressionCmd() tea.Cmd {
    return func() tea.Msg {
        if m.session == "" || m.client == nil {
            return nil
        }
        
        info, err := m.client.CompressionStatus(m.session)
        if err != nil {
            return nil
        }
        
        return CompressionStatusMsg{Info: *info}
    }
}
```

---

### Task 3.5: Trigger Compression Sync

**File:** `internal/tui/tui.go` (MODIFY)

**Find where `syncStatusCmd` is called and add compression sync:**

```go
// In ChatDoneMsg handler:
cmds = append(cmds, m.syncStatusCmd(false))
cmds = append(cmds, m.syncCompressionCmd()) // NEW

// In session change handlers:
case SessionChosenMsg:
    cmds = append(cmds, m.syncCompressionCmd())

case ConnectedMsg:
    cmds = append(cmds, m.syncCompressionCmd())
```

**Verification:** `cd /home/nomadx/Documents/smolbot && go build ./internal/tui/...`
**Expected:** SUCCESS

**Commit:** `git add internal/tui/tui.go internal/tui/compression_sync.go && git commit -m "feat(tui): wire compression status to footer"`

---

## GATE 4: Backend API (Depends on Gate 2)

**Purpose:** Implement server-side endpoint for compression stats (requires Area 2).

**Entry Gate:** Gate 2 complete, Area 2 compression stats implemented
**Exit Gate:** Endpoint returns compression data

### Task 4.1: Review Area 2 Stats API

**File:** `pkg/agent/compression/stats.go` (READ from Area 2)

**Check if `GetStats(sessionKey string)` exists.**

---

### Task 4.2: Add Compression Handler

**File:** `pkg/gateway/compression.go` or integrate into existing API (CREATE or MODIFY)

```go
package gateway

import (
    "net/http"
    "github.com/Nomadcxx/smolbot/pkg/agent/compression"
)

// HandleCompressionStatus returns compression stats for a session
func (s *Server) HandleCompressionStatus(w http.ResponseWriter, r *http.Request) {
    sessionKey := r.PathValue("session")
    if sessionKey == "" {
        http.Error(w, "session required", http.StatusBadRequest)
        return
    }
    
    stats := compression.GetStats(sessionKey)
    if stats == nil || stats.LastCompression == nil {
        // Return disabled if no stats
        json.NewEncoder(w).Encode(struct {
            Enabled bool `json:"enabled"`
        }{Enabled: false})
        return
    }
    
    lc := stats.LastCompression
    json.NewEncoder(w).Encode(client.CompressionInfo{
        Enabled:          true,
        OriginalTokens:    lc.OriginalTokenCount,
        CompressedTokens:  lc.CompressedTokenCount,
        ReductionPercent:  lc.ReductionPercentage,
        LastRun:          stats.LastCompressionTime.Format("2006-01-02T15:04:05Z07:00"),
    })
}
```

---

### Task 4.3: Register Route

**File:** Route registration (MODIFY)

**Add route:**
```go
router.GET("/api/compression/:session", server.HandleCompressionStatus)
```

**Verification:** `cd /home/nomadx/Documents/smolbot && go build ./...`
**Expected:** SUCCESS

**Commit:** `git add pkg/gateway/compression.go && git commit -m "feat(api): add compression status endpoint"`

---

## GATE 5: Verification (PARALLEL with Gate 4)

**Purpose:** Final verification that everything works together.

**Entry Gate:** All gates complete
**Exit Gate:** All tests pass

### Task 5.1: Run All Tests

```bash
cd /home/nomadx/Documents/smolbot
go test ./internal/... -v
go test ./pkg/agent/... -v
```

**Expected:** All PASS

---

### Task 5.2: Build Everything

```bash
cd /home/nomadx/Documents/smolbot
go build ./...
```

**Expected:** SUCCESS

---

### Task 5.3: Final Commit

```bash
git add -A
git commit -m "feat(tui): add compression indicator and enhanced token display

Implements Area 3 (TUI Enhancements) with:

GATE 0 - Foundation:
- CompressionInfo type for compression state
- Compression color theme fields

GATE 1 - Footer:
- Compression indicator (↓XX%) in footer
- Color-coded by reduction: active/success/warning
- Enhanced token usage display with color coding
- Compact and expanded mode support

GATE 2 - Client:
- CompressionStatus() method for fetching stats

GATE 3 - TUI Integration:
- CompressionStatusMsg message type
- Footer compression display updates
- Auto-sync after chat completion

GATE 4 - Backend (if Area 2 complete):
- Compression stats API endpoint

Key features:
- Footer shows compression indicator when active
- Token usage color-coded (green→yellow→red)
- Syncs compression status from backend
- Works in compact and expanded modes

Design inspired by:
- Nanocoder: compression indicator format, color thresholds
- Crush: tool status states, compact rendering"
```

---

## SUMMARY

| Gate | Tasks | Dependencies | Duration | Files |
|------|-------|-------------|----------|-------|
| **GATE 0** | 2 | None | 15 min | protocol.go, theme.go |
| **GATE 1** | 5 | Gate 0 | 45 min | footer.go, footer_test.go |
| **GATE 2** | 2 | Gate 0 | 20 min | messages.go |
| **GATE 3** | 5 | Gates 1 & 2 | 30 min | tui.go, compression_sync.go |
| **GATE 4** | 3 | Gate 2 + Area 2 | 25 min | compression.go (api) |
| **GATE 5** | 3 | Gates 1-4 | 15 min | - |

**Total:** 20 tasks, ~2.5 hours
**Max Parallel:** Gates 1 & 2 run in parallel after Gate 0

**New Files (2):**
- `internal/tui/compression_sync.go`
- `pkg/gateway/compression.go` (or integrated)

**Modified Files (5):**
- `internal/client/protocol.go`
- `internal/client/messages.go`
- `internal/components/status/footer.go`
- `internal/components/status/footer_test.go`
- `internal/tui/tui.go`

---

## DELIVERABLES CHECKLIST

- [ ] `go build ./internal/client/...` succeeds
- [ ] `go build ./internal/components/status/...` succeeds
- [ ] `go build ./internal/tui/...` succeeds
- [ ] `go test ./internal/components/status/... -v` all pass
- [ ] Footer shows compression indicator (↓XX%) when enabled
- [ ] Compression color varies by reduction level
- [ ] Token usage shows color coding
- [ ] Client.CompressionStatus() works
- [ ] TUI updates footer on CompressionStatusMsg
- [ ] Compression syncs after chat completion

---

## PREREQUISITES

**This plan REQUIRES Area 2 (Context Management) to be implemented first for full functionality.**

If Area 2 is not yet implemented:
- Gates 0-3 can proceed with stubbed data
- Gate 4 (API) needs Area 2's `compression.GetStats()` first
- Footer will show disabled state until Area 2 provides real data

---

## INSPIRATION CREDITS

- **Nanocoder**: Compression indicator format (`↓XX%`), color thresholds (green/yellow/red)
- **Crush**: Tool status states (awaiting, running, success, error), compact vs expanded modes
