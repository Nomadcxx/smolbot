# TUI Port from nanobot-tui to smolbot Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Port all missing UI/UX polish from nanobot-tui to smolbot-tui, ensuring feature parity while maintaining smolbot branding.

**Architecture:** Incremental port with testing at each step. Start with foundational components (theme system), then UI components, then integration.

**Tech Stack:** Go, Bubble Tea, Lipgloss, glamour (markdown renderer)

---

## Phase 1: Theme System Foundation

### Task 1: Expand Theme Struct

**Files:**
- Modify: `internal/theme/theme.go:15-45`
- Test: `internal/theme/theme_test.go`

**Step 1: Write failing test for new theme fields**

```go
func TestThemeHasTranscriptColors(t *testing.T) {
    theme := Theme{
        TranscriptUserAccent: color.Color{R: 0, G: 100, B: 200},
    }
    if theme.TranscriptUserAccent.R != 0 {
        t.Error("expected TranscriptUserAccent field to exist")
    }
}
```

**Step 2: Run test to verify it fails**

Run: `cd /home/nomadx/Documents/smolbot && go test ./internal/theme/... -v -run TestThemeHasTranscriptColors`

Expected: FAIL - unknown field 'TranscriptUserAccent'

**Step 3: Add missing theme color fields**

Add these fields to the Theme struct in `internal/theme/theme.go`:

```go
// Transcript colors
TranscriptUserAccent      color.Color
TranscriptAssistantAccent color.Color
TranscriptThinking        color.Color
TranscriptStreaming       color.Color
TranscriptError           color.Color

// Markdown colors
MarkdownHeading color.Color
MarkdownLink    color.Color
MarkdownCode    color.Color

// Syntax highlighting
SyntaxKeyword color.Color
SyntaxString  color.Color
SyntaxComment color.Color

// Tool states
ToolStateRunning color.Color
ToolStateDone    color.Color
ToolStateError   color.Color

// Tool artifacts
ToolArtifactBorder color.Color
ToolArtifactHeader color.Color
ToolArtifactBody   color.Color
```

**Step 4: Run test to verify it passes**

Run: `go test ./internal/theme/... -v -run TestThemeHasTranscriptColors`

Expected: PASS

**Step 5: Commit**

```bash
cd /home/nomadx/Documents/smolbot
git add internal/theme/theme.go internal/theme/theme_test.go
git commit -m "feat(theme): add transcript, markdown, syntax colors to Theme struct"
```

---

### Task 2: Update All Theme Files

**Files:**
- Modify: `internal/theme/themes/*.go` (10 files)
- Test: `internal/theme/theme_test.go`

**Step 1: Update Catppuccin theme**

In `internal/theme/themes/catppuccin.go`, add the new color fields to the Theme literal:

```go
// Add after existing fields:
TranscriptUserAccent:      hexColor("89B4FA"),  // Blue
TranscriptAssistantAccent: hexColor("A6E3A1"),  // Green
TranscriptThinking:        hexColor("F9E2AF"),  // Yellow
TranscriptStreaming:       hexColor("89B4FA"),  // Blue
TranscriptError:           hexColor("F38BA8"),  // Red
MarkdownHeading:           hexColor("F38BA8"),  // Red
MarkdownLink:              hexColor("89B4FA"),  // Blue
MarkdownCode:              hexColor("A6E3A1"),  // Green
SyntaxKeyword:             hexColor("F38BA8"),  // Red
SyntaxString:              hexColor("A6E3A1"),  // Green
SyntaxComment:             hexColor("6C7086"),  // Overlay0
ToolStateRunning:          hexColor("F9E2AF"),  // Yellow
ToolStateDone:             hexColor("A6E3A1"),  // Green
ToolStateError:            hexColor("F38BA8"),  // Red
ToolArtifactBorder:        hexColor("6C7086"),  // Overlay0
ToolArtifactHeader:        hexColor("45475A"),  // Surface1
ToolArtifactBody:          hexColor("1E1E2E"),  // Base
```

**Step 2: Run test for catppuccin theme**

Run: `go test ./internal/theme/... -v`

Expected: PASS

**Step 3: Update remaining 9 theme files**

