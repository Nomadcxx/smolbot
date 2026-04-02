# Tool UI/UX Implementation Plan 2: Advanced Patterns

**Created**: 2026-04-02  
**Status**: Draft  
**Depends On**: `docs/plans/2026-04-02-tool-ui-ux.md` (Plan 1)

---

## Overview

This second plan covers **advanced Claude Code patterns** discovered through deep codebase analysis that were **NOT included in Plan 1**. These patterns provide the polish and professional feel that distinguishes Claude Code from other AI TUIs.

Plan 1 covers: Tool classification, grouping, collapsing, footer tracking, Ctrl+O expansion.  
Plan 2 covers: Error handling, themes, metadata display, keyboard nav, sub-agents, streaming UX.

---

## Summary of New Patterns Discovered

| Category | Key Findings |
|----------|--------------|
| **Error Handling** | Truncation limits, retry countdown, SSL hints, Zod validation categorization |
| **Theme System** | 6 variants, daltonized (color-blind), shimmer animation colors, agent-specific colors |
| **Tool Metadata** | Duration formatting, file size display, token counting, diff stats (+/-) |
| **Keyboard Navigation** | 50+ keybinds, full vim mode, chord support, fuzzy picker, context-aware |
| **Sub-Agent UI** | Grouped agents, tree hierarchy (├─/└─), progress lines, async detection |
| **Streaming UX** | Debounce (1000ms), sticky scroll, viewport culling, circular buffer |

---

## Phase 11: Error Handling UI

### 11.1 Error Truncation Constants

**Source**: `/home/nomadx/claude-code/src/components/messages/SystemAPIErrorMessage.tsx:10`

```go
// internal/components/chat/errors.go (NEW FILE)
package chat

const (
    // Maximum characters for API error messages
    MaxAPIErrorChars = 1000
    
    // Maximum lines for tool error output
    MaxToolErrorLines = 10
    
    // Maximum characters before middle-truncation
    MaxErrorTotalChars = 10000
    
    // Stack trace frame limit
    MaxStackFrames = 5
)
```

### 11.2 Error Truncation Functions

```go
// TruncateError truncates error text with middle ellipsis for very long errors
func TruncateError(text string, maxChars int) string {
    if len(text) <= maxChars {
        return text
    }
    
    half := maxChars / 2
    return text[:half] + "\n... [truncated] ...\n" + text[len(text)-half:]
}

// TruncateErrorLines limits error output to N lines with count indicator
func TruncateErrorLines(text string, maxLines int) string {
    lines := strings.Split(text, "\n")
    if len(lines) <= maxLines {
        return text
    }
    
    truncated := lines[:maxLines]
    remaining := len(lines) - maxLines
    return strings.Join(truncated, "\n") + fmt.Sprintf("\n... and %d more lines", remaining)
}

// ShortErrorStack returns only top N frames of a stack trace
func ShortErrorStack(stack string, maxFrames int) string {
    lines := strings.Split(stack, "\n")
    // Stack traces typically have 2 lines per frame (location + code)
    maxLines := maxFrames * 2
    if len(lines) <= maxLines {
        return stack
    }
    return strings.Join(lines[:maxLines], "\n") + "\n... (stack truncated)"
}
```

### 11.3 Retry Countdown Display

**Source**: `/home/nomadx/claude-code/src/components/messages/` (retry patterns)

```go
// RetryState tracks retry attempts with countdown
type RetryState struct {
    Attempt    int
    MaxAttempt int
    NextRetry  time.Time
    Reason     string
}

// FormatRetry returns "attempt 2/3 · retrying in 5s"
func (r RetryState) Format() string {
    if r.NextRetry.IsZero() {
        return fmt.Sprintf("attempt %d/%d", r.Attempt, r.MaxAttempt)
    }
    
    remaining := time.Until(r.NextRetry).Round(time.Second)
    return fmt.Sprintf("attempt %d/%d · retrying in %s", r.Attempt, r.MaxAttempt, remaining)
}
```

### 11.4 Error Category Detection

**Source**: `/home/nomadx/claude-code/src/services/api/errorUtils.ts`

```go
// ErrorCategory for different error types
type ErrorCategory string

const (
    ErrorCategoryNetwork   ErrorCategory = "network"
    ErrorCategoryAuth      ErrorCategory = "auth"
    ErrorCategoryRateLimit ErrorCategory = "rate_limit"
    ErrorCategoryValidation ErrorCategory = "validation"
    ErrorCategorySSL       ErrorCategory = "ssl"
    ErrorCategoryUnknown   ErrorCategory = "unknown"
)

// sslErrorCodes contains TLS/SSL error identifiers
var sslErrorCodes = []string{
    "CERT_HAS_EXPIRED", "UNABLE_TO_VERIFY_LEAF_SIGNATURE",
    "SELF_SIGNED_CERT_IN_CHAIN", "DEPTH_ZERO_SELF_SIGNED_CERT",
    // ... (39 codes total in Claude Code)
}

// CategorizeError determines error type for display hints
func CategorizeError(err error) (ErrorCategory, string) {
    errStr := err.Error()
    
    // SSL/TLS detection
    for _, code := range sslErrorCodes {
        if strings.Contains(errStr, code) {
            return ErrorCategorySSL, "Try setting NODE_EXTRA_CA_CERTS or check proxy settings"
        }
    }
    
    // Rate limit detection
    if strings.Contains(errStr, "rate limit") || strings.Contains(errStr, "429") {
        return ErrorCategoryRateLimit, "API rate limit reached. Will retry automatically."
    }
    
    // Auth detection
    if strings.Contains(errStr, "401") || strings.Contains(errStr, "invalid_api_key") {
        return ErrorCategoryAuth, "Check your API key configuration"
    }
    
    return ErrorCategoryUnknown, ""
}
```

### 11.5 Zod-Style Validation Error Formatting

