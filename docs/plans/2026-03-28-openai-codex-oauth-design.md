# OpenAI Codex/ChatGPT OAuth Design

> **Status:** Design-only handoff. Do not implement in this branch.

## Context

smolbot currently supports API-key-based authentication for providers. This design captures the correct future direction for OpenAI/Codex-style sign-in, which is fundamentally different from API-key authentication.

## Current Provider Auth Limitations

1. **API key storage**: All providers store raw API keys in `~/.config/smolbot/config.json`
2. **No token refresh**: Static keys with no OAuth refresh flow
3. **No interactive browser flow**: Users cannot sign in via OAuth 2.0 device or browser flows
4. **Codex/ChatGPT distinction**: Codex uses different auth than standard OpenAI API keys
5. **Session management**: No concept of short-lived OAuth tokens vs long-lived API keys

## Comparison: Crush OAuth Token Model

Crush implements OAuth token management with:
- Token storage with expiry metadata
- Device code + browser flow for initial auth
- Automatic token refresh before expiry
- Provider capability flags to distinguish OAuth providers from key-based ones

## Codex/ChatGPT Sign-In Is Not Generic

Key differences from standard OpenAI API:
- **Codex** requires `client_id` + `client_secret` + OAuth scope for `codex-installer`
- **ChatGPT** requires separate consumer OAuth vs enterprise OAuth
- **Redirect URIs** must be registered and validated
- **Token exchange** involves `code` exchange, not direct API keys

## Minimum Future Auth Subsystem Shape

```go
// Provider capability flags
const (
    AuthTypeAPIKey  AuthType = iota  // static key
    AuthTypeOAuth                    // OAuth 2.0 device/browser flow
    AuthTypeToken                    // short-lived token with refresh
)

// Token metadata
type TokenInfo struct {
    AccessToken  string
    RefreshToken string
    ExpiresAt   time.Time
    Scope       string
}

// Provider auth interface
type AuthProvider interface {
    AuthType() AuthType
    RefreshToken(ctx context.Context) (*TokenInfo, error)
    RevokeToken(ctx context.Context) error
}
```

## Implementation Scoping (Out of Scope for This Branch)

1. OAuth 2.0 device authorization flow (`/device/code`, `/device/token`)
2. Browser-based authorization code flow with PKCE
3. Token storage encryption at rest
4. Provider capability registry for OAuth vs key distinction
5. Interactive installer integration for OAuth sign-in

## Design Checklist Before Implementation

- [ ] Define `AuthType` enum with `APIKey`, `OAuth`, `Token` variants
- [ ] Add `TokenInfo` struct with expiry metadata
- [ ] Extend `ProviderConfig` to support `AuthType` field
- [ ] Add `oauth` provider factory in registry
- [ ] Create `OAuthProvider` implementing device code + token refresh
- [ ] Add encrypted token storage to config subsystem
- [ ] Add OAuth flow to installer wizard (new `stepOAuth`?)
- [ ] Define Codex-specific `client_id` and scope requirements
- [ ] Add provider capability flags for UI rendering differences

## References

- OpenAI device flow: https://devices.flows.master.opeai.com/docs
- Codex auth: https://platform.codex.com/overview/authentication
- OAuth 2.0 RFC 8628 (device flow): https://datatracker.ietf.org/doc/html/rfc8628
- Crush auth model: `/home/nomadx/crush/internal/config/config.go` (for reference)
