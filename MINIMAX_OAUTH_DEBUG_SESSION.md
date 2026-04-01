# MiniMax OAuth Debug Session Summary

## Problem Statement

When using MiniMax OAuth authentication in smolbot-tui, users receive a 401 error:
```
ERROR
openai stream http 401: login fail: Please carry the API secret key in the Authorization field
```

This occurs despite successful OAuth token retrieval (user confirms browser opened, authorization clicked, TUI advanced).

---

## Root Cause Analysis

### The Bug

The `detectProviderName` function in `pkg/provider/registry.go` incorrectly routes MiniMax model requests to the wrong provider:

1. **Expected behavior:** Model with prefix `minimax/` should route to `"minimax-portal"` (the OAuth provider)
2. **Actual behavior:** Model with prefix `minimax/` routes to `"minimax"` (non-OAuth factory with empty API key)

### Code Flow Analysis

#### Token Storage (Working Correctly)
**File:** `cmd/installer/oauth.go`
```go
// Token stored under: ["minimax-portal"]["minimax-portal:default"]
entry := map[string]any{
    "access_token":  tok.AccessToken,
    "refresh_token": tok.RefreshToken,
    "expires_at":    tok.ExpiresAt,
    "token_type":    tok.TokenType,
    "scope":         tok.Scope,
    "provider_id":   "minimax-portal",
    "profile_id":    "minimax-portal:default",
    "updated_at":    time.Now(),
}
```

#### Config Setup (Working Correctly)
**File:** `cmd/installer/tasks.go`
```go
case providerMiniMaxOAuth:
    providers["minimax-portal"] = map[string]interface{}{
        "authType":  "oauth",
        "profileId": "minimax-portal:default",
        "apiBase":   "https://api.minimax.io",
    }
```

#### Provider Registry (The Problem Location)
**File:** `pkg/provider/registry.go`

When `ForModelWithCtx` is called:
1. `resolveProvider(model)` is called
2. `detectProviderName()` returns `"minimax"` (wrong!)
3. Registry checks `cfg.Providers["minimax"]` - not configured as OAuth
4. Falls back to `factories["minimax"]` which creates `NewOpenAIProvider("minimax", "", ...)`
5. Empty API key → Empty Authorization header → 401 error

### Why detectProviderName Fails

Current logic in `detectProviderName`:
```go
// This checks factory names (including "minimax")
for _, name := range names {
    lowerName := strings.ToLower(name)
    if strings.HasPrefix(lowerModel, lowerName+"/") {
        return name
    }
}
```

The model `minimax/ChatCompletion` has prefix `minimax/` which matches:
1. Factory `"minimax"` (registered in `NewRegistryWithOAuthStore`)
2. **BEFORE** checking OAuth providers or config

The OAuth factory `"minimax-portal"` is in a separate `oauthFactories` map, but `detectProviderName` doesn't prioritize OAuth-configured providers.

---

## Failed Fix Attempts

### Attempt 1: Two-Pass Prefix Matching
**Claimed:** Modified `detectProviderName` to check exact prefix matches first before substring matches.
**Reality:** No actual edit was made to the file.

### Attempt 2: OAuth Provider Priority
**Claimed:** Added check for `AuthType == "oauth"` providers at the start of `detectProviderName`.
**Reality:** No actual edit was made to the file.

---

## Required Fix

The `detectProviderName` function needs to:

1. **Check OAuth-configured providers FIRST** - before checking factory names
2. **Match provider name against model prefix** - if `providers["minimax-portal"].AuthType == "oauth"` and model starts with `"minimax-portal/"`, return `"minimax-portal"`

### Proposed Implementation

```go
func detectProviderName(model, fallback string, providers map[string]config.ProviderConfig, factories map[string]ProviderFactory, oauthFactories map[string]func(cfg OAuthConfig) OAuthProvider) string {
    lowerModel := strings.ToLower(strings.TrimSpace(model))

    // PRIORITY 1: Check OAuth-configured providers first
    for providerName, providerConfig := range providers {
        if providerConfig.AuthType == "oauth" {
            if strings.HasPrefix(lowerModel, strings.ToLower(providerName)+"/") {
                return providerName
            }
        }
    }

    // PRIORITY 2: Hardcoded prefixes (claude, gpt, azure)
    if strings.HasPrefix(lowerModel, "claude-") || strings.HasPrefix(lowerModel, "anthropic/") {
        return "anthropic"
    }
    if strings.HasPrefix(lowerModel, "gpt-") || strings.HasPrefix(lowerModel, "openai/") {
        return "openai"
    }
    if strings.HasPrefix(lowerModel, "azure/") {
        return "azure_openai"
    }

    // PRIORITY 3: Factory name matches (exact prefix)
    names := make([]string, 0, len(factories))
    for name := range factories {
        if name == "anthropic" || name == "azure_openai" || name == "openai" {
            continue
        }
        names = append(names, name)
    }
    // ... rest of existing logic
}
```

---

## Key Files

| File | Purpose |
|------|---------|
| `pkg/provider/registry.go` | Provider resolution - **NEEDS FIX in detectProviderName** |
| `pkg/provider/minimax_oauth.go` | MiniMax OAuth provider implementation |
| `cmd/installer/oauth.go` | OAuth token storage during installer flow |
| `cmd/smolbot/runtime.go` | Wires OAuthTokenStore into provider registry |
| `pkg/config/oauth_store.go` | Token persistence layer |

---

## Verification Steps After Fix

1. Build gateway: `go build -o gateway ./cmd/smolbot`
2. Start gateway: `./gateway --port 18791`
3. Start TUI: `smolbot-tui`
4. Configure MiniMax as OAuth provider (if not already)
5. Send test message
6. Verify: Should see valid Bearer token in Authorization header, not empty
7. Expected: 200 response from MiniMax API

---

## Summary

The OAuth token flow is correctly implemented (storage, loading, token attachment). The bug is purely in **provider name detection** - the model prefix `minimax/` incorrectly resolves to `"minimax"` factory instead of `"minimax-portal"` OAuth provider.

**Fix location:** `pkg/provider/registry.go:232` (detectProviderName function)

**Fix needed:** Check `providers` map for OAuth-configured providers before matching factory names.

---

## Fix Attempt Result

**The fix was applied but did NOT resolve the issue.**

After modifying `detectProviderName` to check OAuth-configured providers first, the error changed from 401 to:

```
ERROR
openai stream http 404: <html>
<head><title>404 Not Found</title></head>
<body>
<center><h1>404 Not Found</h1></center>
<hr><center>nginx</center>
</body>
</html>
```

This indicates:
1. The provider name detection fix **may or may not be working** (could still be routing incorrectly)
2. The 404 from nginx suggests the **API endpoint URL is wrong**
3. MiniMax OAuth provider may be using incorrect base URL for chat completions

**Current config shows:**
- Model: `minimax-portal/MiniMax-M2.7`
- Provider: `minimax-portal` with `apiBase: "https://api.minimax.io"`
- Auth configured as OAuth

**New hypothesis:** The OAuth provider may be passing requests to wrong endpoint, or the OpenAI-compatible wrapper is using wrong path for MiniMax API.

---

## Next Steps to Investigate

1. **Verify provider resolution:** Add debug logging to `resolveProvider` to confirm `"minimax-portal"` is being returned
2. **Check API endpoint:** MiniMax OpenAI-compatible endpoint may need `/v1/chat/completions` path
3. **Verify BaseURL propagation:** Ensure `https://api.minimax.io` is correctly passed to OpenAI wrapper
4. **MiniMax API docs:** Check if OAuth tokens require different endpoint than API keys

---

## Session Date

2026-03-30