**Source**: `/home/nomadx/claude-code/src/components/messages/FallbackToolUseErrorMessage.tsx`

```go
// ValidationErrorType for parameter validation
type ValidationErrorType string

const (
    ValidationMissing    ValidationErrorType = "missing"
    ValidationUnexpected ValidationErrorType = "unexpected"
    ValidationTypeMismatch ValidationErrorType = "type_mismatch"
)

// ValidationError represents a single validation issue
type ValidationError struct {
    Type    ValidationErrorType
    Field   string
    Message string
}

// FormatValidationErrors renders validation errors clearly
func FormatValidationErrors(errors []ValidationError) string {
    var b strings.Builder
    
    // Group by type
    missing := filterByType(errors, ValidationMissing)
    unexpected := filterByType(errors, ValidationUnexpected)
    typeMismatch := filterByType(errors, ValidationTypeMismatch)
    
    if len(missing) > 0 {
        b.WriteString("Missing required:\n")
        for _, e := range missing {
            b.WriteString(fmt.Sprintf("  • %s\n", e.Field))
        }
    }
    
    if len(unexpected) > 0 {
        b.WriteString("Unexpected:\n")
        for _, e := range unexpected {
            b.WriteString(fmt.Sprintf("  • %s\n", e.Field))
        }
    }
    
    if len(typeMismatch) > 0 {
        b.WriteString("Type errors:\n")
        for _, e := range typeMismatch {
            b.WriteString(fmt.Sprintf("  • %s: %s\n", e.Field, e.Message))
        }
    }
    
    return b.String()
}
```

### 11.6 Integration with RenderToolBlock

```go
// In toolblock.go - modify error rendering

func renderToolError(tc ToolCall, width int) string {
    errText := tc.Output
    
    // Apply truncation
    errText = TruncateErrorLines(errText, MaxToolErrorLines)
    if len(errText) > MaxAPIErrorChars {
        errText = TruncateError(errText, MaxAPIErrorChars)
    }
    
    // Add category hint if applicable
    category, hint := CategorizeError(errors.New(errText))
    if hint != "" {
        errText += "\n" + dimStyle.Render(hint)
    }
    
    return errorStyle.Render(errText)
}
```

---

## Phase 12: Theme System Extensions

> **IMPORTANT**: smolbot already has a robust theme system!
> - 9 built-in themes (nord, dracula, catppuccin, rama, gruvbox, solarized, tokyo_night, material, monochrome)
> - `internal/theme/theme.go` - Theme struct with 60+ color fields
> - `internal/theme/manager.go` - Thread-safe registry (`Register`, `Set`, `Current`, `List`)
> - `internal/theme/themes/register.go` - 15-color base palette with auto-derived colors
> - Theme persistence via `~/.config/smolbot-tui/state.json`
> - Runtime switching via `/theme` command

**This phase EXTENDS the existing system, NOT replaces it.**

### 12.1 Add Agent Colors to Theme Struct

**File to modify**: `internal/theme/theme.go`

```go
// Add to existing Theme struct (after line 67):

	// Agent/sub-agent colors (for distinguishing concurrent agents)
	AgentRed     color.Color
	AgentBlue    color.Color
	AgentGreen   color.Color
	AgentYellow  color.Color
	AgentPurple  color.Color
	AgentOrange  color.Color
	AgentPink    color.Color
	AgentCyan    color.Color
```

### 12.2 Update Theme Registration

**File to modify**: `internal/theme/themes/register.go`

```go
// Add agent color defaults after line 54 (after t := &theme.Theme{...}):

func register(name string, colors [15]string, opts ...themeOption) {
    t := &theme.Theme{
        // ... existing assignments ...
    }

    // Add agent colors - derived from base palette
    // These provide 8 distinct colors for concurrent agent identification
    t.AgentRed = lipgloss.Color(colors[10])     // Error color base
    t.AgentBlue = lipgloss.Color(colors[13])    // Info color base  
    t.AgentGreen = lipgloss.Color(colors[12])   // Success color base
    t.AgentYellow = lipgloss.Color(colors[11])  // Warning color base
    t.AgentPurple = lipgloss.Color(colors[7])   // Accent color base
    t.AgentOrange = darkenColor(lipgloss.Color(colors[11]), 0.85) // Darker warning
    t.AgentPink = darkenColor(lipgloss.Color(colors[10]), 0.7)    // Lighter error
    t.AgentCyan = darkenColor(lipgloss.Color(colors[13]), 0.8)    // Lighter info

    for _, opt := range opts {
        opt(t)
    }
    // ... rest of function ...
}
```

### 12.3 Agent Color Manager (NEW FILE)

**File to create**: `internal/theme/agent_colors.go`

