package usage

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeOllamaMeSigner struct {
	t             *testing.T
	wantChallenge string
	token         string
	err           error
	calls         int
}

func (s *fakeOllamaMeSigner) Sign(ctx context.Context, challenge []byte) (string, error) {
	s.calls++
	if s.wantChallenge != "" && string(challenge) != s.wantChallenge {
		s.t.Fatalf("challenge = %q, want %q", string(challenge), s.wantChallenge)
	}
	return s.token, s.err
}

func TestProbeOllamaMeAuthenticatedPopulated(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 28, 12, 34, 56, 0, time.UTC)
	signer := &fakeOllamaMeSigner{
		t:             t,
		wantChallenge: "POST,/api/me?ts=1774701296",
		token:         "pubkey:signature",
	}

	var gotAuth, gotTS string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/api/me" {
			t.Fatalf("path = %s, want /api/me", r.URL.Path)
		}
		gotTS = r.URL.Query().Get("ts")
		gotAuth = r.Header.Get("Authorization")
		if gotAuth != "pubkey:signature" {
			t.Fatalf("authorization = %q, want %q", gotAuth, "pubkey:signature")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Name":"nomadxxx","Email":"lukegiles32@protonmail.com","Plan":"pro","NotifyUsageLimits":true}`))
	}))
	defer server.Close()

	got, err := ProbeOllamaMe(context.Background(), server.URL, server.Client(), signer, func() time.Time { return now })
	if err != nil {
		t.Fatalf("ProbeOllamaMe: %v", err)
	}
	if signer.calls != 1 {
		t.Fatalf("signer calls = %d, want 1", signer.calls)
	}
	if gotTS != "1774701296" {
		t.Fatalf("ts = %q, want 1774701296", gotTS)
	}
	if got.IdentityState != QuotaIdentityStateAuthenticated {
		t.Fatalf("IdentityState = %q, want %q", got.IdentityState, QuotaIdentityStateAuthenticated)
	}
	if got.AccountName != "nomadxxx" || got.AccountEmail != "lukegiles32@protonmail.com" || got.PlanName != "pro" {
		t.Fatalf("result = %+v", got)
	}
	if !got.NotifyUsageLimits {
		t.Fatal("NotifyUsageLimits = false, want true")
	}
	if !got.AccountMetadataPopulated {
		t.Fatal("AccountMetadataPopulated = false, want true")
	}
}

func TestProbeOllamaMeAuthenticatedEmpty(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 28, 12, 34, 56, 0, time.UTC)
	signer := &fakeOllamaMeSigner{
		t:             t,
		wantChallenge: "POST,/api/me?ts=1774701296",
		token:         "pubkey:signature",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer server.Close()

	got, err := ProbeOllamaMe(context.Background(), server.URL, server.Client(), signer, func() time.Time { return now })
	if err != nil {
		t.Fatalf("ProbeOllamaMe: %v", err)
	}
	if got.IdentityState != QuotaIdentityStateAuthenticatedEmpty {
		t.Fatalf("IdentityState = %q, want %q", got.IdentityState, QuotaIdentityStateAuthenticatedEmpty)
	}
	if got.AccountMetadataPopulated {
		t.Fatal("AccountMetadataPopulated = true, want false")
	}
}

func TestProbeOllamaMeUnauthenticated(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 28, 12, 34, 56, 0, time.UTC)
	signer := &fakeOllamaMeSigner{
		t:             t,
		wantChallenge: "POST,/api/me?ts=1774701296",
		token:         "pubkey:signature",
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid credentials"}`))
	}))
	defer server.Close()

	got, err := ProbeOllamaMe(context.Background(), server.URL, server.Client(), signer, func() time.Time { return now })
	if err != nil {
		t.Fatalf("ProbeOllamaMe: %v", err)
	}
	if got.IdentityState != QuotaIdentityStateUnauthenticated {
		t.Fatalf("IdentityState = %q, want %q", got.IdentityState, QuotaIdentityStateUnauthenticated)
	}
}

func TestProbeOllamaMeSignerError(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("signer failed")
	signer := &fakeOllamaMeSigner{
		t:             t,
		wantChallenge: "POST,/api/me?ts=1774701296",
		err:           wantErr,
	}

	got, err := ProbeOllamaMe(context.Background(), "https://ollama.com", http.DefaultClient, signer, func() time.Time {
		return time.Date(2026, 3, 28, 12, 34, 56, 0, time.UTC)
	})
	if err == nil {
		t.Fatal("err = nil, want error")
	}
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if got.IdentityState != QuotaIdentityStateError {
		t.Fatalf("IdentityState = %q, want %q", got.IdentityState, QuotaIdentityStateError)
	}
}
