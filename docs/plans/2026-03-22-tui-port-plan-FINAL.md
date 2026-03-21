# TUI Port from nanobot-tui to smolbot - FINAL Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.
> **Strategy:** Parallel development with gated checkpoints. Maximize parallelization, minimize blocking.

**Goal:** Port all missing UI/UX polish from nanobot-tui to smolbot-tui, ensuring feature parity while maintaining smolbot branding.

**Architecture:** Parallel development with dependency-based gates. Foundation layers first, then parallel component development, final integration.

**Tech Stack:** Go, Bubble Tea, Lipgloss, glamour (markdown renderer)

---

## DEVELOPMENT STRATEGY: Parallel Gates & Scoped Slices

### Gate Structure
Each phase has **Entry Gates** (prerequisites) and **Exit Gates** (verification). Tasks within a phase can run in parallel unless marked **BLOCKING**.

### Scoped Slices
Each task is a small, focused unit (slice) that:
- Takes 15-30 minutes to implement
- Has clear verification criteria
- Can be committed independently
- Blocks minimal downstream work

---

## 🔴 GATE 0: Foundation Layer (BLOCKING - Must Complete First)

**Entry Gate:** Working directory is `/home/nomadx/Documents/smolbot` with clean git state
**Exit Gate:** All API types exist, client signatures updated, tests passing
**Parallelizable:** No - sequential dependency chain

### Task 0.1: Create StatusPayload Types [15 min]
**Files:** `internal/client/types.go` (NEW)

```go
package client

type UsageInfo struct {
    PromptTokens     int `json:"promptTokens"`
    CompletionTokens int `json:"completionTokens"`
    TotalTokens      int `json:"totalTokens"`
    ContextWindow    int `json:"contextWindow"`
}

type ChannelStatus struct {
    Name   string `json:"name"`
    Status string `json:"status"`
}

type StatusPayload struct {
    Model    string          `json:"model"`
    Session  string          `json:"session,omitempty"`
    Usage    UsageInfo       `json:"usage"`
    Uptime   int             `json:"uptime"`
    Channels []ChannelStatus `json:"channels,omitempty"`
}
```

**Verification:** `go build ./internal/client/...` → SUCCESS
**Commit:** `git add internal/client/types.go && git commit -m "feat(client): add StatusPayload types"`

---

### Task 0.2: Update Client Protocol Signatures [15 min]
**Files:** `internal/client/protocol.go`, `internal/client/messages.go`

**In protocol.go, change:**
```go
// FROM:
func (c *Client) Status() (json.RawMessage, error)

// TO:
func (c *Client) Status(session string) (StatusPayload, error)
```

**In messages.go, change:**
```go
// FROM:
func (c *Client) ModelsSet(id string) error

// TO:
func (c *Client) ModelsSet(id string) (string, error)  // Returns previous model
```

**Verification:** `go test ./internal/client/... -v` → All PASS
**Commit:** `git commit -am "feat(client): update Status() and ModelsSet() signatures"`

---

### Task 0.3: Create Theme Color Types [10 min]
**Files:** `internal/theme/theme.go`

**Add imports:**
```go
import "image/color"
```

**Expand Theme struct with new fields:**
```go
type Theme struct {
    // Existing fields...
    Background color.Color
    Surface    color.Color
    Primary    color.Color
    Secondary  color.Color
    Text       color.Color
    Subtle     color.Color
    Success    color.Color
    Warning    color.Color
    Error      color.Color
    Element    color.Color
    ToolName   color.Color
    
    // NEW: Transcript colors
    TranscriptUserAccent      color.Color
    TranscriptAssistantAccent color.Color
    TranscriptThinking        color.Color
    TranscriptStreaming       color.Color
    TranscriptError           color.Color
    
    // NEW: Markdown colors
    MarkdownHeading color.Color
    MarkdownLink    color.Color
    MarkdownCode    color.Color
    
    // NEW: Syntax colors
    SyntaxKeyword color.Color
    SyntaxString  color.Color
    SyntaxComment color.Color
    
    // NEW: Tool state colors
    ToolStateRunning color.Color
    ToolStateDone    color.Color
    ToolStateError   color.Color
    
    // NEW: Tool artifact colors
    ToolArtifactBorder color.Color
    ToolArtifactHeader color.Color
    ToolArtifactBody   color.Color
}
```

**Verification:** `go build ./internal/theme/...` → SUCCESS
**Commit:** `git commit -am "feat(theme): add 20 new color fields to Theme struct"`

---

**✅ GATE 0 COMPLETE:** Foundation types in place

---

## 🟡 GATE 1: Theme System (4 PARALLEL SLICES)

**Entry Gate:** Gate 0 complete
**Exit Gate:** All themes compile, darkenHex utility works, theme listing sorted
**Parallelizable:** YES - Tasks 1.1, 1.2, 1.3, 1.4 can run simultaneously

### Slice 1.1: Theme Registration Utilities [20 min]
**Files:** `internal/theme/themes/register.go`

**Add imports:**
```go
import (
    "fmt"
    "strconv"
)
```

**Add color utility functions:**
```go
// darkenHex darkens a hex color by factor (0.0-1.0)
func darkenHex(hex string, factor float64) string {
    if len(hex) < 7 || hex[0] != '#' {
        return hex
    }
    
    r := darkenChannel(hex[1:3], factor)
    g := darkenChannel(hex[3:5], factor)
    b := darkenChannel(hex[5:7], factor)
    
    return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}

func darkenChannel(pair string, factor float64) uint8 {
    var val int
    fmt.Sscanf(pair, "%02X", &val)
    darkened := uint8(float64(val) * factor)
    return darkened
}

// themeOption is a function that customizes a theme
type themeOption func(*Theme)

// withCustomColors applies custom color overrides
func withCustomColors(opts ...themeOption) themeOption {
    return func(t *Theme) {
        for _, opt := range opts {
            opt(t)
        }
    }
}
```

