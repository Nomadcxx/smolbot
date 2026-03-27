# Usage Tracking V2 Orchestration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Deliver the Ollama-first usage-tracking MVP through gated parallel workstreams without introducing ambiguous ownership or cross-stream merge conflicts.

**Architecture:** The rollout is split into a contract-first orchestration layer plus two execution streams: backend core and gateway/sidebar/notifications. Gate ownership is explicit so backend contracts stabilize before UI and notification work fan out.

**Tech Stack:** Go, SQLite, Bubble Tea, Lip Gloss, existing gateway WebSocket protocol, existing session-store testing patterns

---

### Task 1: Freeze Contracts And Ownership

**Files:**
- Read: `docs/plans/2026-03-28-usage-tracking-v2-design.md`
- Read: `docs/plans/2026-03-28-usage-tracking-v2-backend-core.md`
- Read: `docs/plans/2026-03-28-usage-tracking-v2-ui-alerts.md`

**Step 1: Confirm file ownership before code starts**

Backend stream owns:
- `pkg/config/paths.go`
- `pkg/usage/*`
- `pkg/agent/loop.go`
- backend tests for the above

UI stream owns:
- `pkg/gateway/server.go`
- `pkg/gateway/server_test.go`
- `internal/client/*`
- `internal/components/sidebar/*`
- `internal/tui/tui.go`

**Step 2: Confirm gate dependencies**

- Gate 0 is satisfied by the approved design and these plans.
- Gate 1 must complete before any stream merges.
- Gate 2 may proceed once Gate 1 storage contracts are green.
- Gate 3 may begin after the backend summary read model is stable.
- Gate 5 begins only after Gate 2 and Gate 3 are both green.

**Step 3: Create implementation branches or worktrees per stream**

Run:
```bash
git worktree add ../smolbot-usage-backend -b feat/usage-backend
git worktree add ../smolbot-usage-ui -b feat/usage-ui
```

Expected:
- both worktrees create successfully
- each stream edits only its owned files

**Step 4: Commit planning artifacts**

Run:
```bash
git add docs/plans/2026-03-28-usage-tracking-v2-design.md docs/plans/2026-03-28-usage-tracking-v2-orchestration.md docs/plans/2026-03-28-usage-tracking-v2-backend-core.md docs/plans/2026-03-28-usage-tracking-v2-ui-alerts.md
git commit -m "docs: add usage tracking v2 design and rollout plans"
```

### Task 2: Gate Review Sequence

**Files:**
- Read: `pkg/usage/*`
- Read: `pkg/gateway/server.go`
- Read: `internal/components/sidebar/*`

**Step 1: Gate 1 review**

Check:
- `usage.db` path exists in config paths
- schema initializes on first use
- `pkg/agent/loop.go` writes persisted usage records
- tests cover reported and estimated usage paths

Run:
```bash
go test ./pkg/usage ./pkg/agent -run 'Usage|usage' -v
```

Expected:
- targeted tests pass

**Step 2: Gate 2 review**

Check:
- daily rollups and summary queries are implemented
- budget threshold evaluation exists
- alert history persists without duplicate threshold spam

Run:
```bash
go test ./pkg/usage -run 'Budget|Summary|Rollup|Alert' -v
```

Expected:
- targeted tests pass

**Step 3: Gate 3 and Gate 4 review**

Check:
- gateway status includes persisted usage summary
- client payloads decode it
- sidebar renders `USAGE` independently from `CONTEXT`

Run:
```bash
go test ./pkg/gateway ./internal/components/sidebar ./internal/tui -run 'Usage|Sidebar|Status' -v
```

Expected:
- targeted tests pass

**Step 4: Gate 5 review**

Check:
- warning states appear only when thresholds are crossed
- notification plumbing has clear seams and no duplicate firing

Run:
```bash
go test ./pkg/usage ./pkg/gateway ./internal/tui -run 'Notification|Warning|Alert' -v
```

Expected:
- targeted tests pass

**Step 5: Gate 6 review**

Run:
```bash
go test ./...
```

Expected:
- full suite passes
- no regressions in existing context usage behavior

### Task 3: Merge Strategy

**Files:**
- Read: `git status`

**Step 1: Merge backend stream first**

Run:
```bash
git checkout main
git merge --no-ff feat/usage-backend
```

Expected:
- backend contracts land before UI integration

**Step 2: Rebase or merge UI stream onto updated main**

Run:
```bash
git checkout feat/usage-ui
git merge main
```

Expected:
- gateway/sidebar work picks up final backend contracts

**Step 3: Merge UI stream after verification**

Run:
```bash
git checkout main
git merge --no-ff feat/usage-ui
```

Expected:
- final merge includes UI, notifications, and hardening updates
