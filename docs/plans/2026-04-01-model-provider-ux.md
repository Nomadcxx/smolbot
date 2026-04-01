# Implementation Plan: Model & Provider UX Parity with opencode

> **Reference**: `docs/MODEL_PROVIDER.md` (gap analysis)
> **Goal**: Achieve UI and functional parity with opencode's model/provider selection UX
> **Approach**: 8 phases, each self-contained and independently testable

---

## Architecture Context

Before diving into phases, here's what we're working with:

### State persistence
- **Config**: `~/.smolbot/config.json` — providers, API keys, agent defaults (via `AtomicWriteConfig`)
- **TUI State**: `~/.config/smolbot-tui/state.json` — theme, lastSession, lastModel, sidebarVisible (via `app.SaveState`)
- Favourites and recents will extend TUI state (not config — these are UI preferences, not runtime config)

### Key types
- `CatalogueEntry{ID, Name, Capability}` — static model registry in `pkg/provider/catalogue.go`
- `ModelInfo{ID, Name, Provider, Description, Source, Capability, Selectable}` — UI-facing model data
- `ModelsModel` — model dialog component (filter, cursor, rows, pending selection)
- `ProvidersModel` — provider dialog component (browse/configure/confirm modes)
- `app.State{Theme, LastSession, LastModel, SidebarVisible}` — persisted TUI state

### Message flow
```
User action → Dialog sends tea.Msg → tui.go Update() handles it
  → calls gateway client → receives result Msg → updates state → persists
```

---

## Phase 1 — Visual Polish & Quick Wins

**Gaps closed**: G5, G7, G8
**Files**: `dialog/models.go`, `dialog/common.go`, `dialog/providers.go`
**Risk**: Low — cosmetic changes with clear test assertions

### 1.1 Active model indicator: `●` prefix instead of trailing " current"

**File**: `internal/components/dialog/models.go` — `func (r modelRenderRow) render()`

Current:
```go
if r.current {
    label += lipgloss.NewStyle().Foreground(t.Success).Bold(true).Render(" current")
}
```

Change to:
```go
prefix := "  "
if r.current {
    prefix = lipgloss.NewStyle().Foreground(t.Success).Bold(true).Render("● ")
}
// Use prefix instead of "> " / "  " when building the line
```

Keep `"> "` for focused (cursor). When both focused AND current: `">●"` or `"● "` with focused styling on the whole row. Decision: focused takes precedence for prefix (`"> "`), but `●` appears after the prefix as part of the label.

Revised approach:
```
Focused + Current:  "> ● GPT-4o  gpt-4o"
Current only:       "  ● GPT-4o  gpt-4o"  
Focused only:       ">   GPT-4o  gpt-4o"
Neither:            "    GPT-4o  gpt-4o"
```

### 1.2 Provider header display names

**File**: `internal/components/dialog/models.go` — `func (r modelRenderRow) render()`

Current:
```go
label := "Provider: " + r.group
```

Change: Use `providerDisplayName(r.group)` to map provider IDs to display names. The `providerMeta` map in `providers.go` already has these mappings. Either:
- Export `providerMeta` (rename to `ProviderMeta`) and use it in models.go, or
- Add a `ProviderDisplayName(id string) string` function in `dialog/common.go` backed by the same data

Also change format from `"Provider: OpenAI (current)"` to just `"OpenAI"` with `"(current)"` as a styled suffix if applicable. This matches opencode's cleaner headers.

### 1.3 Increase visible items from 7 to 10

**File**: `internal/components/dialog/common.go`

```go
const maxVisibleItems = 10  // was 7
```

### 1.4 Update help text

**File**: `internal/components/dialog/models.go` — `View()` help line

Current: `"Type filter • Space mark • Enter save • Esc close"`

Will update incrementally as features land. Phase 1 change: just update to match new keybinds.

### 1.5 Tests

- Update `TestModelsModelShowsCurrentModel` — assert `●` appears instead of `" current"`
- Update `TestModelsModelGroupsByProvider` — assert display name headers ("OpenAI" not "Provider: openai")
- Update `TestModelsModelShowsOverflowCues` — adjust for 10-item window
- Ensure all existing tests pass with cosmetic changes

---

## Phase 2 — State Layer: Favourites & Recents Persistence

**Gaps closed**: F2, R1, R5
**Files**: `internal/app/state.go`, `internal/app/state_test.go` (new)
**Risk**: Low — extends existing persistence, no UI changes yet

### 2.1 Extend State struct

**File**: `internal/app/state.go`

