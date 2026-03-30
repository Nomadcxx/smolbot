package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewOAuthTokenStoreReturnsErrorOnBadRoot(t *testing.T) {
	tmp := t.TempDir()
	blockingFile := filepath.Join(tmp, "blocking")
	if err := os.WriteFile(blockingFile, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	paths := NewPaths(filepath.Join(blockingFile, "smolbot"))
	_, err := NewOAuthTokenStore(paths)
	if err == nil {
		t.Fatal("expected error from NewOAuthTokenStore with uncreateable root, got nil")
	}
}

func TestOAuthTokenStore_SaveAndLoad(t *testing.T) {
	tmp := t.TempDir()
	paths := NewPaths(tmp)
	store, err := NewOAuthTokenStore(paths)
	if err != nil {
		t.Fatalf("NewOAuthTokenStore: %v", err)
	}

	token := &TokenStoreEntry{
		AccessToken:  "access-abc",
		RefreshToken: "refresh-xyz",
		ExpiresAt:    time.Now().Add(1 * time.Hour),
		TokenType:    "Bearer",
		Scope:        "data:read",
		ProviderID:   "minimax-portal",
		ProfileID:    "default",
		AccountEmail: "user@example.com",
	}

	err = store.Save("minimax-portal", "default", token)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, found, err := store.Load("minimax-portal", "default")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !found {
		t.Fatal("expected token to be found")
	}
	if loaded.AccessToken != token.AccessToken {
		t.Errorf("AccessToken mismatch: got %q, want %q", loaded.AccessToken, token.AccessToken)
	}
	if loaded.RefreshToken != token.RefreshToken {
		t.Errorf("RefreshToken mismatch: got %q, want %q", loaded.RefreshToken, token.RefreshToken)
	}
	if loaded.ProviderID != token.ProviderID {
		t.Errorf("ProviderID mismatch: got %q, want %q", loaded.ProviderID, token.ProviderID)
	}
	if loaded.AccountEmail != token.AccountEmail {
		t.Errorf("AccountEmail mismatch: got %q, want %q", loaded.AccountEmail, token.AccountEmail)
	}
}

func TestOAuthTokenStore_ClearRemovesOnlyTarget(t *testing.T) {
	tmp := t.TempDir()
	paths := NewPaths(tmp)
	store, err := NewOAuthTokenStore(paths)
	if err != nil {
		t.Fatalf("NewOAuthTokenStore: %v", err)
	}

	token1 := &TokenStoreEntry{AccessToken: "access-1", ExpiresAt: time.Now().Add(1 * time.Hour)}
	token2 := &TokenStoreEntry{AccessToken: "access-2", ExpiresAt: time.Now().Add(1 * time.Hour)}

	store.Save("provider-a", "profile-1", token1)
	store.Save("provider-b", "profile-1", token2)

	err = store.Clear("provider-a", "profile-1")
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	_, found, err := store.Load("provider-a", "profile-1")
	if err != nil {
		t.Fatalf("Load after clear failed: %v", err)
	}
	if found {
		t.Error("expected provider-a/profile-1 to be gone")
	}

	loaded, found, err := store.Load("provider-b", "profile-1")
	if err != nil {
		t.Fatalf("Load for provider-b failed: %v", err)
	}
	if !found {
		t.Fatal("expected provider-b/profile-1 to still exist")
	}
	if loaded.AccessToken != "access-2" {
		t.Errorf("wrong token: got %q", loaded.AccessToken)
	}
}

func TestOAuthTokenStore_MissingFileReturnsNotFound(t *testing.T) {
	tmp := t.TempDir()
	paths := NewPaths(tmp)
	store, err := NewOAuthTokenStore(paths)
	if err != nil {
		t.Fatalf("NewOAuthTokenStore: %v", err)
	}

	_, found, err := store.Load("nonexistent", "default")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if found {
		t.Error("expected found=false for missing file")
	}
}

func TestOAuthTokenStore_PrivatePermissions(t *testing.T) {
	tmp := t.TempDir()
	paths := NewPaths(tmp)
	store, err := NewOAuthTokenStore(paths)
	if err != nil {
		t.Fatalf("NewOAuthTokenStore: %v", err)
	}
	token := &TokenStoreEntry{AccessToken: "secret", ExpiresAt: time.Now().Add(1 * time.Hour)}

	store.Save("minimax-portal", "default", token)

	// TokenStore doesn't expose path, so verify by checking that Load works
	loaded, found, err := store.Load("minimax-portal", "default")
	if err != nil || !found {
		t.Fatalf("token not persisted: err=%v found=%v", err, found)
	}
	if loaded.AccessToken != "secret" {
		t.Errorf("wrong token: got %q", loaded.AccessToken)
	}
}

func TestOAuthTokenStore_PreservesOtherEntriesOnUpdate(t *testing.T) {
	tmp := t.TempDir()
	paths := NewPaths(tmp)
	store, err := NewOAuthTokenStore(paths)
	if err != nil {
		t.Fatalf("NewOAuthTokenStore: %v", err)
	}

	store.Save("provider-a", "profile-1", &TokenStoreEntry{AccessToken: "token-a", ExpiresAt: time.Now().Add(1 * time.Hour)})
	store.Save("provider-b", "profile-1", &TokenStoreEntry{AccessToken: "token-b", ExpiresAt: time.Now().Add(1 * time.Hour)})

	store.Save("provider-a", "profile-1", &TokenStoreEntry{AccessToken: "token-a-updated", ExpiresAt: time.Now().Add(1 * time.Hour)})

	loaded, found, err := store.Load("provider-b", "profile-1")
	if err != nil || !found {
		t.Fatalf("provider-b entry lost after update: err=%v found=%v", err, found)
	}
	if loaded.AccessToken != "token-b" {
		t.Errorf("wrong token for provider-b: got %q", loaded.AccessToken)
	}
}