Repeat Step 1 for each theme file with appropriate colors:
- dracula.go
- gruvbox.go
- material.go
- monochrome.go
- nord.go
- rama.go
- solarized.go
- tokyo_night.go

**Step 4: Run all theme tests**

Run: `go test ./internal/theme/... -v`

Expected: All PASS

**Step 5: Commit**

```bash
git add internal/theme/themes/
git commit -m "feat(theme): add new color fields to all themes"
```

---

### Task 3: Update Theme Registration System

**Files:**
- Modify: `internal/theme/themes/register.go`
- Modify: `internal/theme/manager.go`

**Step 1: Add theme registration options**

In `internal/theme/themes/register.go`, add support for theme options:

```go
// themeOption configures theme registration
type themeOption func(*Theme)

// Register registers a theme with optional configuration
func Register(name string, base Theme, opts ...themeOption) {
    theme := base
    for _, opt := range opts {
        opt(&theme)
    }
    registry[name] = theme
}
```

**Step 2: Add darkenHex helper**

```go
// darkenHex darkens a hex color by the given percentage (0-100)
func darkenHex(hex string, percent int) string {
    c := hexColor(hex)
    factor := float64(100-percent) / 100
    r := uint8(float64(c.R) * factor)
    g := uint8(float64(c.G) * factor)
    b := uint8(float64(c.B) * factor)
    return fmt.Sprintf("%02X%02X%02X", r, g, b)
}
```

**Step 3: Update manager.go to sort theme list**

In `internal/theme/manager.go`, update the `List()` method:

```go
func (m *Manager) List() []string {
    names := make([]string, 0, len(m.themes))
    for name := range m.themes {
        names = append(names, name)
    }
    slices.Sort(names)  // Add this line
    return names
}
```

**Step 4: Add slices import**

Add to imports in manager.go:
```go
import "slices"
```

**Step 5: Run tests**

Run: `go test ./internal/theme/... -v`

Expected: PASS

**Step 6: Commit**

```bash
git add internal/theme/themes/register.go internal/theme/manager.go
git commit -m "feat(theme): add theme registration options and sorted listing"
```

---

## Phase 2: Create Missing Foundation Components

### Task 4: Create Dialog Common Utilities

**Files:**
- Create: `internal/components/dialog/common.go`
- Test: `internal/components/dialog/common_test.go`

**Step 1: Create common.go with utilities**

```go
package dialog

import (
    "strings"
    "unicode"
)

// maxVisibleItems is the maximum number of items to display in a dialog
const maxVisibleItems = 10

// visibleBounds returns the start and end indices for visible items
func visibleBounds(cursor, total int) (start, end int) {
    if total <= maxVisibleItems {
        return 0, total
    }
    
    half := maxVisibleItems / 2
    if cursor < half {
        return 0, maxVisibleItems
    }
    if cursor > total-half {
        return total - maxVisibleItems, total
    }
    return cursor - half, cursor + half
}

// matchesQuery checks if text matches a search query
func matchesQuery(text, query string) bool {
    if query == "" {
        return true
    }
    return strings.Contains(strings.ToLower(text), strings.ToLower(query))
}

// hasWordPrefix checks if text has query as a word prefix
func hasWordPrefix(text, query string) bool {
    if query == "" {
        return true
    }
    query = strings.ToLower(query)
    text = strings.ToLower(text)
    
    // Check exact match
    if strings.HasPrefix(text, query) {
        return true
    }
    
    // Check word boundary match
    for i, r := range text {
        if i > 0 && unicode.IsSpace(rune(text[i-1])) {
            if strings.HasPrefix(text[i:], query) {
                return true
            }
        }
    }
    return false
}
```

**Step 2: Write tests for common utilities**