```go
package theme

import (
    "image/color"
    "sync"
)

// AgentColorName identifies one of 8 agent colors
type AgentColorName string

const (
    AgentColorRed    AgentColorName = "red"
    AgentColorBlue   AgentColorName = "blue"
    AgentColorGreen  AgentColorName = "green"
    AgentColorYellow AgentColorName = "yellow"
    AgentColorPurple AgentColorName = "purple"
    AgentColorOrange AgentColorName = "orange"
    AgentColorPink   AgentColorName = "pink"
    AgentColorCyan   AgentColorName = "cyan"
)

var agentColorOrder = []AgentColorName{
    AgentColorBlue, AgentColorGreen, AgentColorPurple, AgentColorOrange,
    AgentColorCyan, AgentColorPink, AgentColorYellow, AgentColorRed,
}

var (
    colorAssignments = make(map[string]AgentColorName)
    nextColorIndex   = 0
    colorMu          sync.RWMutex
)

// GetAgentColor returns a consistent color for an agent type.
// Same agent type always gets same color within a session.
func GetAgentColor(agentType string) AgentColorName {
    colorMu.Lock()
    defer colorMu.Unlock()

    if c, ok := colorAssignments[agentType]; ok {
        return c
    }

    c := agentColorOrder[nextColorIndex%len(agentColorOrder)]
    colorAssignments[agentType] = c
    nextColorIndex++
    return c
}

// GetAgentThemeColor returns the color.Color for an agent color name
func GetAgentThemeColor(t *Theme, name AgentColorName) color.Color {
    if t == nil {
        return nil
    }
    switch name {
    case AgentColorRed:
        return t.AgentRed
    case AgentColorBlue:
        return t.AgentBlue
    case AgentColorGreen:
        return t.AgentGreen
    case AgentColorYellow:
        return t.AgentYellow
    case AgentColorPurple:
        return t.AgentPurple
    case AgentColorOrange:
        return t.AgentOrange
    case AgentColorPink:
        return t.AgentPink
    case AgentColorCyan:
        return t.AgentCyan
    default:
        return t.AgentBlue
    }
}

// ResetAgentColors clears color assignments (useful for testing)
func ResetAgentColors() {
    colorMu.Lock()
    defer colorMu.Unlock()
    colorAssignments = make(map[string]AgentColorName)
    nextColorIndex = 0
}
```

### 12.4 Daltonized Theme Variants (OPTIONAL)

If accessibility is a priority, add color-blind friendly themes:

**File to create**: `internal/theme/themes/daltonized.go`

```go
package themes

func init() {
    // Dark daltonized - replaces red/green with blue/orange
    register("dark-daltonized", [15]string{
        "#2E3440", // Background (same as nord)
        "#3B4252", // Panel
        "#434C5E", // Element
        "#4C566A", // Border
        "#81A1C1", // BorderFocus
        "#81A1C1", // Primary (blue)
        "#88C0D0", // Secondary (cyan)
        "#EBCB8B", // Accent (yellow)
        "#ECEFF4", // Text
        "#D8DEE9", // TextMuted
        "#D08770", // Error (orange instead of red)
        "#EBCB8B", // Warning (yellow)
        "#5E81AC", // Success (blue instead of green)
        "#B48EAD", // Info (purple)
        "#4C566A", // ToolBorder
    })
}
```

### 12.5 Auto Dark/Light Detection (OPTIONAL)

**File to create**: `internal/theme/detect.go`

```go
package theme

import (
    "os"
    "strings"
)

// DetectPrefersDark checks environment for dark mode preference
func DetectPrefersDark() bool {
    // Check COLORFGBG (format: "foreground;background")
    if colorfgbg := os.Getenv("COLORFGBG"); colorfgbg != "" {
        parts := strings.Split(colorfgbg, ";")
        if len(parts) >= 2 {
            bg := parts[len(parts)-1]
            // 0=black, 15=white in ANSI
            if bg == "0" || bg == "8" {
                return true
            }
            return false
        }
    }

    // Check common dark mode indicators
    if strings.Contains(strings.ToLower(os.Getenv("TERM_PROGRAM")), "dark") {
        return true
    }

    // Default to dark (most common terminal setting)
    return true
}

// SuggestedTheme returns a theme name based on environment
func SuggestedTheme() string {
    if DetectPrefersDark() {
        return "nord" // dark theme
    }
    return "solarized" // light-friendly theme
}
```

### 12.6 Integration Points

**No changes needed to**:
- `internal/theme/manager.go` - Already provides `Current()`, `Set()`, `List()`
- `internal/app/state.go` - Already persists theme selection
- `/theme` command in `internal/tui/tui.go` - Already works

**Usage in components**:
```go
import "github.com/Nomadcxx/smolbot/internal/theme"

// Get agent color for a sub-agent
agentType := "explore"
colorName := theme.GetAgentColor(agentType)
t := theme.Current()
agentColor := theme.GetAgentThemeColor(t, colorName)

// Use with lipgloss
style := lipgloss.NewStyle().Foreground(agentColor)
```

---

## Phase 13: Tool Metadata Display

### 13.1 Duration Formatting

**Source**: `/home/nomadx/claude-code/src/utils/format.ts:34-95`

```go
// internal/format/duration.go (NEW FILE)
package format

import (
    "fmt"
    "time"
)

// DurationOptions controls output format
type DurationOptions struct {
    HideTrailingZeros  bool
    MostSignificantOnly bool
}

// FormatDuration converts duration to human-readable format
// Examples: "2d 5h 30m", "45s", "1.5s", "0.3s"
func FormatDuration(d time.Duration, opts ...DurationOptions) string {
    opt := DurationOptions{}
    if len(opts) > 0 {
        opt = opts[0]
    }
    
    ms := d.Milliseconds()
    
    // Sub-second: show 1 decimal
    if ms < 1000 {
        return fmt.Sprintf("%.1fs", float64(ms)/1000)
    }
    
    days := ms / (24 * 60 * 60 * 1000)
    ms %= 24 * 60 * 60 * 1000
    hours := ms / (60 * 60 * 1000)
    ms %= 60 * 60 * 1000
    minutes := ms / (60 * 1000)
    ms %= 60 * 1000
    seconds := ms / 1000
    
    if opt.MostSignificantOnly {
        if days > 0 { return fmt.Sprintf("%dd", days) }
        if hours > 0 { return fmt.Sprintf("%dh", hours) }
        if minutes > 0 { return fmt.Sprintf("%dm", minutes) }
        return fmt.Sprintf("%ds", seconds)
    }
    
    parts := []string{}
    if days > 0 { parts = append(parts, fmt.Sprintf("%dd", days)) }
    if hours > 0 || (!opt.HideTrailingZeros && len(parts) > 0) {
        parts = append(parts, fmt.Sprintf("%dh", hours))
    }
    if minutes > 0 || (!opt.HideTrailingZeros && len(parts) > 0) {
        parts = append(parts, fmt.Sprintf("%dm", minutes))
    }
    if seconds > 0 || len(parts) == 0 {
        parts = append(parts, fmt.Sprintf("%ds", seconds))
    }
    
    return strings.Join(parts, " ")
}
```

