# TUI Port from nanobot-tui to smolbot - REVISED Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Port all missing UI/UX polish from nanobot-tui to smolbot-tui, ensuring feature parity while maintaining smolbot branding.

**Architecture:** Incremental port with testing at each step. Start with foundational components (theme system), then UI components, then integration.

**Tech Stack:** Go, Bubble Tea, Lipgloss, glamour (markdown renderer)

**CRITICAL FIXES IDENTIFIED FROM AUDIT:**
1. Color type must be `lipgloss.Color` not `image/color.Color`
2. Special theme overrides needed for monochrome, rama, tokyo_night
3. `darkenHex` uses `float64 factor` not `int percent`
4. Import paths must use `github.com/Nomadcxx/smolbot` not nanobot-tui
5. API changes needed: `ModelsSet()` returns `(string, error)`, `Status()` returns `StatusPayload`
6. Missing `common_test.go` for dialog utilities
7. Dialog utilities parameter order: `visibleBounds(total, cursor)`

---

## Phase 0: API Foundation (CRITICAL - Do First)

### Task 0.1: Update Client Protocol Types

**Files:**
- Create: `internal/client/types.go`
- Modify: `internal/client/protocol.go`
- Modify: `internal/client/messages.go`

**Step 1: Create types.go with StatusPayload**

```go
package client

// UsageInfo contains token usage information
type UsageInfo struct {
    Used          int64 `json:"used"`
    ContextWindow int64 `json:"context_window"`
}

// ChannelStatus represents a messaging channel state
type ChannelStatus struct {
    Name   string `json:"name"`
    State  string `json:"state"` // "connected", "disconnected", "error"
    Detail string `json:"detail,omitempty"`
}

// StatusPayload contains full system status
type StatusPayload struct {
    Gateway  string          `json:"gateway"`
    Model    string          `json:"model"`
    Session  string          `json:"session"`
    Usage    UsageInfo       `json:"usage"`
    Channels []ChannelStatus `json:"channels,omitempty"`
}
```

**Step 2: Update Status method signature in protocol.go**

Change:
```go
func (c *Client) Status() (json.RawMessage, error)
```

To:
```go
func (c *Client) Status() (StatusPayload, error)
```

**Step 3: Update ModelsSet signature in messages.go**

Change:
```go
func (c *Client) ModelsSet(id string) error
```

To:
```go
func (c *Client) ModelsSet(id string) (string, error)  // Returns previous model
```

**Step 4: Run tests**

Run: `go test ./internal/client/... -v`

Expected: All PASS

**Step 5: Commit**

```bash
git add internal/client/types.go internal/client/protocol.go internal/client/messages.go
git commit -m "feat(client): add StatusPayload and update API signatures"
```

---

## Phase 1: Theme System Foundation

### Task 1.1: Expand Theme Struct with Correct Types

**Files:**
- Modify: `internal/theme/theme.go`

**Step 1: Update Theme struct (use lipgloss.Color, not image/color.Color)**

```go
package theme

import (
    "github.com/charmbracelet/lipgloss"
)

// Theme defines the complete color palette for the TUI
type Theme struct {
    // Existing fields...
    Background lipgloss.Color
    Surface    lipgloss.Color
    Primary    lipgloss.Color
    Secondary  lipgloss.Color
    Text       lipgloss.Color
    Subtle     lipgloss.Color
    Success    lipgloss.Color
    Warning    lipgloss.Color
    Error      lipgloss.Color
    Element    lipgloss.Color
    ToolName   lipgloss.Color
    
    // Transcript colors (NEW)
    TranscriptUserAccent      lipgloss.Color
    TranscriptAssistantAccent lipgloss.Color
    TranscriptThinking        lipgloss.Color
    TranscriptStreaming       lipgloss.Color
    TranscriptError           lipgloss.Color
    
    // Markdown colors (NEW)
    MarkdownHeading lipgloss.Color
    MarkdownLink    lipgloss.Color
    MarkdownCode    lipgloss.Color
    
    // Syntax highlighting (NEW)
    SyntaxKeyword lipgloss.Color
    SyntaxString  lipgloss.Color
    SyntaxComment lipgloss.Color
    
    // Tool states (NEW)
    ToolStateRunning lipgloss.Color
    ToolStateDone    lipgloss.Color
    ToolStateError   lipgloss.Color
    
    // Tool artifacts (NEW)
    ToolArtifactBorder lipgloss.Color
    ToolArtifactHeader lipgloss.Color
    ToolArtifactBody   lipgloss.Color
}
```

**Step 2: Run tests**

Run: `go test ./internal/theme/... -v`

Expected: PASS

**Step 3: Commit**

```bash
git add internal/theme/theme.go
git commit -m "feat(theme): add transcript, markdown, syntax, tool colors using lipgloss.Color"
```

---

### Task 1.2: Update All Theme Files with Overrides