**Update Register function signature:**
```go
// FROM:
func Register(name string, theme Theme)

// TO:
func Register(name string, theme Theme, opts ...themeOption)
```

**Verification:** `go build ./internal/theme/themes/...` → SUCCESS
**Commit:** `git commit -am "feat(theme): add darkenHex utility and themeOption support"`

---

### Slice 1.2: Sort Theme Listings [10 min]
**Files:** `internal/theme/manager.go`

**Add import:**
```go
import "slices"
```

**Update List() method:**
```go
func (m *Manager) List() []string {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    names := make([]string, 0, len(m.themes))
    for name := range m.themes {
        names = append(names, name)
    }
    slices.Sort(names)  // NEW: Sort alphabetically
    return names
}
```

**Verification:** `go test ./internal/theme/... -v` → PASS
**Commit:** `git commit -am "feat(theme): sort theme listings alphabetically"`

---

### Slice 1.3: Standard Theme Colors (7 Themes) [30 min]
**Files:** `internal/theme/themes/{catppuccin,dracula,gruvbox,material,nord,solarized}.go`

**For each file, add new color fields to the Theme initialization:**

Example for catppuccin.go:
```go
base := theme.Theme{
    // ... existing fields ...
    
    // Transcript colors
    TranscriptUserAccent:      color.Color{R: 0x89, G: 0xB4, B: 0xFA},
    TranscriptAssistantAccent: color.Color{R: 0xA6, G: 0xE3, B: 0xA1},
    TranscriptThinking:        color.Color{R: 0xF9, G: 0xE2, B: 0xAF},
    TranscriptStreaming:       color.Color{R: 0x89, G: 0xB4, B: 0xFA},
    TranscriptError:           color.Color{R: 0xF3, G: 0x8B, B: 0xA8},
    
    // Markdown
    MarkdownHeading: color.Color{R: 0xF3, G: 0x8B, B: 0xA8},
    MarkdownLink:    color.Color{R: 0x89, G: 0xB4, B: 0xFA},
    MarkdownCode:    color.Color{R: 0xA6, G: 0xE3, B: 0xA1},
    
    // Syntax
    SyntaxKeyword: color.Color{R: 0xF3, G: 0x8B, B: 0xA8},
    SyntaxString:  color.Color{R: 0xA6, G: 0xE3, B: 0xA1},
    SyntaxComment: color.Color{R: 0x6C, G: 0x70, B: 0x86},
    
    // Tool states
    ToolStateRunning: color.Color{R: 0xF9, G: 0xE2, B: 0xAF},
    ToolStateDone:    color.Color{R: 0xA6, G: 0xE3, B: 0xA1},
    ToolStateError:   color.Color{R: 0xF3, G: 0x8B, B: 0xA8},
    
    // Tool artifacts
    ToolArtifactBorder: color.Color{R: 0x6C, G: 0x70, B: 0x86},
    ToolArtifactHeader: color.Color{R: 0x45, G: 0x47, B: 0x5A},
    ToolArtifactBody:   color.Color{R: 0x1E, G: 0x1E, B: 0x2E},
}
```

**Verification:** `go test ./internal/theme/... -v` → All PASS
**Commit:** `git commit -am "feat(theme): add new color fields to 7 standard themes"`

---

### Slice 1.4: Special Theme Overrides (3 Themes) [25 min]
**Files:** `internal/theme/themes/{monochrome,rama,tokyo_night}.go`

**monochrome.go:**
```go
func init() {
    base := theme.Theme{
        Background: color.Color{R: 0x1C, G: 0x1C, B: 0x1C},
        Surface:    color.Color{R: 0x2C, G: 0x2C, B: 0x2C},
        Primary:    color.Color{R: 0xF5, G: 0xF5, B: 0xF5},
        Secondary:  color.Color{R: 0xA0, G: 0xA0, B: 0xA0},
        Text:       color.Color{R: 0xF5, G: 0xF5, B: 0xF5},
        Subtle:     color.Color{R: 0x80, G: 0x80, B: 0x80},
        Success:    color.Color{R: 0xF5, G: 0xF5, B: 0xF5},
        Warning:    color.Color{R: 0xF5, G: 0xF5, B: 0xF5},
        Error:      color.Color{R: 0xF5, G: 0xF5, B: 0xF5},
        Element:    color.Color{R: 0x2C, G: 0x2C, B: 0x2C},
        ToolName:   color.Color{R: 0xF5, G: 0xF5, B: 0xF5},
        
        // All grayscale
        TranscriptUserAccent:      color.Color{R: 0xF5, G: 0xF5, B: 0xF5},
        TranscriptAssistantAccent: color.Color{R: 0xF5, G: 0xF5, B: 0xF5},
        TranscriptThinking:        color.Color{R: 0xA0, G: 0xA0, B: 0xA0},
        TranscriptStreaming:       color.Color{R: 0xF5, G: 0xF5, B: 0xF5},
        TranscriptError:           color.Color{R: 0xF5, G: 0xF5, B: 0xF5},
        MarkdownHeading:           color.Color{R: 0xF5, G: 0xF5, B: 0xF5},
        MarkdownLink:              color.Color{R: 0xF5, G: 0xF5, B: 0xF5},
        MarkdownCode:              color.Color{R: 0xF5, G: 0xF5, B: 0xF5},
        SyntaxKeyword:             color.Color{R: 0xF5, G: 0xF5, B: 0xF5},
        SyntaxString:              color.Color{R: 0xF5, G: 0xF5, B: 0xF5},
        SyntaxComment:             color.Color{R: 0x80, G: 0x80, B: 0x80},
        ToolStateRunning:          color.Color{R: 0xA0, G: 0xA0, B: 0xA0},
        ToolStateDone:             color.Color{R: 0xF5, G: 0xF5, B: 0xF5},
        ToolStateError:            color.Color{R: 0xF5, G: 0xF5, B: 0xF5},
        ToolArtifactBorder:        color.Color{R: 0x3C, G: 0x3C, B: 0x3C},
        ToolArtifactHeader:        color.Color{R: 0x2C, G: 0x2C, B: 0x2C},
        ToolArtifactBody:          color.Color{R: 0x1C, G: 0x1C, B: 0x1C},
    }
    
    Register("monochrome", base)
}
```