```go
package dialog

import "testing"

func TestVisibleBounds(t *testing.T) {
    tests := []struct {
        cursor, total, wantStart, wantEnd int
    }{
        {0, 5, 0, 5},      // Total less than max
        {0, 20, 0, 10},    // Cursor at start
        {19, 20, 10, 20},  // Cursor at end
        {10, 20, 5, 15},   // Cursor in middle
    }
    
    for _, tt := range tests {
        start, end := visibleBounds(tt.cursor, tt.total)
        if start != tt.wantStart || end != tt.wantEnd {
            t.Errorf("visibleBounds(%d, %d) = (%d, %d), want (%d, %d)",
                tt.cursor, tt.total, start, end, tt.wantStart, tt.wantEnd)
        }
    }
}

func TestMatchesQuery(t *testing.T) {
    if !matchesQuery("Hello World", "hello") {
        t.Error("expected match")
    }
    if matchesQuery("Hello World", "xyz") {
        t.Error("expected no match")
    }
}

func TestHasWordPrefix(t *testing.T) {
    if !hasWordPrefix("Hello World", "hel") {
        t.Error("expected match at start")
    }
    if !hasWordPrefix("Hello World", "wor") {
        t.Error("expected match at word boundary")
    }
    if hasWordPrefix("Hello World", "ell") {
        t.Error("expected no match in middle of word")
    }
}
```

**Step 3: Run tests**

Run: `go test ./internal/components/dialog/... -v`

Expected: PASS

**Step 4: Commit**

```bash
git add internal/components/dialog/common.go internal/components/dialog/common_test.go
git commit -m "feat(dialog): add common utilities for dialog components"
```

---

### Task 5: Create Footer Component

**Files:**
- Create: `internal/components/status/footer.go`
- Test: `internal/components/status/footer_test.go`

**Step 1: Create footer.go**

```go
package status

import (
    "fmt"
    
    "github.com/Nomadcxx/smolbot/internal/theme"
    "github.com/charmbracelet/lipgloss"
)

// Footer displays token usage and session information
type Footer struct {
    theme          *theme.Manager
    width          int
    model          string
    session        string
    usage          int64
    contextWindow  int64
    compact        bool
}

// NewFooter creates a new footer component
func NewFooter(theme *theme.Manager) *Footer {
    return &Footer{
        theme: theme,
    }
}

// SetSize sets the footer width
func (f *Footer) SetSize(width int) {
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
func (f *Footer) SetUsage(used, window int64) {
    f.usage = used
    f.contextWindow = window
}

// SetCompact enables compact mode
func (f *Footer) SetCompact(compact bool) {
    f.compact = compact
}

// View renders the footer
func (f *Footer) View() string {
    t := f.theme.Current()
    
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
    if f.contextWindow > 0 {
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

// Height returns the footer height
func (f *Footer) Height() int {
    if f.compact {
        return 1
    }
    return 1
}

// formatUsage formats the usage string
func (f *Footer) formatUsage() string {
    if f.compact {
        pct := float64(f.usage) / float64(f.contextWindow) * 100
        return fmt.Sprintf("%.0f%%", pct)
    }
    return fmt.Sprintf("%s / %s tokens", 
        formatTokens(f.usage), 
        formatTokens(f.contextWindow))
}

// usageColor returns the appropriate color for usage level
func (f *Footer) usageColor() lipgloss.Color {
    t := f.theme.Current()
    pct := float64(f.usage) / float64(f.contextWindow)
    
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

**Step 2: Write tests for footer**

```go
package status

import (
    "strings"
    "testing"
    
    "github.com/Nomadcxx/smolbot/internal/theme"
)

