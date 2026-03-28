package usage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseOllamaSettingsUsageHTML(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "ollama_settings_usage.html"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	fetchedAt := time.Date(2026, 3, 28, 1, 15, 0, 0, time.UTC)
	expiresAt := fetchedAt.Add(time.Hour)

	got, err := ParseOllamaSettingsUsageHTML(data, fetchedAt, expiresAt)
	if err != nil {
		t.Fatalf("ParseOllamaSettingsUsageHTML: %v", err)
	}

	if got.ProviderID != "ollama" {
		t.Fatalf("ProviderID = %q, want ollama", got.ProviderID)
	}
	if got.PlanName != "pro" {
		t.Fatalf("PlanName = %q, want pro", got.PlanName)
	}
	if got.SessionUsedPercent != 2 {
		t.Fatalf("SessionUsedPercent = %v, want 2", got.SessionUsedPercent)
	}
	if got.SessionResetsAt == nil || !got.SessionResetsAt.Equal(time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("SessionResetsAt = %v", got.SessionResetsAt)
	}
	if got.WeeklyUsedPercent != 26.5 {
		t.Fatalf("WeeklyUsedPercent = %v, want 26.5", got.WeeklyUsedPercent)
	}
	if got.WeeklyResetsAt == nil || !got.WeeklyResetsAt.Equal(time.Date(2026, 3, 30, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("WeeklyResetsAt = %v", got.WeeklyResetsAt)
	}
	if !got.NotifyUsageLimits {
		t.Fatal("NotifyUsageLimits = false, want true")
	}
	if got.Source != QuotaSourceOllamaSettingsHTML {
		t.Fatalf("Source = %q, want %q", got.Source, QuotaSourceOllamaSettingsHTML)
	}
	if got.State != QuotaStateLive {
		t.Fatalf("State = %q, want %q", got.State, QuotaStateLive)
	}
	if !got.FetchedAt.Equal(fetchedAt) || !got.ExpiresAt.Equal(expiresAt) {
		t.Fatalf("freshness = %v/%v, want %v/%v", got.FetchedAt, got.ExpiresAt, fetchedAt, expiresAt)
	}
}

func TestParseOllamaSettingsUsageHTMLRejectsMissingUsageBlocks(t *testing.T) {
	_, err := ParseOllamaSettingsUsageHTML([]byte(`<html><body><h2>Cloud Usage</h2></body></html>`), time.Time{}, time.Time{})
	if err == nil {
		t.Fatal("expected error for missing usage blocks")
	}
}