### 13.2 File Size Formatting

**Source**: `/home/nomadx/claude-code/src/utils/format.ts:9-23`

```go
// FormatFileSize converts bytes to human-readable
// Examples: "512 bytes", "1.5KB", "2MB", "1GB"
func FormatFileSize(bytes int64) string {
    const (
        KB = 1024
        MB = KB * 1024
        GB = MB * 1024
    )
    
    if bytes < KB {
        return fmt.Sprintf("%d bytes", bytes)
    }
    
    var value float64
    var unit string
    
    switch {
    case bytes >= GB:
        value = float64(bytes) / float64(GB)
        unit = "GB"
    case bytes >= MB:
        value = float64(bytes) / float64(MB)
        unit = "MB"
    default:
        value = float64(bytes) / float64(KB)
        unit = "KB"
    }
    
    // Remove trailing .0
    formatted := fmt.Sprintf("%.1f", value)
    formatted = strings.TrimSuffix(formatted, ".0")
    
    return formatted + unit
}
```

### 13.3 Token Formatting

**Source**: `/home/nomadx/claude-code/src/cost-tracker.ts:124-135`

```go
// FormatTokens formats token count with compact notation
// Examples: "500", "1.2k", "1.5m"
func FormatTokens(count int) string {
    switch {
    case count >= 1_000_000:
        return fmt.Sprintf("%.1fm", float64(count)/1_000_000)
    case count >= 1_000:
        return fmt.Sprintf("%.1fk", float64(count)/1_000)
    default:
        return fmt.Sprintf("%d", count)
    }
}

// FormatCost formats cost with appropriate precision
// < $0.50: 4 decimal places, >= $0.50: 2 decimal places
func FormatCost(cost float64) string {
    if cost < 0.50 {
        return fmt.Sprintf("$%.4f", cost)
    }
    return fmt.Sprintf("$%.2f", cost)
}
```

### 13.4 Diff Stats Display

**Source**: `/home/nomadx/claude-code/src/components/diff/DiffDialog.tsx:252`

```go
// DiffStats holds change statistics
type DiffStats struct {
    FilesChanged int
    LinesAdded   int
    LinesRemoved int
}

// Format returns colored diff stats string
// Example: "3 files changed +125 -42"
func (d DiffStats) Format(theme Theme) string {
    parts := []string{
        fmt.Sprintf("%d %s changed", d.FilesChanged, Plural(d.FilesChanged, "file")),
    }
    
    if d.LinesAdded > 0 {
        parts = append(parts, lipgloss.NewStyle().
            Foreground(lipgloss.Color(theme.DiffAddedWord)).
            Render(fmt.Sprintf("+%d", d.LinesAdded)))
    }
    
    if d.LinesRemoved > 0 {
        parts = append(parts, lipgloss.NewStyle().
            Foreground(lipgloss.Color(theme.DiffRemovedWord)).
            Render(fmt.Sprintf("-%d", d.LinesRemoved)))
    }
    
    return strings.Join(parts, " ")
}

// Plural returns singular or plural form
func Plural(n int, singular string) string {
    if n == 1 {
        return singular
    }
    return singular + "s"
}
```

### 13.5 Path Truncation (Middle)

**Source**: `/home/nomadx/claude-code/src/utils/truncate.ts:16-56`

```go
// TruncatePathMiddle truncates path preserving directory and filename
// Example: "src/components/deeply/nested/MyComponent.tsx" -> "src/…/MyComponent.tsx"
func TruncatePathMiddle(path string, maxWidth int) string {
    if runewidth.StringWidth(path) <= maxWidth {
        return path
    }
    
    dir := filepath.Dir(path)
    base := filepath.Base(path)
    
    // Always show filename
    baseWidth := runewidth.StringWidth(base)
    if baseWidth >= maxWidth-3 { // 3 for "…/"
        return "…/" + TruncateEnd(base, maxWidth-2)
    }
    
    // Truncate directory portion
    remainingWidth := maxWidth - baseWidth - 2 // 2 for "/…"
    truncDir := TruncateEnd(dir, remainingWidth)
    
    return truncDir + "…/" + base
}

// TruncateEnd truncates string from end with ellipsis
func TruncateEnd(s string, maxWidth int) string {
    if runewidth.StringWidth(s) <= maxWidth {
        return s
    }
    
    runes := []rune(s)
    for i := len(runes) - 1; i >= 0; i-- {
        candidate := string(runes[:i]) + "…"
        if runewidth.StringWidth(candidate) <= maxWidth {
            return candidate
        }
    }
    return "…"
}
```

### 13.6 Elapsed Time Hook

**Source**: `/home/nomadx/claude-code/src/hooks/useElapsedTime.ts`

```go
// internal/components/chat/elapsed.go

// ElapsedTimer tracks running duration with freeze capability
type ElapsedTimer struct {
    startTime time.Time
    endTime   time.Time // zero = still running
    pausedMs  int64
}

// NewElapsedTimer starts a new timer
func NewElapsedTimer() *ElapsedTimer {
    return &ElapsedTimer{startTime: time.Now()}
}

// Stop freezes the timer at current elapsed time
func (e *ElapsedTimer) Stop() {
    if e.endTime.IsZero() {
        e.endTime = time.Now()
    }
}

// Elapsed returns current duration
func (e *ElapsedTimer) Elapsed() time.Duration {
    end := e.endTime
    if end.IsZero() {
        end = time.Now()
    }
    return end.Sub(e.startTime) - time.Duration(e.pausedMs)*time.Millisecond
}

// Format returns formatted elapsed time
func (e *ElapsedTimer) Format() string {
    return FormatDuration(e.Elapsed(), DurationOptions{HideTrailingZeros: true})
}
```