func TestFooterView(t *testing.T) {
    tm := theme.NewManager()
    footer := NewFooter(tm)
    footer.SetSize(80)
    footer.SetModel("gpt-4")
    footer.SetSession("test-session")
    footer.SetUsage(500, 1000)
    
    view := footer.View()
    if !strings.Contains(view, "gpt-4") {
        t.Error("expected footer to contain model name")
    }
    if !strings.Contains(view, "test-session") {
        t.Error("expected footer to contain session name")
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

**Step 3: Run tests**

Run: `go test ./internal/components/status/... -v`

Expected: PASS

**Step 4: Commit**

```bash
git add internal/components/status/footer.go internal/components/status/footer_test.go
git commit -m "feat(status): add footer component with token usage display"
```

---

## Phase 3: Update Existing Components

### Task 6: Update Header Component

**Files:**
- Modify: `internal/components/header/header.go`

**Step 1: Add compact mode support**

Add to the Header struct:
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

**Step 2: Run tests**

Run: `go test ./internal/components/header/... -v`

Expected: PASS

**Step 3: Commit**

```bash
git add internal/components/header/header.go
git commit -m "feat(header): add compact mode support"
```

---

### Task 7: Update Editor Component

**Files:**
- Modify: `internal/components/chat/editor.go`

**Step 1: Add compact mode and quick start hint**

Add to Editor struct:
```go
type Editor struct {
    // ... existing fields ...
    compact      bool
    showHint     bool
}
```

Add SetCompact method:
```go
func (e *Editor) SetCompact(compact bool) {
    e.compact = compact
    if compact {
        e.textarea.SetHeight(1)
    } else {
        e.textarea.SetHeight(3)
    }
}
```

Add SetShowHint method:
```go
func (e *Editor) SetShowHint(show bool) {
    e.showHint = show
}
```

Update View() to include themed background and hint:
```go
func (e *Editor) View() string {
    t := e.theme.Current()
    
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

func (e *Editor) renderQuickStartHint() string {
    t := e.theme.Current()
    style := lipgloss.NewStyle().
        Foreground(t.Subtle).
        Italic(true)
    return style.Render("Press ? for help, / for commands")
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

### Task 8: Update Message Components

**Files:**
- Modify: `internal/components/chat/message.go`
- Modify: `internal/components/chat/messages.go`

**Step 1: Add helper functions to message.go**

Add at end of message.go:

```go
// colorHex converts a lipgloss color to hex string
func colorHex(c lipgloss.Color) string {
    return string(c)
}

// subtleWash returns a subtle version of a color
func subtleWash(c color.Color, factor float64) color.Color {
    return color.Color{
        R: uint8(float64(c.R) * factor),
        G: uint8(float64(c.G) * factor),
        B: uint8(float64(c.B) * factor),
    }
}

// transcriptRoleAccent returns the appropriate transcript color for a role
func transcriptRoleAccent(role string, t *theme.Theme) color.Color {
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

**Step 2: Update renderRoleBlock in message.go**

Enhance the role block rendering with richer styling:

```go
func renderRoleBlock(role, content string, t *theme.Theme) string {
    accent := transcriptRoleAccent(role, t)
    
    // Create subtle background
    bg := subtleWash(accent, 0.1)
    
    style := lipgloss.NewStyle().
        BorderLeft(true).
        BorderStyle(lipgloss.Border{
            Left: "┃",
        }).
        BorderForeground(lipgloss.Color(colorHex(accent))).
        Background(lipgloss.Color(colorHex(bg))).
        PaddingLeft(1).
        MarginLeft(1)
    
    return style.Render(content)
}
```

**Step 3: Update messages.go with custom markdown styling**

Add markdown style configuration:

```go
// markdownStyleConfig returns custom glamour styles for the theme
func markdownStyleConfig(t *theme.Theme) ansi.StyleConfig {
    return ansi.StyleConfig{
        Document: ansi.StyleBlock{
            StylePrimitive: ansi.StylePrimitive{
                BlockPrefix: "",
                BlockSuffix: "",
            },
        },
        Heading: ansi.StyleBlock{
            StylePrimitive: ansi.StylePrimitive{
                Color: stringPtr(colorHex(t.MarkdownHeading)),
                Bold:  boolPtr(true),
            },
        },
        Link: ansi.StylePrimitive{
            Color: stringPtr(colorHex(t.MarkdownLink)),
            Underline: boolPtr(true),
        },
        Code: ansi.StyleBlock{
            StylePrimitive: ansi.StylePrimitive{
                Color: stringPtr(colorHex(t.MarkdownCode)),
            },
        },
    }
}

func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool       { return &b }
```

**Step 4: Run tests**

Run: `go test ./internal/components/chat/... -v`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/components/chat/message.go internal/components/chat/messages.go
git commit -m "feat(chat): add rich message rendering with themed markdown"
```

---

### Task 9: Update Dialog Components

**Files:**
- Modify: `internal/components/dialog/sessions.go`
- Modify: `internal/components/dialog/models.go`
- Modify: `internal/components/dialog/commands.go`

**Step 1: Update sessions.go with windowing and vim keys**

Add to SessionsDialog struct:
```go
type SessionsDialog struct {
    // ... existing fields ...
    visibleStart int
    visibleEnd   int
}
```

Update Update method to handle vim keys:
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
    // Same as down
    if m.cursor < len(m.sessions)-1 {
        m.cursor++
        m.updateVisibleBounds()
    }
case "ctrl+p":
    // Same as up
    if m.cursor > 0 {
        m.cursor--
        m.updateVisibleBounds()
    }
```

Add updateVisibleBounds method:
```go
func (m *SessionsDialog) updateVisibleBounds() {
    m.visibleStart, m.visibleEnd = visibleBounds(m.cursor, len(m.sessions))
}
```

Update View to show only visible items:
```go
func (m *SessionsDialog) View() string {
    // ... existing setup ...
    
    var items []string
    for i := m.visibleStart; i < m.visibleEnd && i < len(m.sessions); i++ {
        session := m.sessions[i]
        // Render item (mark current with special style)
        item := m.renderSessionItem(session, i == m.cursor)
        items = append(items, item)
    }
    
    // ... rest of view ...
}
```

**Step 2: Repeat for models.go and commands.go**

Apply same windowing and key handling pattern to ModelsDialog and CommandsDialog.

**Step 3: Add descriptions to commands.go**

Add descriptions map:
```go
var commandDescriptions = map[string]string{
    "/clear":  "Clear the conversation history",
    "/models": "Show available AI models",
    "/sessions": "Manage conversation sessions",
    // ... etc
}

var commandAliases = map[string]string{
    "/c": "/clear",
    "/m": "/models",
    "/s": "/sessions",
    // ... etc
}
```

Update View to show descriptions:
```go
func (m *CommandsDialog) View() string {
    // ... existing setup ...
    
    for i, cmd := range visibleCommands {
        desc := commandDescriptions[cmd]
        line := lipgloss.JoinHorizontal(lipgloss.Left,
            style.Render(cmd),
            subtleStyle.Render(" - "+desc),
        )
        items = append(items, line)
    }
    
    // ... rest of view ...
}
```

**Step 4: Run tests**

Run: `go test ./internal/components/dialog/... -v`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/components/dialog/sessions.go internal/components/dialog/models.go internal/components/dialog/commands.go
git commit -m "feat(dialog): add windowing, vim keys, and descriptions"
```

---

## Phase 4: Create Menu Dialog

### Task 10: Create Menu Dialog Component

**Files:**
- Create: `internal/tui/menu_dialog.go`
- Test: `internal/tui/menu_dialog_test.go`

**Step 1: Create menu_dialog.go**

```go
package tui

import (
    "github.com/Nomadcxx/smolbot/internal/theme"
    "github.com/charmbracelet/bubbles/key"
    tea "github.com/charmbracelet/bubbletea"
    "github.com/charmbracelet/lipgloss"
)

// MenuDialog provides a multi-page menu system
type MenuDialog struct {
    theme      *theme.Manager
    width      int
    height     int
    active     bool
    page       menuPage
    selected   int
}

type menuPage int

const (
    menuPageMain menuPage = iota
    menuPageThemes
    menuPageSessions
)

type menuItem struct {
    label    string
    key      string
    action   func()
    subPage  menuPage
}

// NewMenuDialog creates a new menu dialog
func NewMenuDialog(theme *theme.Manager) *MenuDialog {
    return &MenuDialog{
        theme: theme,
        page:  menuPageMain,
    }
}

// Init initializes the menu dialog
func (m *MenuDialog) Init() tea.Cmd {
    return nil
}

// Update handles messages
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
            if m.selected > 0 {
                m.selected--
            }
        case "down", "j":
            items := m.currentItems()
            if m.selected < len(items)-1 {
                m.selected++
            }
        case "enter":
            m.executeSelected()
        case "left", "h":
            if m.page != menuPageMain {
                m.page = menuPageMain
                m.selected = 0
            }
        }
    }
    
    return m, nil
}

