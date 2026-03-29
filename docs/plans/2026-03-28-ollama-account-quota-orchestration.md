# Ollama Account Quota Orchestration Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Deliver true Ollama account-backed quota alongside improved observed-usage summaries through gated parallel workstreams with explicit backend/UI ownership.

**Architecture:** The rollout is split into a backend stream for signed `api/me` auth probing, cookie import, settings scraping, normalization, caching, and gateway contracts, plus a UI stream for the `USAGE` section redesign. Merge order is backend first, then UI, with hard gates around auth-source behavior, parser stability, and user-facing clarity.

**Tech Stack:** Go, SQLite, Bubble Tea, WebSocket gateway protocol, HTML parsing, Linux browser-cookie discovery

---

### Task 1: Freeze Scope And Ownership

**Files:**
- Read: `docs/plans/2026-03-28-ollama-account-quota-design.md`
- Read: `docs/plans/2026-03-28-ollama-account-quota-backend.md`
- Read: `docs/plans/2026-03-28-ollama-account-quota-ui.md`

**Step 1: Confirm backend ownership**

Backend stream owns:
- `pkg/config/*`
- `pkg/usage/*`
- `cmd/smolbot/runtime.go`
- `pkg/gateway/server.go`
- backend tests for all of the above

**Step 2: Confirm UI ownership**

UI stream owns:
- `internal/client/*`
- `internal/components/sidebar/*`
- `internal/tui/tui.go`
- UI tests for all of the above

**Step 3: Confirm gate dependencies**

- Gate 0 is satisfied by the approved design and these plans.
- Gate 1 must land before UI starts consuming quota payloads.
- Gate 2 must stabilize parser, cookie import, and cache behavior.
- Gate 3 must stabilize gateway contracts before sidebar work merges.
- Gate 4 must complete sidebar semantics and stale/expired states.
- Gate 5 covers notifications and refresh/auth failure UX.
- Gate 6 is hardening and full-suite verification.

**Step 4: Create or reuse isolated worktrees**

Run:
```bash
git worktree add ../smolbot-ollama-quota-backend -b feat/ollama-quota-backend
git worktree add ../smolbot-ollama-quota-ui -b feat/ollama-quota-ui
```

Expected:
- both worktrees create successfully
- each stream edits only its owned files

### Task 2: Gate Review Checklist

**Files:**
- Read: `pkg/usage/*`
- Read: `pkg/gateway/server.go`
- Read: `internal/components/sidebar/*`

**Step 1: Gate 1 review**

Check:
- config and paths for quota auth/cache storage exist
- quota models exist beside observed usage models
- failing tests drove the first implementation

Run:
```bash
go test ./pkg/config ./pkg/usage -run 'Quota|Cookie|Paths' -v
```

Expected:
- targeted tests pass

**Step 2: Gate 2 review**

Check:
- Ollama settings parser handles the captured HTML fixture
- signed `api/me` probing is covered and classified correctly as identity/auth, not quota
- cookie import stores only Ollama-relevant cookies
- quota cache persists freshness and state

Run:
```bash
go test ./pkg/usage -run 'OllamaSettings|CookieImport|QuotaCache' -v
```

Expected:
- targeted tests pass

**Step 3: Gate 3 review**

Check:
- runtime refresh wiring exists
- gateway status returns both `observed` and `quota`
- identity probe data does not masquerade as quota data
- stale/expired states are explicit in payloads

Run:
```bash
go test ./cmd/smolbot ./pkg/gateway -run 'Quota|Status|Runtime' -v
```

Expected:
- targeted tests pass

**Step 4: Gate 4 review**

Check:
- sidebar shows `Observed` and `Quota` under one `USAGE` section
- quota failures do not hide observed data
- UI labels do not imply account truth for observed usage

Run:
```bash
go test ./internal/components/sidebar ./internal/tui ./internal/client -run 'Usage|Quota|Observed|Status' -v
```

Expected:
- targeted tests pass

**Step 5: Gate 5 review**

Check:
- auth-expired and stale-quota notifications are deduped
- hourly refresh failures are surfaced once and then degraded cleanly

Run:
```bash
go test ./pkg/usage ./internal/tui ./pkg/gateway -run 'Stale|Expired|Notification|Warning' -v
```

Expected:
- targeted tests pass

**Step 6: Gate 6 review**

Run:
```bash
go test ./...
```

Expected:
- full suite passes
- no regression in existing context usage behavior

### Task 3: Merge Order

**Files:**
- Read: `git status`

**Step 1: Merge backend stream first**

Run:
```bash
git checkout main
git merge --no-ff feat/ollama-quota-backend
```

Expected:
- parser, cache, runtime, and gateway contracts land before UI wiring

**Step 2: Merge UI stream after rebasing onto backend contracts**

Run:
```bash
git checkout feat/ollama-quota-ui
git merge main
git checkout main
git merge --no-ff feat/ollama-quota-ui
```

Expected:
- final merge includes clear `USAGE` semantics without contract churn