```go
type State struct {
    Theme          string   `json:"theme,omitempty"`
    LastSession    string   `json:"lastSession,omitempty"`
    LastModel      string   `json:"lastModel,omitempty"`
    SidebarVisible *bool    `json:"sidebarVisible,omitempty"`
    Favorites      []string `json:"favorites,omitempty"`      // NEW: model IDs
    Recents        []string `json:"recents,omitempty"`        // NEW: model IDs, newest first
}
```

### 2.2 Mutation methods on State

```go
const MaxRecents = 10

func (s *State) ToggleFavorite(modelID string) bool  // returns true if now favourite
func (s *State) IsFavorite(modelID string) bool
func (s *State) AddRecent(modelID string)             // prepend, dedup, cap at MaxRecents
func (s *State) RemoveRecent(modelID string)
func (s *State) RecentModelIDs() []string             // defensive copy
func (s *State) FavoriteModelIDs() []string           // defensive copy
```

`AddRecent` behaviour:
1. Remove `modelID` if already present (dedup)
2. Prepend to slice
3. If `len > MaxRecents`, truncate tail

`ToggleFavorite` behaviour:
1. If present, remove and return false
2. If absent, append and return true

### 2.3 Integration point: record recents on model switch

**File**: `internal/tui/tui.go` — `ModelSetMsg` handler

After `m.app.Model = msg.ID`, add:
```go
m.app.State.AddRecent(msg.ID)
```

The existing `persistStateCmd()` already saves state.json, so this flows through automatically.

### 2.4 Tests

New file `internal/app/state_test.go`:
- `TestToggleFavoriteAddsAndRemoves`
- `TestAddRecentPrependsAndDeduplicates`
- `TestAddRecentCapsAtMaxRecents`
- `TestRemoveRecentRemovesCorrectEntry`
- `TestFavoritesAndRecentsPersistedToJSON` (round-trip marshal/unmarshal)

---

## Phase 3 — Model Dialog: Favourites & Recents Sections

**Gaps closed**: G1, G2, G4, F1, F3, F4, R2, R4, S1, S2
**Files**: `dialog/models.go`, `dialog/models_test.go`
**Risk**: Medium — restructures row building logic, but isolated to dialog component

### 3.1 Pass state into ModelsModel

Extend constructor:
```go
func NewModels(cfg map[string]config.ProviderConfig, models []client.ModelInfo, 
               current string, favorites []string, recents []string) ModelsModel
```

Store in struct:
```go
type ModelsModel struct {
    // ... existing fields ...
    favorites []string
    recents   []string
}
```

### 3.2 Restructure `buildModelRows`

New signature:
```go
func buildModelRows(models []client.ModelInfo, currentProvider string, 
                    favorites []string, recents []string) []modelRenderRow
```

New logic:
1. Build lookup: `modelByID map[string]client.ModelInfo`
2. **Favourites section** (if any favourites match available models):
   - Header row: `kind="header", group="Favorites"`
   - Model rows for each favourite ID found in models
3. **Recents section** (if any recents match, excluding those already in favourites):
   - Header row: `kind="header", group="Recent"`
   - Model rows for each recent ID
4. **Provider groups** (existing logic, but skip models already shown in Favourites/Recents):
   - Track `shown map[string]bool` to deduplicate

### 3.3 New row kind for section headers

Add rendering for special section names:
```go
case "header":
    switch r.group {
    case "Favorites":
        label = "★ Favorites"
    case "Recent":
        label = "⏱ Recent"
    default:
        label = ProviderDisplayName(r.group) // from Phase 1
    }
```

### 3.4 Ctrl+F: Toggle favourite

In `ModelsModel.Update()`:
```go
case "ctrl+f":
    if focused := m.focusedModel(); focused.ID != "" {
        return m, func() tea.Msg {
            return ModelFavoriteToggledMsg{ID: focused.ID}
        }
    }
```

New message type:
```go
type ModelFavoriteToggledMsg struct {
    ID string
}
```

Handler in `tui.go`:
```go
case dialog.ModelFavoriteToggledMsg:
    isFav := m.app.State.ToggleFavorite(msg.ID)
    // Rebuild dialog rows with updated favourites
    m.dialog = m.dialog.WithFavorites(m.app.State.FavoriteModelIDs())
    return m, persistStateCmd(m.app)
```

This means `ModelsModel` needs a `WithFavorites([]string) ModelsModel` method that updates internal state and re-runs `applyFilter()`.

### 3.5 Ctrl+X: Remove from recents