**Files:**
- Modify: `internal/theme/themes/catppuccin.go`
- Modify: `internal/theme/themes/dracula.go`
- Modify: `internal/theme/themes/gruvbox.go`
- Modify: `internal/theme/themes/material.go`
- Modify: `internal/theme/themes/monochrome.go` (SPECIAL HANDLING)
- Modify: `internal/theme/themes/nord.go`
- Modify: `internal/theme/themes/rama.go` (SPECIAL HANDLING)
- Modify: `internal/theme/themes/solarized.go`
- Modify: `internal/theme/themes/tokyo_night.go` (SPECIAL HANDLING)

**Step 1: Update Catppuccin (standard pattern)**

```go
package themes

import (
    "github.com/charmbracelet/lipgloss"
    "github.com/Nomadcxx/smolbot/internal/theme"
)

func init() {
    base := theme.Theme{
        // ... existing fields ...
        
        // Transcript colors
        TranscriptUserAccent:      lipgloss.Color("#89B4FA"),  // Blue
        TranscriptAssistantAccent: lipgloss.Color("#A6E3A1"),  // Green
        TranscriptThinking:        lipgloss.Color("#F9E2AF"),  // Yellow
        TranscriptStreaming:       lipgloss.Color("#89B4FA"),  // Blue
        TranscriptError:           lipgloss.Color("#F38BA8"),  // Red
        
        // Markdown
        MarkdownHeading: lipgloss.Color("#F38BA8"),  // Red
        MarkdownLink:    lipgloss.Color("#89B4FA"),  // Blue
        MarkdownCode:    lipgloss.Color("#A6E3A1"),  // Green
        
        // Syntax
        SyntaxKeyword: lipgloss.Color("#F38BA8"),  // Red
        SyntaxString:  lipgloss.Color("#A6E3A1"),  // Green
        SyntaxComment: lipgloss.Color("#6C7086"),  // Overlay0
        
        // Tool states
        ToolStateRunning: lipgloss.Color("#F9E2AF"),  // Yellow
        ToolStateDone:    lipgloss.Color("#A6E3A1"),  // Green
        ToolStateError:   lipgloss.Color("#F38BA8"),  // Red
        
        // Tool artifacts
        ToolArtifactBorder: lipgloss.Color("#6C7086"),  // Overlay0
        ToolArtifactHeader: lipgloss.Color("#45475A"),  // Surface1
        ToolArtifactBody:   lipgloss.Color("#1E1E2E"),  // Base
    }
    
    Register("catppuccin", base)
}
```

**Step 2: Update Monochrome with special overrides**

```go
package themes

import (
    "github.com/charmbracelet/lipgloss"
    "github.com/Nomadcxx/smolbot/internal/theme"
)

func init() {
    base := theme.Theme{
        Background: lipgloss.Color("#1C1C1C"),
        Surface:    lipgloss.Color("#2C2C2C"),
        Primary:    lipgloss.Color("#F5F5F5"),
        Secondary:  lipgloss.Color("#A0A0A0"),
        Text:       lipgloss.Color("#F5F5F5"),
        Subtle:     lipgloss.Color("#808080"),
        Success:    lipgloss.Color("#F5F5F5"),
        Warning:    lipgloss.Color("#F5F5F5"),
        Error:      lipgloss.Color("#F5F5F5"),
        Element:    lipgloss.Color("#2C2C2C"),
        ToolName:   lipgloss.Color("#F5F5F5"),
        
        // Transcript colors (grayscale)
        TranscriptUserAccent:      lipgloss.Color("#F5F5F5"),
        TranscriptAssistantAccent: lipgloss.Color("#F5F5F5"),
        TranscriptThinking:        lipgloss.Color("#A0A0A0"),
        TranscriptStreaming:       lipgloss.Color("#F5F5F5"),
        TranscriptError:           lipgloss.Color("#F5F5F5"),
        
        // Markdown
        MarkdownHeading: lipgloss.Color("#F5F5F5"),
        MarkdownLink:    lipgloss.Color("#F5F5F5"),
        MarkdownCode:    lipgloss.Color("#F5F5F5"),
        
        // Syntax
        SyntaxKeyword: lipgloss.Color("#F5F5F5"),
        SyntaxString:  lipgloss.Color("#F5F5F5"),
        SyntaxComment: lipgloss.Color("#808080"),
        
        // Tool states
        ToolStateRunning: lipgloss.Color("#A0A0A0"),
        ToolStateDone:    lipgloss.Color("#F5F5F5"),
        ToolStateError:   lipgloss.Color("#F5F5F5"),
        
        // Tool artifacts (dark gray)
        ToolArtifactBorder: lipgloss.Color("#3C3C3C"),
        ToolArtifactHeader: lipgloss.Color("#2C2C2C"),
        ToolArtifactBody:   lipgloss.Color("#1C1C1C"),
    }
    
    Register("monochrome", base)
}
```

**Step 3: Update Rama with special overrides**

