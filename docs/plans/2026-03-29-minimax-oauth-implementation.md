# MiniMax OAuth Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add MiniMax Token Plan OAuth login to smolbot without regressing the existing API-key MiniMax provider or the new provider/model UI.

**Architecture:** Treat OAuth MiniMax as a separate provider identity, `minimax-portal`, instead of overloading the existing `minimax` API-key provider. Implement the backend first: correct MiniMax auth protocol, persistent token storage, provider resolution, and refresh behavior. Only after the backend is stable should installer and TUI surfaces expose OAuth status and login flows.

**Tech Stack:** Go standard library (`net/http`, `encoding/json`, `crypto/rand`, `crypto/sha256`, `sync`), existing smolbot provider registry/config system, Bubble Tea installer/TUI.

---

## Execution Workflow

For every task in this plan:

1. Write the failing test first.
2. Run the focused test and confirm it fails for the expected reason.
3. Implement the minimum code to make it pass.
4. Run the focused verification for the task.
5. Run a spec/completeness review for the task.
6. Run a code-quality review for the task.
7. Fix findings and re-run the focused verification.
8. Create a checkpoint commit.
9. Only then move to the next task.

Final push/merge gate:

- focused task tests green
- relevant package tests green
- build green
- final spec review green
- final code-quality review green

## Reference Notes

This plan intentionally does **not** follow the stale handover literally.

- The current branch already contains committed OAuth primitives in:
  - `pkg/provider/oauth.go`
  - `pkg/provider/pkce.go`
- There is an untracked `pkg/provider/minimax_oauth.go` spike in the worktree. It is not authoritative and should be replaced if it conflicts with this plan.
- OpenClaw is the backend reference:
  - MiniMax OAuth is a PKCE-backed **user-code/device-style** login flow
  - it uses a distinct provider identity, `minimax-portal`
  - it keeps API-key MiniMax and OAuth MiniMax separate
- smolbot should **not** add child-session style auth/profile management or an over-general auth framework in this branch. Keep the design narrow and practical.

## Non-Negotiable Invariants

1. Preserve existing API-key MiniMax support under `minimax`.
2. Add OAuth as a separate provider identity: `minimax-portal`.
3. Do not silently downgrade OAuth configuration to API-key mode.
4. Do not assume `/oauth/revoke` is supported unless the implementation verifies it. Revoke is optional in Phase 1.
5. Do not expose Token Plan-only OAuth as if it were interchangeable with PAYG API-key access.
6. Do not default OAuth users onto `MiniMax-M2.7-highspeed`.
7. Do not implement installer/TUI login before backend auth and storage are stable.

## Current Branch State To Reconcile

Before Task 1, confirm the branch state:

- committed:
  - `pkg/provider/oauth.go`
  - `pkg/provider/oauth_test.go`
  - `pkg/provider/pkce.go`
  - `pkg/provider/pkce_test.go`
- untracked spike:
  - `pkg/provider/minimax_oauth.go`

The first task must treat those files as existing context, not as a blank slate.

---

### Task 0: Reconcile The Existing OAuth Branch State

**Files:**
- Review: `pkg/provider/oauth.go`
- Review: `pkg/provider/oauth_test.go`
- Review: `pkg/provider/pkce.go`
- Review: `pkg/provider/pkce_test.go`
- Review: `pkg/provider/minimax_oauth.go`
- Modify: `docs/plans/2026-03-29-handover-oauth-implementation.md`

**Step 1: Verify the currently committed OAuth primitives still build**

Run: `go test ./pkg/provider ./pkg/config ./cmd/installer`
Expected: PASS

**Step 2: Replace the stale handover with an accurate branch-state handover**

The updated handover must state:

- OAuth types and PKCE are already committed
- `minimax_oauth.go` exists only as a local spike unless committed later
- this plan supersedes the stale “0 tasks complete” story

**Step 3: Checkpoint the doc correction**

```bash
git add docs/plans/2026-03-29-handover-oauth-implementation.md docs/plans/2026-03-29-minimax-oauth-implementation.md
git commit -m "docs(provider): correct minimax oauth implementation plan"
```

---

### Task 1: Tighten OAuth Types Around Real Auth Modes

**Files:**
- Modify: `pkg/provider/oauth.go`
- Test: `pkg/provider/oauth_test.go`
- Modify: `pkg/config/config.go`
- Test: `pkg/config/config_test.go`

**Intent:**
Keep the existing OAuth primitives, but extend them to support real provider config and stored-token references.