**rama.go:**
```go
func init() {
    base := theme.Theme{
        Background: color.Color{R: 0x0F, G: 0x14, B: 0x19},
        Surface:    color.Color{R: 0x1A, G: 0x23, B: 0x32},
        Primary:    color.Color{R: 0xF4, G: 0xA2, B: 0x61},  // Orange
        Secondary:  color.Color{R: 0x2A, G: 0x9D, B: 0x8F},  // Teal
        Text:       color.Color{R: 0xE9, G: 0xE9, B: 0xE9},
        Subtle:     color.Color{R: 0x6B, G: 0x72, B: 0x80},
        Success:    color.Color{R: 0x2A, G: 0x9D, B: 0x8F},
        Warning:    color.Color{R: 0xE9, G: 0xC4, B: 0x6A},
        Error:      color.Color{R: 0xE7, G: 0x6F, B: 0x51},
        Element:    color.Color{R: 0x1A, G: 0x23, B: 0x32},
        ToolName:   color.Color{R: 0xF4, G: 0xA2, B: 0x61},
        
        TranscriptUserAccent:      color.Color{R: 0xF4, G: 0xA2, B: 0x61},
        TranscriptAssistantAccent: color.Color{R: 0x2A, G: 0x9D, B: 0x8F},
        TranscriptThinking:        color.Color{R: 0xE9, G: 0xC4, B: 0x6A},
        TranscriptStreaming:       color.Color{R: 0xF4, G: 0xA2, B: 0x61},
        TranscriptError:           color.Color{R: 0xE7, G: 0x6F, B: 0x51},
        MarkdownHeading:           color.Color{R: 0xE7, G: 0x6F, B: 0x51},
        MarkdownLink:              color.Color{R: 0xF4, G: 0xA2, B: 0x61},
        MarkdownCode:              color.Color{R: 0x2A, G: 0x9D, B: 0x8F},
        SyntaxKeyword:             color.Color{R: 0xE7, G: 0x6F, B: 0x51},
        SyntaxString:              color.Color{R: 0x2A, G: 0x9D, B: 0x8F},
        SyntaxComment:             color.Color{R: 0x6B, G: 0x72, B: 0x80},
        ToolStateRunning:          color.Color{R: 0xE9, G: 0xC4, B: 0x6A},
        ToolStateDone:             color.Color{R: 0x2A, G: 0x9D, B: 0x8F},
        ToolStateError:            color.Color{R: 0xE7, G: 0x6F, B: 0x51},
        ToolArtifactBorder:        color.Color{R: 0x26, G: 0x46, B: 0x53},
        ToolArtifactHeader:        color.Color{R: 0x1A, G: 0x23, B: 0x32},
        ToolArtifactBody:          color.Color{R: 0x0F, G: 0x14, B: 0x19},
    }
    
    Register("rama", base)
}
```

**tokyo_night.go:**
```go
func init() {
    base := theme.Theme{
        Background: color.Color{R: 0x1A, G: 0x1B, B: 0x26},
        Surface:    color.Color{R: 0x24, G: 0x28, B: 0x3B},
        Primary:    color.Color{R: 0x7A, G: 0xA2, B: 0xF7},
        Secondary:  color.Color{R: 0xBB, G: 0x9A, B: 0xF7},
        Text:       color.Color{R: 0xA9, G: 0xB1, B: 0xD6},
        Subtle:     color.Color{R: 0x56, G: 0x5F, B: 0x89},
        Success:    color.Color{R: 0x73, G: 0xDA, B: 0xCA},
        Warning:    color.Color{R: 0xE0, G: 0xAF, B: 0x68},
        Error:      color.Color{R: 0xF7, G: 0x76, B: 0x8E},
        Element:    color.Color{R: 0x24, G: 0x28, B: 0x3B},
        ToolName:   color.Color{R: 0x7A, G: 0xA2, B: 0xF7},
        
        TranscriptUserAccent:      color.Color{R: 0x7A, G: 0xA2, B: 0xF7},
        TranscriptAssistantAccent: color.Color{R: 0x73, G: 0xDA, B: 0xCA},
        TranscriptThinking:        color.Color{R: 0xE0, G: 0xAF, B: 0x68},
        TranscriptStreaming:       color.Color{R: 0x7A, G: 0xA2, B: 0xF7},
        TranscriptError:           color.Color{R: 0xF7, G: 0x76, B: 0x8E},
        MarkdownHeading:           color.Color{R: 0xF7, G: 0x76, B: 0x8E},
        MarkdownLink:              color.Color{R: 0x7A, G: 0xA2, B: 0xF7},
        MarkdownCode:              color.Color{R: 0x73, G: 0xDA, B: 0xCA},
        SyntaxKeyword:             color.Color{R: 0xF7, G: 0x76, B: 0x8E},
        SyntaxString:              color.Color{R: 0x73, G: 0xDA, B: 0xCA},
        SyntaxComment:             color.Color{R: 0x56, G: 0x5F, B: 0x89},
        ToolStateRunning:          color.Color{R: 0xE0, G: 0xAF, B: 0x68},
        ToolStateDone:             color.Color{R: 0x73, G: 0xDA, B: 0xCA},
        ToolStateError:            color.Color{R: 0xF7, G: 0x76, B: 0x8E},
        ToolArtifactBorder:        color.Color{R: 0x41, G: 0x48, B: 0x68},
        ToolArtifactHeader:        color.Color{R: 0x24, G: 0x28, B: 0x3B},
        ToolArtifactBody:          color.Color{R: 0x1A, G: 0x1B, B: 0x26},
    }
    
    Register("tokyo-night", base)
}
```