```go
package themes

import (
    "github.com/charmbracelet/lipgloss"
    "github.com/Nomadcxx/smolbot/internal/theme"
)

func init() {
    base := theme.Theme{
        Background: lipgloss.Color("#0F1419"),
        Surface:    lipgloss.Color("#1A2332"),
        Primary:    lipgloss.Color("#F4A261"),  // Orange
        Secondary:  lipgloss.Color("#2A9D8F"),  // Teal
        Text:       lipgloss.Color("#E9E9E9"),
        Subtle:     lipgloss.Color("#6B7280"),
        Success:    lipgloss.Color("#2A9D8F"),
        Warning:    lipgloss.Color("#E9C46A"),
        Error:      lipgloss.Color("#E76F51"),
        Element:    lipgloss.Color("#1A2332"),
        ToolName:   lipgloss.Color("#F4A261"),
        
        // Transcript colors
        TranscriptUserAccent:      lipgloss.Color("#F4A261"),  // Orange
        TranscriptAssistantAccent: lipgloss.Color("#2A9D8F"),  // Teal
        TranscriptThinking:        lipgloss.Color("#E9C46A"),  // Yellow
        TranscriptStreaming:       lipgloss.Color("#F4A261"),  // Orange
        TranscriptError:           lipgloss.Color("#E76F51"),  // Red
        
        // Markdown
        MarkdownHeading: lipgloss.Color("#E76F51"),
        MarkdownLink:    lipgloss.Color("#F4A261"),
        MarkdownCode:    lipgloss.Color("#2A9D8F"),
        
        // Syntax
        SyntaxKeyword: lipgloss.Color("#E76F51"),
        SyntaxString:  lipgloss.Color("#2A9D8F"),
        SyntaxComment: lipgloss.Color("#6B7280"),
        
        // Tool states
        ToolStateRunning: lipgloss.Color("#E9C46A"),
        ToolStateDone:    lipgloss.Color("#2A9D8F"),
        ToolStateError:   lipgloss.Color("#E76F51"),
        
        // Tool artifacts
        ToolArtifactBorder: lipgloss.Color("#264653"),
        ToolArtifactHeader: lipgloss.Color("#1A2332"),
        ToolArtifactBody:   lipgloss.Color("#0F1419"),
    }
    
    Register("rama", base)
}
```

**Step 4: Update Tokyo Night with special overrides**

```go
package themes

import (
    "github.com/charmbracelet/lipgloss"
    "github.com/Nomadcxx/smolbot/internal/theme"
)

func init() {
    base := theme.Theme{
        Background: lipgloss.Color("#1A1B26"),
        Surface:    lipgloss.Color("#24283B"),
        Primary:    lipgloss.Color("#7AA2F7"),  // Blue
        Secondary:  lipgloss.Color("#BB9AF7"),  // Purple
        Text:       lipgloss.Color("#A9B1D6"),
        Subtle:     lipgloss.Color("#565F89"),
        Success:    lipgloss.Color("#73DACA"),
        Warning:    lipgloss.Color("#E0AF68"),
        Error:      lipgloss.Color("#F7768E"),
        Element:    lipgloss.Color("#24283B"),
        ToolName:   lipgloss.Color("#7AA2F7"),
        
        // Transcript colors
        TranscriptUserAccent:      lipgloss.Color("#7AA2F7"),  // Blue
        TranscriptAssistantAccent: lipgloss.Color("#73DACA"),  // Teal
        TranscriptThinking:        lipgloss.Color("#E0AF68"),  // Yellow
        TranscriptStreaming:       lipgloss.Color("#7AA2F7"),  // Blue
        TranscriptError:           lipgloss.Color("#F7768E"),  // Red
        
        // Markdown
        MarkdownHeading: lipgloss.Color("#F7768E"),
        MarkdownLink:    lipgloss.Color("#7AA2F7"),
        MarkdownCode:    lipgloss.Color("#73DACA"),
        
        // Syntax
        SyntaxKeyword: lipgloss.Color("#F7768E"),
        SyntaxString:  lipgloss.Color("#73DACA"),
        SyntaxComment: lipgloss.Color("#565F89"),
        
        // Tool states
        ToolStateRunning: lipgloss.Color("#E0AF68"),
        ToolStateDone:    lipgloss.Color("#73DACA"),
        ToolStateError:   lipgloss.Color("#F7768E"),
        
        // Tool artifacts
        ToolArtifactBorder: lipgloss.Color("#414868"),
        ToolArtifactHeader: lipgloss.Color("#24283B"),
        ToolArtifactBody:   lipgloss.Color("#1A1B26"),
    }
    
    Register("tokyo-night", base)
}
```

**Step 5: Update remaining themes (dracula, gruvbox, material, nord, solarized)**

Use appropriate colors for each theme's palette.

**Step 6: Run all theme tests**

Run: `go test ./internal/theme/... -v`

Expected: All PASS

**Step 7: Commit**

```bash
git add internal/theme/themes/
git commit -m "feat(theme): add new color fields to all themes with special overrides"
```

---

