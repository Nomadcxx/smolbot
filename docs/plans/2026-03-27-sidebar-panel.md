# Sidebar Panel — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a collapsible sidebar panel to the TUI that shows session info, context/token state, connected channels, active MCP servers, and scheduled/cron tasks — providing persistent situational awareness without needing `/status` or dialogs.

**Architecture:** The sidebar is a new component that sits to the **right** of the main chat area (following crush's convention). The main TUI layout changes from a single vertical column to a horizontal split (main | sidebar). The sidebar consumes data that already flows through existing gateway events and status responses — no new backend protocol is needed initially, though we add a `cron.list` method for scheduled tasks.

**Reference implementations:**
- **crush** (`/home/nomadx/crush/internal/ui/model/sidebar.go`) — Primary reference. Right-side sidebar, 30 chars wide, sections for session/model/files/LSPs/MCPs. Compact mode (`Ctrl+D`) at width < 120 replaces sidebar with a horizontal overlay. Smart dynamic height distribution across sections.
- **opencode** (`/home/nomadx/opencode/packages/tui/`) — No sidebar. Uses inline header stats (tokens/cost/%) and modal dialogs for everything. Max 86-char container. Status bar at bottom. Good reference for what a "no-sidebar" fallback looks like.

**Tech Stack:** Go 1.26, charm.land/bubbletea v2.0.2, charm.land/lipgloss v2.0.2

---

## Current State

- **Layout**: Vertical stack — Header → Transcript → Status → Editor → Footer. No horizontal splits.
- **Session info**: Session name shown in footer only. Session list available as modal dialog.
- **Context info**: Token usage % in header + footer. Compression indicator in footer.
- **Channels**: Channel status available from `status()` response but only shown via `/status` command output.
- **MCPs**: Config has `tools.mcpServers` map but no runtime status exposed to TUI.
- **Cron/Timers**: No infrastructure exists. Needs to be designed.
- **Reusable components**: `internal/components/chatlist/list.go` exists as an unused generic scrollable list.
- **`internal/tui/drawable.go`**: Defines unused `Drawable` interface.

---

## Design Decisions (Informed by crush & opencode)

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Sidebar position | **Right** | Follows crush convention. Chat content (the primary focus) should be leftmost where the eye naturally starts. |
| Width | **30 chars fixed** | Matches crush. 22 is too narrow for status lines like `● WhatsApp connected`. |
| Compact breakpoint | **Width < 120** | Matches crush. Below this the sidebar auto-hides. |
| Compact mode toggle | **`Ctrl+D`** | Matches crush's "details" keybinding. In compact mode, toggle shows a horizontal overlay above the chat with sidebar sections laid out side-by-side. |
| Section overflow | **"…and N more"** pattern | Matches crush. Consistent truncation with a count of hidden items. |
| Status indicators | **`●` colored** | Matches crush: green (connected/online), yellow (starting/busy), red (error), gray (offline/disabled). |
| Dynamic height | **Smart distribution** | Follows crush's `getDynamicHeightLimits()` pattern — split available height evenly, cap per-section, redistribute overflow by priority. |
| Mouse interaction | **Consume clicks, no action** | Matches crush. Sidebar is read-only, clicks don't change focus. |
| Section headers | **UPPERCASE + separator line** | Matches crush's `Section()` pattern with a horizontal rule after the title. |

---

## File Map

| File | Change |
|------|--------|
| `internal/components/sidebar/sidebar.go` | **New** — Main sidebar component with dynamic height distribution |
| `internal/components/sidebar/section.go` | **New** — Section interface and shared rendering helpers |
| `internal/components/sidebar/session_section.go` | **New** — Session name + CWD + model info section |
| `internal/components/sidebar/context_section.go` | **New** — Token usage bar, counts, compression status |
| `internal/components/sidebar/channels_section.go` | **New** — Connected channels (WhatsApp, Signal) with status |
| `internal/components/sidebar/mcps_section.go` | **New** — MCP server status list |
| `internal/components/sidebar/cron_section.go` | **New** — Scheduled tasks / cron jobs list |
| `internal/tui/tui.go` | Add sidebar model; restructure layout to horizontal split; add `Ctrl+D` toggle; add compact mode overlay; forward events to sidebar |
| `internal/app/state.go` | Persist sidebar visibility preference |
| `internal/client/types.go` | Add `CronJob` type |
| `internal/client/messages.go` | Add `CronJobs()` gateway call |
| `pkg/gateway/server.go` | Add `cron.list` method (when cron infrastructure exists) |

---

## Sidebar Visual Design

### Normal Mode (width ≥ 120)

```
┌── Chat ─────────────────────────────────────────┬── Details ─────────────┐
│                                                  │                        │
│  USER                                            │  SESSION               │
│  What is the weather?                            │  ─────────────────     │
│                                                  │  tui:main              │
│  ASSISTANT                                       │  ~/Documents/smolbot   │
│  I'll check the weather for you using the        │  kimi-k2.5:cloud       │
│  web_search tool...                              │                        │
│                                                  │  CONTEXT               │
│  ─── system ───                                  │  ─────────────────     │
│  Context compacted: 120K → 70K (42%)             │  ██████████░░░░ 78%    │
│                                                  │  99.8K / 128K          │
│                                                  │  ↓ 42% compacted       │
│                                                  │                        │
│                                                  │  CHANNELS              │
│                                                  │  ─────────────────     │
│                                                  │  ● WhatsApp            │
│                                                  │  ○ Signal              │
│                                                  │                        │
│                                                  │  MCPS                  │
│                                                  │  ─────────────────     │
│                                                  │  ● hybrid-memory       │
│                                                  │                        │
│                                                  │  SCHEDULED             │
│                                                  │  ─────────────────     │
│                                                  │  ⏱ check-deploys       │
│                                                  │    every 5m            │
│                                                  │  ⏱ backup-notes        │
│                                                  │    daily 02:00         │
│                                                  │                        │
├──────────────────────────────────────────────────┤                        │
│ > _                                              │                        │
├──────────────────────────────────────────────────┴────────────────────────┤
│ ● connected │ kimi-k2.5:cloud │ tui:main                          78%   │
└──────────────────────────────────────────────────────────────────────────┘
```

### Compact Mode (width < 120, `Ctrl+D` pressed)

Following crush's pattern — sidebar sections appear as a horizontal overlay at the top of the chat area:

```
┌──────────────────────────────────────────────────────────────────────────┐
│ ┌── Session ─────┐ ┌── Context ──────┐ ┌── Channels ──┐ ┌── MCPs ────┐ │
│ │ tui:main       │ │ ██████░░░ 78%   │ │ ● WhatsApp   │ │ ● hybrid-  │ │
│ │ kimi-k2.5      │ │ 99.8K / 128K    │ │ ○ Signal     │ │   memory   │ │
│ │                │ │ ↓ 42% compacted │ │              │ │            │ │
│ └────────────────┘ └─────────────────┘ └──────────────┘ └────────────┘ │
├──────────────────────────────────────────────────────────────────────────┤
│  USER                                                                    │
│  What is the weather?                                                    │
│  ...                                                                     │
└──────────────────────────────────────────────────────────────────────────┘
```

### Sidebar Specs

- **Width**: Fixed 30 chars (matching crush).
- **Position**: Right side of chat area.
- **Breakpoints**:
  - **≥120 cols**: Sidebar visible by default. `Ctrl+D` toggles it off.
  - **80-119 cols**: Sidebar hidden by default. `Ctrl+D` shows compact overlay.
  - **<80 cols**: Sidebar disabled. `Ctrl+D` does nothing (terminal too narrow).
- **Separator**: Single `│` character column between chat and sidebar, styled with `t.Border` color.
- **Persistence**: Sidebar visibility stored in `~/.config/smolbot-tui/state.json`.

---

## Task 1: Sidebar Component Scaffold + Section System

**Files:**
- New: `internal/components/sidebar/sidebar.go`
- New: `internal/components/sidebar/section.go`

Build the core sidebar component with crush-inspired dynamic height distribution.

- [ ] **Step 1: Define section interface**

```go
// internal/components/sidebar/section.go
package sidebar

import "github.com/Nomadcxx/smolbot/internal/theme"

// Section represents a discrete block within the sidebar.
type Section interface {
    Title() string
    // Render draws the section content (excluding the title header).
    // maxItems limits list entries; 0 means no limit.
    Render(width, maxItems int, t *theme.Theme) string
    // ItemCount returns the number of displayable items (for height allocation).
    ItemCount() int
}

// renderSectionHeader draws "TITLE" followed by a separator line.
// Follows crush's common.Section() pattern.
func renderSectionHeader(title string, width int, t *theme.Theme) string {
    styled := lipgloss.NewStyle().Bold(true).Foreground(t.TextMuted).Render(strings.ToUpper(title))
    line := lipgloss.NewStyle().Foreground(t.Border).Render(strings.Repeat("─", width))
    return styled + "\n" + line
}
```

- [ ] **Step 2: Build sidebar model with dynamic height**

Following crush's `getDynamicHeightLimits()` approach:

```go
// internal/components/sidebar/sidebar.go
package sidebar

const (
    DefaultWidth    = 30
    minItemsPerSection = 2
)

type Model struct {
    width    int
    height   int
    visible  bool
    sections []Section

    // Per-section defaults and ceilings
    sectionDefaults map[string]int // default max items per section title
}

func New() Model {
    return Model{
        width:   DefaultWidth,
        visible: true,
        sectionDefaults: map[string]int{
            "CHANNELS":  8,
            "MCPS":      8,
            "SCHEDULED": 6,
        },
    }
}

// getDynamicLimits distributes available height across sections.
// Mirrors crush's getDynamicHeightLimits() from sidebar.go:56-99.
func (m Model) getDynamicLimits(availableHeight int) map[string]int {
    limits := make(map[string]int)
    listSections := []Section{} // sections with variable-length content
    fixedHeight := 0

    for _, s := range m.sections {
        count := s.ItemCount()
        if count <= 0 {
            fixedHeight += 3 // header + "none" + spacing
            continue
        }
        listSections = append(listSections, s)
    }

    remaining := availableHeight - fixedHeight
    if remaining <= 0 || len(listSections) == 0 {
        for _, s := range m.sections {
            limits[s.Title()] = minItemsPerSection
        }
        return limits
    }

    // Divide evenly, cap at defaults, redistribute excess
    perSection := remaining / len(listSections)
    for _, s := range listSections {
        title := s.Title()
        ceiling := m.sectionDefaults[title]
        if ceiling == 0 { ceiling = 10 }
        limit := min(perSection, ceiling)
        limit = max(limit, minItemsPerSection)
        limits[title] = limit
    }
    return limits
}

func (m Model) View() string {
    if !m.visible { return "" }

    t := theme.Current()
    headerHeight := 0
    var blocks []string

    // Calculate height budget
    limits := m.getDynamicLimits(m.height)

    for i, s := range m.sections {
        header := renderSectionHeader(s.Title(), m.width-2, t)
        content := s.Render(m.width-2, limits[s.Title()], t)
        block := header + "\n" + content
        blocks = append(blocks, block)
        if i < len(m.sections)-1 {
            blocks = append(blocks, "") // spacing
        }
    }

    inner := lipgloss.JoinVertical(lipgloss.Left, blocks...)
    return lipgloss.NewStyle().
        Width(m.width).
        Height(m.height).
        Padding(1, 1).
        Render(inner)
}
```

- [ ] **Step 3: Build compact overlay renderer**

For terminals < 120 cols, render sections side-by-side:

```go
func (m Model) CompactView() string {
    if len(m.sections) == 0 { return "" }
    t := theme.Current()
    sectionWidth := (m.width - len(m.sections)) / len(m.sections)
    maxItems := 3 // compact mode shows fewer items

    var cols []string
    for _, s := range m.sections {
        header := renderSectionHeader(s.Title(), sectionWidth, t)
        content := s.Render(sectionWidth, maxItems, t)
        col := lipgloss.NewStyle().Width(sectionWidth).Render(header + "\n" + content)
        cols = append(cols, col)
    }
    return lipgloss.JoinHorizontal(lipgloss.Top, cols...)
}
```

- [ ] **Step 4: Write tests**

Test dynamic height allocation: even split, ceiling capping, overflow redistribution. Test View rendering dimensions. Test CompactView horizontal layout. Test empty sections.

---

## Task 2: Session Info Section (Header Block)

**Files:**
- New: `internal/components/sidebar/session_section.go`

Following crush's pattern: session name, CWD, model name grouped together as the top section.

- [ ] **Step 1: Implement session section**

```go
type SessionSection struct {
    sessionKey string
    cwd        string
    model      string
}

func (s *SessionSection) SetSession(key string) { s.sessionKey = key }
func (s *SessionSection) SetCWD(cwd string)     { s.cwd = cwd }
func (s *SessionSection) SetModel(model string)  { s.model = model }
func (s SessionSection) Title() string            { return "SESSION" }
func (s SessionSection) ItemCount() int           { return 0 } // fixed-height section

func (s SessionSection) Render(width, _ int, t *theme.Theme) string {
    name := s.sessionKey
    if name == "" { name = "—" }

    // Session name in primary text
    line1 := lipgloss.NewStyle().Foreground(t.TextPrimary).
        Width(width).Render(truncate(name, width))

    // CWD in muted text, collapse home dir to ~
    cwd := prettyPath(s.cwd)
    line2 := lipgloss.NewStyle().Foreground(t.TextMuted).
        Width(width).Render(truncate(cwd, width))

    // Model name in accent
    line3 := lipgloss.NewStyle().Foreground(t.Accent).
        Width(width).Render(truncate(s.model, width))

    return line1 + "\n" + line2 + "\n" + line3
}

// prettyPath replaces home dir prefix with ~
func prettyPath(path string) string {
    home, _ := os.UserHomeDir()
    if home != "" && strings.HasPrefix(path, home) {
        return "~" + path[len(home):]
    }
    return path
}

// truncate clips a string to fit width, using ANSI-aware truncation
func truncate(s string, width int) string {
    if lipgloss.Width(s) <= width { return s }
    return ansi.Truncate(s, width-1, "…")
}
```

- [ ] **Step 2: Write tests**

Test rendering with long session names, long paths, missing model. Test `prettyPath` home dir collapsing. Test truncation.

---

## Task 3: Context Section (Token Bar)

**Files:**
- New: `internal/components/sidebar/context_section.go`

Visual progress bar with token counts and compression status. The most valuable sidebar section.

- [ ] **Step 1: Implement context section**

```go
type ContextSection struct {
    usage       client.UsageInfo
    compression *client.CompressionInfo
}

func (s *ContextSection) SetUsage(u client.UsageInfo)              { s.usage = u }
func (s *ContextSection) SetCompression(c *client.CompressionInfo) { s.compression = c }
func (s ContextSection) Title() string                              { return "CONTEXT" }
func (s ContextSection) ItemCount() int                             { return 0 } // fixed-height

func (s ContextSection) Render(width, _ int, t *theme.Theme) string {
    if s.usage.ContextWindow <= 0 {
        return lipgloss.NewStyle().Foreground(t.TextMuted).Render("—")
    }

    pct := float64(s.usage.TotalTokens) / float64(s.usage.ContextWindow)
    pctInt := int(pct*100 + 0.5)

    // Progress bar: ██████████░░░░ XX%
    pctLabel := fmt.Sprintf(" %d%%", pctInt)
    barWidth := width - lipgloss.Width(pctLabel)
    if barWidth < 4 { barWidth = 4 }
    filled := int(pct * float64(barWidth))
    filled = max(0, min(filled, barWidth))
    empty := barWidth - filled

    var barColor color.Color
    switch {
    case pct >= 0.9:  barColor = t.TokenHighUsage
    case pct >= 0.8:  barColor = t.Warning
    case pct >= 0.6:  barColor = t.CompressionWarning
    default:          barColor = t.Accent
    }

    bar := lipgloss.NewStyle().Foreground(barColor).Render(strings.Repeat("█", filled)) +
        lipgloss.NewStyle().Foreground(t.TextMuted).Render(strings.Repeat("░", empty)) +
        lipgloss.NewStyle().Foreground(barColor).Render(pctLabel)

    // Token counts: 99.8K / 128K
    tokens := lipgloss.NewStyle().Foreground(t.TextMuted).
        Render(fmt.Sprintf("%s / %s", formatTokens(s.usage.TotalTokens), formatTokens(s.usage.ContextWindow)))

    lines := []string{bar, tokens}

    // Compression indicator: ↓ 42% compacted
    if s.compression != nil && s.compression.Enabled && s.compression.ReductionPercent > 0 {
        comp := lipgloss.NewStyle().Foreground(t.CompressionSuccess).
            Render(fmt.Sprintf("↓ %.0f%% compacted", s.compression.ReductionPercent))
        lines = append(lines, comp)
    }

    return strings.Join(lines, "\n")
}
```

- [ ] **Step 2: Write tests**

Test bar rendering at 0%, 50%, 78%, 90%, 100%. Test color thresholds match footer colors. Test with/without compression. Test very narrow widths.

---

## Task 4: Channels Section

**Files:**
- New: `internal/components/sidebar/channels_section.go`

Connected messaging channels with live status indicators. This is the key smolbot differentiator.

- [ ] **Step 1: Implement channels section**

Use crush's `●` status icon pattern with color coding:

```go
type ChannelEntry struct {
    Name  string
    State string // "connected", "disconnected", "error", "qr", "registered"
}

type ChannelsSection struct {
    channels []ChannelEntry
}

func (s *ChannelsSection) SetChannels(channels []ChannelEntry) { s.channels = channels }
func (s ChannelsSection) Title() string                         { return "CHANNELS" }
func (s ChannelsSection) ItemCount() int                        { return len(s.channels) }

func (s ChannelsSection) Render(width, maxItems int, t *theme.Theme) string {
    if len(s.channels) == 0 {
        return lipgloss.NewStyle().Foreground(t.TextMuted).Render("none configured")
    }

    visible := s.channels
    overflow := 0
    if maxItems > 0 && len(visible) > maxItems {
        overflow = len(visible) - maxItems + 1
        visible = visible[:maxItems-1]
    }

    var lines []string
    for _, ch := range visible {
        icon, iconColor := statusIcon(ch.State, t)
        name := lipgloss.NewStyle().Foreground(t.TextPrimary).Render(ch.Name)
        state := lipgloss.NewStyle().Foreground(t.TextMuted).Render(ch.State)
        line := lipgloss.NewStyle().Foreground(iconColor).Render(icon) +
            " " + name + " " + state
        lines = append(lines, truncate(line, width))
    }

    // Crush-style overflow indicator
    if overflow > 0 {
        lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).
            Render(fmt.Sprintf("…and %d more", overflow)))
    }

    return strings.Join(lines, "\n")
}

func statusIcon(state string, t *theme.Theme) (string, color.Color) {
    switch state {
    case "connected":
        return "●", t.Accent          // green
    case "error":
        return "●", t.TokenHighUsage  // red
    case "qr", "starting":
        return "●", t.Warning         // yellow
    default: // disconnected, registered, offline
        return "●", t.TextMuted       // gray
    }
}
```

- [ ] **Step 2: Wire channel updates from gateway**

The `StatusPayload.Channels` already carries `[]ChannelStatus{Name, Status}`. Map these to `ChannelEntry` structs in the TUI event handler.

- [ ] **Step 3: Write tests**

Test rendering with 0, 1, 3 channels. Test all status states and icon colors. Test overflow with maxItems=2 and 5 channels.

---

## Task 5: MCP Servers Section

**Files:**
- New: `internal/components/sidebar/mcps_section.go`

Same pattern as channels but for MCP servers.

- [ ] **Step 1: Implement MCPs section**

```go
type MCPEntry struct {
    Name   string
    Status string // "connected", "configured", "error", "disabled"
    Tools  int    // number of tools provided
}

type MCPsSection struct {
    servers []MCPEntry
}

func (s MCPsSection) Title() string    { return "MCPS" }
func (s MCPsSection) ItemCount() int   { return len(s.servers) }

func (s MCPsSection) Render(width, maxItems int, t *theme.Theme) string {
    if len(s.servers) == 0 {
        return lipgloss.NewStyle().Foreground(t.TextMuted).Render("none")
    }

    visible := s.servers
    overflow := 0
    if maxItems > 0 && len(visible) > maxItems {
        overflow = len(visible) - maxItems + 1
        visible = visible[:maxItems-1]
    }

    var lines []string
    for _, srv := range visible {
        icon, iconColor := statusIcon(srv.Status, t)
        name := lipgloss.NewStyle().Foreground(t.TextPrimary).Render(srv.Name)
        extra := ""
        if srv.Tools > 0 {
            extra = lipgloss.NewStyle().Foreground(t.TextMuted).
                Render(fmt.Sprintf(" %d tools", srv.Tools))
        }
        line := lipgloss.NewStyle().Foreground(iconColor).Render(icon) +
            " " + name + extra
        lines = append(lines, truncate(line, width))
    }

    if overflow > 0 {
        lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).
            Render(fmt.Sprintf("…and %d more", overflow)))
    }

    return strings.Join(lines, "\n")
}
```

- [ ] **Step 2: Populate from config initially**

Parse `config.Tools.MCPServers` to get server names. Show all as "configured" until a `mcps.list` gateway method exists (see F1 menu plan).

- [ ] **Step 3: Write tests**

Test empty, one, multiple servers. Test overflow. Test tool count display.

---

## Task 6: Scheduled Tasks / Cron Section

**Files:**
- New: `internal/components/sidebar/cron_section.go`
- Modify: `internal/client/types.go`

This is the smolbot-unique feature: showing what the assistant has scheduled.

- [ ] **Step 1: Define cron types**

```go
// internal/client/types.go
type CronJob struct {
    ID       string `json:"id"`
    Name     string `json:"name"`
    Schedule string `json:"schedule"` // human-readable: "every 5m", "daily 02:00"
    Status   string `json:"status"`   // "active", "paused", "completed"
    NextRun  string `json:"nextRun"`  // ISO 8601 timestamp
}
```

- [ ] **Step 2: Implement cron section**

Each job takes 2 lines (name + schedule), following a compact display:

```go
type CronSection struct {
    jobs []client.CronJob
}

func (s CronSection) Title() string    { return "SCHEDULED" }
func (s CronSection) ItemCount() int   { return len(s.jobs) }

func (s CronSection) Render(width, maxItems int, t *theme.Theme) string {
    if len(s.jobs) == 0 {
        return lipgloss.NewStyle().Foreground(t.TextMuted).Render("none")
    }

    visible := s.jobs
    overflow := 0
    if maxItems > 0 && len(visible) > maxItems {
        overflow = len(visible) - maxItems + 1
        visible = visible[:maxItems-1]
    }

    var lines []string
    for _, job := range visible {
        icon := "⏱"
        iconColor := t.Accent
        if job.Status == "paused" {
            icon = "⏸"
            iconColor = t.TextMuted
        }
        name := lipgloss.NewStyle().Foreground(iconColor).Render(icon) +
            " " + lipgloss.NewStyle().Foreground(t.TextPrimary).Render(job.Name)
        schedule := "  " + lipgloss.NewStyle().Foreground(t.TextMuted).Render(job.Schedule)
        lines = append(lines, truncate(name, width), truncate(schedule, width))
    }

    if overflow > 0 {
        lines = append(lines, lipgloss.NewStyle().Foreground(t.TextMuted).
            Render(fmt.Sprintf("…and %d more", overflow)))
    }

    return strings.Join(lines, "\n")
}
```

- [ ] **Step 3: Gateway integration (deferred)**

The cron backend is a separate feature. For initial implementation, the section renders "none" until a cron system exists and feeds data through a `cron.list` gateway method.

- [ ] **Step 4: Write tests**

Test with 0, 1, 5 jobs. Test active vs paused indicators. Test overflow. Test 2-line-per-item height calculation.

---

## Task 7: Main TUI Layout Restructure

**Files:**
- Modify: `internal/tui/tui.go`
- Modify: `internal/app/state.go`

This is the core integration task. The layout changes from a single vertical column to a horizontal split following crush's pattern.

- [ ] **Step 1: Add sidebar + compact mode state to Model**

```go
type Model struct {
    // ... existing fields
    sidebar     sidebar.Model
    compactMode bool  // auto-set when width < 120
    detailsOpen bool  // Ctrl+D toggle in compact mode
}
```

Initialize in `New()`:
```go
m.sidebar = sidebar.New()
// Sections registered on init
```

- [ ] **Step 2: Add `Ctrl+D` toggle (matching crush)**

```go
case "ctrl+d":
    if m.compactMode {
        m.detailsOpen = !m.detailsOpen
    } else {
        m.sidebar.Toggle()
    }
    m.recalcLayout()
    return m, nil
```

- [ ] **Step 3: Restructure `WindowSizeMsg` handler**

```go
case tea.WindowSizeMsg:
    m.width = msg.Width
    m.height = msg.Height

    // Auto compact mode at crush's breakpoint
    m.compactMode = m.width < 120

    if m.compactMode {
        m.sidebar.SetVisible(false)
        m.detailsOpen = false // reset on resize
    }

    m.recalcLayout()
```

```go
func (m *Model) recalcLayout() {
    sidebarWidth := 0
    if !m.compactMode && m.sidebar.Visible() {
        sidebarWidth = m.sidebar.Width() + 1 // +1 for separator
    }
    chatWidth := m.width - sidebarWidth

    m.sidebar.SetSize(m.sidebar.Width(), m.height-2) // full height minus footer

    // Existing vertical sizing with chatWidth instead of m.width
    m.header.SetWidth(chatWidth)
    m.messages.SetSize(chatWidth-2, transcriptHeight)
    m.editor.SetWidth(chatWidth)
    m.footer.SetWidth(m.width) // footer spans full width
    // ... etc
}
```

- [ ] **Step 4: Restructure `View()` method**

```go
func (m Model) View() string {
    t := theme.Current()

    // Build main chat column (same as before)
    main := lipgloss.JoinVertical(lipgloss.Left,
        m.header.View(),
        m.messages.View(),
        m.status.View(),
        m.editor.View(),
    )

    // In compact mode with details open, prepend horizontal overlay
    if m.compactMode && m.detailsOpen {
        overlay := m.sidebar.CompactView()
        main = lipgloss.JoinVertical(lipgloss.Left, overlay, main)
    }

    // Normal mode with sidebar visible
    if !m.compactMode && m.sidebar.Visible() {
        sep := lipgloss.NewStyle().
            Foreground(t.Border).
            Render(strings.Repeat("│\n", m.height-3) + "│")

        main = lipgloss.JoinHorizontal(lipgloss.Top,
            main,
            sep,
            m.sidebar.View(),
        )
    }

    // Footer always spans full width
    full := lipgloss.JoinVertical(lipgloss.Left, main, m.footer.View())

    return m.renderWithDialog(full)
}
```

- [ ] **Step 5: Consume mouse clicks in sidebar area**

Following crush's pattern:
```go
case tea.MouseMsg:
    // If click is in sidebar area (x > chatWidth), consume and ignore
    if !m.compactMode && m.sidebar.Visible() && msg.X >= m.width - m.sidebar.Width() {
        return m, nil
    }
    // ... existing mouse handling
```

- [ ] **Step 6: Persist sidebar/details state**

In `internal/app/state.go`:
```go
type State struct {
    LastSession    string `json:"lastSession"`
    SidebarVisible *bool  `json:"sidebarVisible,omitempty"`
}
```

- [ ] **Step 7: Forward events to sidebar**

In existing event handlers, add sidebar updates:
```go
case StatusLoadedMsg:
    // ... existing handling
    m.sidebar.SetSession(m.app.Session)
    m.sidebar.SetCWD(m.app.Workspace)
    m.sidebar.SetModel(m.app.Model)
    m.sidebar.SetUsage(msg.Payload.Usage)
    m.sidebar.SetChannels(mapChannels(msg.Payload.Channels))

case "context.compressed":
    // ... existing handling
    m.sidebar.SetCompression(&p)

case "chat.usage":
    // ... existing handling
    m.sidebar.SetUsage(usageInfo)
```

- [ ] **Step 8: Write tests**

Test layout with sidebar visible (verify widths sum to terminal width). Test layout with sidebar hidden. Test compact mode auto-activation at width=119. Test `Ctrl+D` toggles details in compact vs sidebar in normal. Test mouse click consumption in sidebar area. Test event forwarding.

---

## Task 8: Section Headers & Theming

**Files:**
- Modify: `internal/components/sidebar/sidebar.go`
- Modify: `internal/theme/theme.go` (only if needed)

- [ ] **Step 1: Style section headers with separator line**

Following crush's `common.Section()` pattern:
```go
func renderSectionHeader(title string, width int, t *theme.Theme) string {
    label := lipgloss.NewStyle().Bold(true).Foreground(t.TextMuted).Render(strings.ToUpper(title))
    lineWidth := width - lipgloss.Width(label)
    if lineWidth < 2 { lineWidth = 2 }
    line := lipgloss.NewStyle().Foreground(t.Border).Render(" " + strings.Repeat("─", lineWidth-1))
    return label + line
}
```

This renders as: `SESSION ──────────────`

- [ ] **Step 2: Verify theme color coverage**

Existing theme colors that map to sidebar needs:
- `t.TextPrimary` → session name, item names
- `t.TextMuted` → section headers, descriptions, separator lines, gray indicators
- `t.Accent` → model name, active/connected indicator (green)
- `t.Warning` → starting/busy indicator (yellow)
- `t.TokenHighUsage` → error indicator (red)
- `t.Border` → separator line, vertical border
- `t.Panel` → sidebar background (if needed)
- `t.CompressionSuccess` → compression indicator

No new theme colors required. All sidebar rendering uses existing theme fields.

- [ ] **Step 3: Write tests**

Test section header renders at various widths. Test separator line fills remaining space.

---

## Priority Order

| Priority | Task | Rationale |
|----------|------|-----------|
| P0 | Task 1: Sidebar scaffold + height distribution | Foundation for all sections |
| P0 | Task 7: Layout restructure + compact mode | Core integration — sidebar can't display without this |
| P0 | Task 2: Session section | Simplest section, validates architecture end-to-end |
| P0 | Task 3: Context section | Most valuable — token bar is the primary use case |
| P0 | Task 8: Headers & theming | Needed before anything looks right |
| P1 | Task 4: Channels section | Key smolbot differentiator — WhatsApp/Signal |
| P2 | Task 5: MCP servers section | Informational, depends on gateway method |
| P3 | Task 6: Cron section | Depends on cron backend (separate feature) |

---

## Dependencies on Other Plans

| This Plan | Depends On |
|-----------|------------|
| Task 5 (MCPs section data) | `2026-03-27-compaction-ux-and-f1-menu.md` Task 7 (`mcps.list` gateway method) |
| Task 6 (Cron section data) | New cron/scheduler backend feature (not yet planned) |

---

## Implementation Notes

### Key Differences from crush

| Aspect | crush | smolbot |
|--------|-------|---------|
| Sections | Session, Files, LSPs, MCPs | Session, Context, Channels, MCPs, Scheduled |
| Unique sections | Modified Files, LSP diagnostics | Channels (WhatsApp/Signal), Cron/Scheduled tasks |
| Context bar | Token count inline in model info | Dedicated progress bar section with color coding |
| Compact mode | Side-by-side overlay via `Ctrl+D` | Same pattern |
| Rendering | Ultraviolet `uv.Screen` + `uv.Rectangle` | lipgloss `JoinHorizontal` + `JoinVertical` |

### Data Flow (No Backend Changes Needed)

All sidebar data comes from existing sources:
1. **Session/CWD/Model**: Already in `m.app.*`
2. **Token Usage**: Already from `StatusLoadedMsg` and `chat.usage` events
3. **Compression**: Already from `context.compressed` events
4. **Channels**: Already in `StatusPayload.Channels`
5. **MCPs**: Parse from config initially, upgrade to gateway method later
6. **Cron**: Empty until cron backend exists

### Performance

Following crush's approach — keep section renders cheap:
- Precompute widths on resize, not per-frame
- Use `strings.Builder` for section composition
- Truncate long strings with `ansi.Truncate()` (ANSI-aware) rather than naive slicing
- No `lipgloss.Width()` calls in tight loops

---

## Testing Strategy

- Unit tests per section component (render output at various widths and item counts)
- Dynamic height distribution tests (even split, capping, redistribution)
- Layout integration tests (sidebar + chat widths sum correctly)
- Compact mode tests (overlay renders, toggle behavior)
- Snapshot tests for sidebar View at 120-col and 80-col widths
- Event forwarding tests (status → sidebar section updates)
- State persistence tests (toggle saves/restores)

## Risks

1. **lipgloss `JoinHorizontal`** with ANSI-colored strings may have edge cases at join boundaries. crush uses ultraviolet's screen buffer which avoids this. Test thoroughly with colored content and verify no visual artifacts at the separator.
2. **bubbletea viewport** width must be recalculated correctly when sidebar shows/hides. Verify scrolling and word wrap still work after toggle.
3. **Compact overlay height** eats into chat viewport height. Need to recalculate transcript height when `detailsOpen` toggles.
4. **Cron backend** is a significant separate feature. The sidebar section must handle empty state gracefully and not look broken with "none" displayed.
5. **Footer width** — crush extends the footer full-width below both chat and sidebar. smolbot should do the same to avoid a visual gap.
