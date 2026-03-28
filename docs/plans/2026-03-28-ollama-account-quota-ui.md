# Ollama Account Quota UI Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Redesign the TUI `USAGE` section so it presents `Observed` and `Quota` together without conflating smolbot-recorded usage with true Ollama account-backed limits.

**Architecture:** Consume the new dual-source usage payload from the gateway, keep `CONTEXT` independent, and render one `USAGE` section containing two clearly separated sub-blocks. Degraded quota states remain visible and explicit.

**Tech Stack:** Go, Bubble Tea, Lip Gloss, existing sidebar/footer/status models, existing gateway client

---

### Task 1: Add Dual-Source Usage Client Types

**Files:**
- Modify: `internal/client/types.go`
- Modify: `internal/client/protocol.go`
- Modify: `internal/client/client_test.go`

**Step 1: Write the failing test**

Add tests for decoding:
- observed usage block
- quota block
- stale/expired quota states

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/client -run 'Quota|Observed|StatusResponse' -v
```

Expected:
- FAIL because the dual-source types do not exist yet

**Step 3: Write minimal implementation**

Add client-facing types for:
- `ObservedUsageSummary`
- `QuotaUsageSummary`
- nested `UsageStatus` payload

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/client -run 'Quota|Observed|StatusResponse' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add internal/client/types.go internal/client/protocol.go internal/client/client_test.go
git commit -m "feat(client): decode observed and quota usage payloads"
```

### Task 2: Redesign Sidebar Usage Section

**Files:**
- Modify: `internal/components/sidebar/usage_section.go`
- Modify: `internal/components/sidebar/sidebar.go`
- Modify: `internal/components/sidebar/sidebar_test.go`

**Step 1: Write the failing test**

Add tests for:
- rendering `Observed` subsection
- rendering `Quota` subsection
- showing explicit `Quota unavailable` / `Quota expired` states
- preserving `CONTEXT` separately from `USAGE`

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/components/sidebar -run 'Usage|Quota|Observed' -v
```

Expected:
- FAIL because the section only supports one persisted summary

**Step 3: Write minimal implementation**

Render under `USAGE`:
- `Observed`
  - session tokens
  - request count
- `Quota`
  - plan
  - session %
  - weekly %
  - reset countdown/state

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/components/sidebar -run 'Usage|Quota|Observed' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add internal/components/sidebar/usage_section.go internal/components/sidebar/sidebar.go internal/components/sidebar/sidebar_test.go
git commit -m "feat(sidebar): render observed and quota usage blocks"
```

### Task 3: Wire TUI Status Hydration And Degraded States

**Files:**
- Modify: `internal/tui/tui.go`
- Modify: `internal/tui/tui_test.go`

**Step 1: Write the failing test**

Add tests for:
- status hydration updates both observed and quota blocks
- `chat.usage` updates context/live usage only
- stale/expired quota states show correctly after reconnect
- observed usage remains visible when quota fetch fails

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/tui -run 'Quota|Observed|Usage|Reconnect' -v
```

Expected:
- FAIL because the TUI does not manage dual-source usage state

**Step 3: Write minimal implementation**

Update status handling to:
- keep footer/context behavior unchanged
- hydrate sidebar with observed + quota
- dedupe stale/expired quota warnings
- avoid wiping observed usage when quota is unavailable

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/tui -run 'Quota|Observed|Usage|Reconnect' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add internal/tui/tui.go internal/tui/tui_test.go
git commit -m "feat(tui): hydrate usage section with quota states"
```

### Task 4: Add User-Facing Status Language And Follow-Up UX

**Files:**
- Modify: `internal/tui/tui.go`
- Modify: `internal/tui/tui_test.go`

**Step 1: Write the failing test**

Add tests for:
- quota auth expired message
- quota stale message after refresh failure
- no message spam across repeated status refreshes

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./internal/tui -run 'QuotaExpired|QuotaStale|UsageWarning' -v
```

Expected:
- FAIL because the new messages do not exist yet

**Step 3: Write minimal implementation**

Add concise messages for:
- `Quota connected`
- `Quota stale`
- `Quota auth expired`
- `Quota unavailable`

Only notify when state changes.

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./internal/tui -run 'QuotaExpired|QuotaStale|UsageWarning' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add internal/tui/tui.go internal/tui/tui_test.go
git commit -m "feat(tui): add quota freshness and auth state messaging"
```