In `ModelsModel.Update()`:
```go
case "ctrl+x":
    if focused := m.focusedModel(); focused.ID != "" {
        return m, func() tea.Msg {
            return ModelRemovedFromRecentsMsg{ID: focused.ID}
        }
    }
```

Handler in `tui.go`:
```go
case dialog.ModelRemovedFromRecentsMsg:
    m.app.State.RemoveRecent(msg.ID)
    m.dialog = m.dialog.WithRecents(m.app.State.RecentModelIDs())
    return m, persistStateCmd(m.app)
```

### 3.6 Visual indicators in row rendering

For favourite models (in any section):
```go
if isFavorite {
    // Add ★ prefix or "(Favorite)" in description
    descParts = append(descParts, lipgloss.NewStyle().Foreground(t.Warning).Render("★"))
}
```

### 3.7 Update help text

```
"Type filter • Ctrl+F fav • Ctrl+X remove • Space mark • Enter save • Esc close"
```

### 3.8 Tests

- `TestModelsModelShowsFavoritesSection` — favourites appear at top
- `TestModelsModelShowsRecentsSection` — recents appear after favourites
- `TestModelsModelDeduplicatesFavoritesFromProviderGroups` — model in favourites not repeated
- `TestModelsModelCtrlFSendsFavoriteToggleMsg`
- `TestModelsModelCtrlXSendsRemoveFromRecentsMsg`
- `TestModelsModelFilterIncludesFavoritesAndRecents` — filter applies across all sections

---

## Phase 4 — F2 Quick-Cycle Recent (Outside Dialog)

**Gaps closed**: G11, R3
**Files**: `internal/tui/tui.go`
**Risk**: Low — single keybind in main TUI loop

### 4.1 F2 behaviour when dialog is NOT open

Currently F2 may open model dialog. Change:
- If recents list is empty or has only current model → open model dialog (existing behaviour)
- If recents has other models → cycle to next recent

```go
case "f2":
    if m.dialog != nil {
        break // let dialog handle it
    }
    recents := m.app.State.RecentModelIDs()
    next := nextRecentModel(recents, m.app.Model)
    if next == "" {
        // No recents to cycle through, open dialog instead
        return m, loadModelsCmd(m.client)
    }
    return m, setModelCmd(m.client, next)
```

Helper:
```go
func nextRecentModel(recents []string, current string) string {
    if len(recents) < 2 { return "" }
    for i, id := range recents {
        if id == current {
            return recents[(i+1) % len(recents)]
        }
    }
    return recents[0] // current not in recents, pick first
}
```

### 4.2 Tests

- `TestF2CyclesToNextRecentModel`
- `TestF2OpensDialogWhenNoRecents`
- `TestF2WrapsAroundRecentsList`

---

## Phase 5 — Catalogue Enrichment & Sorting

**Gaps closed**: M1, M4, M6, M8, S3, S4
**Files**: `pkg/provider/catalogue.go`, `pkg/provider/discovery.go`, `internal/client/protocol.go`
**Risk**: Medium — extends data model through the full pipeline

### 5.1 Extend CatalogueEntry

```go
type CatalogueEntry struct {
    ID            string
    Name          string
    Capability    string
    ReleaseDate   string  // "2024-10" format (year-month for display)
    IsFree        bool
    ContextWindow int     // token count, 0 if unknown
}
```

### 5.2 Populate metadata for all 27 models

Update each entry in the catalogue map. Example:
```go
"openai": {
    {ID: "gpt-4o", Name: "GPT-4o", Capability: "chat", 
     ReleaseDate: "2024-05", IsFree: false, ContextWindow: 128000},
    {ID: "gpt-4o-mini", Name: "GPT-4o mini", Capability: "chat",
     ReleaseDate: "2024-07", IsFree: false, ContextWindow: 128000},
    // ...
}
```

### 5.3 Extend ModelInfo to carry metadata through

**File**: `internal/client/protocol.go` (and `pkg/provider/discovery.go` mirror)

```go
type ModelInfo struct {
    ID            string `json:"id"`
    Name          string `json:"name"`
    Provider      string `json:"provider"`
    Description   string `json:"description,omitempty"`
    Source        string `json:"source,omitempty"`
    Capability    string `json:"capability,omitempty"`
    Selectable    bool   `json:"selectable"`
    ReleaseDate   string `json:"releaseDate,omitempty"`   // NEW
    IsFree        bool   `json:"isFree,omitempty"`        // NEW
    ContextWindow int    `json:"contextWindow,omitempty"` // NEW
}
```

### 5.4 Flow metadata through discovery

