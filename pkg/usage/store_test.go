package usage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestNewStoreCreatesPhaseOneTables(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()
	if got := store.db.Stats().MaxOpenConnections; got != 1 {
		t.Fatalf("MaxOpenConnections = %d, want 1 for in-memory sqlite", got)
	}

	for _, table := range []string{
		"usage_records",
		"daily_usage_rollups",
		"budgets",
		"budget_alerts",
		"historical_usage_samples",
	} {
		if !tableExists(t, store, table) {
			t.Fatalf("expected table %q to exist", table)
		}
	}
}

func TestNewStoreCreatesPhaseOneTablesFileBacked(t *testing.T) {
	store, err := NewStore(filepath.Join(t.TempDir(), "usage.db"))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	for _, table := range []string{
		"usage_records",
		"daily_usage_rollups",
		"budgets",
		"budget_alerts",
		"historical_usage_samples",
	} {
		if !tableExists(t, store, table) {
			t.Fatalf("expected table %q to exist", table)
		}
	}
}

func TestRecordCompletionAndListUsageRecords(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	record := CompletionRecord{
		SessionKey:       "s1",
		ProviderID:       "ollama",
		ModelName:        "llama3.2",
		RequestType:      "chat",
		PromptTokens:     12,
		CompletionTokens: 8,
		TotalTokens:      20,
		DurationMS:       17,
		Status:           "success",
		UsageSource:      "reported",
	}
	if err := store.RecordCompletion(context.Background(), record); err != nil {
		t.Fatalf("RecordCompletion: %v", err)
	}

	records, err := store.ListUsageRecords("s1")
	if err != nil {
		t.Fatalf("ListUsageRecords: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records length = %d, want 1", len(records))
	}
	got := records[0]
	if got.SessionKey != record.SessionKey || got.ProviderID != record.ProviderID || got.UsageSource != record.UsageSource {
		t.Fatalf("stored record = %+v, want %+v", got, record)
	}
	if got.TotalTokens != record.TotalTokens {
		t.Fatalf("stored totalTokens = %d, want %d", got.TotalTokens, record.TotalTokens)
	}
}

func TestRecordCompletionStoresEstimatedUsageSource(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	record := CompletionRecord{
		SessionKey:       "s2",
		ProviderID:       "ollama",
		ModelName:        "llama3.2",
		RequestType:      "chat",
		PromptTokens:     13,
		CompletionTokens: 7,
		TotalTokens:      20,
		DurationMS:       11,
		Status:           "success",
		UsageSource:      "estimated",
	}
	if err := store.RecordCompletion(context.Background(), record); err != nil {
		t.Fatalf("RecordCompletion: %v", err)
	}

	records, err := store.ListUsageRecords("s2")
	if err != nil {
		t.Fatalf("ListUsageRecords: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("records length = %d, want 1", len(records))
	}
	if records[0].UsageSource != "estimated" {
		t.Fatalf("usage source = %q, want estimated", records[0].UsageSource)
	}
}

func TestSummaryQueriesAndCurrentProviderSummary(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC)
	records := []CompletionRecord{
		{
			SessionKey:       "s1",
			ProviderID:       "ollama",
			ModelName:        "llama3.2",
			RequestType:      "chat",
			PromptTokens:     12,
			CompletionTokens: 8,
			TotalTokens:      20,
			Status:           "success",
			UsageSource:      "reported",
			RecordedAt:       now.Add(-2 * time.Hour),
		},
		{
			SessionKey:       "s1",
			ProviderID:       "ollama",
			ModelName:        "llama3.2",
			RequestType:      "chat",
			PromptTokens:     18,
			CompletionTokens: 12,
			TotalTokens:      30,
			Status:           "success",
			UsageSource:      "reported",
			RecordedAt:       now.Add(-30 * time.Minute),
		},
		{
			SessionKey:       "s2",
			ProviderID:       "ollama",
			ModelName:        "llama3.2",
			RequestType:      "chat",
			PromptTokens:     24,
			CompletionTokens: 16,
			TotalTokens:      40,
			Status:           "reported",
			UsageSource:      "reported",
			RecordedAt:       now.AddDate(0, 0, -3),
		},
	}
	for _, record := range records {
		if err := store.RecordCompletion(context.Background(), record); err != nil {
			t.Fatalf("RecordCompletion: %v", err)
		}
	}

	sessionSummary, err := store.SessionSummary("s1")
	if err != nil {
		t.Fatalf("SessionSummary: %v", err)
	}
	if sessionSummary.TotalTokens != 50 {
		t.Fatalf("session total tokens = %d, want 50", sessionSummary.TotalTokens)
	}
	if sessionSummary.TotalRequests != 2 {
		t.Fatalf("session requests = %d, want 2", sessionSummary.TotalRequests)
	}

	dailySummary, err := store.DailySummary("ollama", now)
	if err != nil {
		t.Fatalf("DailySummary: %v", err)
	}
	if dailySummary.TotalTokens != 50 {
		t.Fatalf("daily total tokens = %d, want 50", dailySummary.TotalTokens)
	}

	weeklySummary, err := store.WeeklySummary("ollama", now)
	if err != nil {
		t.Fatalf("WeeklySummary: %v", err)
	}
	if weeklySummary.TotalTokens != 90 {
		t.Fatalf("weekly total tokens = %d, want 90", weeklySummary.TotalTokens)
	}

	currentSummary, err := store.CurrentProviderSummary("s1", "ollama", "llama3.2", now)
	if err != nil {
		t.Fatalf("CurrentProviderSummary: %v", err)
	}
	if currentSummary.ProviderID != "ollama" {
		t.Fatalf("provider id = %q, want ollama", currentSummary.ProviderID)
	}
	if currentSummary.SessionTokens != 50 || currentSummary.TodayTokens != 50 || currentSummary.WeeklyTokens != 90 {
		t.Fatalf("current summary = %+v", currentSummary)
	}
}