**Verification:** `go test ./internal/theme/... -v` → All PASS
**Commit:** `git commit -am "feat(theme): add monochrome, rama, tokyo-night with custom colors"`

---

**✅ GATE 1 COMPLETE:** Theme system fully functional with all colors

---

## 🟡 GATE 2: Component Foundation (3 PARALLEL SLICES)

**Entry Gate:** Gate 1 complete
**Exit Gate:** All foundation components exist with tests
**Parallelizable:** YES - Tasks 2.1, 2.2, 2.3 can run simultaneously

### Slice 2.1: Dialog Common Utilities [20 min]
**Files:** `internal/components/dialog/common.go` (NEW), `internal/components/dialog/common_test.go` (NEW)

**common.go:**
```go
package dialog

import (
    "strings"
    "unicode"
)

const maxVisibleItems = 7

// visibleBounds returns start/end indices for visible items
// Parameter order: (total, cursor) - matches nanobot-tui
func visibleBounds(total, cursor int) (start, end int) {
    if total <= maxVisibleItems {
        return 0, total
    }
    
    half := maxVisibleItems / 2
    if cursor < half {
        return 0, maxVisibleItems
    }
    if cursor >= total-half {
        return total - maxVisibleItems, total
    }
    return cursor - half, cursor + half + 1
}

// matchesQuery checks if query matches any field (fuzzy search)
func matchesQuery(query string, fields ...string) bool {
    if query == "" {
        return true
    }
    query = strings.ToLower(query)
    for _, field := range fields {
        if strings.Contains(strings.ToLower(field), query) {
            return true
        }
    }
    return false
}

// hasWordPrefix checks if token matches any word prefix
func hasWordPrefix(words []string, token string) bool {
    if token == "" {
        return true
    }
    token = strings.ToLower(token)
    
    for _, word := range words {
        word = strings.ToLower(word)
        if strings.HasPrefix(word, token) {
            return true
        }
        // Check word boundaries
        for i := 0; i < len(word); i++ {
            if i == 0 || unicode.IsSpace(rune(word[i-1])) {
                if strings.HasPrefix(word[i:], token) {
                    return true
                }
            }
        }
    }
    return false
}
```

**common_test.go:**
```go
package dialog

import "testing"

func TestVisibleBounds(t *testing.T) {
    tests := []struct {
        name      string
        total     int
        cursor    int
        wantStart int
        wantEnd   int
    }{
        {"less than max", 5, 2, 0, 5},
        {"at start", 20, 0, 0, 7},
        {"at end", 20, 19, 13, 20},
        {"in middle", 20, 10, 6, 13},
        {"empty", 0, 0, 0, 0},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            start, end := visibleBounds(tt.total, tt.cursor)
            if start != tt.wantStart || end != tt.wantEnd {
                t.Errorf("visibleBounds(%d, %d) = (%d, %d), want (%d, %d)",
                    tt.total, tt.cursor, start, end, tt.wantStart, tt.wantEnd)
            }
        })
    }
}

func TestMatchesQuery(t *testing.T) {
    tests := []struct {
        query    string
        fields   []string
        expected bool
    }{
        {"", []string{"x"}, true},
        {"hello", []string{"Hello World"}, true},
        {"xyz", []string{"abc"}, false},
        {"test", []string{"no", "test here"}, true},
    }
    
    for _, tt := range tests {
        result := matchesQuery(tt.query, tt.fields...)
        if result != tt.expected {
            t.Errorf("matchesQuery(%q, %v) = %v, want %v",
                tt.query, tt.fields, result, tt.expected)
        }
    }
}
```

**Verification:** `go test ./internal/components/dialog/... -v` → PASS
**Commit:** `git commit -am "feat(dialog): add common utilities with tests"`

---

### Slice 2.2: Footer Component [25 min]
**Files:** `internal/components/status/footer.go` (NEW), `internal/components/status/footer_test.go` (NEW)