// View renders the menu
func (m *MenuDialog) View() string {
    if !m.active {
        return ""
    }
    
    t := m.theme.Current()
    
    // Create dialog box
    dialogStyle := lipgloss.NewStyle().
        Border(lipgloss.RoundedBorder()).
        BorderForeground(t.Primary).
        Background(t.Background).
        Padding(1, 2).
        Width(40)
    
    // Build content
    var content string
    switch m.page {
    case menuPageMain:
        content = m.renderMainPage()
    case menuPageThemes:
        content = m.renderThemesPage()
    case menuPageSessions:
        content = m.renderSessionsPage()
    }
    
    return dialogStyle.Render(content)
}

func (m *MenuDialog) renderMainPage() string {
    t := m.theme.Current()
    items := []menuItem{
        {label: "Themes", key: "t", subPage: menuPageThemes},
        {label: "Sessions", key: "s", subPage: menuPageSessions},
        {label: "Help", key: "?"},
        {label: "Quit", key: "q"},
    }
    
    return m.renderItems(items)
}

func (m *MenuDialog) renderThemesPage() string {
    themes := m.theme.List()
    var items []menuItem
    for _, theme := range themes {
        items = append(items, menuItem{label: theme})
    }
    return m.renderItems(items)
}

func (m *MenuDialog) renderSessionsPage() string {
    // This would integrate with actual session data
    return "Sessions list here..."
}