func TestBudgetThresholdAlertsDedupingAndReset(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC)
	budget := Budget{
		ID:              "daily-ollama",
		Name:            "Daily Ollama",
		BudgetType:      "daily",
		LimitAmount:     100,
		LimitUnit:       "tokens",
		ScopeType:       "provider",
		ScopeTarget:     "ollama",
		AlertThresholds: []int{50, 80, 95},
		IsActive:        true,
	}
	if err := store.UpsertBudget(context.Background(), budget); err != nil {
		t.Fatalf("UpsertBudget: %v", err)
	}

	for _, record := range []CompletionRecord{
		{
			SessionKey:       "s1",
			ProviderID:       "ollama",
			ModelName:        "llama3.2",
			RequestType:      "chat",
			PromptTokens:     30,
			CompletionTokens: 19,
			TotalTokens:      49,
			Status:           "success",
			UsageSource:      "reported",
			RecordedAt:       now.Add(-2 * time.Hour),
		},
		{
			SessionKey:       "s1",
			ProviderID:       "ollama",
			ModelName:        "llama3.2",
			RequestType:      "chat",
			PromptTokens:     1,
			CompletionTokens: 1,
			TotalTokens:      2,
			Status:           "success",
			UsageSource:      "reported",
			RecordedAt:       now.Add(-90 * time.Minute),
		},
		{
			SessionKey:       "s1",
			ProviderID:       "ollama",
			ModelName:        "llama3.2",
			RequestType:      "chat",
			PromptTokens:     15,
			CompletionTokens: 20,
			TotalTokens:      35,
			Status:           "success",
			UsageSource:      "reported",
			RecordedAt:       now.Add(-30 * time.Minute),
		},
		{
			SessionKey:       "s1",
			ProviderID:       "ollama",
			ModelName:        "llama3.2",
			RequestType:      "chat",
			PromptTokens:     5,
			CompletionTokens: 5,
			TotalTokens:      10,
			Status:           "success",
			UsageSource:      "reported",
			RecordedAt:       now.Add(-15 * time.Minute),
		},
	} {
		if err := store.RecordCompletion(context.Background(), record); err != nil {
			t.Fatalf("RecordCompletion: %v", err)
		}
	}

	alerts, err := store.ListBudgetAlerts("daily-ollama")
	if err != nil {
		t.Fatalf("ListBudgetAlerts: %v", err)
	}
	if len(alerts) != 3 {
		t.Fatalf("alerts = %d, want 3", len(alerts))
	}
	if alerts[0].ThresholdPercent != 50 || alerts[1].ThresholdPercent != 80 || alerts[2].ThresholdPercent != 95 {
		t.Fatalf("threshold order = %+v", alerts)
	}

	summary, err := store.CurrentProviderSummary("s1", "ollama", "llama3.2", now)
	if err != nil {
		t.Fatalf("CurrentProviderSummary: %v", err)
	}
	if summary.BudgetStatus != "warning" {
		t.Fatalf("budget status = %q, want warning", summary.BudgetStatus)
	}
	if summary.WarningLevel != "critical" {
		t.Fatalf("warning level = %q, want critical", summary.WarningLevel)
	}

	nextDay := now.AddDate(0, 0, 1)
	if err := store.RecordCompletion(context.Background(), CompletionRecord{
		SessionKey:       "s1",
		ProviderID:       "ollama",
		ModelName:        "llama3.2",
		RequestType:      "chat",
		PromptTokens:     30,
		CompletionTokens: 30,
		TotalTokens:      60,
		Status:           "success",
		UsageSource:      "reported",
		RecordedAt:       nextDay,
	}); err != nil {
		t.Fatalf("RecordCompletion next day: %v", err)
	}

	alerts, err = store.ListBudgetAlerts("daily-ollama")
	if err != nil {
		t.Fatalf("ListBudgetAlerts second pass: %v", err)
	}
	if len(alerts) != 4 {
		t.Fatalf("alerts after reset window = %d, want 4", len(alerts))
	}
}

