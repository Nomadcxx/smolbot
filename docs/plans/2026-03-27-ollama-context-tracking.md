# Ollama Context Tracking Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Restore accurate live context tracking for Ollama-backed models by emitting streamed usage and detecting the real model context window instead of relying on the static config default.

**Architecture:** Keep the existing TUI and gateway event contract, but improve the provider-side data. Continue using the OpenAI-compatible Ollama chat path for message streaming and usage, while adding a small Ollama metadata client for context-window detection via `/api/ps` and `/api/show`. Feed the detected denominator through gateway `status` and `chat.usage` payloads so header, footer, and sidebar stay aligned.

**Tech Stack:** Go, existing provider registry, gateway websocket events, Bubble Tea TUI, Ollama HTTP APIs, OpenAI-compatible chat completions.

---

### Task 1: Lock the expected Ollama behavior in provider tests

**Files:**
- Modify: `pkg/provider/openai_test.go`
- Modify: `pkg/provider/registry_test.go` if needed

**Step 1: Write failing tests for Ollama streamed usage support**

Add tests that assert:
- provider name `ollama` includes `stream_options.include_usage=true` on streamed OpenAI-compatible requests
- if the Ollama-compatible backend rejects `stream_options.include_usage`, the provider retries without it and still succeeds

**Step 2: Run the focused provider tests**

Run: `go test ./pkg/provider -run 'Test(OpenAIProvider.*|Registry.*)'`

Expected: FAIL on the Ollama usage-support assertion because `supportsStreamUsage` currently disables Ollama.

**Step 3: Commit the failing-test checkpoint**

```bash
git add pkg/provider/openai_test.go pkg/provider/registry_test.go
git commit -m "test: define ollama streamed usage expectations"
```

### Task 2: Add an Ollama metadata client for context-window detection

**Files:**
- Modify: `pkg/provider/ollama_discovery.go`
- Modify: `pkg/provider/provider.go` if shared interfaces/helpers are needed
- Create: `pkg/provider/ollama_discovery_test.go`

**Step 1: Write failing tests for context-window lookup**

Add tests covering:
- `/api/ps` exact-model match returns `context_length`
- fallback to `/api/show` when the model is not running
- `/api/show` supports both `parameters` `num_ctx` parsing and `model_info.*.context_length`
- local/default fallback behavior if Ollama metadata is unavailable

**Step 2: Run the new focused tests**

Run: `go test ./pkg/provider -run 'TestOllama.*Context.*'`

Expected: FAIL because the metadata client does not expose context lookup yet.

**Step 3: Implement minimal metadata support**

Add a small Ollama client that:
- normalizes the base URL for native Ollama endpoints
- queries `/api/ps` for running model context
- falls back to `/api/show` for model metadata
- returns a single detected context-window integer plus a source/found flag

Do not broaden this into a generic provider abstraction yet.

**Step 4: Run provider tests again**

Run: `go test ./pkg/provider -run 'Test(Ollama.*Context.*|OpenAIProvider.*)'`

Expected: PASS

**Step 5: Commit**

```bash
git add pkg/provider/ollama_discovery.go pkg/provider/ollama_discovery_test.go pkg/provider/provider.go pkg/provider/openai_test.go
git commit -m "feat: detect ollama model context window"
```

### Task 3: Re-enable Ollama streamed usage in the OpenAI-compatible client

**Files:**
- Modify: `pkg/provider/openai.go`
- Modify: `pkg/provider/openai_test.go`

**Step 1: Write or update failing tests for Ollama-specific usage gating**

Add assertions that:
- `supportsStreamUsage` allows Ollama
- retry logic still strips `stream_options` if the backend rejects it

**Step 2: Run the focused tests**

Run: `go test ./pkg/provider -run 'Test(OpenAIProviderStream.*|OpenAIProvider.*Ollama.*)'`

Expected: FAIL until the Ollama special-case is removed or adjusted.

**Step 3: Implement the minimal change**

Update the OpenAI-compatible provider so:
- Ollama uses streamed usage by default
- existing fallback retry on `stream_options/include_usage` errors remains intact
- no other provider capability logic is widened without evidence

**Step 4: Run the focused provider suite**

Run: `go test ./pkg/provider`

Expected: PASS

**Step 5: Commit**