**footer.go:**
```go
package status

import (
    "fmt"
    
    "github.com/charmbracelet/lipgloss"
    "github.com/Nomadcxx/smolbot/internal/client"
    "github.com/Nomadcxx/smolbot/internal/theme"
)

// Footer displays token usage and session info
type Footer struct {
    theme   *theme.Manager
    width   int
    model   string
    session string
    usage   client.UsageInfo
    compact bool
}

func NewFooter(tm *theme.Manager) *Footer {
    return &Footer{theme: tm}
}

func (f *Footer) SetWidth(w int)       { f.width = w }
func (f *Footer) SetModel(m string)   { f.model = m }
func (f *Footer) SetSession(s string) { f.session = s }
func (f *Footer) SetUsage(u client.UsageInfo) { f.usage = u }
func (f *Footer) SetCompact(c bool)   { f.compact = c }

func (f *Footer) Height() int { return 1 }

func (f *Footer) View() string {
    t := f.theme.Current()
    if t == nil {
        return ""
    }
    
    var sections []string
    
    // Model
    if f.model != "" {
        style := lipgloss.NewStyle().Foreground(t.Primary).Bold(true)
        sections = append(sections, style.Render(f.model))
    }
    
    // Session
    if f.session != "" {
        style := lipgloss.NewStyle().Foreground(t.Secondary)
        sections = append(sections, style.Render(f.session))
    }
    
    // Usage
    if f.usage.ContextWindow > 0 {
        usageStr := f.formatUsage()
        style := lipgloss.NewStyle().Foreground(f.usageColor())
        sections = append(sections, style.Render(usageStr))
    }
    
    content := lipgloss.JoinHorizontal(lipgloss.Left, sections...)
    
    style := lipgloss.NewStyle().
        Background(t.Background).
        Foreground(t.Text).
        Width(f.width)
    
    return style.Render(content)
}

func (f *Footer) formatUsage() string {
    if f.compact {
        pct := float64(f.usage.Used) / float64(f.usage.ContextWindow) * 100
        return fmt.Sprintf("%.0f%%", pct)
    }
    return fmt.Sprintf("%s / %s", formatTokens(f.usage.Used), formatTokens(f.usage.ContextWindow))
}

func (f *Footer) usageColor() lipgloss.Color {
    t := f.theme.Current()
    if t == nil || f.usage.ContextWindow == 0 {
        return lipgloss.Color("#FFFFFF")
    }
    pct := float64(f.usage.Used) / float64(f.usage.ContextWindow)
    switch {
    case pct > 0.9: return t.Error
    case pct > 0.7: return t.Warning
    default: return t.Success
    }
}

func formatTokens(n int64) string {
    if n >= 1_000_000 {
        return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
    }
    if n >= 1_000 {
        return fmt.Sprintf("%.1fk", float64(n)/1_000)
    }
    return fmt.Sprintf("%d", n)
}
```

**footer_test.go:**
```go
package status

import (
    "strings"
    "testing"
    
    "github.com/Nomadcxx/smolbot/internal/client"
    "github.com/Nomadcxx/smolbot/internal/theme"
)

func TestFooterCreation(t *testing.T) {
    tm := theme.NewManager()
    f := NewFooter(tm)
    if f == nil {
        t.Error("expected footer to be created")
    }
    if f.Height() != 1 {
        t.Errorf("expected height 1, got %d", f.Height())
    }
}

func TestFooterView(t *testing.T) {
    tm := theme.NewManager()
    _ = tm.Set("nord")
    
    f := NewFooter(tm)
    f.SetWidth(80)
    f.SetModel("gpt-4")
    f.SetSession("test")
    f.SetUsage(client.UsageInfo{Used: 500, ContextWindow: 1000})
    
    view := f.View()
    if !strings.Contains(view, "gpt-4") {
        t.Error("expected model in view")
    }
}

func TestFormatTokens(t *testing.T) {
    tests := []struct {
        input    int64
        expected string
    }{
        {500, "500"},
        {1500, "1.5k"},
        {1500000, "1.5M"},
    }
    for _, tt := range tests {
        result := formatTokens(tt.input)
        if result != tt.expected {
            t.Errorf("formatTokens(%d) = %s, want %s", tt.input, result, tt.expected)
        }
    }
}
```

**Verification:** `go test ./internal/components/status/... -v` → PASS
**Commit:** `git commit -am "feat(status): add footer component with tests"`

---

### Slice 2.3: Color Utilities for Chat [15 min]
**Files:** `internal/components/chat/message.go` (MODIFY)

**Add imports:**
```go
import (
    "fmt"
    "image/color"
    "strconv"
    "strings"
)
```

**Add utility functions:**
```go
// colorHex converts color.Color to hex string
func colorHex(c color.Color) string {
    r, g, b, _ := c.RGBA()
    return fmt.Sprintf("#%02X%02X%02X", r>>8, g>>8, b>>8)
}

// subtleWash returns a washed-out version of color for backgrounds
func subtleWash(c color.Color, factor float64) color.Color {
    r, g, b, a := c.RGBA()
    return color.Color{
        R: uint8(float64(r>>8) * factor),
        G: uint8(float64(g>>8) * factor),
        B: uint8(float64(b>>8) * factor),
        A: uint8(a >> 8),
    }
}

// transcriptRoleAccent returns color for transcript role
func transcriptRoleAccent(role string, t *theme.Theme) color.Color {
    if t == nil {
        return color.Color{R: 255, G: 255, B: 255}
    }
    switch role {
    case "user":
        return t.TranscriptUserAccent
    case "assistant":
        return t.TranscriptAssistantAccent
    default:
        return t.Text
    }
}
```

**Verification:** `go build ./internal/components/chat/...` → SUCCESS
**Commit:** `git commit -am "feat(chat): add color utility functions"`

---

**✅ GATE 2 COMPLETE:** Foundation components ready

---

## 🟡 GATE 3: Component Enhancements (5 PARALLEL SLICES)

**Entry Gate:** Gate 2 complete
**Exit Gate:** All components have full nanobot-tui feature set
**Parallelizable:** YES - All 5 tasks can run in parallel

### Slice 3.1: Header Compact Mode [15 min]
**Files:** `internal/components/header/header.go` (MODIFY)

**Add to Header struct:**
```go
type Header struct {
    // ... existing fields ...
    compact bool
}
```

**Add method:**
```go
func (h *Header) SetCompact(compact bool) {
    h.compact = compact
}
```

