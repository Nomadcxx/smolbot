# MiniMax OAuth Implementation - Handover

## What Was Done

This worktree (`provider-ux-minimax`) implemented provider UX improvements for MiniMax. All 7 original tasks are complete. See commit history:

```
0f5558a docs(provider): capture oauth follow-up design
3b601ce test(provider): add end-to-end provider regressions
ab89b1a feat(provider): add minimax configuration support
84ee4d9 feat(tui): improve provider detail dialog
24a571e feat(tui): upgrade model picker workflow
```

### Task 4: Provider Detail Surface (COMPLETE)
- **File:** `internal/components/dialog/providers.go`
- Rewrote `ProvidersModel` using structured `ProviderInfo` → `providerRenderRow` data model
- `NewProvidersFromData()` constructor takes raw model list, current model, status, and config
- Renders three sections: **Active**, **Configured**, **Not Configured**
- Active provider shows: name, type, current model, API Base, Auth status
- Auth status shows "Configured" vs "Not configured"
- **File:** `internal/components/dialog/providers_test.go` - Tests for all rendering cases
- **File:** `internal/tui/tui.go` - Updated `ProvidersLoadedMsg` to use `NewProvidersFromData` instead of `buildProviderLines`
- Removed dead helpers from tui.go: `buildProviderLines`, `firstNonEmptyString`, `providerNameForModel`, `sortStrings`
- **File:** `internal/tui/tui_test.go` - Updated `TestHandleSlashCommandProvidersShowsCurrentProviderConfig` to match new structured format

### Task 5: MiniMax First-Class Support (COMPLETE)
- **File:** `cmd/installer/types.go` - Added `providerMiniMax = "minimax"` const, added to providers list, renamed `providerAzure` const from `"azure"` to `"azure_openai"`
- **File:** `cmd/installer/views.go` - Added MiniMax to the provider selection menu (5th option)
- **File:** `cmd/installer/tasks.go` - Added `case providerMiniMax:` to write `providers["minimax"]` with API key; renamed `providerAzure` case to use `"azure_openai"` key
- **File:** `cmd/installer/utils.go` - Added `providerMiniMax` to API key validation (requires `m.apiKey != ""`)
- **File:** `pkg/provider/registry.go` - Added default API base `https://api.minimax.io/v1` for minimax in the OpenAI-compatible provider factory loop

### Task 6: Provider E2E Tests (COMPLETE)
- **File:** `internal/tui/provider_flow_test.go` - 4 regression tests:
  - `TestProviderDialogShowsActiveConfiguredUnconfiguredProviders`
  - `TestProviderDialogShowsMiniMaxWithCorrectType`
  - `TestModelPickerGroupsByProviderWithCurrentModelHighlighted`
  - `TestModelPickerSkipsConfigOnlyRowsWhenNavigating`

### Task 7: OAuth Design (COMPLETE)
- **File:** `docs/plans/2026-03-28-openai-codex-oauth-design.md` - Captures design direction for future OAuth implementation. Notes Codex uses different auth than standard OpenAI API. Contains implementation checklist.

---

## What Still Needs To Be Done

### OAuth Implementation (8 tasks planned, 0 completed)

The OAuth implementation was started in a separate session but **nothing was successfully committed**. A subagent failed to create the files. The following files **do not exist** and need to be created:

#### Task 1: OAuth Types and Interfaces (NOT DONE)
**Create:** `pkg/provider/oauth.go`
```go
// AuthType enum: APIKey, OAuth, Token
// TokenInfo struct: AccessToken, RefreshToken, ExpiresAt, Scope
// OAuthProvider interface: AuthType(), RefreshToken(), RevokeToken()
// OAuthConfig struct for MiniMax: ClientID, ClientSecret, AuthURL, TokenURL, RedirectURI, Scopes
```

**Create:** `pkg/provider/oauth_test.go`
- Test `TokenInfo.IsExpired()` with buffer
- Test `AuthType.String()`

#### Task 2: PKCE Utilities (NOT DONE - code exists but was not committed)
**Create:** `pkg/provider/pkce.go`
- `GeneratePKCE()` - returns verifier (32+ bytes base64url) and challenge (S256 hash)
- `BuildAuthURL()` - constructs device code auth URL
- `ExchangeCodeForToken()` - POST to token endpoint
- MiniMax config:
  - Global: `baseUrl: "https://api.minimax.io"`, `clientId: "78257093-7e40-4613-99e0-527b14b39113"`
  - CN: `baseUrl: "https://api.minimaxi.com"`, same `clientId`
- OAuth endpoints: `${baseUrl}/oauth/code`, `${baseUrl}/oauth/token`
- Device code grant type: `"urn:ietf:params:oauth:grant-type:device_code"`