func TestHistoricalSamplesAndPruneOlderThan(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	now := time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC)
	oldSample := HistoricalUsageSample{
		ProviderID:    "ollama",
		ModelName:     "llama3.2",
		WindowType:    "weekly",
		Source:        "live",
		SampledAt:     now.AddDate(0, 0, -90),
		UsedPercent:   45,
		ResetsAt:      now.AddDate(0, 0, -83),
		WindowMinutes: 10080,
		TotalTokens:   400,
	}
	newSample := HistoricalUsageSample{
		ProviderID:    "ollama",
		ModelName:     "llama3.2",
		WindowType:    "weekly",
		Source:        "live",
		SampledAt:     now.AddDate(0, 0, -5),
		UsedPercent:   55,
		ResetsAt:      now.AddDate(0, 0, 2),
		WindowMinutes: 10080,
		TotalTokens:   500,
	}
	if err := store.RecordHistoricalSample(context.Background(), oldSample); err != nil {
		t.Fatalf("RecordHistoricalSample old: %v", err)
	}
	if err := store.RecordHistoricalSample(context.Background(), newSample); err != nil {
		t.Fatalf("RecordHistoricalSample new: %v", err)
	}

	if err := store.RecordCompletion(context.Background(), CompletionRecord{
		SessionKey:       "old",
		ProviderID:       "ollama",
		ModelName:        "llama3.2",
		RequestType:      "chat",
		PromptTokens:     10,
		CompletionTokens: 10,
		TotalTokens:      20,
		Status:           "success",
		UsageSource:      "reported",
		RecordedAt:       now.AddDate(0, 0, -90),
	}); err != nil {
		t.Fatalf("RecordCompletion old: %v", err)
	}
	if err := store.RecordCompletion(context.Background(), CompletionRecord{
		SessionKey:       "new",
		ProviderID:       "ollama",
		ModelName:        "llama3.2",
		RequestType:      "chat",
		PromptTokens:     10,
		CompletionTokens: 15,
		TotalTokens:      25,
		Status:           "success",
		UsageSource:      "reported",
		RecordedAt:       now.AddDate(0, 0, -2),
	}); err != nil {
		t.Fatalf("RecordCompletion new: %v", err)
	}

	if err := store.PruneOlderThan(context.Background(), now.AddDate(0, 0, -56)); err != nil {
		t.Fatalf("PruneOlderThan: %v", err)
	}

	records, err := store.ListUsageRecords("")
	if err != nil {
		t.Fatalf("ListUsageRecords: %v", err)
	}
	if len(records) != 1 || records[0].SessionKey != "new" {
		t.Fatalf("records after prune = %+v, want only recent record", records)
	}

	samples, err := store.ListHistoricalSamples("ollama")
	if err != nil {
		t.Fatalf("ListHistoricalSamples: %v", err)
	}
	if len(samples) != 1 || !samples[0].SampledAt.Equal(newSample.SampledAt) {
		t.Fatalf("samples after prune = %+v, want only recent sample", samples)
	}
}

func tableExists(t *testing.T, store *Store, name string) bool {
	t.Helper()

	var found int
	err := store.db.QueryRow(
		`SELECT 1 FROM sqlite_master WHERE type = 'table' AND name = ?`,
		name,
	).Scan(&found)
	if err != nil {
		return false
	}
	return found == 1
}