**Required behavior:**

- `ProviderConfig` gains explicit auth/profile linkage fields, such as:
  - `AuthType string`
  - `ProfileID string`
- OAuth token metadata can represent:
  - provider id
  - profile id
  - optional account email/name
  - updated-at time
- `AuthType` string mapping remains stable for `api_key`, `oauth`, and `token`

**Step 1: Write failing tests**

Add tests that prove:

- `ProviderConfig` round-trips `authType` and `profileId` through JSON
- OAuth token metadata round-trips through JSON
- existing API-key-only configs still load without requiring OAuth fields

**Step 2: Run focused tests and confirm failure**

Run: `go test ./pkg/provider ./pkg/config -run 'Test(AuthType|TokenInfo|ProviderConfig.*OAuth)'`
Expected: FAIL because config/token metadata fields are missing

**Step 3: Implement minimal changes**

- extend `ProviderConfig`
- add any missing OAuth token/profile metadata types
- keep backwards compatibility with existing config files

**Step 4: Run focused verification**

Run: `go test ./pkg/provider ./pkg/config -run 'Test(AuthType|TokenInfo|ProviderConfig.*OAuth)'`
Expected: PASS

**Step 5: Review gate**

- spec review: confirm config model supports separate API-key and OAuth MiniMax
- code-quality review: confirm fields are minimal and backwards compatible

**Step 6: Checkpoint**

```bash
git add pkg/provider/oauth.go pkg/provider/oauth_test.go pkg/config/config.go pkg/config/config_test.go
git commit -m "feat(provider): extend oauth config metadata"
```

---

### Task 2: Add Persistent OAuth Token Storage

**Files:**
- Create: `pkg/config/oauth_store.go`
- Test: `pkg/config/oauth_store_test.go`

**Intent:**
Store OAuth tokens outside main config, keyed by provider/profile identity.

**Required behavior:**

- store path defaults under the smolbot config directory
- save/load/clear by `(provider, profileID)`
- atomic writes
- private file permissions
- safe behavior when the store file does not yet exist

**Step 1: Write failing tests**

Cover:

- save then load round-trip
- clear removes a single entry without clobbering others
- missing file returns not-found semantics, not a parse failure
- store writes private permissions

**Step 2: Run focused tests and confirm failure**

Run: `go test ./pkg/config -run 'TestOAuthTokenStore'`
Expected: FAIL because the store does not exist yet

**Step 3: Implement minimal store**

- single JSON file
- atomic temp-write + rename
- preserve existing entries on update

**Step 4: Run focused verification**

Run: `go test ./pkg/config -run 'TestOAuthTokenStore'`
Expected: PASS

**Step 5: Review gate**

- spec review: confirm storage separation from main config
- code-quality review: confirm permissions and update behavior are safe

**Step 6: Checkpoint**

```bash
git add pkg/config/oauth_store.go pkg/config/oauth_store_test.go
git commit -m "feat(config): add oauth token store"
```

---

### Task 3: Implement A Correct MiniMax Portal Auth Client

**Files:**
- Modify/Create: `pkg/provider/minimax_oauth.go`
- Test: `pkg/provider/minimax_oauth_test.go`

**Intent:**
Implement the real MiniMax user-code login flow and token refresh logic.

**Required behavior:**

- provider identity is `minimax-portal`
- separate global/CN endpoint config is supported
- auth-code request uses PKCE and state
- token polling uses MiniMax’s user-code flow shape, not a generic `device_code`
- token expiry comes from token response, not auth-code expiry
- refresh is supported
- revoke is optional; if implemented, it must be verified and tested
- HTTP client must be injectable for tests

**MiniMax protocol details to preserve:**

- auth code endpoint: `/oauth/code`
- token endpoint: `/oauth/token`
- login request includes:
  - `response_type=code`
  - `client_id`
  - `scope`
  - `code_challenge`
  - `code_challenge_method=S256`
  - `state`
- polling request uses:
  - `grant_type=urn:ietf:params:oauth:grant-type:user_code`
  - `client_id`
  - `user_code`
  - `code_verifier`

**Step 1: Write failing tests**

Cover:

- initiating auth sends the expected form payload
- state mismatch is rejected
- pending authorization is distinguishable from hard failure
- successful token exchange sets access/refresh token and expiry correctly
- refresh request posts the expected form and updates the token
- `Chat`/`ChatStream` delegates through OpenAI-compatible transport using the OAuth access token

**Step 2: Run focused tests and confirm failure**