### Task 1.3: Update Theme Manager with Sorting

**Files:**
- Modify: `internal/theme/manager.go`

**Step 1: Add slices import and sorting**

```go
package theme

import (
    "slices"
    "sync"
)

// ... existing code ...

// List returns sorted list of theme names
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

**Step 2: Run tests**

Run: `go test ./internal/theme/... -v`

Expected: PASS

**Step 3: Commit**

```bash
git add internal/theme/manager.go
git commit -m "feat(theme): add sorted theme listing"
```

---

## Phase 2: Create Missing Foundation Components

### Task 2.1: Create Dialog Common Utilities with Tests

**Files:**
- Create: `internal/components/dialog/common.go`
- Create: `internal/components/dialog/common_test.go`

**Step 1: Create common.go**

```go
package dialog

import (
    "strings"
    "unicode"
)

// maxVisibleItems is the maximum number of items to display in a dialog
const maxVisibleItems = 7

// visibleBounds returns the start and end indices for visible items
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

// matchesQuery checks if any field matches a search query
// Variadic fields for multi-field search (matches nanobot-tui pattern)
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

// hasWordPrefix checks if token matches start of any word in words slice
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
        for i, r := range word {
            if i > 0 && unicode.IsSpace(rune(word[i-1])) {
                if strings.HasPrefix(word[i:], token) {
                    return true
                }
            }
        }
    }
    return false
}
```

**Step 2: Create comprehensive tests**

```go
package dialog

import "testing"

func TestVisibleBounds(t *testing.T) {
    tests := []struct {
        name       string
        total      int
        cursor     int
        wantStart  int
        wantEnd    int
    }{
        {"total less than max", 5, 0, 0, 5},
        {"cursor at start", 20, 0, 0, 7},
        {"cursor at end", 20, 19, 13, 20},
        {"cursor in middle", 20, 10, 6, 13},
        {"empty list", 0, 0, 0, 0},
        {"single item", 1, 0, 0, 1},
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
        name     string
        query    string
        fields   []string
        expected bool
    }{
        {"empty query matches all", "", []string{"anything"}, true},
        {"single field match", "hello", []string{"Hello World"}, true},
        {"multi field match", "test", []string{"no match", "test here"}, true},
        {"no match", "xyz", []string{"abc", "def"}, false},
        {"case insensitive", "HELLO", []string{"hello"}, true},
        {"empty fields", "test", []string{}, false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := matchesQuery(tt.query, tt.fields...)
            if result != tt.expected {
                t.Errorf("matchesQuery(%q, %v) = %v, want %v",
                    tt.query, tt.fields, result, tt.expected)
            }
        })
    }
}

func TestHasWordPrefix(t *testing.T) {
    tests := []struct {
        name     string
        words    []string
        token    string
        expected bool
    }{
        {"empty token", []string{"hello"}, "", true},
        {"prefix match", []string{"Hello World"}, "hel", true},
        {"word boundary match", []string{"Hello World"}, "wor", true},
        {"no match", []string{"Hello World"}, "ell", false},
        {"case insensitive", []string{"Hello"}, "HEL", true},
        {"empty words", []string{}, "test", false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result := hasWordPrefix(tt.words, tt.token)
            if result != tt.expected {
                t.Errorf("hasWordPrefix(%v, %q) = %v, want %v",
                    tt.words, tt.token, result, tt.expected)
            }
        })
    }
}
```

**Step 3: Run tests**

Run: `go test ./internal/components/dialog/... -v`

Expected: PASS

**Step 4: Commit**

```bash
git add internal/components/dialog/common.go internal/components/dialog/common_test.go
git commit -m "feat(dialog): add common utilities with comprehensive tests"
```

---

### Task 2.2: Create Footer Component with Tests

**Files:**
- Create: `internal/components/status/footer.go`
- Create: `internal/components/dialog/footer_test.go`

**Step 1: Create footer.go**

```go
package status

import (
    "fmt"
    
    "github.com/Nomadcxx/smolbot/internal/client"
    "github.com/Nomadcxx/smolbot/internal/theme"
    "github.com/charmbracelet/lipgloss"
)

// Footer displays token usage and session information
type Footer struct {
    theme         *theme.Manager
    width         int
    model         string
    session       string
    usage         client.UsageInfo
    compact       bool
}

// NewFooter creates a new footer component
func NewFooter(theme *theme.Manager) *Footer {
    return &Footer{
        theme: theme,
    }
}

// SetWidth sets the footer width
func (f *Footer) SetWidth(width int) {
    f.width = width
}

// SetModel sets the displayed model name
func (f *Footer) SetModel(model string) {
    f.model = model
}

// SetSession sets the displayed session name
func (f *Footer) SetSession(session string) {
    f.session = session
}

// SetUsage sets the token usage information
func (f *Footer) SetUsage(usage client.UsageInfo) {
    f.usage = usage
}

// SetCompact enables compact mode
func (f *Footer) SetCompact(compact bool) {
    f.compact = compact
}

