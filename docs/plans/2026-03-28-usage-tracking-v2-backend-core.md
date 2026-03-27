# Usage Tracking V2 Backend Core Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement the Ollama-first backend core for usage persistence, aggregation, budgets, and alert state so gateway and TUI consumers can rely on stable persisted summaries.

**Architecture:** Add a dedicated `pkg/usage` subsystem, write usage records from the agent loop, derive summary and budget state from `usage.db`, and preserve the current live `chat.usage` event path as a separate UI signal.

**Tech Stack:** Go, SQLite, existing `mattn/go-sqlite3` driver, existing tokenizer, existing session-store schema/testing patterns

---

### Task 1: Add Usage DB Path Support

**Files:**
- Modify: `pkg/config/paths.go`
- Test: `pkg/config/config_test.go`

**Step 1: Write the failing test**

Add assertions for:
- `UsageDB()` returns `/home/test/.smolbot/usage.db`

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./pkg/config -run TestDefaultPaths -v
```

Expected:
- FAIL because `UsageDB()` does not exist yet

**Step 3: Write minimal implementation**

Add:
```go
func (p *Paths) UsageDB() string { return filepath.Join(p.root, "usage.db") }
```

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./pkg/config -run TestDefaultPaths -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add pkg/config/paths.go pkg/config/config_test.go
git commit -m "feat(config): add usage db path"
```

### Task 2: Create Usage Store Foundation

**Files:**
- Create: `pkg/usage/models.go`
- Create: `pkg/usage/store.go`
- Create: `pkg/usage/store_test.go`

**Step 1: Write the failing test**

Add tests for:
- opening a fresh store initializes all phase-1 tables
- schema works with file-backed DB and `:memory:`

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./pkg/usage -run TestNewStore -v
```

Expected:
- FAIL because package/files do not exist yet

**Step 3: Write minimal implementation**

Create:
- a `Store` type wrapping `*sql.DB`
- constructor `NewStore(dsn string) (*Store, error)`
- inline schema creation for:
  - `usage_records`
  - `daily_usage_rollups`
  - `budgets`
  - `budget_alerts`
  - `historical_usage_samples`

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./pkg/usage -run TestNewStore -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add pkg/usage/models.go pkg/usage/store.go pkg/usage/store_test.go
git commit -m "feat(usage): add usage store foundation"
```

### Task 3: Add Recorder API And Persisted Usage Records

**Files:**
- Modify: `pkg/usage/models.go`
- Modify: `pkg/usage/store.go`
- Create: `pkg/usage/recorder.go`
- Modify: `pkg/usage/store_test.go`

**Step 1: Write the failing test**

Add tests for:
- inserting one reported usage record
- inserting one estimated usage record
- querying usage records for a session

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./pkg/usage -run 'Record|UsageRecord' -v
```

Expected:
- FAIL because insert/query APIs do not exist yet

**Step 3: Write minimal implementation**

Define:
```go
type CompletionRecord struct {
    SessionKey       string
    ProviderID       string
    ModelName        string
    RequestType      string
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
    DurationMS       int
    Status           string
    UsageSource      string
}
```

Add store methods:
- `RecordCompletion`
- `ListUsageRecords`

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./pkg/usage -run 'Record|UsageRecord' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add pkg/usage/models.go pkg/usage/store.go pkg/usage/recorder.go pkg/usage/store_test.go
git commit -m "feat(usage): add usage recorder and records"
```

### Task 4: Wire Agent Loop Persistence

**Files:**
- Modify: `pkg/agent/loop.go`
- Modify: `pkg/agent/loop_test.go`

**Step 1: Write the failing test**

Add tests for:
- provider-reported usage writes one persisted record
- missing provider usage falls back to tokenizer estimation

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./pkg/agent -run 'UsageEvents|PersistedUsage|EstimatedUsage' -v
```

Expected:
- FAIL because the loop does not yet call the usage recorder

**Step 3: Write minimal implementation**

Add a `UsageRecorder` dependency to `LoopDeps`.

In `ProcessDirect`:
- record one completion after `resp.Usage` is finalized
- use provider usage when present
- estimate prompt/completion totals only when usage is absent
- keep `EventUsage` emission unchanged

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./pkg/agent -run 'UsageEvents|PersistedUsage|EstimatedUsage' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add pkg/agent/loop.go pkg/agent/loop_test.go
git commit -m "feat(agent): persist usage records from provider completions"
```

### Task 5: Add Summary Queries And Daily Rollups

**Files:**
- Create: `pkg/usage/aggregator.go`
- Create: `pkg/usage/summary.go`
- Modify: `pkg/usage/store.go`
- Modify: `pkg/usage/store_test.go`

**Step 1: Write the failing test**

Add tests for:
- session summary
- current-day summary
- rolling 7-day summary
- provider/model grouping for Ollama-first summaries

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./pkg/usage -run 'Summary|Rollup|Daily' -v
```

Expected:
- FAIL because summary APIs do not exist yet

**Step 3: Write minimal implementation**

Create:
- `Summary` read model
- rollup update logic on record insertion or a deterministic recompute helper
- query methods:
  - `SessionSummary`
  - `DailySummary`
  - `WeeklySummary`
  - `CurrentProviderSummary`

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./pkg/usage -run 'Summary|Rollup|Daily' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add pkg/usage/aggregator.go pkg/usage/summary.go pkg/usage/store.go pkg/usage/store_test.go
git commit -m "feat(usage): add rollups and summary queries"
```

### Task 6: Add Budgets And Alert State

**Files:**
- Create: `pkg/usage/budget.go`
- Create: `pkg/usage/alerts.go`
- Modify: `pkg/usage/store.go`
- Modify: `pkg/usage/store_test.go`

**Step 1: Write the failing test**

Add tests for:
- threshold evaluation at 50/80/95 style boundaries
- one alert per threshold crossing per active window
- reset behavior when budget window changes

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./pkg/usage -run 'Budget|Alert|Threshold' -v
```

Expected:
- FAIL because budget logic does not exist yet

**Step 3: Write minimal implementation**

Add:
- budget models and CRUD-ready store methods
- threshold evaluation helper
- alert history recording
- summary enrichment with active budget state

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./pkg/usage -run 'Budget|Alert|Threshold' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add pkg/usage/budget.go pkg/usage/alerts.go pkg/usage/store.go pkg/usage/store_test.go
git commit -m "feat(usage): add budgets and alert state"
```

### Task 7: Add Historical Samples And Retention

**Files:**
- Modify: `pkg/usage/store.go`
- Modify: `pkg/usage/store_test.go`

**Step 1: Write the failing test**

Add tests for:
- weekly sample insertion
- pruning records older than retention window

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./pkg/usage -run 'Historical|Retention|Prune' -v
```

Expected:
- FAIL because retention behavior is absent

**Step 3: Write minimal implementation**

Add:
- sample insert helper
- prune helper for old samples and stale alerts/records as designed

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./pkg/usage -run 'Historical|Retention|Prune' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add pkg/usage/store.go pkg/usage/store_test.go
git commit -m "feat(usage): add history samples and retention"
```