Run: `go test ./pkg/provider -run 'TestMiniMaxOAuth'`
Expected: FAIL because the current spike uses the wrong protocol/expiry behavior or the file is absent

**Step 3: Implement minimal auth client**

- replace incorrect spike logic as needed
- inject `http.Client`
- distinguish retryable pending auth from terminal errors
- avoid timer leaks in poll loop

**Step 4: Run focused verification**

Run: `go test ./pkg/provider -run 'TestMiniMaxOAuth'`
Expected: PASS

**Step 5: Review gate**

- spec review: confirm `minimax-portal` semantics and protocol shape
- code-quality review: confirm timer/error handling and client injection are solid

**Step 6: Checkpoint**

```bash
git add pkg/provider/minimax_oauth.go pkg/provider/minimax_oauth_test.go
git commit -m "feat(provider): add minimax portal oauth client"
```

---

### Task 4: Wire OAuth Resolution Into The Provider Registry

**Files:**
- Modify: `pkg/provider/registry.go`
- Test: `pkg/provider/registry_test.go`
- Modify: `cmd/smolbot/runtime.go`
- Test: `cmd/smolbot/runtime_model_test.go`

**Intent:**
Allow runtime provider resolution to construct `minimax-portal` from config + token store without breaking existing `minimax`.

**Required behavior:**

- `minimax` stays API-key based
- `minimax-portal` is selected when model/provider resolution points there
- registry loads tokens via provider/profile linkage
- expired access tokens refresh before use when refresh token is available
- runtime/provider switching does not collapse `minimax-portal` back to `openai`

**Step 1: Write failing tests**

Cover:

- registry returns API-key provider for `minimax`
- registry returns OAuth-backed provider for `minimax-portal`
- missing OAuth token gives a clear error
- expired access token triggers refresh path

**Step 2: Run focused tests and confirm failure**

Run: `go test ./pkg/provider ./cmd/smolbot -run 'TestRegistry.*MiniMax|Test.*MiniMaxPortal'`
Expected: FAIL because registry does not yet understand OAuth-backed provider resolution

**Step 3: Implement minimal registry/runtime changes**

- add factory path for `minimax-portal`
- connect token store/profile loading
- keep provider detection and cache keys explicit

**Step 4: Run focused verification**

Run: `go test ./pkg/provider ./cmd/smolbot -run 'TestRegistry.*MiniMax|Test.*MiniMaxPortal'`
Expected: PASS

**Step 5: Review gate**

- spec review: confirm separate provider identities are preserved
- code-quality review: confirm no cache-key or fallback regression

**Step 6: Checkpoint**

```bash
git add pkg/provider/registry.go pkg/provider/registry_test.go cmd/smolbot/runtime.go cmd/smolbot/runtime_model_test.go
git commit -m "feat(provider): wire minimax portal oauth into registry"
```

---

### Task 5: Add Installer Support For MiniMax OAuth

**Files:**
- Modify: `cmd/installer/types.go`
- Modify: `cmd/installer/views.go`
- Modify: `cmd/installer/tasks.go`
- Modify: `cmd/installer/utils.go`
- Test: `cmd/installer/*_test.go`

**Intent:**
Expose MiniMax OAuth as a first-class installer choice without disturbing the existing API-key flow.

**Required behavior:**

- installer shows separate choices for:
  - MiniMax API key
  - MiniMax OAuth
- MiniMax OAuth writes provider config for `minimax-portal`
- main config stores auth mode/profile linkage, not raw access tokens
- user sees verification URL and user code
- successful login stores token in OAuth store and selects a safe default model

**Step 1: Write failing tests**

Cover:

- selecting MiniMax OAuth writes `providers["minimax-portal"]`
- selected model defaults to a Token Plan-safe model
- installer persists `authType`/`profileId`
- installer fails cleanly when login/store save fails

**Step 2: Run focused tests and confirm failure**

Run: `go test ./cmd/installer -run 'Test.*MiniMaxOAuth'`
Expected: FAIL because installer has no OAuth path yet

**Step 3: Implement minimal installer flow**

- add explicit provider option
- perform login
- save token in store
- write config patch

**Step 4: Run focused verification**

Run: `go test ./cmd/installer -run 'Test.*MiniMaxOAuth'`
Expected: PASS

**Step 5: Review gate**

- spec review: confirm MiniMax API-key and OAuth choices are distinct
- code-quality review: confirm installer failures are visible and non-destructive

**Step 6: Checkpoint**

