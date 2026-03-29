package provider

import (
	"encoding/json"
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

func TestTokenInfoOAuthJSONRoundTrip(t *testing.T) {
	updatedAt := time.Date(2026, 3, 29, 12, 30, 0, 0, time.UTC)
	expiresAt := time.Date(2026, 3, 29, 14, 30, 0, 0, time.UTC)
	input := TokenInfo{
		AccessToken:  "access-token",
		RefreshToken: "refresh-token",
		ExpiresAt:    expiresAt,
		TokenType:    "Bearer",
		Scope:        "profile.read",
		ProviderID:   "minimax-portal",
		ProfileID:    "profile-123",
		AccountEmail: "user@example.com",
		AccountName:  "Example User",
		UpdatedAt:    updatedAt,
	}

	data, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got TokenInfo
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.ProviderID != input.ProviderID {
		t.Fatalf("ProviderID = %q, want %q", got.ProviderID, input.ProviderID)
	}
	if got.ProfileID != input.ProfileID {
		t.Fatalf("ProfileID = %q, want %q", got.ProfileID, input.ProfileID)
	}
	if got.AccountEmail != input.AccountEmail {
		t.Fatalf("AccountEmail = %q, want %q", got.AccountEmail, input.AccountEmail)
	}
	if got.AccountName != input.AccountName {
		t.Fatalf("AccountName = %q, want %q", got.AccountName, input.AccountName)
	}
	if !got.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("UpdatedAt = %v, want %v", got.UpdatedAt, updatedAt)
	}
	if !got.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("ExpiresAt = %v, want %v", got.ExpiresAt, expiresAt)
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