**File**: `pkg/provider/discovery.go` — catalogue model builder

```go
info := ModelInfo{
    ID:            entry.ID,
    Name:          entry.Name,
    Provider:      providerID,
    Source:        "catalogue",
    Capability:    entry.Capability,
    Selectable:    true,
    ReleaseDate:   entry.ReleaseDate,    // pass through
    IsFree:        entry.IsFree,         // pass through
    ContextWindow: entry.ContextWindow,  // pass through
}
```

### 5.5 Sort within provider groups

In `buildModelRows` (or a new `sortModelsInGroup` helper):
```go
sort.SliceStable(grouped[group], func(i, j int) bool {
    a, b := grouped[group][i], grouped[group][j]
    // Free first
    if a.IsFree != b.IsFree { return a.IsFree }
    // Newer first (reverse lexicographic on "YYYY-MM")
    if a.ReleaseDate != b.ReleaseDate { return a.ReleaseDate > b.ReleaseDate }
    // Alphabetical fallback
    return a.Name < b.Name
})
```

### 5.6 Tests

- `TestCatalogueEntriesHaveMetadata` — spot-check a few entries have non-zero values
- `TestDiscoveryFlowsMetadataToModelInfo` — verify ReleaseDate, IsFree, ContextWindow populated
- `TestModelsSortedByFreeAndReleaseDateWithinGroup`
- Update `TestModelsListReturnsRichDiscoveryPayload` in gateway tests

---

## Phase 6 — Model Row Display Enrichment

**Gaps closed**: M2, M5, G9, G10
**Files**: `dialog/models.go`
**Risk**: Low — rendering changes only

### 6.1 Show release date

In `func (r modelRenderRow) render()`, after the name/ID:
```go
if r.model.ReleaseDate != "" {
    descParts = append(descParts, 
        lipgloss.NewStyle().Foreground(t.TextMuted).Render(r.model.ReleaseDate))
}
```

### 6.2 Show "Free" badge

```go
if r.model.IsFree {
    descParts = append(descParts, 
        lipgloss.NewStyle().Foreground(t.Success).Bold(true).Render("Free"))
}
```

### 6.3 Tests

- `TestModelsModelRendersReleaseDateInRow`
- `TestModelsModelRendersFreeBadge`

---

## Phase 7 — Fuzzy Search

**Gaps closed**: G6
**Files**: `dialog/common.go`, `dialog/common_test.go` (new or existing)
**Risk**: Low-Medium — replaces filter logic, but behaviour is a superset

### 7.1 Replace matchesQuery with fuzzy matching

Implement character-sequence fuzzy matching. For each query token, check if the characters appear **in order** (not necessarily contiguous) in the haystack:

```go
func fuzzyMatch(needle, haystack string) bool {
    n, h := strings.ToLower(needle), strings.ToLower(haystack)
    ni := 0
    for hi := 0; hi < len(h) && ni < len(n); hi++ {
        if h[hi] == n[ni] {
            ni++
        }
    }
    return ni == len(n)
}
```

Multi-token: all tokens must fuzzy-match (AND logic, same as current).

Optionally add scoring (bonus for contiguous matches, word-start matches) to rank results. If scoring is added, `applyFilter` should sort filtered results by score descending.

### 7.2 Tests

- `TestFuzzyMatchFindsSubsequences` — "gp4o" matches "gpt-4o"
- `TestFuzzyMatchMultiTokenAND` — "cl son" matches "Claude Sonnet"
- `TestFuzzyMatchCaseInsensitive`
- `TestFuzzyMatchEmptyQueryMatchesAll`
- Verify existing filter tests still pass (fuzzy is a superset of prefix)

---

## Phase 8 — Inline Provider Addition (Ctrl+A from Model Dialog)

**Gaps closed**: P1, P3, P4
**Files**: `dialog/models.go`, `dialog/providers.go`, `internal/tui/tui.go`
**Risk**: Medium-High — dialog-within-dialog state management

### 8.1 Approach: Sub-dialog pattern

When Ctrl+A is pressed in the model dialog, the model dialog emits a message requesting provider addition. The TUI layer swaps the dialog to providers, and on successful configuration, swaps back to models filtered to the new provider.