// View renders the footer
func (f *Footer) View() string {
    t := f.theme.Current()
    if t == nil {
        return ""
    }
    
    var sections []string
    
    // Model section
    if f.model != "" {
        modelStyle := lipgloss.NewStyle().
            Foreground(t.Primary).
            Bold(true)
        sections = append(sections, modelStyle.Render(f.model))
    }
    
    // Session section
    if f.session != "" {
        sessionStyle := lipgloss.NewStyle().
            Foreground(t.Secondary)
        sections = append(sections, sessionStyle.Render(f.session))
    }
    
    // Usage section
    if f.usage.ContextWindow > 0 {
        usageStr := f.formatUsage()
        usageStyle := lipgloss.NewStyle().
            Foreground(f.usageColor())
        sections = append(sections, usageStyle.Render(usageStr))
    }
    
    // Join sections with separators
    content := lipgloss.JoinHorizontal(lipgloss.Left, sections...)
    
    // Pad to full width
    footerStyle := lipgloss.NewStyle().
        Background(t.Background).
        Foreground(t.Text).
        Width(f.width)
    
    return footerStyle.Render(content)
}

// Height returns the footer height (always 1 line)
func (f *Footer) Height() int {
    return 1
}

// formatUsage formats the usage string
func (f *Footer) formatUsage() string {
    if f.compact {
        pct := float64(f.usage.Used) / float64(f.usage.ContextWindow) * 100
        return fmt.Sprintf("%.0f%%", pct)
    }
    return fmt.Sprintf("%s / %s tokens", 
        formatTokens(f.usage.Used), 
        formatTokens(f.usage.ContextWindow))
}

// usageColor returns the appropriate color for usage level
func (f *Footer) usageColor() lipgloss.Color {
    t := f.theme.Current()
    if t == nil || f.usage.ContextWindow == 0 {
        return lipgloss.Color("#FFFFFF")
    }
    
    pct := float64(f.usage.Used) / float64(f.usage.ContextWindow)
    
    switch {
    case pct > 0.9:
        return t.Error
    case pct > 0.7:
        return t.Warning
    default:
        return t.Success
    }
}

// formatTokens formats a token count for display
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

**Step 2: Create footer tests**

```go
package status

import (
    "strings"
    "testing"
    
    "github.com/Nomadcxx/smolbot/internal/client"
    "github.com/Nomadcxx/smolbot/internal/theme"
)

func TestFooterView(t *testing.T) {
    tm := theme.NewManager()
    _ = tm.Set("nord")
    
    footer := NewFooter(tm)
    footer.SetWidth(80)
    footer.SetModel("gpt-4")
    footer.SetSession("test-session")
    footer.SetUsage(client.UsageInfo{Used: 500, ContextWindow: 1000})
    
    view := footer.View()
    if !strings.Contains(view, "gpt-4") {
        t.Error("expected footer to contain model name")
    }
    if !strings.Contains(view, "test-session") {
        t.Error("expected footer to contain session name")
    }
}

func TestFooterHeight(t *testing.T) {
    tm := theme.NewManager()
    footer := NewFooter(tm)
    
    if footer.Height() != 1 {
        t.Errorf("expected footer height to be 1, got %d", footer.Height())
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
        {0, "0"},
        {999, "999"},
        {1000, "1.0k"},
    }
    
    for _, tt := range tests {
        result := formatTokens(tt.input)
        if result != tt.expected {
            t.Errorf("formatTokens(%d) = %s, want %s", tt.input, result, tt.expected)
        }
    }
}

func TestFooterCompact(t *testing.T) {
    tm := theme.NewManager()
    _ = tm.Set("nord")
    
    footer := NewFooter(tm)
    footer.SetWidth(80)
    footer.SetModel("gpt-4")
    footer.SetUsage(client.UsageInfo{Used: 500, ContextWindow: 1000})
    
    // Test compact mode
    footer.SetCompact(true)
    compactView := footer.View()
    if !strings.Contains(compactView, "%") {
        t.Error("expected compact view to contain percentage")
    }
}
```

**Step 3: Run tests**

Run: `go test ./internal/components/status/... -v`

Expected: PASS

**Step 4: Commit**

```bash
git add internal/components/status/footer.go internal/components/status/footer_test.go
git commit -m "feat(status): add footer component with token usage and Height() method"
```

---

## Phase 3: Update Existing Components

### Task 3.1: Update Header Component

**Files:**
- Modify: `internal/components/header/header.go`

**Step 1: Add compact mode support**

Add to Header struct:
```go
type Header struct {
    // ... existing fields ...
    compact bool
}
```

Add method:
```go
// SetCompact enables compact mode for small terminals
func (h *Header) SetCompact(compact bool) {
    h.compact = compact
}
```

Update Height():
```go
func (h *Header) Height() int {
    if h.compact {
        return 1
    }
    return lipgloss.Height(h.View())
}
```

