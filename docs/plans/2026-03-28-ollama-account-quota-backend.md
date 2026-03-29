# Ollama Account Quota Backend Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add signed `api/me` auth probing plus browser-cookie-backed Ollama account quota fetching, normalization, caching, and gateway exposure while preserving observed usage as a separate source.

**Architecture:** Extend `pkg/usage` with quota-specific models, a signed `api/me` auth/identity probe, Linux browser-cookie discovery, Ollama settings HTML parsing, a persisted quota cache, and runtime refresh wiring. Gateway status becomes the normalized read surface for both observed and quota state.

**Tech Stack:** Go, SQLite, `net/http`, HTML parsing, Linux browser-cookie discovery, existing runtime/gateway architecture

---

### Task 1: Add Config And Path Support For Quota Auth/Cache

**Files:**
- Modify: `pkg/config/config.go`
- Modify: `pkg/config/paths.go`
- Modify: `pkg/config/config_test.go`

**Step 1: Write the failing test**

Add assertions for:
- quota refresh interval config
- browser-cookie discovery enable flag
- cookie jar/cache file paths under `~/.smolbot/`

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./pkg/config -run 'DefaultPaths|DefaultConfig' -v
```

Expected:
- FAIL because quota config and paths do not exist yet

**Step 3: Write minimal implementation**

Add config and path helpers for:
- Ollama quota enabled/discovery flags
- quota refresh minutes
- imported Ollama cookie jar file
- persisted quota cache file or DB-backed path

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./pkg/config -run 'DefaultPaths|DefaultConfig' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add pkg/config/config.go pkg/config/paths.go pkg/config/config_test.go
git commit -m "feat(config): add ollama quota config and paths"
```

### Task 2: Add Quota Models And Cache Persistence

**Files:**
- Modify: `pkg/usage/models.go`
- Modify: `pkg/usage/store.go`
- Modify: `pkg/usage/store_test.go`
- Create: `pkg/usage/quota.go`

**Step 1: Write the failing test**

Add tests for:
- storing one normalized quota snapshot
- reading the latest quota snapshot
- persisting freshness/state fields across restart

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./pkg/usage -run 'QuotaSnapshot|QuotaCache' -v
```

Expected:
- FAIL because quota models and persistence APIs do not exist yet

**Step 3: Write minimal implementation**

Add normalized types for:
- `QuotaSummary`
- `QuotaState`
- `QuotaSource`

Add store methods for:
- `SaveQuotaSummary`
- `LatestQuotaSummary`

Persist:
- provider
- plan
- session percent
- weekly percent
- reset timestamps
- fetched/expires timestamps
- source/state

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./pkg/usage -run 'QuotaSnapshot|QuotaCache' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add pkg/usage/models.go pkg/usage/store.go pkg/usage/store_test.go pkg/usage/quota.go
git commit -m "feat(usage): add persisted quota summaries"
```

### Task 3: Implement Signed `api/me` Auth Probe

**Files:**
- Create: `pkg/usage/ollama_me_probe.go`
- Create: `pkg/usage/ollama_me_probe_test.go`

**Step 1: Write the failing test**

Add tests for:
- generating the expected signed challenge for `POST /api/me`
- classifying populated `api/me` responses as identity metadata
- classifying zero-value `200 OK` responses as authenticated-but-empty
- not treating `api/me` as quota data

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./pkg/usage -run 'OllamaMe|ApiMe|IdentityProbe' -v
```

Expected:
- FAIL because the probe does not exist yet

**Step 3: Write minimal implementation**

Implement:
- signed terminal-auth `POST https://ollama.com/api/me`
- response normalization for account identity fields
- explicit classification states:
  - `authenticated`
  - `authenticated_empty`
  - `unauthenticated`
  - `error`

Do not derive quota from this endpoint.

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./pkg/usage -run 'OllamaMe|ApiMe|IdentityProbe' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add pkg/usage/ollama_me_probe.go pkg/usage/ollama_me_probe_test.go
git commit -m "feat(usage): add signed ollama api me probe"
```

### Task 4: Implement Linux Browser-Cookie Discovery And Import

**Files:**
- Create: `pkg/usage/browser_cookies_linux.go`
- Create: `pkg/usage/browser_cookies_linux_test.go`
- Create: `pkg/usage/cookie_jar_store.go`

**Step 1: Write the failing test**

Add tests for:
- discovering Chromium-family cookie DB candidates on Linux
- filtering only Ollama cookies
- writing an imported cookie jar with restricted permissions

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./pkg/usage -run 'BrowserCookie|CookieJar' -v
```

Expected:
- FAIL because discovery/import helpers do not exist yet