---

## Phase 14: Keyboard Navigation System

> **IMPORTANT**: smolbot already has keybinding infrastructure!
> - `internal/components/dialog/keybindings.go` - Keybindings help dialog
> - `internal/tui/tui.go` - Main key handling in Update()
> - Bubbletea's `tea.KeyMsg` for key events
> - j/k navigation already in dialogs

**This phase EXTENDS existing keybinds, adding context-awareness and vim-style navigation.**

### 14.1 Current Keybindings (Already Implemented)

From `internal/components/dialog/keybindings.go`:
```
Global:        F1/Ctrl+M (menu), Ctrl+C (stop/quit), Esc/i (mode toggle)
               y/c (copy), PgUp/PgDn (scroll), Home/End, Ctrl+L (jump)
Editor:        Enter (send), Shift+Enter (newline), Up/Down (history)
Dialogs:       Esc (close), ↑/↓/j/k (navigate), Enter (select), Type (filter)
```

### 14.2 New Keybindings to Add

**Tool-specific (for Plan 1 integration)**:
| Key | Context | Action |
|-----|---------|--------|
| `Ctrl+O` | Global | Toggle tool output expansion |
| `Ctrl+E` | Transcript | Toggle show all (verbose) |

**Navigation enhancements**:
| Key | Context | Action |
|-----|---------|--------|
| `Ctrl+P` | Select | Previous (alias for k/↑) |
| `Ctrl+N` | Select | Next (alias for j/↓) |
| `g g` | Scroll | Jump to top (vim chord) |
| `G` | Scroll | Jump to bottom |

### 14.3 Add Ctrl+O Handler to TUI

**File to modify**: `internal/tui/tui.go`

Find the key handling section in `Update()` and add:

```go
// In the tea.KeyMsg switch/case:

case "ctrl+o":
    // Toggle tool expansion in transcript
    m.messages = m.messages.ToggleVerbose()
    return m, nil

case "ctrl+e":
    // If in transcript view, toggle show all
    if m.viewMode == ViewModeTranscript {
        m.messages = m.messages.ToggleShowAll()
        return m, nil
    }
```

### 14.4 Update Keybindings Help Dialog

**File to modify**: `internal/components/dialog/keybindings.go`

Add new entries to the View() function:

```go
lines := []string{
    lipgloss.NewStyle().Foreground(t.Primary).Bold(true).Render("Keybindings"),
    "",
    "Global",
    "  F1 / Ctrl+M    Open menu",
    "  Ctrl+C         Stop / Quit",
    "  Ctrl+O         Expand/collapse tool output",  // NEW
    "  Esc / i        Leave / enter insert mode",
    "  y / c          Copy last assistant reply",
    "  PgUp/PgDn      Scroll transcript",
    "  Home/End       Top/Bottom of transcript",
    "  Ctrl+L         Jump to latest",
    "",
    "Editor",
    "  Enter          Send message",
    "  Shift+Enter    New line",
    "  Up/Down        Input history",
    "",
    "Transcript",                                    // NEW SECTION
    "  Ctrl+E         Toggle verbose view",         // NEW
    "  Ctrl+O         Expand/collapse tools",       // NEW
    "",
    "Dialogs",
    "  Esc            Close / Back",
    "  ↑/↓ or j/k     Navigate",
    "  Ctrl+P/Ctrl+N  Navigate (vim aliases)",      // NEW
    "  Enter          Select",
    "  Type           Filter",
    // ... rest unchanged
}
```

### 14.5 Context-Aware Keybinding System (OPTIONAL)

For more sophisticated keybinding management, create a dedicated system:

**File to create**: `internal/keybindings/context.go`

```go
package keybindings

import tea "charm.land/bubbletea/v2"

// Context identifies which keybindings are active
type Context string

const (
    ContextGlobal     Context = "global"
    ContextChat       Context = "chat"
    ContextSelect     Context = "select"
    ContextScroll     Context = "scroll"
    ContextTranscript Context = "transcript"
)

// Action identifies what a keybinding does
type Action string

const (
    ActionToggleVerbose   Action = "app:toggleVerbose"
    ActionToggleTranscript Action = "app:toggleTranscript"
    ActionScrollUp        Action = "scroll:up"
    ActionScrollDown      Action = "scroll:down"
    ActionSelect          Action = "nav:select"
    // ... etc
)

// Binding maps a key to an action in a context
type Binding struct {
    Key     string
    Context Context
    Action  Action
}

// DefaultBindings returns all default keybindings
func DefaultBindings() []Binding {
    return []Binding{
        // Global
        {"ctrl+o", ContextGlobal, ActionToggleVerbose},
        {"ctrl+l", ContextGlobal, ActionScrollDown},
        
        // Select context (dialogs)
        {"up", ContextSelect, ActionScrollUp},
        {"k", ContextSelect, ActionScrollUp},
        {"ctrl+p", ContextSelect, ActionScrollUp},
        {"down", ContextSelect, ActionScrollDown},
        {"j", ContextSelect, ActionScrollDown},
        {"ctrl+n", ContextSelect, ActionScrollDown},
        {"enter", ContextSelect, ActionSelect},
        
        // Transcript
        {"ctrl+e", ContextTranscript, ActionToggleVerbose},
    }
}

// Resolver matches key events to actions
type Resolver struct {
    bindings []Binding
    contexts map[Context]bool
}

// NewResolver creates a resolver with default bindings
func NewResolver() *Resolver {
    return &Resolver{
        bindings: DefaultBindings(),
        contexts: map[Context]bool{ContextGlobal: true},
    }
}

// SetContext enables/disables a context
func (r *Resolver) SetContext(ctx Context, active bool) {
    r.contexts[ctx] = active
}

// Resolve finds the action for a key event
func (r *Resolver) Resolve(key tea.KeyMsg) (Action, bool) {
    keyStr := key.String()
    for _, b := range r.bindings {
        if r.contexts[b.Context] && b.Key == keyStr {
            return b.Action, true
        }
    }
    return "", false
}
```