#### Task 3: MiniMax OAuth Provider (NOT DONE)
**Create:** `pkg/provider/minimax_oauth.go`
- `MiniMaxOAuthProvider` struct implementing `OAuthProvider` interface
- `NewMiniMaxOAuthProvider(config OAuthConfig)` constructor
- Device code flow: `Authenticate()` → show user code + URL, poll for token
- `RefreshToken()` - refresh before expiry
- `RevokeToken()` - revoke refresh token
- Stores tokens in `TokenInfo`

#### Task 4: OAuth Token Storage (NOT DONE)
**Create:** `pkg/config/oauth_store.go`
- Store path: `~/.config/smolbot/oauth_tokens.json`
- `OAuthTokenStore` struct with file lock
- `Save(provider, profileId, tokenInfo)` 
- `Load(provider, profileId) (TokenInfo, error)`
- `Clear(provider, profileId)`
- Profile ID format: `"minimax-portal:default"`

#### Task 5: Register MiniMax OAuth in Registry (NOT DONE)
**Modify:** `pkg/provider/registry.go`
- Add `RegisterOAuthProvider(name string, factory OAuthProviderFactory)`
- Register `"minimax-oauth"` provider factory
- When `ProviderConfig.AuthType == "oauth"`, use OAuth provider instead of API key provider

#### Task 6: Installer OAuth Step (NOT DONE)
**Modify:** `cmd/installer/`
- Add `stepMiniMaxOAuth` to install steps
- Add `"MiniMax OAuth"` to provider list in views.go
- Device code display: show user code + verification URL
- Poll for token, on success save to config
- Update `tasks.go` to write `authType: "oauth"` in provider config

#### Task 7: TUI OAuth Status Display (NOT DONE)
**Modify:** `internal/components/dialog/providers.go`
- Add `"OAuth"` auth status display in Active section
- When `ProviderConfig.AuthType == "oauth"`, show "OAuth" instead of "API Key"
- Add OAuth-specific fields to `ProviderInfo`: `AuthType`, `Email`, `ExpiresAt`

#### Task 8: Model Picker OAuth Model Filter (NOT DONE)
**Modify:** `internal/components/dialog/models.go`
- When a model requires OAuth but OAuth isn't configured, show "(OAuth required)" hint
- Filter models by `ModelInfo.AuthRequired` flag if added

---

## Key File Locations

```
pkg/provider/
  registry.go          # Provider factories, default API bases
  openai.go            # OpenAI-compatible provider (used by minimax)
  azure.go             # Azure OpenAI provider
  minimax_oauth.go     # NEEDS TO BE CREATED - OAuth provider
  oauth.go             # NEEDS TO BE CREATED - types/interfaces
  pkce.go              # NEEDS TO BE CREATED - PKCE utilities

pkg/config/
  config.go            # Config struct, ProviderConfig, Load, Save
  oauth_store.go        # NEEDS TO BE CREATED - OAuth token storage

cmd/installer/
  types.go             # Provider constants (providerMiniMax, providerAzure="azure_openai")
  views.go             # Provider selection menu rendering
  tasks.go             # Config writing per provider
  utils.go             # Validation logic

internal/components/dialog/
  providers.go         # Provider detail dialog with structured ProviderInfo
  providers_test.go    # Provider rendering tests
  models.go            # Model picker dialog

internal/tui/
  tui.go               # Handles /providers command, calls NewProvidersFromData
  tui_test.go          # TUI tests
  provider_flow_test.go # Provider E2E tests

docs/plans/
  2026-03-28-openai-codex-oauth-design.md  # OAuth design direction
  2026-03-29-handover-oauth-implementation.md  # This file
```

---

## About the Error You Saw

`invalid params, invalid chat setting (2013)` is a **MiniMax API error**, not a code bug in smolbot. It means:

1. **You tried to use `MiniMax-M2.7-highspeed`** via API key, but this model requires Pay-As-You-Go billing (not Token Plan)
2. **Or** smolbot sent a parameter (like `max_tokens` or `temperature`) that `MiniMax-M2.7` doesn't support

**Fix:** Use `MiniMax-M2.7` (not `-highspeed`) for Token Plan accounts. The provider detail dialog should show which models are available. OpenClaw's MiniMax provider has logic to strip unsupported params - you may need similar handling in `pkg/provider/openai.go` when calling MiniMax.

---

## Current Branch Status

```
Branch: feat/provider-ux-minimax
Worktree: .worktrees/provider-ux-minimax
Last commit: 0f5558a (docs: oauth follow-up design)
Build: PASSING
Tests: PASSING (all affected packages)
```

To continue OAuth work, start a new session in the `provider-ux-minimax` worktree and use the `docs/plans/2026-03-29-minimax-oauth-implementation.md` plan (if it was saved - if not, use this handover as the reference).