**Step 3: Write minimal implementation**

Implement:
- Linux browser-cookie source discovery
- cookie filtering for `ollama.com`
- import into a smolbot-managed jar or JSON cookie store
- permission handling for the imported cookie file

Prefer:
- Chromium-family discovery first
- explicit override path from config

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./pkg/usage -run 'BrowserCookie|CookieJar' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add pkg/usage/browser_cookies_linux.go pkg/usage/browser_cookies_linux_test.go pkg/usage/cookie_jar_store.go
git commit -m "feat(usage): add linux ollama cookie import"
```

### Task 5: Implement Ollama Settings Parser

**Files:**
- Create: `pkg/usage/ollama_settings_parser.go`
- Create: `pkg/usage/ollama_settings_parser_test.go`
- Create: `pkg/usage/testdata/ollama_settings_usage.html`

**Step 1: Write the failing test**

Add tests for parsing:
- plan tier
- session percent used
- session reset timestamp
- weekly percent used
- weekly reset timestamp
- notification toggle state when present

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./pkg/usage -run 'OllamaSettingsParser' -v
```

Expected:
- FAIL because parser and fixture do not exist yet

**Step 3: Write minimal implementation**

Use the captured HTML fixture to parse the server-rendered usage content into a normalized quota model.

Be tolerant of:
- whitespace changes
- class-name churn
- ordering changes between session and weekly blocks

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./pkg/usage -run 'OllamaSettingsParser' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add pkg/usage/ollama_settings_parser.go pkg/usage/ollama_settings_parser_test.go pkg/usage/testdata/ollama_settings_usage.html
git commit -m "feat(usage): parse ollama settings quota html"
```

### Task 6: Implement Fetcher And Hourly Refresh Service

**Files:**
- Create: `pkg/usage/ollama_quota_fetcher.go`
- Create: `pkg/usage/ollama_quota_fetcher_test.go`
- Modify: `cmd/smolbot/runtime.go`
- Modify: `cmd/smolbot/runtime_test.go`

**Step 1: Write the failing test**

Add tests for:
- successful signed `api/me` probe classification before quota fetch
- successful authenticated fetch using imported cookies
- auth-expired response becomes `expired`
- failed fetch keeps last-good cached quota as `stale`
- runtime schedules hourly refresh instead of per-request fetches

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./pkg/usage ./cmd/smolbot -run 'QuotaFetcher|QuotaRefresh|Runtime' -v
```

Expected:
- FAIL because no fetcher/refresh service exists yet

**Step 3: Write minimal implementation**

Implement:
- signed `POST https://ollama.com/api/me` probe for auth/account metadata
- authenticated `GET https://ollama.com/settings`
- parser invocation
- snapshot persistence
- hourly refresh loop in runtime
- startup behavior that uses cached quota immediately and refreshes in background

Fetch order:
1. `api/me` probe
2. `settings` quota fetch

Use `api/me` only for auth/account metadata and source state, not for percentage quota fields.

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./pkg/usage ./cmd/smolbot -run 'QuotaFetcher|QuotaRefresh|Runtime' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add pkg/usage/ollama_quota_fetcher.go pkg/usage/ollama_quota_fetcher_test.go cmd/smolbot/runtime.go cmd/smolbot/runtime_test.go
git commit -m "feat(usage): add ollama quota fetcher and hourly refresh"
```

### Task 7: Expose Observed And Quota Usage Through Gateway Status

**Files:**
- Modify: `pkg/gateway/server.go`
- Modify: `pkg/gateway/server_test.go`
- Modify: `internal/client/types.go`
- Modify: `internal/client/client_test.go`

**Step 1: Write the failing test**

Add tests for:
- status payload includes both observed and quota blocks
- status payload optionally includes account identity/probe metadata when available
- stale and expired quota states are explicit
- missing quota leaves observed intact

**Step 2: Run test to verify it fails**

Run:
```bash
go test ./pkg/gateway ./internal/client -run 'Quota|Observed|Status' -v
```

Expected:
- FAIL because the payload shape does not exist yet

**Step 3: Write minimal implementation**

Extend status payload to include:
- `usage.observed`
- `usage.quota`
- `usage.account` or equivalent probe metadata block when useful

Do not overload the existing live context usage object.

**Step 4: Run test to verify it passes**

Run:
```bash
go test ./pkg/gateway ./internal/client -run 'Quota|Observed|Status' -v
```

Expected:
- PASS

**Step 5: Commit**

```bash
git add pkg/gateway/server.go pkg/gateway/server_test.go internal/client/types.go internal/client/client_test.go
git commit -m "feat(gateway): expose observed and quota usage status"
```
