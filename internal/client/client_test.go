package client

import (
	"encoding/json"
	"testing"
)

func TestCloseResetsLastSequence(t *testing.T) {
	c := New("ws://127.0.0.1/ws")
	c.lastSeq = 9

	c.Close()

	if c.lastSeq != 0 {
		t.Fatalf("expected close to reset sequence tracking, got %d", c.lastSeq)
	}
}

func TestCronJobsResponseDecoding(t *testing.T) {
	raw := []byte(`{"type":"res","id":"1","ok":true,"payload":{"jobs":[{"id":"job-1","name":"Daily cleanup","schedule":"every 5m","status":"active","nextRun":"2026-03-27T10:00:00Z"},{"id":"job-2","name":"Paused sync","schedule":"daily 02:00","status":"paused","nextRun":""}]}}`)

	var res Response
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !res.OK || res.Type != FrameRes {
		t.Fatalf("unexpected response frame: %#v", res)
	}

	var payload struct {
		Jobs []CronJob `json:"jobs"`
	}
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		t.Fatalf("unmarshal cron jobs payload: %v", err)
	}
	if len(payload.Jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %#v", payload.Jobs)
	}
	if payload.Jobs[0].ID != "job-1" || payload.Jobs[0].Status != "active" || payload.Jobs[1].Status != "paused" {
		t.Fatalf("unexpected decoded cron jobs: %#v", payload.Jobs)
	}
}

func TestStatusResponseDecodesPersistedUsageSummary(t *testing.T) {
	raw := []byte(`{"type":"res","id":"1","ok":true,"payload":{"model":"ollama/llama3.2","provider":"ollama","session":"s1","usage":{"promptTokens":12,"completionTokens":8,"totalTokens":20,"contextWindow":8192},"persistedUsage":{"providerId":"ollama","modelName":"llama3.2","sessionTokens":50,"todayTokens":50,"weeklyTokens":90,"sessionRequests":2,"todayRequests":2,"weeklyRequests":3,"budgetStatus":"warning","warningLevel":"medium","quota":{"providerId":"ollama","accountName":"nomadxxx","accountEmail":"lukegiles32@protonmail.com","planName":"pro","sessionUsedPercent":2,"sessionResetsAt":"2026-03-28T00:00:00Z","weeklyUsedPercent":26.5,"weeklyResetsAt":"2026-03-30T00:00:00Z","notifyUsageLimits":true,"state":"live","source":"ollama_settings_html","fetchedAt":"2026-03-27T23:00:00Z","expiresAt":"2026-03-28T00:00:00Z","identityState":"authenticated","identitySource":"ollama_api_me","identityAccountName":"nomadxxx","identityAccountEmail":"lukegiles32@protonmail.com"}},"usageAlert":{"providerId":"ollama","modelName":"llama3.2","budgetStatus":"warning","warningLevel":"medium","message":"Usage warning for ollama/llama3.2: medium budget threshold reached."},"uptime":120}}`)

	var res Response
	if err := json.Unmarshal(raw, &res); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if !res.OK || res.Type != FrameRes {
		t.Fatalf("unexpected response frame: %#v", res)
	}

	var payload StatusPayload
	if err := json.Unmarshal(res.Payload, &payload); err != nil {
		t.Fatalf("unmarshal status payload: %v", err)
	}
	if payload.Usage.ContextWindow != 8192 || payload.Usage.TotalTokens != 20 {
		t.Fatalf("unexpected live usage payload: %#v", payload.Usage)
	}
	if payload.Provider != "ollama" {
		t.Fatalf("unexpected provider payload: %q", payload.Provider)
	}
	if payload.PersistedUsage == nil {
		t.Fatal("expected persisted usage summary")
	}
	if payload.PersistedUsage.ProviderID != "ollama" || payload.PersistedUsage.SessionTokens != 50 || payload.PersistedUsage.WeeklyTokens != 90 {
		t.Fatalf("unexpected persisted usage payload: %#v", payload.PersistedUsage)
	}
	if payload.PersistedUsage.SessionRequests != 2 || payload.PersistedUsage.TodayRequests != 2 || payload.PersistedUsage.WeeklyRequests != 3 {
		t.Fatalf("unexpected persisted request counts: %#v", payload.PersistedUsage)
	}
	if payload.PersistedUsage.BudgetStatus != "warning" || payload.PersistedUsage.WarningLevel != "medium" {
		t.Fatalf("unexpected persisted budget state: %#v", payload.PersistedUsage)
	}
	if payload.PersistedUsage.Quota == nil {
		t.Fatal("expected persisted quota summary")
	}
	if payload.PersistedUsage.Quota.ProviderID != "ollama" || payload.PersistedUsage.Quota.PlanName != "pro" {
		t.Fatalf("unexpected persisted quota payload: %#v", payload.PersistedUsage.Quota)
	}
	if payload.PersistedUsage.Quota.State != "live" || payload.PersistedUsage.Quota.Source != "ollama_settings_html" {
		t.Fatalf("unexpected persisted quota state: %#v", payload.PersistedUsage.Quota)
	}
	if payload.UsageAlert == nil {
		t.Fatal("expected usage alert payload")
	}
	if payload.UsageAlert.Message == "" || payload.UsageAlert.WarningLevel != "medium" {
		t.Fatalf("unexpected usage alert payload: %#v", payload.UsageAlert)
	}
}
