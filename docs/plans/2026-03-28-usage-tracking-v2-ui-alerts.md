# Usage Tracking V2 Gateway, Sidebar, And Notifications Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Expose persisted usage summaries through the gateway/client protocol, render them in a dedicated sidebar section, and wire quota warnings and notification seams without disturbing the existing context-usage UX.

**Architecture:** The UI stream treats `pkg/usage` as the source for persisted provider usage and keeps `chat.usage` as a live context signal. Gateway status payloads are extended with a persisted usage summary, the TUI loads that summary on connect, and notifications are driven from budget state rather than raw token events.

**Tech Stack:** Go, existing gateway WebSocket protocol, Bubble Tea, Lip Gloss, sidebar section architecture, usage summary read model from `pkg/usage`

---

### Task 1: Extend Client Models For Persisted Usage Summary

**Files:**
- Modify: `internal/client/types.go`
- Modify: `internal/client/protocol.go`
- Modify: `internal/client/client_test.go`

**Step 1: Write the failing test**

Add tests for:
- decoding a `status` payload that includes persisted usage summary fields
- preserving current `UsageInfo` behavior for context-window display

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/client -run 'Status|UsageSummary' -v
```

Expected:
- FAIL because the new payload shape is unknown

**Step 3: Write minimal implementation**

Add models such as:
```go
type UsageSummary struct {
    ProviderID    string `json:"providerId"`
    ModelName     string `json:"modelName"`
    SessionTokens int    `json:"sessionTokens"`
    TodayTokens   int    `json:"todayTokens"`
    WeeklyTokens  int    `json:"weeklyTokens"`
    EstimatedCost string `json:"estimatedCost,omitempty"`
    BudgetStatus  string `json:"budgetStatus,omitempty"`
    WarningLevel  string `json:"warningLevel,omitempty"`
}
```

Add to `StatusPayload`:
- `PersistedUsage *UsageSummary`

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/client -run 'Status|UsageSummary' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add internal/client/types.go internal/client/protocol.go internal/client/client_test.go
git commit -m "feat(client): add persisted usage summary models"
```

### Task 2: Expose Persisted Usage In Gateway Status

**Files:**
- Modify: `pkg/gateway/server.go`
- Modify: `pkg/gateway/server_test.go`

**Step 1: Write the failing test**

Add tests for:
- `status` includes persisted usage summary from `pkg/usage`
- no summary is returned cleanly when usage store is unavailable

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./pkg/gateway -run 'Status|UsageSummary' -v
```

Expected:
- FAIL because server does not know about the usage store

**Step 3: Write minimal implementation**

Add:
- usage summary dependency to `ServerDeps`
- status payload enrichment from backend summary queries
- no change to `chat.usage` event semantics

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./pkg/gateway -run 'Status|UsageSummary' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add pkg/gateway/server.go pkg/gateway/server_test.go
git commit -m "feat(gateway): expose persisted usage summary"
```

### Task 3: Add Sidebar Usage Section

**Files:**
- Create: `internal/components/sidebar/usage_section.go`
- Modify: `internal/components/sidebar/sidebar.go`
- Modify: `internal/components/sidebar/sidebar_test.go`

**Step 1: Write the failing test**

Add tests for:
- usage section renders provider/session/today/week values
- empty state renders cleanly
- layout still respects height budgeting with the extra section

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/components/sidebar -run 'Usage|Sidebar' -v
```

Expected:
- FAIL because the section does not exist

**Step 3: Write minimal implementation**

Add a `USAGE` section that renders:
- provider/model label
- session tokens
- today tokens
- weekly tokens
- optional estimated cost
- budget or warning badge

Wire it into:
- full sidebar view
- compact sidebar view
- dynamic height allocation

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/components/sidebar -run 'Usage|Sidebar' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add internal/components/sidebar/usage_section.go internal/components/sidebar/sidebar.go internal/components/sidebar/sidebar_test.go
git commit -m "feat(sidebar): add persisted usage section"
```

### Task 4: Wire TUI State To Persisted Usage Summary

**Files:**
- Modify: `internal/tui/tui.go`
- Modify: `internal/tui/tui_test.go`

**Step 1: Write the failing test**

Add tests for:
- initial status load hydrates sidebar usage section
- live `chat.usage` still updates context-only UI
- persisted usage summary and context usage do not overwrite each other

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/tui -run 'Sidebar|Usage|Status' -v
```

Expected:
- FAIL because TUI does not map persisted usage summary yet

**Step 3: Write minimal implementation**

Update:
- status load handler to pass persisted summary into the sidebar
- reconnect/status refresh path to preserve both data models

Keep:
- `CONTEXT` sourced from live usage payloads
- `USAGE` sourced from persisted summary payloads

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/tui -run 'Sidebar|Usage|Status' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add internal/tui/tui.go internal/tui/tui_test.go
git commit -m "feat(tui): hydrate sidebar usage from persisted summaries"
```

### Task 5: Add Quota Warning States

**Files:**
- Modify: `internal/components/sidebar/usage_section.go`
- Modify: `internal/tui/tui.go`
- Modify: `internal/tui/tui_test.go`

**Step 1: Write the failing test**

Add tests for:
- warning badge appears at threshold states
- warning message does not spam repeatedly on unchanged state

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/tui ./internal/components/sidebar -run 'Warning|Quota|Usage' -v
```

Expected:
- FAIL because warning-state rendering is absent

**Step 3: Write minimal implementation**

Add:
- warning level mapping in sidebar usage section
- one-shot or deduped system message behavior in the TUI if desired

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/tui ./internal/components/sidebar -run 'Warning|Quota|Usage' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add internal/components/sidebar/usage_section.go internal/tui/tui.go internal/tui/tui_test.go
git commit -m "feat(tui): add usage warning states"
```

### Task 6: Add Notification Wiring Seams

**Files:**
- Modify: `pkg/gateway/server.go`
- Modify: `pkg/gateway/server_test.go`
- Modify: `internal/client/protocol.go`

**Step 1: Write the failing test**

Add tests for:
- gateway can surface alert history or latest warning state cleanly
- notification payload shape remains optional and backward compatible

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./pkg/gateway ./internal/client -run 'Notification|Alert|Protocol' -v
```

Expected:
- FAIL because notification wiring is absent

**Step 3: Write minimal implementation**

Add notification seams only:
- latest alert state in status or separate event payload if needed
- no provider-specific webhook delivery yet unless backend stream already exposes it

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./pkg/gateway ./internal/client -run 'Notification|Alert|Protocol' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add pkg/gateway/server.go pkg/gateway/server_test.go internal/client/protocol.go
git commit -m "feat(gateway): add usage notification seams"
```

### Task 7: Hardening And Full Verification

**Files:**
- Read: backend and UI files touched in this stream

**Step 1: Run focused integration checks**

Run:
```bash
go test ./pkg/gateway ./internal/client ./internal/components/sidebar ./internal/tui -v
```

Expected:
- PASS

**Step 2: Run full suite after backend merge**

Run:
```bash
go test ./...
```

Expected:
- PASS with no regressions in current context usage behavior

**Step 3: Commit final hardening changes**

```bash
git add pkg/gateway/server.go pkg/gateway/server_test.go internal/client internal/components/sidebar internal/tui
git commit -m "test: harden usage tracking ui and gateway integration"
```
