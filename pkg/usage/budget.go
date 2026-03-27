package usage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

func (s *Store) UpsertBudget(ctx context.Context, budget Budget) error {
	if ctx == nil {
		ctx = context.Background()
	}
	thresholds, err := json.Marshal(sortedThresholds(budget.AlertThresholds))
	if err != nil {
		return fmt.Errorf("marshal alert thresholds: %w", err)
	}

	if _, err := s.db.ExecContext(
		ctx,
		`INSERT INTO budgets (
			id, name, budget_type, limit_amount, limit_unit, scope_type, scope_target,
			alert_thresholds, is_active, window_start, window_end, resets_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET
			name = excluded.name,
			budget_type = excluded.budget_type,
			limit_amount = excluded.limit_amount,
			limit_unit = excluded.limit_unit,
			scope_type = excluded.scope_type,
			scope_target = excluded.scope_target,
			alert_thresholds = excluded.alert_thresholds,
			is_active = excluded.is_active,
			window_start = excluded.window_start,
			window_end = excluded.window_end,
			resets_at = excluded.resets_at,
			updated_at = CURRENT_TIMESTAMP`,
		budget.ID,
		budget.Name,
		budget.BudgetType,
		budget.LimitAmount,
		budget.LimitUnit,
		budget.ScopeType,
		budget.ScopeTarget,
		string(thresholds),
		boolToInt(budget.IsActive),
		budget.WindowStart,
		budget.WindowEnd,
		budget.ResetsAt,
	); err != nil {
		return fmt.Errorf("upsert budget: %w", err)
	}
	return nil
}

func (s *Store) ListBudgetAlerts(budgetID string) ([]BudgetAlert, error) {
	rows, err := s.db.Query(
		`SELECT id, budget_id, alert_type, threshold_percent, tokens_at_alert, sent_at, channel
		FROM budget_alerts
		WHERE budget_id = ?
		ORDER BY sent_at ASC, id ASC`,
		budgetID,
	)
	if err != nil {
		return nil, fmt.Errorf("query budget alerts: %w", err)
	}
	defer rows.Close()

	var alerts []BudgetAlert
	for rows.Next() {
		var alert BudgetAlert
		if err := rows.Scan(&alert.ID, &alert.BudgetID, &alert.AlertType, &alert.ThresholdPercent, &alert.TokensAtAlert, &alert.SentAt, &alert.Channel); err != nil {
			return nil, fmt.Errorf("scan budget alert: %w", err)
		}
		alerts = append(alerts, alert)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate budget alerts: %w", err)
	}
	return alerts, nil
}

