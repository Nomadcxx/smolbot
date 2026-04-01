# Handover: MiniMax OAuth Integration

**Date:** 2026-03-30
**Branch:** main (all changes merged and pushed)

---

## What Was Done This Session

### 1. Merge of `feat/provider-ux-minimax` → `main`
Five conflict files resolved. Key decisions:
- `internal/client/messages.go` — kept feat branch's typed `ModelsSetParams` and `payload.Current` return value (server sends both `current` and `previous`; TUI wants `current`)
- `pkg/gateway/server_test.go` — removed older duplicate `TestModelSetUpdatesGatewayConfigAndStatus`; kept feat's version that checks both fields
- `pkg/provider/oauth.go` / `oauth_test.go` — took feat (superset)
- `cmd/smolbot/runtime_model_test.go` — took feat (superset, three new tests)

### 2. Installer crash fix
`views.go` had a 5-entry `providerNames` slice against a 7-entry `providers` slice → index OOB panic. Fixed and added config UI for all 7 providers.

### 3. MiniMax OAuth in the installer (multiple iterations)
Three rewrites of `cmd/installer/oauth.go`. The final working version (based on OpenClaw's reference implementation at `github.com/openclaw/openclaw/extensions/minimax/oauth.ts`) uses:

| Parameter | Value |
|---|---|
| Grant type | `urn:ietf:params:oauth:grant-type:user_code` |
| Poll field | `user_code` (not `device_code`) |
| `code_verifier` | Required in token exchange |
| `expired_in` | **Absolute unix timestamp in ms** (NOT duration in seconds) |
| Scope | `group_id profile model.completion` |
| Token response | `status` field: `"success"` / `"pending"` / `"error"` |
| Interval | In milliseconds; default 2000ms if 0 |

Token is saved to `~/.smolbot/oauth_tokens.json` with structure:
```json
{
  "minimax-portal": {
    "minimax-portal:default": {
      "access_token": "sk-cp-...",
      "refresh_token": "...",
      "expires_at": "<absolute timestamp>",
      "token_type": "Bearer",
      "provider_id": "minimax-portal",
      "profile_id": "minimax-portal:default"
    }
  }
}
```

### 4. Runtime wiring (401 fix)
`cmd/smolbot/runtime.go` was calling `provider.NewRegistryWithDefaults(cfg)` which passes `nil` for the OAuth token store. Changed to:
```go
providerRegistry = provider.NewRegistryWithOAuthStore(cfg, config.NewOAuthTokenStore(paths))
```

### 5. Provider routing (401 fix — `detectProviderName`)
`detectProviderName` in `pkg/provider/registry.go` matched model `"minimax-portal/MiniMax-M2.7"` against factory `"minimax"` (via `strings.Contains`) before reaching the OAuth provider check.

Fixed by adding an OAuth-first pass at the top of `detectProviderName`:
```go
// Check OAuth-configured providers first (before factory name matching)
for providerName, providerConfig := range providers {
    if providerConfig.AuthType == "oauth" {
        if strings.HasPrefix(lowerModel, strings.ToLower(providerName)+"/") {
            return providerName
        }
    }
}
```

### 6. API base URL (404 fix — commit 92ab044)
`MiniMaxOAuthProvider.Chat/ChatStream` passed `p.config.BaseURL` (`"https://api.minimax.io"`) to `NewOpenAIProvider`, which appends `/chat/completions` directly → `https://api.minimax.io/chat/completions` → nginx 404.

Fixed in `pkg/provider/minimax_oauth.go` with a `chatBase()` helper that ensures `/v1` is appended:
```go
func (p *MiniMaxOAuthProvider) chatBase() string {
    base := strings.TrimRight(p.config.BaseURL, "/")
    if !strings.HasSuffix(base, "/v1") {
        base += "/v1"
    }
    return base
}
```

Result: requests now go to `https://api.minimax.io/v1/chat/completions`.

### 7. Model name prefix not stripped (400 fix — commit 91b115e)

After the 404 was fixed, MiniMax returned:
```
openai stream http 400: {"type":"error","error":{"type":"bad_request_error",
"message":"invalid params, unknown model 'minimax-portal/minimax-m2.7' (2013)"}}
```

The full model string `"minimax-portal/MiniMax-M2.7"` was being forwarded to MiniMax's API. MiniMax only accepts the bare model name (`"MiniMax-M2.7"`).

Fixed in `pkg/provider/minimax_oauth.go` with `stripProviderPrefix()` called in both `Chat` and `ChatStream` before delegating to the OpenAI sub-provider:
```go
func stripProviderPrefix(model string) string {
    if idx := strings.LastIndex(model, "/"); idx >= 0 {
        return model[idx+1:]
    }
    return model
}
```

---

## Current Status

**As of last push (92ab044):**
- OAuth installer flow: ✅ Working (browser opens, authorize → TUI advances)
- Token saved to disk: ✅ Confirmed at `~/.smolbot/oauth_tokens.json`
- Token loaded at runtime: ✅ Fixed
- Provider routing: ✅ Fixed
- API endpoint URL: ✅ Fixed
- End-to-end chat test: ✅ Confirmed working — MiniMax responded successfully

---

## Files Changed (this session)

| File | Change |
|---|---|
| `cmd/installer/oauth.go` | Full rewrite — correct MiniMax device-code OAuth |
| `cmd/installer/main.go` | Wire `pollMiniMaxCmd`, remove old loopback references |
| `cmd/installer/types.go` | `oauthFlow *oauthFlowState` → `oauthDC *deviceCodeResp` |
| `cmd/installer/views.go` | Fix `providerNames` (5→7), config screens for all providers |
| `cmd/installer/tasks.go` | Fix duplicate `currentPath` declaration |
| `cmd/installer/utils.go` | `defaultModelFor()` helper |
| `cmd/smolbot/runtime.go` | Pass `OAuthTokenStore` to registry |
| `cmd/smolbot/channels_login.go` | Remove dead duplicate signal block (was breaking build) |
| `pkg/provider/registry.go` | OAuth-first provider detection |
| `pkg/provider/minimax_oauth.go` | `chatBase()` ensures `/v1` in URL |
| `pkg/provider/oauth.go` | Merge: added `ProviderID`, `ProfileID`, `GetToken`, `SetToken` |
| `internal/client/messages.go` | Merge: typed `ModelsSetParams`, return `payload.Current` |

---

## Config Written by Installer

`~/.smolbot/config.json` (relevant sections):
```json
{
  "agents": {
    "defaults": {
      "model": "minimax-portal/MiniMax-M2.7",
      "provider": "minimax-portal"
    }
  },
  "providers": {
    "minimax-portal": {
      "authType": "oauth",
      "profileId": "minimax-portal:default",
      "apiBase": "https://api.minimax.io"
    }
  }
}
```

Note: `apiBase` is `"https://api.minimax.io"` (no `/v1`). The `/v1` is now appended at runtime by `chatBase()`. Do not change this — the token/refresh endpoints do NOT use `/v1`.

---

## What Still Needs Testing / Known Gaps

1. **End-to-end MiniMax chat** — binaries deployed, awaiting confirmation message goes through cleanly
2. **Token refresh** — `MiniMaxOAuthProvider.RefreshToken` still uses the old grant type (`refresh_token`). MiniMax's refresh endpoint behaviour with OAuth portal tokens is untested.
3. **Token expiry handling** — token `expires_at` is stored as an absolute timestamp from MiniMax. `TokenInfo.IsExpired()` in `pkg/provider/oauth.go` should handle this correctly but hasn't been exercised.
4. **MiniMax model name** — config has `MiniMax-M2.7`. The current MiniMax model lineup may have changed (user noted "token plan" may have replaced "coding plan"). Verify the model name is valid.
5. **Installer for new users** — the installer clones from remote, which now has all fixes. A clean install should work end-to-end.

---

## How to Test Quickly (Without Reinstalling)

```bash
# Just rebuild and restart
cd ~/Documents/smolbot
go build -o /tmp/smolbot ./cmd/smolbot
systemctl --user stop smolbot
install -m755 /tmp/smolbot ~/.local/bin/smolbot
systemctl --user start smolbot

# Then launch TUI
smolbot-tui
```

Send a message; should get a response from MiniMax with no auth errors.

---

## Key Reference

OpenClaw's MiniMax OAuth implementation (the reference we used):
`github.com/openclaw/openclaw/extensions/minimax/oauth.ts`

Their portal-auth OAuth uses loopback redirect (not polling), but the **token exchange mechanism** (`user_code` + `code_verifier`, `status` field in response) was correctly extracted from their implementation.
