# Usage Tracking V2 Design

**Date:** 2026-03-28
**Status:** Approved for planning
**Scope:** Ollama-first usage persistence, aggregation, sidebar reporting, budgets, and notifications with future multi-provider seams

## Goal

Build a native Go usage-tracking subsystem for smolbot that persists provider usage, exposes aggregate summaries to the gateway and TUI, and supports Ollama-first budget and warning workflows without coupling the first implementation to a full multi-provider rollout.

## Validated Assumptions

- The existing runtime already emits authoritative usage data on the hot path:
  - providers populate `provider.Usage`
  - the agent emits `agent.EventUsage`
  - the gateway emits `chat.usage`
  - the TUI maps that to footer/sidebar context state
- The repo already has:
  - a reusable tokenizer in `pkg/tokenizer`
  - a SQLite storage pattern in `pkg/session/store.go`
  - a modular sidebar layout in `internal/components/sidebar`
- The current sidebar usage display is context-window pressure only. It is not persisted provider usage.

## Corrections To The Handover

- There is no current `pkg/provider/ollama.go`; Ollama is handled through the openai-compatible path plus `pkg/provider/ollama_discovery.go`.
- There is no general migration framework yet; schema creation is currently constructor-local.
- The current TUI already has a clean usage event seam. The missing pieces are persistence, aggregation, provider-scoped summaries, budgets, and notification wiring.

## Design Decision

Use an Ollama-first domain layer with future seams.

That means:

- Build a dedicated `pkg/usage` package now.
- Keep storage and query models provider-aware from day one.
- Fully wire phase 1 around the current Ollama/openai-compatible path.
- Avoid speculative multi-provider fetchers, credential storage, or dashboard backfill in the first implementation.

## Architecture

### Source Of Truth

Persist usage from the backend write path, not from the TUI and not from gateway events.

- Primary record point: `pkg/agent/loop.go` after the provider response is accumulated and `resp.Usage` is known.
- Live UX signal: keep `chat.usage` unchanged for current context updates.
- Persisted summary source: `pkg/usage` queried by gateway status handlers and future endpoints.

### Package Layout

```text
pkg/
  usage/
    models.go
    store.go
    recorder.go
    aggregator.go
    budget.go
    alerts.go
    summary.go
```

### Runtime Integration

- `pkg/agent/loop.go`
  - call `usage.Recorder.RecordCompletion(...)`
  - record provider, model, session, token counts, duration, and provenance
- `pkg/gateway/server.go`
  - extend `status` response with a persisted usage summary
  - keep `chat.usage` for active-run context display
- `internal/tui`
  - continue rendering `CONTEXT` from live context-window usage
  - add a separate `USAGE` section driven by persisted usage summary

## Storage Design

### Database

- New file: `~/.smolbot/usage.db`
- Add helper to `pkg/config/paths.go`
- Keep usage storage separate from `sessions.db`

### Phase 1 Tables

- `usage_records`
  - one row per provider completion or tracked request unit
- `daily_usage_rollups`
  - cached daily totals for fast summary and budget checks
- `budgets`
  - threshold definitions, scope, active window state
- `budget_alerts`
  - alert history and dedupe support
- `historical_usage_samples`
  - lightweight weekly-window samples for future trend UI

### Phase 1 Record Fields

- `session_key`
- `provider_id`
- `model_name`
- `request_type`
- `prompt_tokens`
- `completion_tokens`
- `total_tokens`
- `duration_ms`
- `status`
- `usage_source` (`reported` or `estimated`)
- `created_at`

## Ollama-First Strategy

- If the provider response includes usage, persist it directly.
- If usage is absent, estimate tokens with `pkg/tokenizer/tokenizer.go`.
- Keep record shapes generic enough for future provider-specific enrichments.
- Do not implement provider credential storage or dashboard scraping in phase 1.

## UI Design

### Sidebar

Preserve two distinct concepts:

- `CONTEXT`
  - active-run context pressure
  - current behavior remains
- `USAGE`
  - persisted provider/account usage
  - session, today, rolling week, optional estimated cost, budget state

### Footer

- Keep the footer focused on current session/model/context pressure.
- Add only minimal budget warning affordances later if they prove necessary.

## Gates

- Gate 0: design and storage contract
- Gate 1: backend persistence
- Gate 2: aggregation and budget engine
- Gate 3: gateway/client protocol
- Gate 4: sidebar UX
- Gate 5: notifications and quota warnings
- Gate 6: hardening

## Parallel Workstreams

- Workstream A: storage and recorder
- Workstream B: aggregation and budgets
- Workstream C: gateway/client contracts
- Workstream D: sidebar/UI
- Workstream E: notifications

Dependencies:

- A unblocks B and C
- C unblocks D
- B plus C unblock E
- Gate 6 is cross-stream hardening

## Risks

- Missing usage in some Ollama-compatible responses
  - mitigation: `usage_source` provenance and tokenizer fallback
- Double counting across retries or stream fragments
  - mitigation: record once per completed provider response boundary
- UI confusion between context usage and provider quota usage
  - mitigation: separate sections and names
- Overbuilding multi-provider abstractions too early
  - mitigation: generic record model, Ollama-first implementation only

## Scope Cuts

### In Scope

- `usage.db`
- recorder and summaries
- daily rollups
- budgets and alert history
- gateway summary payloads
- sidebar `USAGE` section
- warning indicators and notification seams

### Out Of Scope

- provider dashboard scraping or `/api/me` fetchers
- credential storage
- full pricing catalog ingestion
- hard request blocking on budget exceed
- multi-provider implementation beyond Ollama-first seams