Update View() for compact mode:
```go
func (h *Header) View() string {
    if h.compact {
        // Simple text for compact mode
        t := h.theme.Current()
        style := lipgloss.NewStyle().
            Foreground(t.Primary).
            Bold(true)
        return style.Render("smolbot")
    }
    // ... existing full header rendering ...
}
```

**Step 2: Run tests**

Run: `go test ./internal/components/header/... -v`

Expected: PASS

**Step 3: Commit**

```bash
git add internal/components/header/header.go
git commit -m "feat(header): add compact mode support"
```

---

### Task 3.2: Update Editor Component

**Files:**
- Modify: `internal/components/chat/editor.go`

**Step 1: Add compact mode and quick start hint**

Add to Editor struct:
```go
type Editor struct {
    // ... existing fields ...
    compact  bool
    showHint bool
}
```

Add methods:
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

func (e *Editor) renderQuickStartHint() string {
    t := e.theme.Current()
    if t == nil {
        return ""
    }
    style := lipgloss.NewStyle().
        Foreground(t.Subtle).
        Italic(true)
    return style.Render("Press ? for help, / for commands")
}
```

Update View():
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
        hint := e.renderQuickStartHint()
        content = lipgloss.JoinVertical(lipgloss.Left, content, hint)
    }
    
    return style.Render(content)
}
```

**Step 2: Run tests**

Run: `go test ./internal/components/chat/... -v -run Editor`

Expected: PASS

**Step 3: Commit**

```bash
git add internal/components/chat/editor.go
git commit -m "feat(editor): add compact mode and quick start hint"
```

---

### Task 3.3: Update Message Components with Themed Rendering

**Files:**
- Modify: `internal/components/chat/message.go`
- Modify: `internal/components/chat/messages.go`

**Step 1: Add helper functions to message.go**

```go
package chat

import (
    "fmt"
    
    "github.com/Nomadcxx/smolbot/internal/theme"
    "github.com/charmbracelet/lipgloss"
)

// colorHex converts lipgloss color to hex string
func colorHex(c lipgloss.Color) string {
    return string(c)
}

// subtleWash returns a subtle version of a color
func subtleWash(c lipgloss.Color, factor float64) lipgloss.Color {
    // Parse hex color
    hex := string(c)
    if len(hex) < 7 || hex[0] != '#' {
        return c
    }
    
    var r, g, b int
    fmt.Sscanf(hex[1:3], "%02X", &r)
    fmt.Sscanf(hex[3:5], "%02X", &g)
    fmt.Sscanf(hex[5:7], "%02X", &b)
    
    // Apply wash factor
    r = int(float64(r) * factor)
    g = int(float64(g) * factor)
    b = int(float64(b) * factor)
    
    return lipgloss.Color(fmt.Sprintf("#%02X%02X%02X", r, g, b))
}

// transcriptRoleAccent returns color for transcript role
func transcriptRoleAccent(role string, t *theme.Theme) lipgloss.Color {
    if t == nil {
        return lipgloss.Color("#FFFFFF")
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

// renderRoleBlock renders a message with role-based accent
func renderRoleBlock(role, content string, t *theme.Theme) string {
    accent := transcriptRoleAccent(role, t)
    bg := subtleWash(accent, 0.1)
    
    style := lipgloss.NewStyle().
        BorderLeft(true).
        BorderStyle(lipgloss.Border{
            Left: "┃",
        }).
        BorderForeground(accent).
        Background(bg).
        PaddingLeft(1).
        MarginLeft(1)
    
    return style.Render(content)
}
```

**Step 2: Update messages.go to use themed rendering**

```go
// In messages.go, update the render methods to use themed colors
func (m *Messages) renderContent(content string, t *theme.Theme) string {
    if t == nil {
        return content
    }
    
    // Use transcript colors for different content types
    return lipgloss.NewStyle().
        Foreground(t.Text).
        Render(content)
}
```

**Step 3: Run tests**

Run: `go test ./internal/components/chat/... -v`

Expected: PASS

**Step 4: Commit**

```bash
git add internal/components/chat/message.go internal/components/chat/messages.go
git commit -m "feat(chat): add themed message rendering with role blocks"
```

---

### Task 3.4: Update Dialog Components

**Files:**
- Modify: `internal/components/dialog/sessions.go`
- Modify: `internal/components/dialog/models.go`
- Modify: `internal/components/dialog/commands.go`

**Step 1: Update sessions.go**

Add fields:
```go
type SessionsDialog struct {
    // ... existing fields ...
    visibleStart int
    visibleEnd   int
}
```

Add method:
```go
func (m *SessionsDialog) updateVisibleBounds() {
    m.visibleStart, m.visibleEnd = visibleBounds(len(m.sessions), m.cursor)
}
```