**Note**: This is OPTIONAL. The simpler approach (14.3) works for basic needs.

---

## Phase 15: Sub-Agent UI Patterns

### 15.1 Agent Progress Line

**Source**: `/home/nomadx/claude-code/src/components/AgentProgressLine.tsx`

```go
// internal/components/chat/agent_progress.go (NEW FILE)
package chat

import (
    "fmt"
    "github.com/charmbracelet/lipgloss"
)

// AgentProgress tracks a single agent's execution state
type AgentProgress struct {
    ID            string
    Type          string            // "Agent", "explore", "code-review", etc.
    Description   string
    IsResolved    bool
    IsError       bool
    IsAsync       bool              // background agent
    ToolUseCount  int
    TokenCount    int
    LastToolInfo  string            // current tool being executed
    Color         AgentColorName
}

// TreeChar returns └─ for last agent, ├─ for others
func TreeChar(isLast bool) string {
    if isLast {
        return "└─"
    }
    return "├─"
}

// RenderAgentProgressLine renders a single agent progress line
func RenderAgentProgressLine(ap AgentProgress, theme Theme, isLast bool) string {
    treeChar := TreeChar(isLast)
    
    // Agent type badge with color
    color := GetThemeColor(theme, ap.Color)
    typeBadge := lipgloss.NewStyle().
        Background(lipgloss.Color(color)).
        Foreground(lipgloss.Color(theme.InverseText)).
        Padding(0, 1).
        Render(ap.Type)
    
    // Status line
    var status string
    if !ap.IsResolved {
        status = ap.LastToolInfo
        if status == "" {
            status = "Initializing…"
        }
    } else if ap.IsAsync {
        status = "Running in the background"
    } else {
        status = "Done"
    }
    
    // Metrics
    metrics := fmt.Sprintf("%d tool uses · %s tokens",
        ap.ToolUseCount,
        FormatTokens(ap.TokenCount))
    
    // Compose line
    line := fmt.Sprintf("%s %s %s · %s\n   ⇿ %s",
        treeChar,
        typeBadge,
        ap.Description,
        lipgloss.NewStyle().Foreground(lipgloss.Color(theme.Inactive)).Render(metrics),
        status)
    
    return line
}
```

### 15.2 Grouped Agent Container

**Source**: `/home/nomadx/claude-code/src/tools/AgentTool/UI.tsx:738-758`

```go
// GroupedAgentView renders multiple agents as a single collapsible unit
type GroupedAgentView struct {
    Agents       []AgentProgress
    AllSameType  bool
    AnyUnresolved bool
    AnyError     bool
    AllAsync     bool
}

// Summarize returns the group summary line
func (g GroupedAgentView) Summarize() string {
    count := len(g.Agents)
    
    if g.AllAsync {
        return fmt.Sprintf("%d background agents launched", count)
    }
    
    if g.AnyUnresolved {
        if g.AllSameType && count > 1 {
            return fmt.Sprintf("Running %d %s agents…", count, g.Agents[0].Type)
        }
        return fmt.Sprintf("Running %d agents…", count)
    }
    
    // All resolved
    if count == 1 {
        return "1 Agent finished"
    }
    if g.AllSameType {
        return fmt.Sprintf("%d %s agents finished", count, g.Agents[0].Type)
    }
    return fmt.Sprintf("%d agents finished", count)
}

// Render renders the full grouped agent view
func (g GroupedAgentView) Render(theme Theme, expanded bool) string {
    var b strings.Builder
    
    // Header with spinner if unresolved
    if g.AnyUnresolved {
        b.WriteString(spinner.Render() + " ")
    } else if g.AnyError {
        b.WriteString(errorGlyph + " ")
    } else {
        b.WriteString(successGlyph + " ")
    }
    
    b.WriteString(g.Summarize())
    
    if !g.AllAsync && !expanded {
        b.WriteString(" " + dimStyle.Render("(ctrl+o to expand)"))
    }
    
    if !expanded {
        return b.String()
    }
    
    // Expanded: show each agent's progress line
    b.WriteString("\n")
    for i, ap := range g.Agents {
        isLast := i == len(g.Agents)-1
        b.WriteString(RenderAgentProgressLine(ap, theme, isLast))
        if !isLast {
            b.WriteString("\n")
        }
    }
    
    return b.String()
}
```

### 15.3 Async Agent Detection

**Source**: `/home/nomadx/claude-code/src/tools/AgentTool/UI.tsx:704-710`

```go
// DetectAsyncAgent checks if agent was launched in background mode
func DetectAsyncAgent(input map[string]interface{}, output map[string]interface{}) bool {
    // Check input flag
    if runInBg, ok := input["run_in_background"].(bool); ok && runInBg {
        return true
    }
    
    // Check output status
    if status, ok := output["status"].(string); ok {
        return status == "async_launched" || status == "remote_launched"
    }
    
    return false
}

// DetectTeammateSpawn checks if this is a teammate spawn
func DetectTeammateSpawn(output map[string]interface{}) bool {
    if status, ok := output["status"].(string); ok {
        return status == "teammate_spawned"
    }
    return false
}
```

### 15.4 SubAgent Context (Suppress Hints in Nested)

**Source**: `/home/nomadx/claude-code/src/components/CtrlOToExpand.tsx:10-13`

```go
// RenderContext tracks rendering nesting level
type RenderContext struct {
    InSubAgent    bool
    InVirtualList bool
    NestingLevel  int
}

// ShouldShowExpandHint returns whether to show "(ctrl+o to expand)"
func (rc RenderContext) ShouldShowExpandHint() bool {
    // Don't show hint inside sub-agent output or virtual lists
    return !rc.InSubAgent && !rc.InVirtualList
}
```

