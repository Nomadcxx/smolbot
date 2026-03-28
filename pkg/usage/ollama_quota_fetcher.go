package usage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type CookieLoader interface {
	Load() ([]*http.Cookie, error)
}

type OllamaQuotaFetcher struct {
	BaseURL      string
	Client       *http.Client
	Clock        func() time.Time
	Signer       OllamaMeSigner
	CookieLoader CookieLoader
}

func (f *OllamaQuotaFetcher) Fetch(ctx context.Context) (QuotaSummary, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	now := time.Now
	if f != nil && f.Clock != nil {
		now = f.Clock
	}

	baseURL := defaultOllamaBaseURL
	if f != nil && strings.TrimSpace(f.BaseURL) != "" {
		baseURL = strings.TrimSpace(f.BaseURL)
	}

	summary := QuotaSummary{
		ProviderID: "ollama",
		State:      QuotaStateUnavailable,
		Source:     QuotaSourceUnknown,
		FetchedAt:  now().UTC(),
		ExpiresAt:  now().UTC(),
	}

	if f != nil && f.Signer != nil {
		identity, err := ProbeOllamaMe(ctx, baseURL, cloneHTTPClient(f.Client), f.Signer, now)
		if err != nil {
			summary.IdentityState = QuotaIdentityStateError
		} else {
			summary.IdentityState = identity.IdentityState
			summary.IdentitySource = identity.Source
			summary.AccountName = identity.AccountName
			summary.AccountEmail = identity.AccountEmail
			summary.PlanName = identity.PlanName
			summary.NotifyUsageLimits = identity.NotifyUsageLimits
			summary.IdentityAccountName = identity.AccountName
			summary.IdentityAccountEmail = identity.AccountEmail
		}
	}

	cookies, err := f.loadCookies()
	if err != nil {
		return summary, fmt.Errorf("load ollama cookies: %w", err)
	}
	if len(cookies) == 0 {
		return summary, fmt.Errorf("no ollama cookies available")
	}

	var baseClient *http.Client
	if f != nil {
		baseClient = f.Client
	}
	client := cloneHTTPClient(baseClient)
	if client == nil {
		client = http.DefaultClient
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return summary, fmt.Errorf("create cookie jar: %w", err)
	}
	settingsURL, err := url.Parse(baseURL)
	if err != nil {
		return summary, fmt.Errorf("parse ollama base url: %w", err)
	}
	for _, cookie := range cookies {
		jar.SetCookies(settingsURL, []*http.Cookie{cookie})
	}
	client.Jar = jar
	client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
		return http.ErrUseLastResponse
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(baseURL, "/")+"/settings", nil)
	if err != nil {
		return summary, fmt.Errorf("build ollama settings request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return summary, fmt.Errorf("fetch ollama settings: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if err != nil {
			summary.State = QuotaStateUnavailable
			return summary, fmt.Errorf("read ollama settings response: %w", err)
		}
		parsed, err := ParseOllamaSettingsUsageHTML(body, now().UTC(), now().Add(time.Hour).UTC())
		if err != nil {
			summary.State = QuotaStateUnavailable
			return summary, fmt.Errorf("parse ollama settings usage html: %w", err)
		}
		summary.SessionUsedPercent = parsed.SessionUsedPercent
		summary.SessionResetsAt = parsed.SessionResetsAt
		summary.WeeklyUsedPercent = parsed.WeeklyUsedPercent
		summary.WeeklyResetsAt = parsed.WeeklyResetsAt
		summary.NotifyUsageLimits = parsed.NotifyUsageLimits
		summary.State = parsed.State
		summary.Source = parsed.Source
		summary.FetchedAt = parsed.FetchedAt
		summary.ExpiresAt = parsed.ExpiresAt
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusFound, http.StatusSeeOther, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
		summary.State = QuotaStateExpired
		summary.Source = QuotaSourceOllamaSettingsHTML
		return summary, nil
	default:
		summary.State = QuotaStateUnavailable
		return summary, fmt.Errorf("unexpected ollama settings status: %s", resp.Status)
	}

	if summary.IdentitySource == "" && f != nil && f.Signer != nil {
		summary.IdentitySource = QuotaSourceOllamaAPIMe
	}
	if summary.IdentityState == "" {
		summary.IdentityState = QuotaIdentityStateUnknown
	}
	return summary, nil
}

func (f *OllamaQuotaFetcher) loadCookies() ([]*http.Cookie, error) {
	if f == nil {
		return nil, fmt.Errorf("ollama quota fetcher unavailable")
	}
	if f.CookieLoader == nil {
		return nil, fmt.Errorf("ollama cookie loader unavailable")
	}
	return f.CookieLoader.Load()
}

func cloneHTTPClient(client *http.Client) *http.Client {
	if client == nil {
		return &http.Client{}
	}
	clone := *client
	return &clone
}