```bash
git add pkg/provider/openai.go pkg/provider/openai_test.go
git commit -m "fix: enable streamed usage for ollama"
```

### Task 4: Carry the detected context window through gateway status and usage events

**Files:**
- Modify: `pkg/gateway/server.go`
- Modify: `pkg/gateway/server_test.go`
- Modify: `internal/client/types.go` only if payload shape changes are needed

**Step 1: Write failing gateway tests**

Add tests that assert:
- `status` returns detected Ollama context window instead of the config default when provider/model is Ollama
- `chat.usage` events emit the detected Ollama context window
- non-Ollama providers still fall back to `config.Agents.Defaults.ContextWindowTokens`

**Step 2: Run the focused gateway tests**

Run: `go test ./pkg/gateway -run 'TestServerMethods/(status|chat usage|models list and set)'`

Expected: FAIL on context-window expectations because the gateway still emits the static config default.

**Step 3: Implement minimal gateway wiring**

Add gateway-side logic that:
- resolves the current provider/model
- asks the Ollama metadata client for a context window when the current provider is `ollama`
- falls back to config if detection fails
- reuses the same helper for both `status` and `chat.usage`

Prefer a small helper over duplicating the logic in multiple event handlers.

**Step 4: Run gateway tests again**

Run: `go test ./pkg/gateway -run 'TestServerMethods/(status|chat usage|models list and set)'`

Expected: PASS

**Step 5: Commit**

```bash
git add pkg/gateway/server.go pkg/gateway/server_test.go internal/client/types.go
git commit -m "feat: surface ollama context window in gateway events"
```

### Task 5: Verify the current TUI consumes the improved payload correctly

**Files:**
- Modify: `internal/tui/tui_test.go` only if expectations need tightening
- Modify: `internal/components/sidebar/context_section.go` only if a small display fix is needed
- Modify: `internal/components/status/footer.go` only if a small display fix is needed

**Step 1: Write or update focused TUI tests**

Assert that:
- `chat.usage` with Ollama-sourced values updates sidebar context state
- header/footer/sidebar all show context when usage and detected denominator are present
- status refresh after connect picks up a detected context window

**Step 2: Run the focused TUI tests**

Run: `go test ./internal/tui -run 'Test(StatusLoadedUpdatesFooterUsage|SidebarDataUpdatesFromStatusAndCompressionEvents|UsageWarningIsAppendedOncePerSession)'`

Expected: PASS or a narrowly scoped FAIL if the current TUI needs a small compatibility fix.

**Step 3: Apply the smallest necessary TUI change**

Only if tests expose a real mismatch:
- keep the current event contract
- avoid redesigning header/footer/sidebar presentation
- fix stale clearing behavior only if it blocks accurate Ollama rendering

**Step 4: Re-run focused TUI tests**

Run: `go test ./internal/tui -run 'Test(StatusLoadedUpdatesFooterUsage|SidebarDataUpdatesFromStatusAndCompressionEvents|UsageWarningIsAppendedOncePerSession)'`

Expected: PASS

**Step 5: Commit**

```bash
git add internal/tui/tui_test.go internal/components/sidebar/context_section.go internal/components/status/footer.go
git commit -m "test: verify ollama context data renders in tui"
```

### Task 6: End-to-end verification and documentation

**Files:**
- Modify: `docs/plans/2026-03-27-ollama-context-tracking.md` if implementation notes need updating

**Step 1: Run the full focused verification set**

Run:
- `go test ./pkg/provider`
- `go test ./pkg/gateway`
- `go test ./internal/tui -run 'Test(StatusLoadedUpdatesFooterUsage|SidebarDataUpdatesFromStatusAndCompressionEvents|UsageWarningIsAppendedOncePerSession)'`

Expected:
- provider and gateway suites PASS
- TUI focused tests PASS
- ignore unrelated broader-suite baseline failures outside this slice if they remain unchanged

**Step 2: Manual smoke-check guidance**

Verify with a real Ollama-backed model:
- start a streamed chat
- confirm footer/header/sidebar begin showing usage
- confirm the denominator reflects the model context window rather than the static config default
- confirm fallback still works if the backend rejects `stream_options.include_usage`

**Step 3: Commit final documentation touch-up if needed**

```bash
git add docs/plans/2026-03-27-ollama-context-tracking.md
git commit -m "docs: finalize ollama context tracking plan notes"
```