```bash
git add cmd/installer/types.go cmd/installer/views.go cmd/installer/tasks.go cmd/installer/utils.go cmd/installer/*_test.go
git commit -m "feat(installer): add minimax oauth setup"
```

---

### Task 6: Surface OAuth Status In The Provider UI

**Files:**
- Modify: `internal/components/dialog/providers.go`
- Test: `internal/components/dialog/providers_test.go`
- Modify: `internal/tui/tui.go`
- Test: `internal/tui/provider_flow_test.go`
- Modify: `internal/client/types.go`

**Intent:**
Make `/providers` and the provider dialog show auth mode and status clearly.

**Required behavior:**

- provider dialog distinguishes:
  - API key configured
  - OAuth configured
  - OAuth missing token/profile
- active provider can show account identity/expiry when known
- `minimax-portal` is rendered as MiniMax OAuth, not as a generic OpenAI-compatible row

**Step 1: Write failing tests**

Cover:

- active provider row shows OAuth auth type
- configured section distinguishes API-key MiniMax from MiniMax OAuth
- missing token/profile renders as incomplete configuration

**Step 2: Run focused tests and confirm failure**

Run: `go test ./internal/components/dialog ./internal/tui -run 'Test.*Provider.*OAuth|Test.*MiniMax.*OAuth'`
Expected: FAIL because dialog/client payloads do not yet expose OAuth status

**Step 3: Implement minimal provider-status surface**

- extend provider info/status payloads as needed
- render auth mode cleanly

**Step 4: Run focused verification**

Run: `go test ./internal/components/dialog ./internal/tui -run 'Test.*Provider.*OAuth|Test.*MiniMax.*OAuth'`
Expected: PASS

**Step 5: Review gate**

- spec review: confirm provider UI now reflects real auth mode
- code-quality review: confirm no hardcoded provider hacks beyond MiniMax naming/typing

**Step 6: Checkpoint**

```bash
git add internal/components/dialog/providers.go internal/components/dialog/providers_test.go internal/tui/tui.go internal/tui/provider_flow_test.go internal/client/types.go
git commit -m "feat(tui): show minimax oauth provider status"
```

---

### Task 7: Add OAuth-Aware Model Filtering

**Files:**
- Modify: `internal/components/dialog/models.go`
- Test: `internal/components/dialog/models_test.go`
- Modify: `pkg/provider/discovery.go`
- Test: `pkg/provider/discovery_test.go`

**Intent:**
Prevent obvious bad MiniMax OAuth model choices and make the model picker honest about auth requirements.

**Required behavior:**

- `minimax-portal` defaults to Token Plan-safe models
- models not available to OAuth/Token Plan users are either hidden or clearly marked non-selectable
- model picker still preserves the improved group/filter/save workflow already implemented on this branch

**Step 1: Write failing tests**

Cover:

- `minimax-portal` rows do not default to `MiniMax-M2.7-highspeed`
- unavailable rows render as non-selectable info rows or carry a clear hint

**Step 2: Run focused tests and confirm failure**

Run: `go test ./internal/components/dialog ./pkg/provider -run 'Test.*MiniMax.*Portal|Test.*ModelPicker.*OAuth'`
Expected: FAIL because discovery/model picker do not yet distinguish OAuth-specific availability

**Step 3: Implement minimal filtering**

- extend discovery metadata only as needed
- avoid broad auth requirement machinery for unrelated providers

**Step 4: Run focused verification**

Run: `go test ./internal/components/dialog ./pkg/provider -run 'Test.*MiniMax.*Portal|Test.*ModelPicker.*OAuth'`
Expected: PASS

**Step 5: Review gate**

- spec review: confirm Token Plan-safe behavior
- code-quality review: confirm filtering logic is explicit and local, not speculative

**Step 6: Checkpoint**

```bash
git add internal/components/dialog/models.go internal/components/dialog/models_test.go pkg/provider/discovery.go pkg/provider/discovery_test.go
git commit -m "feat(provider): add minimax oauth model filtering"
```

---

## Final Verification Gate

Run:

```bash
go test ./pkg/provider ./pkg/config ./cmd/installer ./cmd/smolbot ./internal/components/dialog ./internal/tui
go build ./cmd/smolbot ./cmd/smolbot-tui ./cmd/installer
```

Expected:

- all targeted package tests pass
- all three binaries build

Then run:

- final spec review on the whole change
- final code-quality review on the whole change

Only after both are green may the branch be pushed or merged.