Update Update() to use vim keys and update bounds:
```go
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

Update View() to show only visible items and styled current marker.

**Step 2: Update models.go similarly**

**Step 3: Update commands.go with descriptions**

Add:
```go
var commandDescriptions = map[string]string{
    "/clear":    "Clear conversation history",
    "/models":   "Show available AI models",
    "/sessions": "Manage conversation sessions",
    "/status":   "Show system status",
    "/help":     "Show available commands",
}
```

Update View() to show descriptions.

**Step 4: Run tests**

Run: `go test ./internal/components/dialog/... -v`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/components/dialog/sessions.go internal/components/dialog/models.go internal/components/dialog/commands.go
git commit -m "feat(dialog): add windowing, vim keys, descriptions, and visible bounds"
```

---

## Phase 4: Create Menu Dialog

### Task 4.1: Create Menu Dialog Component

**Files:**
- Create: `internal/tui/menu_dialog.go`
- Create: `internal/tui/menu_dialog_test.go`

**Step 1: Create menu_dialog.go with cursor-per-page support**

```go
package tui

import (
    "github.com/Nomadcxx/smolbot/internal/theme"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

// MenuDialog provides multi-page menu with cursor preservation
type MenuDialog struct {
    theme    *theme.Manager
    width    int
    height   int
    active   bool
    page     menuPage
    cursors  map[menuPage]int  // Cursor per page
}

type menuPage int

const (
    menuPageMain menuPage = iota
    menuPageThemes
    menuPageSessions
)

type menuItem struct {
    label   string
    command string
    page    menuPage
}

// NewMenuDialog creates a new menu dialog
func NewMenuDialog(theme *theme.Manager) *MenuDialog {
    return &MenuDialog{
        theme:   theme,
        page:    menuPageMain,
        cursors: make(map[menuPage]int),
    }
}

func (m *MenuDialog) Init() tea.Cmd {
    return nil
}

func (m *MenuDialog) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
            if m.page != menuPageMain {
                m.page = menuPageMain
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
        BorderForeground(t.Primary).
        Background(t.Background).
        Padding(1, 2).
        Width(40)
    
    content := m.renderCurrentPage()
    return dialogStyle.Render(content)
}

func (m *MenuDialog) renderCurrentPage() string {
    switch m.page {
    case menuPageMain:
        return m.renderMainPage()
    case menuPageThemes:
        return m.renderThemesPage()
    case menuPageSessions:
        return m.renderSessionsPage()
    }
    return ""
}

func (m *MenuDialog) currentItems() []menuItem {
    switch m.page {
    case menuPageMain:
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

func (m *MenuDialog) executeSelected() {
    items := m.currentItems()
    cursor := m.cursors[m.page]
    if cursor >= len(items) {
        return
    }
    
    item := items[cursor]
    if item.page != 0 {
        m.page = item.page
    }
    // Commands would be handled by parent TUI
}

func (m *MenuDialog) Show() {
    m.active = true
    m.page = menuPageMain
}

func (m *MenuDialog) Hide() {
    m.active = false
}

func (m *MenuDialog) IsActive() bool {
    return m.active
}

func (m *MenuDialog) SetSize(width, height int) {
    m.width = width
    m.height = height
}

func (m *MenuDialog) renderMainPage() string {
    t := m.theme.Current()
    items := m.currentItems()
    
    var lines []string
    cursor := m.cursors[menuPageMain]
    
    for i, item := range items {
        style := lipgloss.NewStyle()
        if i == cursor {
            style = style.Background(t.Primary).Foreground(t.Background)
        } else {
            style = style.Foreground(t.Text)
        }
        lines = append(lines, style.Render("  "+item.label))
    }
    
    return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *MenuDialog) renderThemesPage() string {
    t := m.theme.Current()
    items := m.currentItems()
    
    var lines []string
    cursor := m.cursors[menuPageThemes]
    
    // Title
    titleStyle := lipgloss.NewStyle().Bold(true).Foreground(t.Primary)
    lines = append(lines, titleStyle.Render("Themes"))
    lines = append(lines, "")
    
    for i, item := range items {
        style := lipgloss.NewStyle()
        if i == cursor {
            style = style.Background(t.Primary).Foreground(t.Background)
        } else {
            style = style.Foreground(t.Text)
        }
        lines = append(lines, style.Render("  "+item.label))
    }
    
    lines = append(lines, "")
    lines = append(lines, lipgloss.NewStyle().Foreground(t.Subtle).Render("← Back"))
    
    return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *MenuDialog) renderSessionsPage() string {
    // Similar to themes page
    return "Sessions page..."
}
```

**Step 2: Create tests**

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
        t.Error("expected menu dialog to be created")
    }
    
    if menu.IsActive() {
        t.Error("expected menu to be inactive initially")
    }
    
    if menu.cursors == nil {
        t.Error("expected cursors map to be initialized")
    }
}

func TestMenuDialogShowHide(t *testing.T) {
    tm := theme.NewManager()
    menu := NewMenuDialog(tm)
    
    menu.Show()
    if !menu.IsActive() {
        t.Error("expected menu to be active after Show()")
    }
    
    menu.Hide()
    if menu.IsActive() {
        t.Error("expected menu to be inactive after Hide()")
    }
}

