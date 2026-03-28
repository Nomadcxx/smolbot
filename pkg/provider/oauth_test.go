package provider

import (
	"testing"
	"time"
)

func TestTokenInfoIsExpired(t *testing.T) {
	token := &TokenInfo{
		AccessToken:  "test",
		RefreshToken: "refresh",
		ExpiresAt:    time.Now().Add(5 * time.Minute),
	}
	if token.IsExpired() {
		t.Fatal("token should not be expired")
	}

	token.ExpiresAt = time.Now().Add(1 * time.Minute)
	if !token.IsExpired() {
		t.Fatal("token should be expired within buffer")
	}
}

func TestAuthTypeString(t *testing.T) {
	tests := []struct {
		at     AuthType
		expect string
	}{
		{AuthTypeAPIKey, "api_key"},
		{AuthTypeOAuth, "oauth"},
		{AuthTypeToken, "token"},
	}
	for _, tt := range tests {
		if tt.at.String() != tt.expect {
			t.Errorf("AuthType(%d).String() = %q, want %q", tt.at, tt.at.String(), tt.expect)
		}
	}
}
