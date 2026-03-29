package usage

import (
	"database/sql"
	"fmt"
	"time"
)

func (s *Store) SessionSummary(sessionKey string) (Summary, error) {
	row := s.db.QueryRow(
		`SELECT COALESCE(COUNT(*), 0), COALESCE(SUM(prompt_tokens), 0), COALESCE(SUM(completion_tokens), 0), COALESCE(SUM(total_tokens), 0)
		FROM usage_records
		WHERE session_key = ?`,
		sessionKey,
	)
	return scanSummary(row)
}

func (s *Store) DailySummary(providerID string, day time.Time) (Summary, error) {
	row := s.db.QueryRow(
		`SELECT COALESCE(SUM(total_requests), 0), COALESCE(SUM(total_prompt_tokens), 0), COALESCE(SUM(total_completion_tokens), 0), COALESCE(SUM(total_tokens), 0)
		FROM daily_usage_rollups
		WHERE provider_id = ? AND date = ?`,
		providerID,
		day.UTC().Format("2006-01-02"),
	)
	return scanSummary(row)
}

func (s *Store) WeeklySummary(providerID string, now time.Time) (Summary, error) {
	end := now.UTC().Format("2006-01-02")
	start := now.UTC().AddDate(0, 0, -6).Format("2006-01-02")
	row := s.db.QueryRow(
		`SELECT COALESCE(SUM(total_requests), 0), COALESCE(SUM(total_prompt_tokens), 0), COALESCE(SUM(total_completion_tokens), 0), COALESCE(SUM(total_tokens), 0)
		FROM daily_usage_rollups
		WHERE provider_id = ? AND date >= ? AND date <= ?`,
		providerID,
		start,
		end,
	)
	return scanSummary(row)
}

func (s *Store) CurrentProviderSummary(sessionKey, providerID, modelName string, now time.Time) (ProviderSummary, error) {
	var summary ProviderSummary
	summary.ProviderID = providerID
	summary.ModelName = modelName
	summary.SessionKey = sessionKey

	row := s.db.QueryRow(
		`SELECT COALESCE(COUNT(*), 0), COALESCE(SUM(total_tokens), 0)
		FROM usage_records
		WHERE session_key = ? AND provider_id = ? AND model_name = ?`,
		sessionKey,
		providerID,
		modelName,
	)
	if err := row.Scan(&summary.SessionRequests, &summary.SessionTokens); err != nil {
		return ProviderSummary{}, fmt.Errorf("scan session provider summary: %w", err)
	}

	dayKey := now.UTC().Format("2006-01-02")
	row = s.db.QueryRow(
		`SELECT COALESCE(SUM(total_requests), 0), COALESCE(SUM(total_tokens), 0)
		FROM daily_usage_rollups
		WHERE provider_id = ? AND model_name = ? AND date = ?`,
		providerID,
		modelName,
		dayKey,
	)
	if err := row.Scan(&summary.TodayRequests, &summary.TodayTokens); err != nil {
		return ProviderSummary{}, fmt.Errorf("scan daily provider summary: %w", err)
	}

	startKey := now.UTC().AddDate(0, 0, -6).Format("2006-01-02")
	row = s.db.QueryRow(
		`SELECT COALESCE(SUM(total_requests), 0), COALESCE(SUM(total_tokens), 0)
		FROM daily_usage_rollups
		WHERE provider_id = ? AND model_name = ? AND date >= ? AND date <= ?`,
		providerID,
		modelName,
		startKey,
		dayKey,
	)
	if err := row.Scan(&summary.WeeklyRequests, &summary.WeeklyTokens); err != nil {
		return ProviderSummary{}, fmt.Errorf("scan weekly provider summary: %w", err)
	}

	budgetStatus, warningLevel, err := s.currentBudgetState(providerID, now)
	if err != nil {
		return ProviderSummary{}, err
	}
	summary.BudgetStatus = budgetStatus
	summary.WarningLevel = warningLevel

	quota, err := s.LatestQuotaSummary(providerID)
	switch {
	case err == nil:
		summary.Quota = &quota
	case err == sql.ErrNoRows:
	default:
		return ProviderSummary{}, fmt.Errorf("load latest quota summary: %w", err)
	}
	return summary, nil
}

func scanSummary(row *sql.Row) (Summary, error) {
	var summary Summary
	if err := row.Scan(&summary.TotalRequests, &summary.TotalPromptTokens, &summary.TotalCompletionTokens, &summary.TotalTokens); err != nil {
		return Summary{}, fmt.Errorf("scan summary: %w", err)
	}
	return summary, nil
}