---

## Phase 16: Streaming UX

### 16.1 Debounce/Throttle Utilities

**Source**: `/home/nomadx/claude-code/src/utils/bufferedWriter.ts`

```go
// internal/utils/debounce.go (NEW FILE)
package utils

import (
    "sync"
    "time"
)

// BufferedWriter batches writes with time/size limits
type BufferedWriter struct {
    buffer       []interface{}
    maxSize      int
    flushInterval time.Duration
    flushFn      func([]interface{})
    
    mu           sync.Mutex
    timer        *time.Timer
}

// NewBufferedWriter creates a buffered writer
// Default: 100 items max, 1000ms flush interval
func NewBufferedWriter(flushFn func([]interface{})) *BufferedWriter {
    return &BufferedWriter{
        maxSize:      100,
        flushInterval: 1000 * time.Millisecond,
        flushFn:      flushFn,
    }
}

// Write adds item to buffer, flushing if needed
func (bw *BufferedWriter) Write(item interface{}) {
    bw.mu.Lock()
    defer bw.mu.Unlock()
    
    bw.buffer = append(bw.buffer, item)
    
    // Size limit flush
    if len(bw.buffer) >= bw.maxSize {
        bw.flushLocked()
        return
    }
    
    // Start timer if not running
    if bw.timer == nil {
        bw.timer = time.AfterFunc(bw.flushInterval, func() {
            bw.mu.Lock()
            defer bw.mu.Unlock()
            bw.flushLocked()
        })
    }
}

func (bw *BufferedWriter) flushLocked() {
    if len(bw.buffer) == 0 {
        return
    }
    
    items := bw.buffer
    bw.buffer = nil
    
    if bw.timer != nil {
        bw.timer.Stop()
        bw.timer = nil
    }
    
    // Call flush function outside lock
    go bw.flushFn(items)
}
```

### 16.2 Sticky Scroll

**Source**: `/home/nomadx/claude-code/src/ink/components/ScrollBox.tsx`

```go
// internal/components/scroll/scroll.go

// ScrollState tracks scroll position and sticky behavior
type ScrollState struct {
    Offset      int
    ViewHeight  int
    ContentHeight int
    StickyScroll bool  // auto-follow new content
}

// NewScrollState creates initial state with sticky scroll enabled
func NewScrollState(viewHeight int) *ScrollState {
    return &ScrollState{
        ViewHeight:   viewHeight,
        StickyScroll: true,
    }
}

// SetContent updates content height, auto-scrolling if sticky
func (s *ScrollState) SetContent(height int) {
    s.ContentHeight = height
    
    if s.StickyScroll {
        // Scroll to bottom
        s.Offset = max(0, height - s.ViewHeight)
    }
}

// ScrollBy moves offset, disabling sticky if scrolling up
func (s *ScrollState) ScrollBy(delta int) {
    newOffset := s.Offset + delta
    newOffset = max(0, min(newOffset, s.ContentHeight - s.ViewHeight))
    
    // Scrolling up breaks sticky
    if delta < 0 {
        s.StickyScroll = false
    }
    
    // Scrolling to bottom restores sticky
    if newOffset >= s.ContentHeight - s.ViewHeight {
        s.StickyScroll = true
    }
    
    s.Offset = newOffset
}

// PageUp/PageDown helpers
func (s *ScrollState) PageUp() { s.ScrollBy(-s.ViewHeight) }
func (s *ScrollState) PageDown() { s.ScrollBy(s.ViewHeight) }
func (s *ScrollState) ScrollToTop() { s.Offset = 0; s.StickyScroll = false }
func (s *ScrollState) ScrollToBottom() { 
    s.Offset = max(0, s.ContentHeight - s.ViewHeight)
    s.StickyScroll = true
}
```

### 16.3 Circular Buffer for Large Outputs

**Source**: `/home/nomadx/claude-code/src/utils/CircularBuffer.ts`

```go
// internal/utils/circular_buffer.go

// CircularBuffer is a fixed-capacity buffer that evicts oldest items
type CircularBuffer[T any] struct {
    items    []T
    capacity int
    head     int
    size     int
}

// NewCircularBuffer creates a buffer with given capacity
func NewCircularBuffer[T any](capacity int) *CircularBuffer[T] {
    return &CircularBuffer[T]{
        items:    make([]T, capacity),
        capacity: capacity,
    }
}

// Push adds item, evicting oldest if at capacity
func (c *CircularBuffer[T]) Push(item T) {
    idx := (c.head + c.size) % c.capacity
    c.items[idx] = item
    
    if c.size < c.capacity {
        c.size++
    } else {
        c.head = (c.head + 1) % c.capacity
    }
}

// ToSlice returns all items in order (oldest first)
func (c *CircularBuffer[T]) ToSlice() []T {
    result := make([]T, c.size)
    for i := 0; i < c.size; i++ {
        result[i] = c.items[(c.head+i)%c.capacity]
    }
    return result
}

// Last returns the most recent N items
func (c *CircularBuffer[T]) Last(n int) []T {
    if n > c.size {
        n = c.size
    }
    result := make([]T, n)
    startIdx := c.size - n
    for i := 0; i < n; i++ {
        result[i] = c.items[(c.head+startIdx+i)%c.capacity]
    }
    return result
}
```

### 16.4 Min Display Time (Anti-Flicker)

**Source**: `/home/nomadx/claude-code/src/hooks/useMinDisplayTime.ts`