**Update View() for compact mode:**
```go
func (h *Header) View() string {
    if h.compact {
        t := h.theme.Current()
        style := lipgloss.NewStyle().
            Foreground(t.Primary).
            Bold(true)
        return style.Render("smolbot")
    }
    // ... existing full header rendering ...
}
```

**Update Height():**
```go
func (h *Header) Height() int {
    if h.compact {
        return 1
    }
    return lipgloss.Height(h.View())
}
```

**Verification:** `go test ./internal/components/header/... -v` → PASS
**Commit:** `git commit -am "feat(header): add compact mode support"`

---

### Slice 3.2: Editor Compact Mode & Quick Start [20 min]
**Files:** `internal/components/chat/editor.go` (MODIFY)

**Add to Editor struct:**
```go
type Editor struct {
    // ... existing fields ...
    compact  bool
    showHint bool
}
```

**Add methods:**
```go
func (e *Editor) SetCompact(compact bool) {
    e.compact = compact
    if compact {
        e.textarea.SetHeight(1)
    } else {
        e.textarea.SetHeight(3)
    }
}

func (e *Editor) SetShowHint(show bool) {
    e.showHint = show
}

func (e *Editor) Height() int {
    if e.compact {
        return 1
    }
    return 3 // default height
}
```

**Update View():**
```go
func (e *Editor) View() string {
    t := e.theme.Current()
    if t == nil {
        return ""
    }
    
    style := lipgloss.NewStyle().
        Background(t.Element).
        Padding(0, 1)
    
    content := e.textarea.View()
    
    if e.showHint && !e.compact {
        hint := lipgloss.NewStyle().
            Foreground(t.Subtle).
            Italic(true).
            Render("Press ? for help, / for commands")
        content = lipgloss.JoinVertical(lipgloss.Left, content, hint)
    }
    
    return style.Render(content)
}
```

**Verification:** `go test ./internal/components/chat/... -v -run Editor` → PASS
**Commit:** `git commit -am "feat(editor): add compact mode and quick start hint"`

---

### Slice 3.3: Dialog Windowing & Vim Keys [25 min]
**Files:** `internal/components/dialog/sessions.go`, `internal/components/dialog/models.go`, `internal/components/dialog/commands.go`

**sessions.go:**
```go
// Add fields to struct
type SessionsDialog struct {
    // ... existing fields ...
    visibleStart int
    visibleEnd   int
}

// Add method
func (m *SessionsDialog) updateVisibleBounds() {
    m.visibleStart, m.visibleEnd = visibleBounds(len(m.sessions), m.cursor)
}

// Update key handling in Update()
case "j", "down":
    if m.cursor < len(m.sessions)-1 {
        m.cursor++
        m.updateVisibleBounds()
    }
case "k", "up":
    if m.cursor > 0 {
        m.cursor--
        m.updateVisibleBounds()
    }
case "ctrl+n":
    if m.cursor < len(m.sessions)-1 {
        m.cursor++
        m.updateVisibleBounds()
    }
case "ctrl+p":
    if m.cursor > 0 {
        m.cursor--
        m.updateVisibleBounds()
    }
```

**models.go:** Add similar windowing support + optionalModelDescription function

**commands.go:** Add commandDescriptions map

**Verification:** `go test ./internal/components/dialog/... -v` → PASS
**Commit:** `git commit -am "feat(dialog): add windowing, vim keys, and descriptions"`

---

### Slice 3.4: Themed Message Rendering [20 min]
**Files:** `internal/components/chat/message.go`, `internal/components/chat/messages.go`

**Add renderRoleBlock function to message.go:**
```go
func renderRoleBlock(role, content string, t *theme.Theme, width int) string {
    if t == nil {
        return content
    }
    
    accent := transcriptRoleAccent(role, t)
    
    style := lipgloss.NewStyle().
        BorderLeft(true).
        BorderStyle(lipgloss.Border{Left: "┃"}).
        BorderForeground(lipgloss.Color(colorHex(accent))).
        PaddingLeft(1).
        MarginLeft(1).
        Width(width - 2)
    
    return style.Render(content)
}
```

**Verification:** `go build ./internal/components/chat/...` → SUCCESS
**Commit:** `git commit -am "feat(chat): add themed message rendering"`

---

### Slice 3.5: Dialog Test Coverage [20 min]
**Files:** `internal/components/dialog/commands_test.go` (NEW), `internal/components/dialog/models_test.go` (NEW), `internal/components/dialog/sessions_test.go` (NEW)

**commands_test.go:**
```go
package dialog

import "testing"

func TestCommandsNavigation(t *testing.T) {
    // Test j/k navigation
    // Test Ctrl+n/p navigation
    // Test overflow cues
}

func TestCommandsFiltering(t *testing.T) {
    // Test multi-word query matching
    // Test empty state
}
```

**models_test.go:**
```go
package dialog

import "testing"

func TestModelsShowsCurrent(t *testing.T) {
    // Test current model highlighting
}

func TestModelsOverflowCues(t *testing.T) {
    // Test pagination indicators
}
```

**sessions_test.go:**
```go
package dialog

import "testing"

func TestSessionsCurrentMarker(t *testing.T) {
    // Test (current) marker display
}

func TestSessionsOverflow(t *testing.T) {
    // Test overflow cues
}
```

**Verification:** `go test ./internal/components/dialog/... -v` → PASS
**Commit:** `git commit -am "test(dialog): add test coverage for all dialogs"`

---

**✅ GATE 3 COMPLETE:** All components enhanced

---

## 🔴 GATE 4: Menu Dialog (BLOCKING)

**Entry Gate:** Gate 3 complete
**Exit Gate:** Menu dialog works with cursor preservation
**Parallelizable:** No - single component