func (m *MenuDialog) renderItems(items []menuItem) string {
    t := m.theme.Current()
    
    var lines []string
    for i, item := range items {
        style := lipgloss.NewStyle()
        if i == m.selected {
            style = style.Background(t.Primary).Foreground(t.Background)
        } else {
            style = style.Foreground(t.Text)
        }
        
        line := style.Render("  " + item.label)
        lines = append(lines, line)
    }
    
    return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

func (m *MenuDialog) currentItems() []menuItem {
    switch m.page {
    case menuPageMain:
        return []menuItem{
            {label: "Themes"},
            {label: "Sessions"},
            {label: "Help"},
            {label: "Quit"},
        }
    case menuPageThemes:
        var items []menuItem
        for _, name := range m.theme.List() {
            items = append(items, menuItem{label: name})
        }
        return items
    default:
        return nil
    }
}

func (m *MenuDialog) executeSelected() {
    items := m.currentItems()
    if m.selected >= len(items) {
        return
    }
    
    item := items[m.selected]
    if item.subPage != 0 {
        m.page = item.subPage
        m.selected = 0
    } else if item.action != nil {
        item.action()
    }
}

// Show activates the menu
func (m *MenuDialog) Show() {
    m.active = true
    m.page = menuPageMain
    m.selected = 0
}

// Hide deactivates the menu
func (m *MenuDialog) Hide() {
    m.active = false
}

// IsActive returns whether the menu is currently shown
func (m *MenuDialog) IsActive() bool {
    return m.active
}

// SetSize sets the dialog size
func (m *MenuDialog) SetSize(width, height int) {
    m.width = width
    m.height = height
}

// KeyMap defines menu-specific key bindings
type MenuKeyMap struct {
    Toggle key.Binding
}

// DefaultMenuKeyMap returns default key bindings
func DefaultMenuKeyMap() MenuKeyMap {
    return MenuKeyMap{
        Toggle: key.NewBinding(
            key.WithKeys("f1", "ctrl+m"),
            key.WithHelp("F1/Ctrl+M", "menu"),
        ),
    }
}
```

**Step 2: Write basic test**

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
    
    menu.Show()
    if !menu.IsActive() {
        t.Error("expected menu to be active after Show()")
    }
}
```

**Step 3: Run tests**

Run: `go test ./internal/tui/... -v -run Menu`

Expected: PASS

**Step 4: Commit**

```bash
git add internal/tui/menu_dialog.go internal/tui/menu_dialog_test.go
git commit -m "feat(tui): add F1 menu dialog with multi-page navigation"
```

---

## Phase 5: Integration

### Task 11: Update Main TUI Controller

**Files:**
- Modify: `internal/tui/tui.go`

**Step 1: Add footer and menu to main model**

Add to Model struct:
```go
type Model struct {
    // ... existing fields ...
    footer      *status.Footer
    menu        *MenuDialog
    compact     bool
}
```

**Step 2: Initialize footer and menu in New()**

```go
func New(client gatewayClient, theme *theme.Manager) *Model {
    m := &Model{
        // ... existing initialization ...
        footer: status.NewFooter(theme),
        menu:   NewMenuDialog(theme),
    }
    
    // Set initial sizes
    m.updateSizes()
    
    return m
}
```

**Step 3: Add F1 menu key handling**

In Update method, add case:
```go
case "f1", "ctrl+m":
    if m.menu.IsActive() {
        m.menu.Hide()
    } else {
        m.menu.Show()
    }
    return m, nil
```

**Step 4: Add compact mode handling on resize**

```go
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height
    m.compact = msg.Width < 80 || msg.Height < 24
    m.updateSizes()
```

**Step 5: Add updateSizes method**

```go
func (m *Model) updateSizes() {
    if m.compact {
        m.header.SetCompact(true)
        m.editor.SetCompact(true)
        m.footer.SetCompact(true)
    }
    
    // Calculate available space for chat
    headerHeight := m.header.Height()
    footerHeight := m.footer.Height()
    editorHeight := 3
    if m.compact {
        editorHeight = 1
    }
    
    chatHeight := m.height - headerHeight - footerHeight - editorHeight - 2
    m.messages.SetSize(m.width-4, chatHeight)
    
    m.footer.SetSize(m.width)
    m.menu.SetSize(m.width/2, m.height/2)
}
```

**Step 6: Update View to include footer and menu**

```go
func (m Model) View() string {
    // Build layout
    var sections []string
    
    sections = append(sections, m.header.View())
    
    // Chat area with frame
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

**Step 7: Run full test suite**

Run: `go test ./internal/tui/... -v`

Expected: PASS

**Step 8: Commit**

```bash
git add internal/tui/tui.go
git commit -m "feat(tui): integrate footer, menu, and compact mode"
```

---

### Task 12: Update Client Protocol

**Files:**
- Modify: `internal/client/protocol.go`
- Modify: `internal/client/messages.go`

**Step 1: Add ChannelStatus struct**

```go
// ChannelStatus represents the status of a messaging channel
type ChannelStatus struct {
    Name   string `json:"name"`
    State  string `json:"state"`  // "connected", "disconnected", "error"
    Detail string `json:"detail,omitempty"`
}

// StatusPayload contains full status information
type StatusPayload struct {
    Gateway     string            `json:"gateway"`
    Channels    []ChannelStatus   `json:"channels,omitempty"`
    Usage       UsageInfo         `json:"usage"`
}
```

**Step 2: Update Status method signature**

```go
// Status retrieves current system status
func (c *Client) Status() (StatusPayload, error) {
    // ... implementation ...
}
```

**Step 3: Update ModelsSet to return current model**

```go
// ModelsSet sets the active model and returns the previous one
func (c *Client) ModelsSet(model string) (string, error) {
    // ... implementation ...
}
```

**Step 4: Run tests**

Run: `go test ./internal/client/... -v`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/client/protocol.go internal/client/messages.go
git commit -m "feat(client): add ChannelStatus and update Status/ModelsSet signatures"
```

---

## Phase 6: Testing and Verification

### Task 13: Build and Test Complete TUI

**Step 1: Build the TUI**

Run: `cd /home/nomadx/Documents/smolbot && go build -o smolbot-tui ./cmd/smolbot-tui`

Expected: SUCCESS (no errors)

**Step 2: Run all TUI tests**

Run: `go test ./internal/... -v`

Expected: All PASS

**Step 3: Test integration**

Run: `./smolbot-tui --help` or basic smoke test

Expected: Shows help or starts without errors

**Step 4: Commit any final fixes**

```bash
git add -A
git commit -m "chore: final integration fixes"
```

---

### Task 14: Push to Repository

**Step 1: Push changes**

```bash
git push origin main
```

Expected: SUCCESS

---

## Summary

This plan ports all missing UI/UX features from nanobot-tui to smolbot-tui:

**Phase 1:** Theme system expansion (3 tasks)
**Phase 2:** Foundation components (2 tasks)  
**Phase 3:** Component updates (4 tasks)
**Phase 4:** Menu system (1 task)
**Phase 5:** Integration (2 tasks)
**Phase 6:** Testing (2 tasks)

**Total:** 14 tasks, approximately 2-3 hours of implementation time with testing at each step.