func (s *Store) processBudgetAlertsTx(ctx context.Context, tx *sql.Tx, record CompletionRecord, recordedAt time.Time) error {
	rows, err := tx.QueryContext(
		ctx,
		`SELECT id, name, budget_type, limit_amount, limit_unit, scope_type, scope_target, alert_thresholds, is_active
		FROM budgets
		WHERE is_active = 1`,
	)
	if err != nil {
		return fmt.Errorf("query active budgets: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var budget Budget
		var thresholdJSON string
		var active int
		if err := rows.Scan(&budget.ID, &budget.Name, &budget.BudgetType, &budget.LimitAmount, &budget.LimitUnit, &budget.ScopeType, &budget.ScopeTarget, &thresholdJSON, &active); err != nil {
			return fmt.Errorf("scan budget: %w", err)
		}
		budget.IsActive = active == 1
		if err := json.Unmarshal([]byte(thresholdJSON), &budget.AlertThresholds); err != nil {
			return fmt.Errorf("unmarshal budget thresholds: %w", err)
		}
		if !budgetMatchesRecord(budget, record) {
			continue
		}
		if err := processBudgetThresholdsTx(ctx, tx, budget, recordedAt); err != nil {
			return err
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate budgets: %w", err)
	}
	return nil
}

func processBudgetThresholdsTx(ctx context.Context, tx *sql.Tx, budget Budget, recordedAt time.Time) error {
	totalTokens, windowStart, windowEnd, err := budgetTotalTokensTx(ctx, tx, budget, recordedAt)
	if err != nil {
		return err
	}
	if budget.LimitAmount <= 0 {
		return nil
	}

	for _, threshold := range sortedThresholds(budget.AlertThresholds) {
		required := int(float64(threshold) * budget.LimitAmount / 100.0)
		if totalTokens < required {
			continue
		}
		var exists int
		if err := tx.QueryRowContext(
			ctx,
			`SELECT COUNT(1)
			FROM budget_alerts
			WHERE budget_id = ? AND threshold_percent = ? AND sent_at >= ? AND sent_at < ?`,
			budget.ID,
			threshold,
			windowStart,
			windowEnd,
		).Scan(&exists); err != nil {
			return fmt.Errorf("query alert dedupe: %w", err)
		}
		if exists > 0 {
			continue
		}
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO budget_alerts (budget_id, alert_type, threshold_percent, tokens_at_alert, message, sent_at, channel)
			VALUES (?, 'threshold', ?, ?, ?, ?, 'ui')`,
			budget.ID,
			threshold,
			totalTokens,
			fmt.Sprintf("%s reached %d%% of budget", budget.Name, threshold),
			recordedAt.UTC(),
		); err != nil {
			return fmt.Errorf("insert budget alert: %w", err)
		}
	}
	return nil
}

func (s *Store) currentBudgetState(providerID string, now time.Time) (string, string, error) {
	row := s.db.QueryRow(
		`SELECT threshold_percent
		FROM budget_alerts ba
		JOIN budgets b ON b.id = ba.budget_id
		WHERE b.scope_type = 'provider' AND b.scope_target = ? AND ba.sent_at >= ? AND ba.sent_at < ?
		ORDER BY threshold_percent DESC, sent_at DESC
		LIMIT 1`,
		providerID,
		startOfDay(now.UTC()),
		startOfDay(now.UTC()).Add(24*time.Hour),
	)

	var threshold int
	if err := row.Scan(&threshold); err != nil {
		if err == sql.ErrNoRows {
			return "", "", nil
		}
		return "", "", fmt.Errorf("query current budget state: %w", err)
	}

	switch {
	case threshold >= 95:
		return "warning", "critical", nil
	case threshold >= 80:
		return "warning", "high", nil
	case threshold >= 50:
		return "warning", "medium", nil
	default:
		return "", "", nil
	}
}

func budgetTotalTokensTx(ctx context.Context, tx *sql.Tx, budget Budget, recordedAt time.Time) (int, time.Time, time.Time, error) {
	windowStart, windowEnd := budgetWindow(budget.BudgetType, recordedAt.UTC())
	if budget.ScopeType != "provider" || budget.LimitUnit != "tokens" {
		return 0, windowStart, windowEnd, nil
	}
	row := tx.QueryRowContext(
		ctx,
		`SELECT COALESCE(SUM(total_tokens), 0)
		FROM usage_records
		WHERE provider_id = ? AND created_at >= ? AND created_at < ?`,
		budget.ScopeTarget,
		windowStart,
		windowEnd,
	)
	var total int
	if err := row.Scan(&total); err != nil {
		return 0, windowStart, windowEnd, fmt.Errorf("scan budget totals: %w", err)
	}
	return total, windowStart, windowEnd, nil
}

func budgetMatchesRecord(budget Budget, record CompletionRecord) bool {
	if !budget.IsActive {
		return false
	}
	if budget.ScopeType == "provider" {
		return budget.ScopeTarget == record.ProviderID
	}
	return false
}

func budgetWindow(kind string, recordedAt time.Time) (time.Time, time.Time) {
	switch kind {
	case "daily":
		start := startOfDay(recordedAt)
		return start, start.Add(24 * time.Hour)
	default:
		start := startOfDay(recordedAt)
		return start, start.Add(24 * time.Hour)
	}
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func sortedThresholds(values []int) []int {
	out := append([]int(nil), values...)
	sort.Ints(out)
	return out
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
