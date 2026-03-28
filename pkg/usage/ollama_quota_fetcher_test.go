package usage

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

type fakeOllamaCookieLoader struct {
	cookies []*http.Cookie
	err     error
	calls   int
}

func (f *fakeOllamaCookieLoader) Load() ([]*http.Cookie, error) {
	f.calls++
	return f.cookies, f.err
}

func TestOllamaQuotaFetcherFetchSuccess(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 28, 12, 34, 56, 0, time.UTC)
	signer := &fakeOllamaMeSigner{
		t:             t,
		wantChallenge: "POST,/api/me?ts=1774701296",
		token:         "pubkey:signature",
	}
	cookieLoader := &fakeOllamaCookieLoader{
		cookies: []*http.Cookie{
			{Name: "session", Value: "abc123", Domain: "127.0.0.1", Path: "/", Secure: true, HttpOnly: true},
		},
	}

	var gotCookie string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/me":
			if r.Method != http.MethodPost {
				t.Fatalf("method = %s, want POST", r.Method)
			}
			if r.Header.Get("Authorization") != "pubkey:signature" {
				t.Fatalf("authorization = %q, want pubkey:signature", r.Header.Get("Authorization"))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Name":"nomadxxx","Email":"lukegiles32@protonmail.com","Plan":"pro","NotifyUsageLimits":true}`))
		case "/settings":
			gotCookie = r.Header.Get("Cookie")
			if gotCookie == "" {
				t.Fatal("missing cookie header")
			}
			_, _ = w.Write([]byte(`<!doctype html><html><body><div class="flex flex-col space-y-6"><h2 class="text-xl font-medium flex items-center space-x-2"><span>Cloud Usage</span><span class="text-xs font-normal px-2 py-0.5 rounded-full bg-neutral-100 text-neutral-600 capitalize">pro</span></h2><div><div class="flex justify-between mb-2"><span class="text-sm">Session usage</span><span class="text-sm">2% used</span></div><div class="text-xs text-neutral-500 mt-1 local-time" data-time="2026-03-28T00:00:00Z">Resets in 3 hours</div></div><div><div class="flex justify-between mb-2"><span class="text-sm">Weekly usage</span><span class="text-sm">26.5% used</span></div><div class="text-xs text-neutral-500 mt-1 local-time" data-time="2026-03-30T00:00:00Z">Resets in 2 days</div></div><form method="POST" action="/settings" class="pt-2"><label class="flex items-center gap-2 text-xs text-neutral-700"><input type="checkbox" name="notify-usage-limits" class="rounded border-neutral-300" checked /><span>Notify me when I'm close to hitting my usage limits</span></label></form></div></body></html>`))
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	fetcher := &OllamaQuotaFetcher{
		BaseURL:      server.URL,
		Client:       server.Client(),
		Clock:        func() time.Time { return now },
		Signer:       signer,
		CookieLoader: cookieLoader,
	}

	got, err := fetcher.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if cookieLoader.calls != 1 {
		t.Fatalf("cookie loader calls = %d, want 1", cookieLoader.calls)
	}
	if got.State != QuotaStateLive {
		t.Fatalf("State = %q, want %q", got.State, QuotaStateLive)
	}
	if got.Source != QuotaSourceOllamaSettingsHTML {
		t.Fatalf("Source = %q, want %q", got.Source, QuotaSourceOllamaSettingsHTML)
	}
	if got.IdentityState != QuotaIdentityStateAuthenticated {
		t.Fatalf("IdentityState = %q, want %q", got.IdentityState, QuotaIdentityStateAuthenticated)
	}
	if got.AccountName != "nomadxxx" || got.AccountEmail != "lukegiles32@protonmail.com" || got.PlanName != "pro" {
		t.Fatalf("account metadata = %+v", got)
	}
	if got.IdentitySource != QuotaSourceOllamaAPIMe {
		t.Fatalf("IdentitySource = %q, want %q", got.IdentitySource, QuotaSourceOllamaAPIMe)
	}
	if got.SessionUsedPercent != 2 || got.WeeklyUsedPercent != 26.5 {
		t.Fatalf("usage = %+v", got)
	}
	if got.SessionResetsAt == nil || got.WeeklyResetsAt == nil {
		t.Fatalf("reset times missing: %+v", got)
	}
	if got.FetchedAt != now.UTC() || got.ExpiresAt != now.Add(time.Hour).UTC() {
		t.Fatalf("freshness = %v/%v", got.FetchedAt, got.ExpiresAt)
	}
	if gotCookie == "" {
		t.Fatal("cookie header was not sent")
	}
}

func TestOllamaQuotaFetcherClassifiesExpiredSettingsAuth(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 3, 28, 12, 34, 56, 0, time.UTC)
	signer := &fakeOllamaMeSigner{
		t:             t,
		wantChallenge: "POST,/api/me?ts=1774701296",
		token:         "pubkey:signature",
	}
	cookieLoader := &fakeOllamaCookieLoader{
		cookies: []*http.Cookie{{Name: "session", Value: "abc123", Domain: "127.0.0.1", Path: "/"}},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/me":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		case "/settings":
			w.Header().Set("Location", "/signin")
			w.WriteHeader(http.StatusFound)
		default:
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
	}))
	defer server.Close()

	fetcher := &OllamaQuotaFetcher{
		BaseURL:      server.URL,
		Client:       server.Client(),
		Clock:        func() time.Time { return now },
		Signer:       signer,
		CookieLoader: cookieLoader,
	}

	got, err := fetcher.Fetch(context.Background())
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got.State != QuotaStateExpired {
		t.Fatalf("State = %q, want %q", got.State, QuotaStateExpired)
	}
	if got.IdentityState != QuotaIdentityStateAuthenticatedEmpty {
		t.Fatalf("IdentityState = %q, want %q", got.IdentityState, QuotaIdentityStateAuthenticatedEmpty)
	}
}

func TestOllamaQuotaFetcherClassifiesUnavailableOnCookieLoadError(t *testing.T) {
	t.Parallel()

	cookieLoader := &fakeOllamaCookieLoader{err: errors.New("cookie load failed")}
	fetcher := &OllamaQuotaFetcher{
		Clock:        time.Now,
		CookieLoader: cookieLoader,
	}

	got, err := fetcher.Fetch(context.Background())
	if err == nil {
		t.Fatal("err = nil, want error")
	}
	if got.State != QuotaStateUnavailable {
		t.Fatalf("State = %q, want %q", got.State, QuotaStateUnavailable)
	}
}
