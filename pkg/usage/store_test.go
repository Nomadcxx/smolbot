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

func TestBudgetCRUDOperations(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	windowStart := time.Date(2026, 3, 24, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	resetsAt := time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)
	budget := Budget{
		ID:              "weekly-ollama",
		Name:            "Weekly Ollama",
		BudgetType:      "weekly",
		LimitAmount:     500,
		LimitUnit:       "tokens",
		ScopeType:       "provider",
		ScopeTarget:     "ollama",
		AlertThresholds: []int{80, 50},
		IsActive:        true,
		WindowStart:     &windowStart,
		WindowEnd:       &windowEnd,
		ResetsAt:        &resetsAt,
	}
	if err := store.UpsertBudget(context.Background(), budget); err != nil {
		t.Fatalf("UpsertBudget: %v", err)
	}

	got, err := store.GetBudget(context.Background(), "weekly-ollama")
	if err != nil {
		t.Fatalf("GetBudget: %v", err)
	}
	if got.ID != budget.ID || got.Name != budget.Name || got.ScopeTarget != budget.ScopeTarget {
		t.Fatalf("GetBudget = %+v, want %+v", got, budget)
	}
	if len(got.AlertThresholds) != 2 || got.AlertThresholds[0] != 50 || got.AlertThresholds[1] != 80 {
		t.Fatalf("GetBudget thresholds = %+v, want sorted [50 80]", got.AlertThresholds)
	}
	if got.WindowStart == nil || !got.WindowStart.Equal(windowStart) {
		t.Fatalf("GetBudget WindowStart = %v, want %v", got.WindowStart, windowStart)
	}

	budgets, err := store.ListBudgets(context.Background())
	if err != nil {
		t.Fatalf("ListBudgets: %v", err)
	}
	if len(budgets) != 1 || budgets[0].ID != budget.ID {
		t.Fatalf("ListBudgets = %+v, want single budget %q", budgets, budget.ID)
	}

	if err := store.DeleteBudget(context.Background(), budget.ID); err != nil {
		t.Fatalf("DeleteBudget: %v", err)
	}
	if _, err := store.GetBudget(context.Background(), budget.ID); err == nil {
		t.Fatalf("GetBudget after delete = nil error, want not found")
	}
}

func TestBudgetThresholdAlertsPersistAcrossWeeklyWindow(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	recordedAt := time.Date(2026, 3, 23, 10, 0, 0, 0, time.UTC)
	now := recordedAt.Add(48 * time.Hour)
	budget := Budget{
		ID:              "weekly-ollama",
		Name:            "Weekly Ollama",
		BudgetType:      "weekly",
		LimitAmount:     100,
		LimitUnit:       "tokens",
		ScopeType:       "provider",
		ScopeTarget:     "ollama",
		AlertThresholds: []int{50},
		IsActive:        true,
	}
	if err := store.UpsertBudget(context.Background(), budget); err != nil {
		t.Fatalf("UpsertBudget: %v", err)
	}

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
		RecordedAt:       recordedAt,
	}); err != nil {
		t.Fatalf("RecordCompletion: %v", err)
	}

	summary, err := store.CurrentProviderSummary("s1", "ollama", "llama3.2", now)
	if err != nil {
		t.Fatalf("CurrentProviderSummary: %v", err)
	}
	if summary.BudgetStatus != "warning" || summary.WarningLevel != "medium" {
		t.Fatalf("summary budget state = (%q, %q), want (warning, medium)", summary.BudgetStatus, summary.WarningLevel)
	}
}

