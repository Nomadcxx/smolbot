package provider

import (
	"context"
	"time"
)

// OAuthConfig holds OAuth provider configuration
type OAuthConfig struct {
	BaseURL   string // e.g., "https://api.minimax.io"
	ClientID  string
	AuthURL   string // e.g., "/oauth/code"
	TokenURL  string // e.g., "/oauth/token"
	RevokeURL string // e.g., "/oauth/revoke"
	Scopes    []string
}

// TokenInfo represents an OAuth token with metadata
type TokenInfo struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"` // Unix timestamp ms
	TokenType    string    `json:"token_type"`
	Scope        string    `json:"scope"`
	ProviderID   string    `json:"providerId,omitempty"`
	ProfileID    string    `json:"profileId,omitempty"`
	AccountEmail string    `json:"accountEmail,omitempty"`
	AccountName  string    `json:"accountName,omitempty"`
	UpdatedAt    time.Time `json:"updatedAt,omitempty"`
}

// IsExpired returns true if the token is expired or will expire within 2 minutes
func (t *TokenInfo) IsExpired() bool {
	return time.Until(t.ExpiresAt) < 2*time.Minute
}

// OAuthProvider defines the interface for OAuth-based providers
type OAuthProvider interface {
	Provider
	AuthType() AuthType
	RefreshToken(ctx context.Context) (*TokenInfo, error)
	RevokeToken(ctx context.Context) error
	GetAuthConfig() OAuthConfig
}

// AuthType distinguishes key-based vs OAuth vs token-based auth
type AuthType int

const (
	AuthTypeAPIKey AuthType = iota
	AuthTypeOAuth
	AuthTypeToken
)

func (a AuthType) String() string {
	switch a {
	case AuthTypeAPIKey:
		return "api_key"
	case AuthTypeOAuth:
		return "oauth"
	case AuthTypeToken:
		return "token"
	default:
		return "unknown"
	}
}
