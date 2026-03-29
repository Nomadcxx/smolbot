# OpenAI Codex/ChatGPT OAuth Design

> **Status:** Design-only handoff. Implementation planned separately.

## Context

smolbot currently supports API-key-based authentication for providers. This design captures the correct future direction for OAuth-based provider authentication, informed by OpenClaw's MiniMax OAuth implementation.

## How OpenClaw Does MiniMax OAuth

Based on `/home/nomadx/openclaw/extensions/minimax/`:

### Device Code Flow (RFC 8628)

OpenClaw uses the **device code flow** - ideal for CLI tools:

1. `POST /oauth/code` with `client_id` → returns `device_code`, `user_code`, `verification_uri`, `interval`, `expires_in`
2. User opens `verification_uri` in browser, enters `user_code`
3. Poll `POST /oauth/token` with `grant_type: urn:ietf:params:oauth:grant-type:device_code` + `device_code`
4. When user authorizes, response includes `access_token`, `refresh_token`, `expires_in`

### OAuth Configuration (from OpenClaw)

```typescript
const MINIMAX_OAUTH_CONFIG = {
  global: {
    baseUrl: "https://api.minimax.io",
    clientId: "78257093-7e40-4613-99e0-527b14b39113",
  },
};
```

### Token Storage (from OpenClaw)

Two storage locations:
1. **Auth profiles store** (`~/.openclaw/auth-profiles.json`) - main store
2. **External CLI sync** (`~/.minimax/oauth_creds.json`) - compatibility with MiniMax CLI

Profile ID format: `provider:profile-name` (e.g., `minimax-portal:default`)

### Token Response

```json
{
  "access_token": "...",
  "refresh_token": "...",
  "expires_in": 3600,
  "token_type": "Bearer",
  "scope": ""
}
```

### Auth Method Kinds (from OpenClaw plugin-sdk)

```typescript
type ProviderAuthKind = "oauth" | "api_key" | "token" | "device_code" | "custom";
```

MiniMax uses `device_code` kind.

## smolbot OAuth Architecture

### 1. AuthType Enum

```go
const (
    AuthTypeAPIKey AuthType = iota
    AuthTypeOAuth
    AuthTypeToken
)
```

### 2. OAuthProvider Interface

```go
type OAuthProvider interface {
    Provider
    AuthType() AuthType
    InitiateDeviceCode(ctx context.Context) (*DeviceCodeResponse, error)
    PollForToken(ctx context.Context, dc *DeviceCodeResponse) (*TokenInfo, error)
    RefreshToken(ctx context.Context) (*TokenInfo, error)
    RevokeToken(ctx context.Context) error
    GetAuthConfig() OAuthConfig
}
```

### 3. Token Storage

`pkg/config/oauth_store.go` - persistent JSON storage with:
- `access_token`, `refresh_token`, `expires_at` (Unix ms)
- `provider`, `profile_id`, `email`, `updated_at`
- Atomic save (write to tmp, rename)
- File locking for concurrent access

### 4. MiniMax Provider

`pkg/provider/minimax_oauth.go` - implements OAuthProvider using OpenAI-compatible chat:
- Inherits from `OpenAIProvider` with OAuth token as API key
- Device code flow: `InitiateDeviceCode()` → `PollForToken()`
- Token refresh via `/oauth/token` with `grant_type: refresh_token`

### 5. Provider Registry

`minimax-oauth` factory registered alongside `minimax` (API key):
- If OAuth token exists in store, load it
- Otherwise, require OAuth flow

## Billing Model Distinction

From MiniMax docs:
- **Token Plan** → OAuth flow (account tied to subscription)
- **Pay-As-You-Go** → Direct API key

Model availability differs:
- `MiniMax-M2.7` → available on Token Plan
- `MiniMax-M2.7-highspeed` → Pay-As-You-Go only

This means model picker must filter based on auth type.

## Design Checklist

- [x] Define `AuthType` enum with `APIKey`, `OAuth`, `Token` variants
- [x] Add `TokenInfo` and `TokenEntry` structs with expiry metadata
- [x] Create `OAuthTokenStore` with atomic save and file locking
- [x] Implement `MiniMaxOAuthProvider` with device code flow
- [ ] Extend `ProviderConfig` to support `AuthType` field
- [ ] Add `minimax-oauth` provider factory in registry
- [ ] Add OAuth flow to installer wizard (device code + browser)
- [ ] Add OAuth status display in TUI `/providers`
- [ ] Filter highspeed models for Token Plan OAuth

## References

- OpenClaw MiniMax plugin: `/home/nomadx/openclaw/extensions/minimax/`
- OAuth 2.0 RFC 8628 (device flow): https://datatracker.ietf.org/doc/html/rfc8628
- MiniMax OAuth endpoints: `https://api.minimax.io/oauth/code`, `https://api.minimax.io/oauth/token`