This avoids nesting dialogs (which bubbletea doesn't naturally support) and reuses the existing ProvidersModel.

### 8.2 New message types

```go
// Emitted by model dialog when user presses Ctrl+A
type RequestProviderAddMsg struct{}

// Emitted by TUI after provider is configured, to reopen model dialog
type ProviderAddedReturnToModelsMsg struct {
    ProviderID string  // newly configured provider to filter to
}
```

### 8.3 Model dialog: Ctrl+A keybind

```go
case "ctrl+a":
    return m, func() tea.Msg { return RequestProviderAddMsg{} }
```

### 8.4 TUI wiring

```go
case dialog.RequestProviderAddMsg:
    // Close model dialog, open provider dialog
    m.dialog = nil
    m.returnToModelsAfterProvider = true
    return m, loadProvidersCmd(m.client)

case dialog.ConfigureProviderMsg:
    // ... existing handling ...
    if m.returnToModelsAfterProvider {
        m.pendingProviderFilter = msg.ProviderID
    }

// After provider config succeeds (in the provider result handler):
if m.returnToModelsAfterProvider {
    m.returnToModelsAfterProvider = false
    // Reopen model dialog, optionally pre-filter to new provider
    return m, loadModelsCmd(m.client)
}
```

### 8.5 Provider categories: Popular vs Other

**File**: `dialog/providers.go` — `buildProviderRows()`

Add `isPopular` check:
```go
var popularProviders = map[string]bool{
    "anthropic": true, "openai": true, "gemini": true, 
    "groq": true, "deepseek": true,
}
```

In browse mode, show two sections for unconfigured providers:
- "Popular" — providers in the popular set
- "Other" — remaining providers

### 8.6 Update help text

Models dialog:
```
"Type filter • Ctrl+F fav • Ctrl+A add provider • Ctrl+X remove • Enter save • Esc close"
```

### 8.7 Tests

- `TestModelsModelCtrlASendsRequestProviderAddMsg`
- `TestTUICtrlATransitionsToProviderDialog`
- `TestTUIReturnsToModelsAfterProviderConfig`
- `TestProviderBrowseShowsPopularAndOtherSections`

---

## Out of Scope (Future Work)

These are noted in the gap analysis but deliberately deferred:

- **P2 / OAuth flows** — Requires provider-specific OAuth integration (browser redirect, device code). Large scope, low urgency given API keys work well.
- **C1-C5 / Live catalogue (models.dev)** — Architectural change requiring HTTP fetch, caching, fallback. Consider as a separate plan once static catalogue is enriched.
- **M3 / Pricing display** — Data not reliably available; defer until live catalogue provides it.
- **M7 / Extended capabilities** — vision, tools, function-calling flags. Useful but not user-facing until tool routing depends on it.

---

## Phase Dependency Graph

```
Phase 1 (Visual Polish)
    ↓
Phase 2 (State Layer) ──→ Phase 3 (Fav/Recent Sections) ──→ Phase 4 (F2 Cycle)
    ↓                                                              
Phase 5 (Catalogue Enrich) ──→ Phase 6 (Row Display)
    
Phase 7 (Fuzzy Search) — independent, can be done any time after Phase 1

Phase 8 (Inline Provider Add) — depends on Phase 1 only
```

Phases 1→2→3→4 form the critical path for core UX parity.
Phases 5→6 form the data enrichment path.
Phase 7 and Phase 8 are independent branches.

---

## File Change Summary

| File | Phases | Changes |
|------|--------|---------|
| `internal/components/dialog/common.go` | 1, 7 | maxVisibleItems→10, ProviderDisplayName(), fuzzy search |
| `internal/components/dialog/models.go` | 1, 3, 6, 8 | ● indicator, section headers, Ctrl+F/X/A, metadata display |
| `internal/components/dialog/models_test.go` | 1, 3, 6, 8 | Updated + new tests for all dialog changes |
| `internal/components/dialog/providers.go` | 1, 8 | Display name headers, Popular/Other categories |
| `internal/components/dialog/providers_test.go` | 1, 8 | Updated assertions |
| `internal/app/state.go` | 2 | Favorites, Recents fields + mutation methods |
| `internal/app/state_test.go` | 2 | New: state mutation tests |
| `internal/tui/tui.go` | 2, 3, 4, 8 | Record recents, handle fav/recent msgs, F2 cycle, Ctrl+A flow |
| `internal/tui/tui_test.go` | 3, 4, 8 | New tests for TUI integration |
| `pkg/provider/catalogue.go` | 5 | Extend CatalogueEntry, populate metadata |
| `pkg/provider/discovery.go` | 5 | Flow metadata to ModelInfo |
| `pkg/provider/discovery_test.go` | 5 | Metadata flow assertions |
| `internal/client/protocol.go` | 5 | Extend ModelInfo struct |
| `pkg/gateway/server_test.go` | 5 | Updated payload assertions |
