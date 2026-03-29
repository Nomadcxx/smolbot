# Ollama Account Quota Design

**Date:** 2026-03-28
**Status:** Approved for planning
**Scope:** Add true Ollama account-backed quota to the existing usage-tracking subsystem while retaining smolbot-observed usage as a separate source.

## Goal

Deliver an Ollama-first usage experience that shows both:

- `Observed` usage: provider completions smolbot has seen and recorded
- `Quota` usage: actual Ollama account usage windows and reset times from the authenticated Ollama settings page

The MVP must avoid conflating the two.

## Validated Facts

- Ollama’s public docs currently document cloud API access and per-request usage metrics, but do not document a public account quota endpoint.
- `POST https://ollama.com/api/me` exists and accepts signed terminal auth using the local `~/.ollama/id_ed25519` key.
- The public `api/me` response appears to be identity/account metadata, not quota data.
- The local Ollama daemon on `127.0.0.1:11434` does not expose `GET /api/v1/me` or `GET /api/v1/settings` in the current environment.
- The authenticated `https://ollama.com/settings` page contains server-rendered quota data for:
  - session usage percent
  - weekly usage percent
  - reset timestamps
  - plan tier
- The auth boundary for `/settings` remains browser web-session cookies, not the documented API key flow.
- The desired refresh interval is hourly, not per-request.
- Browser-cookie discovery is useful for MVP; smolbot should not require manual cookie copying in the common case.

## Product Decision

Support two independent usage lanes under one `USAGE` section:

- `Observed`
  - derived from `usage.db`
  - useful for session-level debugging, burn tracking, and request counts
- `Quota`
  - derived from authenticated Ollama settings HTML
  - useful for actual account-backed limits, reset windows, and warning state

Rules:

- never merge `Observed` and `Quota` into one number
- never silently substitute `Observed` while labeling it as `Quota`
- show quota source/state explicitly: `live`, `stale`, `expired`, `unavailable`

## Architecture

### Source Model

Extend the usage domain with a normalized account-backed read model:

```text
UsageView
  ObservedUsageSummary
  QuotaSummary
```

`ObservedUsageSummary` continues to come from persisted provider completions in `usage.db`.

`QuotaSummary` is fetched independently and cached.

### Ollama Identity And Quota Flow

1. Probe `POST https://ollama.com/api/me` using signed terminal auth from `~/.ollama/id_ed25519`.
2. Use the response for identity/account metadata only when populated:
   - account presence
   - basic plan/account fields
   - auth validity
3. Discover candidate browser-cookie sources on Linux.
4. Import only Ollama-relevant cookies into a smolbot-managed cookie jar.
5. Fetch `https://ollama.com/settings` using that jar.
6. Parse the server-rendered HTML for:
   - plan tier
   - session percent used
   - session reset timestamp
   - weekly percent used
   - weekly reset timestamp
7. Normalize the result into `QuotaSummary`.
8. Cache the result and expose it through gateway status payloads.

`/api/me` is treated as an auth/identity probe, not the primary quota source.

### Cookie Strategy

Use import/copy, not direct live browser reads on every refresh.

- browser discovery happens initially and on explicit re-auth refresh
- quota fetches read from a smolbot-owned cookie jar
- the jar is stored under `~/.smolbot/` with restricted permissions
- cookie re-import happens only when:
  - quota auth expires
  - the user explicitly refreshes auth
  - no valid smolbot jar exists yet

### Runtime Refresh Model

- hourly refresh for Ollama quota
- no quota fetch on every status request
- gateway reads the latest cached quota state
- status payload includes freshness metadata:
  - `fetchedAt`
  - `expiresAt`
  - `state`
  - `source`

## Data Model

### Observed Usage

Observed usage remains rooted in `usage.db` and should be summarized more clearly:

- session tokens
- request count
- optional last-turn or last-run tokens if needed later

### Quota Summary

Quota summary needs normalized fields like:

- `provider_id`
- `account_name`
- `account_email`
- `plan_name`
- `session_used_percent`
- `session_resets_at`
- `weekly_used_percent`
- `weekly_resets_at`
- `notify_usage_limits`
- `state`
- `source`
- `fetched_at`
- `expires_at`

Persist the latest normalized quota snapshot locally so gateway/TUI can show data immediately after restart without forcing a live fetch.

## UI Model

Keep one `USAGE` section, but render two clearly separated subsections:

- `Observed`
  - session tokens
  - request count
  - optional last run
- `Quota`
  - account or plan label when available
  - session percent used
  - weekly percent used
  - reset countdowns
  - freshness/auth state

This solves the current ambiguity where one cumulative token number can be mistaken for account truth.

## Security Model

- import only the minimum Ollama cookies needed
- write the smolbot-managed jar with `0600`
- never log cookie values
- redact auth failures
- provide clear UI states for:
  - `connected`
  - `stale`
  - `expired`
  - `unavailable`

## Risks

- Ollama changes HTML structure on `/settings`
  - mitigation: parser fixtures, tolerant selectors, explicit `source/state`
- browser-cookie discovery varies by browser
  - mitigation: discovery abstraction plus manual override path
- `api/me` may return valid-but-empty account payloads
  - mitigation: treat it as advisory identity metadata, not quota truth
- auth expiry causes confusing empty quota states
  - mitigation: cached last-good snapshot plus state badge
- users misread observed usage as quota
  - mitigation: separate sub-sections and labels

## Scope

### In Scope

- Linux browser-cookie discovery
- signed `api/me` auth/identity probe
- imported Ollama cookie jar
- hourly `settings` fetcher
- HTML parser for session/weekly usage
- normalized quota summary
- gateway/client payload extension
- sidebar `USAGE` redesign with `Observed` and `Quota`

### Out Of Scope

- OAuth app flow
- headless browser login automation
- provider-agnostic account quota for non-Ollama providers
- heavy graph/dashboard work
- continuous browser-cookie watching