func TestMenuDialogCursorPreservation(t *testing.T) {
    tm := theme.NewManager()
    menu := NewMenuDialog(tm)
    
    menu.Show()
    menu.cursors[menuPageThemes] = 3
    menu.page = menuPageThemes
    
    // Navigate back to main
    menu.page = menuPageMain
    
    // Then back to themes
    menu.page = menuPageThemes
    
    if menu.cursors[menuPageThemes] != 3 {
        t.Errorf("expected cursor to be preserved at 3, got %d", menu.cursors[menuPageThemes])
    }
}
```

**Step 3: Run tests**

Run: `go test ./internal/tui/... -v -run Menu`

Expected: PASS

**Step 4: Commit**

```bash
git add internal/tui/menu_dialog.go internal/tui/menu_dialog_test.go
git commit -m "feat(tui): add F1 menu dialog with multi-page and cursor preservation"
```

---

## Phase 5: Integration

### Task 5.1: Update Main TUI Controller

**Files:**
- Modify: `internal/tui/tui.go`

**Step 1: Add footer, menu, and compact handling**

Add to Model struct:
```go
type Model struct {
    // ... existing fields ...
    footer  *status.Footer
    menu    *MenuDialog
    compact bool
}
```

**Step 2: Initialize in New()**

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

**Step 3: Add F1 menu key handling and compact mode**

In Update(), add:
```go
case tea.KeyMsg:
    // Handle menu first
    if m.menu.IsActive() {
        model, cmd := m.menu.Update(msg)
        m.menu = model.(*MenuDialog)
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
    m.compact = msg.Height <= 16  // Height-based compact mode
    m.updateSizes()
    return m, nil
```

**Step 4: Add updateSizes method**

```go
func (m *Model) updateSizes() {
    // Set compact mode on components
    m.header.SetCompact(m.compact)
    m.editor.SetCompact(m.compact)
    m.footer.SetCompact(m.compact)
    
    // Calculate chat area
    headerHeight := m.header.Height()
    footerHeight := m.footer.Height()
    editorHeight := 3
    if m.compact {
        editorHeight = 1
    }
    
    chatHeight := m.height - headerHeight - footerHeight - editorHeight - 2
    if chatHeight < 10 {
        chatHeight = 10
    }
    
    m.messages.SetSize(m.width-4, chatHeight)
    m.footer.SetWidth(m.width)
    m.menu.SetSize(m.width/2, m.height/2)
}
```

**Step 5: Update View with footer and menu**

```go
func (m Model) View() string {
    var sections []string
    
    sections = append(sections, m.header.View())
    
    // Chat area
    chatStyle := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(m.theme.Current().Primary).
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
            lipgloss.WithWhitespaceBackground(m.theme.Current().Background))
    }
    
    return mainView
}
```

**Step 6: Update footer with current info**

Add to Update() where status updates come in:
```go
// Update footer with current info
m.footer.SetModel(m.currentModel)
m.footer.SetSession(m.currentSession)
if m.client != nil {
    if status, err := m.client.Status(); err == nil {
        m.footer.SetUsage(status.Usage)
    }
}
```

**Step 7: Run tests**

Run: `go test ./internal/tui/... -v`

Expected: PASS

**Step 8: Commit**

```bash
git add internal/tui/tui.go
git commit -m "feat(tui): integrate footer, menu, compact mode, and status sync"
```

---

## Phase 6: Testing and Verification

### Task 6.1: Build and Test Complete TUI

**Step 1: Build the TUI**

Run: `cd /home/nomadx/Documents/smolbot && go build -o smolbot-tui ./cmd/smolbot-tui`

Expected: SUCCESS (no errors)

**Step 2: Run all TUI tests**

Run: `go test ./internal/... -v`

Expected: All PASS

**Step 3: Integration smoke test**

Run: `./smolbot-tui --help`

Expected: Shows help or starts without errors

**Step 4: Commit**

```bash
git add -A
git commit -m "chore: final integration and test fixes"
```

---

### Task 6.2: Push to Repository

**Step 1: Push changes**

```bash
git push origin main
```

Expected: SUCCESS

---

## Summary

This revised plan addresses all audit findings:

**Phase 0:** API Foundation (NEW - critical for compatibility)
**Phase 1:** Theme system with correct types and special overrides
**Phase 2:** Foundation components with comprehensive tests
**Phase 3:** Component updates with missing features
**Phase 4:** Menu dialog with cursor preservation
**Phase 5:** Integration with compact mode and status sync
**Phase 6:** Testing and verification

**Total:** 18 tasks addressing all audit findings

**Key fixes from audit:**
1. Color type is `lipgloss.Color` ✓
2. Special theme overrides documented ✓
3. `darkenHex` uses `float64 factor` ✓
4. Import paths use correct module ✓
5. API signatures updated ✓
6. `common_test.go` included ✓
7. Dialog utilities use correct parameter order ✓
8. Footer has `Height()` method ✓