```go
// internal/utils/min_display.go

// MinDisplayValue ensures a value is displayed for minimum duration
type MinDisplayValue[T any] struct {
    current     T
    displayedAt time.Time
    minDuration time.Duration
    pending     *T
}

// NewMinDisplayValue creates with minimum display duration
func NewMinDisplayValue[T any](initial T, minDuration time.Duration) *MinDisplayValue[T] {
    return &MinDisplayValue[T]{
        current:     initial,
        displayedAt: time.Now(),
        minDuration: minDuration,
    }
}

// Set updates the value, respecting minimum display time
func (m *MinDisplayValue[T]) Set(value T) {
    elapsed := time.Since(m.displayedAt)
    
    if elapsed >= m.minDuration {
        // Enough time passed, update immediately
        m.current = value
        m.displayedAt = time.Now()
        m.pending = nil
    } else {
        // Queue for later
        m.pending = &value
    }
}

// Get returns current display value
func (m *MinDisplayValue[T]) Get() T {
    // Check if pending value can now be shown
    if m.pending != nil && time.Since(m.displayedAt) >= m.minDuration {
        m.current = *m.pending
        m.displayedAt = time.Now()
        m.pending = nil
    }
    return m.current
}
```

### 16.5 Rate Limit Display

**Source**: `/home/nomadx/claude-code/src/components/StatusLine.tsx:51-63`

```go
// RateLimitStatus for display in footer
type RateLimitStatus struct {
    FiveHourUsed    float64   // 0.0-1.0
    FiveHourResets  time.Time
    SevenDayUsed    float64   // 0.0-1.0
    SevenDayResets  time.Time
}

// FormatRateLimit returns formatted rate limit indicator
func FormatRateLimit(status RateLimitStatus, theme Theme) string {
    pct := int(status.FiveHourUsed * 100)
    
    // Color based on usage level
    var color string
    switch {
    case pct >= 90:
        color = theme.Error
    case pct >= 70:
        color = theme.Warning
    default:
        color = theme.Success
    }
    
    // Bar visualization (10 chars)
    filled := pct / 10
    bar := strings.Repeat("█", filled) + strings.Repeat("░", 10-filled)
    
    resetIn := FormatDuration(time.Until(status.FiveHourResets), 
        DurationOptions{MostSignificantOnly: true})
    
    return lipgloss.NewStyle().Foreground(lipgloss.Color(color)).
        Render(fmt.Sprintf("%s %d%% · resets in %s", bar, pct, resetIn))
}
```

---

## Phase 17: Integration & Polish

### 17.1 Files to Create

| File | Purpose |
|------|---------|
| `internal/components/chat/errors.go` | Error truncation, categorization |
| `internal/theme/theme.go` | Theme definitions |
| `internal/theme/agent_colors.go` | Agent color manager |
| `internal/theme/detect.go` | Auto dark/light detection |
| `internal/format/duration.go` | Duration formatting |
| `internal/format/size.go` | File size formatting |
| `internal/format/tokens.go` | Token/cost formatting |
| `internal/format/path.go` | Path truncation |
| `internal/format/diff.go` | Diff stats formatting |
| `internal/keybindings/keybindings.go` | Keybinding system |
| `internal/components/chat/agent_progress.go` | Agent progress UI |
| `internal/utils/debounce.go` | Debounce utilities |
| `internal/utils/circular_buffer.go` | Circular buffer |
| `internal/utils/min_display.go` | Anti-flicker |
| `internal/components/scroll/scroll.go` | Sticky scroll |

### 17.2 Files to Modify

| File | Changes |
|------|---------|
| `internal/components/chat/toolblock.go` | Use error truncation, themes |
| `internal/components/chat/messages.go` | Integrate keybindings, scroll |
| `internal/components/status/footer.go` | Rate limit display, agent colors |
| `internal/tui/tui.go` | Theme initialization, keybinding resolver |

### 17.3 Testing Strategy

Each phase should include:
1. **Unit tests** for new utilities (formatting, buffers, etc.)
2. **Integration tests** for keybinding resolution
3. **Visual tests** for theme rendering (screenshot comparison)
4. **Edge case tests** for truncation, overflow, etc.

---

## Dependency Graph

```
Plan 1 (Phases 1-10)
        │
        ▼
Plan 2 (Phases 11-17)
        │
        ├── Phase 11: Error Handling (standalone)
        ├── Phase 12: Theme System (standalone)
        ├── Phase 13: Tool Metadata (standalone)
        ├── Phase 14: Keyboard Nav (depends on Plan 1 Phase 9)
        ├── Phase 15: Sub-Agent UI (depends on Plan 1 Phase 3)
        ├── Phase 16: Streaming UX (standalone)
        └── Phase 17: Integration (depends on all above)
```

---

## Priority Order

**High Priority** (immediate value):
1. Phase 13: Tool Metadata (duration, size formatting)
2. Phase 11: Error Handling (cleaner error display)
3. Phase 16: Streaming UX (debounce, sticky scroll)

**Medium Priority** (polish):
4. Phase 12: Theme System (professional look)
5. Phase 14: Keyboard Navigation (power users)

**Lower Priority** (advanced features):
6. Phase 15: Sub-Agent UI (when smolbot supports sub-agents)
7. Full vim mode (optional)

---

## Metrics

| Metric | Current | Target |
|--------|---------|--------|
| Error output lines | unlimited | ≤10 lines |
| Theme variants | 0 | 6 (with daltonized) |
| Keybindings | ~5 | 50+ context-aware |
| Duration display | none | "2m 30s" format |
| File size display | bytes | "1.5MB" format |
| Scroll behavior | basic | sticky with restore |
| Rate limit display | none | visual bar + timer |

---

## References

- Claude Code analysis agents: cc-error-states, cc-themes-colors, cc-tool-metadata, cc-keyboard-nav, cc-sub-agents, cc-streaming-ux
- Plan 1: `docs/plans/2026-04-02-tool-ui-ux.md`
- Current state: `docs/TOOL_UI_UX_CURRENTSTATE.md`