func TestBudgetThresholdAlertsResetAcrossWeeklyWindows(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	budget := Budget{
		ID:              "weekly-ollama",
		Name:            "Weekly Ollama",
		BudgetType:      "weekly",
		LimitAmount:     100,
		LimitUnit:       "tokens",
		ScopeType:       "provider",
		ScopeTarget:     "ollama",
		AlertThresholds: []int{50},
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
			CompletionTokens: 30,
			TotalTokens:      60,
			Status:           "success",
			UsageSource:      "reported",
			RecordedAt:       time.Date(2026, 3, 23, 10, 0, 0, 0, time.UTC),
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
			RecordedAt:       time.Date(2026, 3, 24, 10, 0, 0, 0, time.UTC),
		},
	} {
		if err := store.RecordCompletion(context.Background(), record); err != nil {
			t.Fatalf("RecordCompletion same week: %v", err)
		}
	}

	alerts, err := store.ListBudgetAlerts(budget.ID)
	if err != nil {
		t.Fatalf("ListBudgetAlerts same week: %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("alerts in first week = %d, want 1", len(alerts))
	}

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
		RecordedAt:       time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RecordCompletion next week: %v", err)
	}

	alerts, err = store.ListBudgetAlerts(budget.ID)
	if err != nil {
		t.Fatalf("ListBudgetAlerts next week: %v", err)
	}
	if len(alerts) != 2 {
		t.Fatalf("alerts after weekly reset = %d, want 2", len(alerts))
	}
}

func TestBudgetThresholdAlertsRespectExplicitWindowBounds(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	windowStart := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 3, 27, 0, 0, 0, 0, time.UTC)
	budget := Budget{
		ID:              "custom-ollama",
		Name:            "Custom Ollama",
		BudgetType:      "custom",
		LimitAmount:     100,
		LimitUnit:       "tokens",
		ScopeType:       "provider",
		ScopeTarget:     "ollama",
		AlertThresholds: []int{50},
		IsActive:        true,
		WindowStart:     &windowStart,
		WindowEnd:       &windowEnd,
	}
	if err := store.UpsertBudget(context.Background(), budget); err != nil {
		t.Fatalf("UpsertBudget: %v", err)
	}

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
		RecordedAt:       windowStart.Add(24 * time.Hour),
	}); err != nil {
		t.Fatalf("RecordCompletion in window: %v", err)
	}
	if err := store.RecordCompletion(context.Background(), CompletionRecord{
		SessionKey:       "s1",
		ProviderID:       "ollama",
		ModelName:        "llama3.2",
		RequestType:      "chat",
		PromptTokens:     20,
		CompletionTokens: 20,
		TotalTokens:      40,
		Status:           "success",
		UsageSource:      "reported",
		RecordedAt:       windowEnd.Add(24 * time.Hour),
	}); err != nil {
		t.Fatalf("RecordCompletion out of window: %v", err)
	}

	alerts, err := store.ListBudgetAlerts(budget.ID)
	if err != nil {
		t.Fatalf("ListBudgetAlerts: %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("alerts = %d, want 1 within explicit window", len(alerts))
	}

	summary, err := store.CurrentProviderSummary("s1", "ollama", "llama3.2", windowEnd.Add(-time.Hour))
	if err != nil {
		t.Fatalf("CurrentProviderSummary: %v", err)
	}
	if summary.BudgetStatus != "warning" || summary.WarningLevel != "medium" {
		t.Fatalf("summary budget state = (%q, %q), want (warning, medium)", summary.BudgetStatus, summary.WarningLevel)
	}
}

func TestBudgetThresholdAlertsResetAcrossMonthlyWindows(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	budget := Budget{
		ID:              "monthly-ollama",
		Name:            "Monthly Ollama",
		BudgetType:      "monthly",
		LimitAmount:     100,
		LimitUnit:       "tokens",
		ScopeType:       "provider",
		ScopeTarget:     "ollama",
		AlertThresholds: []int{50},
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
			CompletionTokens: 30,
			TotalTokens:      60,
			Status:           "success",
			UsageSource:      "reported",
			RecordedAt:       time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC),
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
			RecordedAt:       time.Date(2026, 3, 20, 10, 0, 0, 0, time.UTC),
		},
	} {
		if err := store.RecordCompletion(context.Background(), record); err != nil {
			t.Fatalf("RecordCompletion same month: %v", err)
		}
	}

	alerts, err := store.ListBudgetAlerts(budget.ID)
	if err != nil {
		t.Fatalf("ListBudgetAlerts same month: %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("alerts in first month = %d, want 1", len(alerts))
	}

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
		RecordedAt:       time.Date(2026, 4, 2, 10, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RecordCompletion next month: %v", err)
	}

	alerts, err = store.ListBudgetAlerts(budget.ID)
	if err != nil {
		t.Fatalf("ListBudgetAlerts next month: %v", err)
	}
	if len(alerts) != 2 {
		t.Fatalf("alerts after monthly reset = %d, want 2", len(alerts))
	}
}