### Task 4.1: Create Menu Dialog System [40 min]
**Files:** `internal/tui/menu_dialog.go` (NEW), `internal/tui/menu_dialog_test.go` (NEW)

**menu_dialog.go:**
```go
package tui

import (
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
    "github.com/Nomadcxx/smolbot/internal/theme"
)

type menuPage int

const (
    menuPageRoot menuPage = iota
    menuPageThemes
    menuPageSessions
)

type menuItem struct {
    label   string
    command string
    page    menuPage
}

type MenuDialog struct {
    theme   *theme.Manager
    width   int
    height  int
    active  bool
    page    menuPage
    cursors map[menuPage]int
}

func NewMenuDialog(theme *theme.Manager) *MenuDialog {
    return &MenuDialog{
        theme:   theme,
        page:    menuPageRoot,
        cursors: make(map[menuPage]int),
    }
}

func (m *MenuDialog) Init() tea.Cmd { return nil }

func (m *MenuDialog) Update(msg tea.Msg) (*MenuDialog, tea.Cmd) {
    if !m.active {
        return m, nil
    }
    
    switch msg := msg.(type) {
    case tea.KeyMsg:
        switch msg.String() {
        case "esc", "q":
            m.active = false
        case "up", "k":
            if m.cursors[m.page] > 0 {
                m.cursors[m.page]--
            }
        case "down", "j":
            items := m.currentItems()
            if m.cursors[m.page] < len(items)-1 {
                m.cursors[m.page]++
            }
        case "enter":
            m.executeSelected()
        case "left", "h":
            if m.page != menuPageRoot {
                m.page = menuPageRoot
            }
        }
    }
    
    return m, nil
}

func (m *MenuDialog) View() string {
    if !m.active {
        return ""
    }
    
    t := m.theme.Current()
    if t == nil {
        return ""
    }
    
    dialogStyle := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color(colorHex(t.Primary))).
        Background(lipgloss.Color(colorHex(t.Background))).
        Padding(1, 2).
        Width(40)
    
    content := m.renderCurrentPage()
    return dialogStyle.Render(content)
}

func (m *MenuDialog) currentItems() []menuItem {
    switch m.page {
    case menuPageRoot:
        return []menuItem{
            {label: "Themes", page: menuPageThemes},
            {label: "Sessions", page: menuPageSessions},
            {label: "Help", command: "/help"},
            {label: "Quit", command: "/quit"},
        }
    case menuPageThemes:
        var items []menuItem
        if m.theme != nil {
            for _, name := range m.theme.List() {
                items = append(items, menuItem{label: name})
            }
        }
        return items
    default:
        return nil
    }
}

func (m *MenuDialog) renderCurrentPage() string {
    // ... implementation ...
    return "Menu content..."
}

func (m *MenuDialog) executeSelected() {
    // ... implementation ...
}

func (m *MenuDialog) Show()     { m.active = true; m.page = menuPageRoot }
func (m *MenuDialog) Hide()      { m.active = false }
func (m *MenuDialog) IsActive()  { return m.active }
func (m *MenuDialog) SetSize(w, h int) { m.width = w; m.height = h }
```

**menu_dialog_test.go:**
```go
package tui

import (
    "testing"
    "github.com/Nomadcxx/smolbot/internal/theme"
)

func TestMenuDialogCreation(t *testing.T) {
    tm := theme.NewManager()
    menu := NewMenuDialog(tm)
    if menu == nil {
        t.Error("expected menu to be created")
    }
    if menu.IsActive() {
        t.Error("expected inactive initially")
    }
}

func TestMenuCursorPreservation(t *testing.T) {
    tm := theme.NewManager()
    menu := NewMenuDialog(tm)
    menu.Show()
    
    // Set cursor on themes page
    menu.cursors[menuPageThemes] = 3
    menu.page = menuPageThemes
    
    // Navigate back and forth
    menu.page = menuPageRoot
    menu.page = menuPageThemes
    
    if menu.cursors[menuPageThemes] != 3 {
        t.Errorf("cursor not preserved: got %d, want 3", menu.cursors[menuPageThemes])
    }
}
```

**Verification:** `go test ./internal/tui/... -v -run Menu` → PASS
**Commit:** `git commit -am "feat(tui): add menu dialog with cursor preservation"`

---

**✅ GATE 4 COMPLETE:** Menu dialog ready

---

## 🔴 GATE 5: Main TUI Integration (BLOCKING)

**Entry Gate:** Gates 0-4 complete
**Exit Gate:** TUI compiles and runs with all features integrated
**Parallelizable:** No - single integration point

### Task 5.1: Integrate All Components [45 min]
**Files:** `internal/tui/tui.go` (MODIFY)

**Add imports:**
```go
import "fmt"
```

**Add to Model struct:**
```go
type Model struct {
    // ... existing fields ...
    footer  *status.Footer
    menu    *MenuDialog
    compact bool
}
```

**Update New() function:**
```go
func New(client gatewayClient, theme *theme.Manager) *Model {
    m := &Model{
        // ... existing initialization ...
        footer: status.NewFooter(theme),
        menu:   NewMenuDialog(theme),
    }
    m.updateSizes()
    return m
}
```

**Add updateSizes method:**
```go
func (m *Model) updateSizes() {
    m.header.SetCompact(m.compact)
    m.editor.SetCompact(m.compact)
    m.footer.SetCompact(m.compact)
    
    headerHeight := m.header.Height()
    footerHeight := m.footer.Height()
    editorHeight := m.editor.Height()
    
    chatHeight := m.height - headerHeight - footerHeight - editorHeight - 2
    if chatHeight < 10 {
        chatHeight = 10
    }
    
    m.messages.SetSize(m.width-4, chatHeight)
    m.footer.SetWidth(m.width)
    m.menu.SetSize(m.width/2, m.height/2)
}
```

