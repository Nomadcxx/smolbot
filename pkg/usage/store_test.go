package usage

import (
	"context"
	"path/filepath"
	"testing"
)

func TestNewStoreCreatesPhaseOneTables(t *testing.T) {
	store, err := NewStore(":memory:")
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