func TestBudgetThresholdAlertsRespectOneSidedActivationBounds(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	start := time.Date(2026, 3, 29, 0, 0, 0, 0, time.UTC)
	startOnlyBudget := Budget{
		ID:              "start-only",
		Name:            "Start Only",
		BudgetType:      "daily",
		LimitAmount:     100,
		LimitUnit:       "tokens",
		ScopeType:       "provider",
		ScopeTarget:     "ollama",
		AlertThresholds: []int{50},
		IsActive:        true,
		WindowStart:     &start,
	}
	if err := store.UpsertBudget(context.Background(), startOnlyBudget); err != nil {
		t.Fatalf("UpsertBudget start-only: %v", err)
	}

	for _, record := range []CompletionRecord{
		{
			SessionKey:       "s1",
			ProviderID:       "ollama",
			ModelName:        "llama3.2",
			RequestType:      "chat",
			PromptTokens:     30,
			CompletionTokens: 30,
			TotalTokens:      60,
			Status:           "success",
			UsageSource:      "reported",
			RecordedAt:       time.Date(2026, 3, 28, 10, 0, 0, 0, time.UTC),
		},
		{
			SessionKey:       "s1",
			ProviderID:       "ollama",
			ModelName:        "llama3.2",
			RequestType:      "chat",
			PromptTokens:     30,
			CompletionTokens: 30,
			TotalTokens:      60,
			Status:           "success",
			UsageSource:      "reported",
			RecordedAt:       time.Date(2026, 3, 29, 10, 0, 0, 0, time.UTC),
		},
	} {
		if err := store.RecordCompletion(context.Background(), record); err != nil {
			t.Fatalf("RecordCompletion start-only: %v", err)
		}
	}

	alerts, err := store.ListBudgetAlerts(startOnlyBudget.ID)
	if err != nil {
		t.Fatalf("ListBudgetAlerts start-only: %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("start-only alerts = %d, want 1", len(alerts))
	}

	end := time.Date(2026, 3, 28, 0, 0, 0, 0, time.UTC)
	endOnlyBudget := Budget{
		ID:              "end-only",
		Name:            "End Only",
		BudgetType:      "daily",
		LimitAmount:     100,
		LimitUnit:       "tokens",
		ScopeType:       "provider",
		ScopeTarget:     "ollama",
		AlertThresholds: []int{50},
		IsActive:        true,
		WindowEnd:       &end,
	}
	if err := store.UpsertBudget(context.Background(), endOnlyBudget); err != nil {
		t.Fatalf("UpsertBudget end-only: %v", err)
	}
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
		RecordedAt:       time.Date(2026, 3, 29, 12, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RecordCompletion end-only: %v", err)
	}

	alerts, err = store.ListBudgetAlerts(endOnlyBudget.ID)
	if err != nil {
		t.Fatalf("ListBudgetAlerts end-only: %v", err)
	}
	if len(alerts) != 0 {
		t.Fatalf("end-only alerts = %d, want 0", len(alerts))
	}
}

func TestCurrentProviderSummaryIgnoresInactiveBudgets(t *testing.T) {
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
		AlertThresholds: []int{50},
		IsActive:        true,
	}
	if err := store.UpsertBudget(context.Background(), budget); err != nil {
		t.Fatalf("UpsertBudget active: %v", err)
	}
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
		RecordedAt:       now,
	}); err != nil {
		t.Fatalf("RecordCompletion: %v", err)
	}

	budget.IsActive = false
	if err := store.UpsertBudget(context.Background(), budget); err != nil {
		t.Fatalf("UpsertBudget inactive: %v", err)
	}

	summary, err := store.CurrentProviderSummary("s1", "ollama", "llama3.2", now)
	if err != nil {
		t.Fatalf("CurrentProviderSummary: %v", err)
	}
	if summary.BudgetStatus != "" || summary.WarningLevel != "" {
		t.Fatalf("summary budget state = (%q, %q), want empty state for inactive budget", summary.BudgetStatus, summary.WarningLevel)
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

func TestPruneOlderThanRemovesOldBudgetAlerts(t *testing.T) {
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
		AlertThresholds: []int{50},
		IsActive:        true,
	}
	if err := store.UpsertBudget(context.Background(), budget); err != nil {
		t.Fatalf("UpsertBudget: %v", err)
	}

	for _, record := range []CompletionRecord{
		{
			SessionKey:       "old",
			ProviderID:       "ollama",
			ModelName:        "llama3.2",
			RequestType:      "chat",
			PromptTokens:     30,
			CompletionTokens: 30,
			TotalTokens:      60,
			Status:           "success",
			UsageSource:      "reported",
			RecordedAt:       now.AddDate(0, 0, -90),
		},
		{
			SessionKey:       "new",
			ProviderID:       "ollama",
			ModelName:        "llama3.2",
			RequestType:      "chat",
			PromptTokens:     30,
			CompletionTokens: 30,
			TotalTokens:      60,
			Status:           "success",
			UsageSource:      "reported",
			RecordedAt:       now.AddDate(0, 0, -2),
		},
	} {
		if err := store.RecordCompletion(context.Background(), record); err != nil {
			t.Fatalf("RecordCompletion: %v", err)
		}
	}

	if err := store.PruneOlderThan(context.Background(), now.AddDate(0, 0, -56)); err != nil {
		t.Fatalf("PruneOlderThan: %v", err)
	}

	alerts, err := store.ListBudgetAlerts(budget.ID)
	if err != nil {
		t.Fatalf("ListBudgetAlerts: %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("alerts after prune = %+v, want only recent alert", alerts)
	}
}

func TestPruneOlderThanPreservesAlertsForActiveLongWindowBudgets(t *testing.T) {
	store, err := NewStore(":memory:")
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	windowStart := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	windowEnd := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	budget := Budget{
		ID:              "long-window",
		Name:            "Long Window",
		BudgetType:      "custom",
		LimitAmount:     100,
		LimitUnit:       "tokens",
		ScopeType:       "provider",
		ScopeTarget:     "ollama",
		AlertThresholds: []int{50},
		IsActive:        true,
		WindowStart:     &windowStart,
		WindowEnd:       &windowEnd,
	}
	if err := store.UpsertBudget(context.Background(), budget); err != nil {
		t.Fatalf("UpsertBudget: %v", err)
	}

	firstRecordAt := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)
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
		RecordedAt:       firstRecordAt,
	}); err != nil {
		t.Fatalf("RecordCompletion first: %v", err)
	}

	cutoff := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	if err := store.PruneOlderThan(context.Background(), cutoff); err != nil {
		t.Fatalf("PruneOlderThan: %v", err)
	}

	summary, err := store.CurrentProviderSummary("s1", "ollama", "llama3.2", time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("CurrentProviderSummary after prune: %v", err)
	}
	if summary.BudgetStatus != "warning" || summary.WarningLevel != "medium" {
		t.Fatalf("summary budget state after prune = (%q, %q), want (warning, medium)", summary.BudgetStatus, summary.WarningLevel)
	}

	if err := store.RecordCompletion(context.Background(), CompletionRecord{
		SessionKey:       "s1",
		ProviderID:       "ollama",
		ModelName:        "llama3.2",
		RequestType:      "chat",
		PromptTokens:     5,
		CompletionTokens: 5,
		TotalTokens:      10,
		Status:           "success",
		UsageSource:      "reported",
		RecordedAt:       time.Date(2026, 3, 16, 10, 0, 0, 0, time.UTC),
	}); err != nil {
		t.Fatalf("RecordCompletion second: %v", err)
	}

	alerts, err := store.ListBudgetAlerts(budget.ID)
	if err != nil {
		t.Fatalf("ListBudgetAlerts: %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("alerts after prune in active window = %d, want 1", len(alerts))
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