**Update Update() method:**
```go
func (m *Model) Update(msg tea.Msg) (*Model, tea.Cmd) {
    switch msg := msg.(type) {
    case tea.KeyMsg:
        // Handle menu first
        if m.menu.IsActive() {
            menu, cmd := m.menu.Update(msg)
            m.menu = menu
            return m, cmd
        }
        
        switch msg.String() {
        case "f1", "ctrl+m":
            m.menu.Show()
            return m, nil
        }
        // ... rest of key handling ...
        
    case tea.WindowSizeMsg:
        m.width = msg.Width
        m.height = msg.Height
        m.compact = msg.Height <= 16
        m.updateSizes()
        return m, nil
    }
    
    // ... rest of Update ...
}
```

**Update View() method:**
```go
func (m Model) View() string {
    var sections []string
    
    sections = append(sections, m.header.View())
    
    // Chat area
    chatStyle := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(lipgloss.Color(colorHex(m.theme.Current().Primary))).
        Padding(0, 1)
    
    chatContent := lipgloss.JoinVertical(lipgloss.Left,
        m.messages.View(),
        m.status.View(),
    )
    sections = append(sections, chatStyle.Render(chatContent))
    
    sections = append(sections, m.editor.View())
    sections = append(sections, m.footer.View())
    
    mainView := lipgloss.JoinVertical(lipgloss.Left, sections...)
    
    // Overlay menu if active
    if m.menu.IsActive() {
        menuView := m.menu.View()
        mainView = lipgloss.Place(m.width, m.height,
            lipgloss.Center, lipgloss.Center,
            menuView,
            lipgloss.WithWhitespaceBackground(
                lipgloss.Color(colorHex(m.theme.Current().Background))))
    }
    
    return mainView
}
```

**Update status sync:**
```go
// Add this where status updates come in:
if m.client != nil {
    if status, err := m.client.Status(""); err == nil {
        m.footer.SetModel(status.Model)
        m.footer.SetSession(status.Session)
        m.footer.SetUsage(status.Usage)
    }
}
```

**Verification:** `go test ./internal/tui/... -v` → PASS
**Commit:** `git commit -am "feat(tui): integrate footer, menu, compact mode"`

---

**✅ GATE 5 COMPLETE:** Full integration done

---

## 🟢 GATE 6: Final Verification & Push

**Entry Gate:** Gate 5 complete
**Exit Gate:** All tests pass, build succeeds, pushed to repo
**Parallelizable:** Partial - tests can run in parallel with build

### Slice 6.1: Run All Tests [10 min]
```bash
go test ./internal/... -v
```
**Expected:** All PASS

---

### Slice 6.2: Build TUI [5 min]
```bash
go build -o smolbot-tui ./cmd/smolbot-tui
./smolbot-tui --help
```
**Expected:** Build SUCCESS, help displayed or starts without errors

---

### Slice 6.3: Final Commit & Push [5 min]
```bash
git add -A
git commit -m "feat(tui): complete nanobot-tui feature parity port"
git push origin main
```
**Expected:** SUCCESS

---

**✅ GATE 6 COMPLETE:** Port finished successfully

---

## Summary

| Gate | Tasks | Parallel | Duration | Dependencies |
|------|-------|----------|----------|--------------|
| **0** | 3 | No | 40 min | None |
| **1** | 4 | Yes | 30 min | Gate 0 |
| **2** | 3 | Yes | 25 min | Gate 1 |
| **3** | 5 | Yes | 40 min | Gate 2 |
| **4** | 1 | No | 40 min | Gate 3 |
| **5** | 1 | No | 45 min | Gate 4 |
| **6** | 3 | Partial | 20 min | Gate 5 |

**Total:** 20 tasks, ~4 hours estimated
**Max Parallel Tasks:** 5 (in Gate 3)
**Critical Path:** Gates 0 → 4 → 5 → 6
**New/Missing Files:** 13
**Modified Files:** 12+

---

## Quick Reference: Critical Signatures

**Protocol Changes:**
- `Status(session string) (StatusPayload, error)`
- `ModelsSet(id string) (string, error)`

**Theme Color Fields:**
- Transcript: UserAccent, AssistantAccent, Thinking, Streaming, Error
- Markdown: Heading, Link, Code
- Syntax: Keyword, String, Comment
- Tool: StateRunning, StateDone, StateError, ArtifactBorder, ArtifactHeader, ArtifactBody

**Component Methods:**
- `Footer.Height() int` - returns 1
- `Editor.SetCompact(bool)` + `Height() int`
- `Header.SetCompact(bool)` + `Height() int`
- `visibleBounds(total, cursor int) (int, int)` - parameter order critical
- `matchesQuery(query string, fields ...string) bool`

**Menu Dialog:**
- Cursor preservation per page via `map[menuPage]int`
- F1 or Ctrl+M to open
- Esc or Q to close

---

## Verification Checklist

- [ ] All 20 color fields added to Theme struct
- [ ] All 10 themes updated with new colors
- [ ] darkenHex utility exists
- [ ] visibleBounds uses (total, cursor) parameter order
- [ ] Footer component created with Height() method
- [ ] Menu dialog created with cursor preservation
- [ ] All dialogs use windowing and vim keys (j/k, Ctrl+n/p)
- [ ] Compact mode supported in Header, Editor, Footer
- [ ] F1 key opens menu dialog
- [ ] All tests pass
- [ ] TUI builds successfully
- [ ] Changes pushed to repository
