package usage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

const budgetSelectColumns = `SELECT
	id, name, budget_type, limit_amount, limit_unit, scope_type, scope_target,
	alert_thresholds, is_active, window_start, window_end, resets_at
	FROM budgets`

type budgetScanner interface {
	Scan(dest ...any) error
}

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

func (s *Store) GetBudget(ctx context.Context, id string) (Budget, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	budget, err := scanBudget(s.db.QueryRowContext(ctx, budgetSelectColumns+` WHERE id = ?`, id))
	if err != nil {
		if err == sql.ErrNoRows {
			return Budget{}, err
		}
		return Budget{}, fmt.Errorf("query budget: %w", err)
	}
	return budget, nil
}

func (s *Store) ListBudgets(ctx context.Context) ([]Budget, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	rows, err := s.db.QueryContext(ctx, budgetSelectColumns+` ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("query budgets: %w", err)
	}
	defer rows.Close()

	var budgets []Budget
	for rows.Next() {
		budget, err := scanBudget(rows)
		if err != nil {
			return nil, fmt.Errorf("scan budget: %w", err)
		}
		budgets = append(budgets, budget)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate budgets: %w", err)
	}
	return budgets, nil
}

func (s *Store) DeleteBudget(ctx context.Context, id string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin delete budget transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM budget_alerts WHERE budget_id = ?`, id); err != nil {
		return fmt.Errorf("delete budget alerts: %w", err)
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM budgets WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete budget: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("budget rows affected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit delete budget: %w", err)
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
		budgetSelectColumns+`
		WHERE is_active = 1
		ORDER BY id ASC`,
	)
	if err != nil {
		return fmt.Errorf("query active budgets: %w", err)
	}
	defer rows.Close()

	var budgets []Budget
	for rows.Next() {
		budget, err := scanBudget(rows)
		if err != nil {
			return fmt.Errorf("scan budget: %w", err)
		}
		budgets = append(budgets, budget)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate budgets: %w", err)
	}
	for _, budget := range budgets {
		if !budgetActiveAt(budget, recordedAt.UTC()) {
			continue
		}
		if !budgetMatchesRecord(budget, record) {
			continue
		}
		if err := processBudgetThresholdsTx(ctx, tx, budget, recordedAt); err != nil {
			return err
		}
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
	rows, err := s.db.Query(
		budgetSelectColumns+`
		WHERE scope_type = 'provider' AND scope_target = ? AND is_active = 1
		ORDER BY created_at ASC, id ASC`,
		providerID,
	)
	if err != nil {
		return "", "", fmt.Errorf("query provider budgets: %w", err)
	}
	defer rows.Close()

	var budgets []Budget
	for rows.Next() {
		budget, err := scanBudget(rows)
		if err != nil {
			return "", "", fmt.Errorf("scan provider budget: %w", err)
		}
		budgets = append(budgets, budget)
	}
	if err := rows.Err(); err != nil {
		return "", "", fmt.Errorf("iterate provider budgets: %w", err)
	}

	highestThreshold := 0
	for _, budget := range budgets {
		if !budgetActiveAt(budget, now.UTC()) {
			continue
		}
		windowStart, windowEnd := budgetWindow(budget, now.UTC())
		threshold, err := budgetHighestAlertThreshold(s.db, budget.ID, windowStart, windowEnd)
		if err != nil {
			return "", "", err
		}
		if threshold > highestThreshold {
			highestThreshold = threshold
		}
	}
	switch {
	case highestThreshold >= 95:
		return "warning", "critical", nil
	case highestThreshold >= 80:
		return "warning", "high", nil
	case highestThreshold >= 50:
		return "warning", "medium", nil
	default:
		return "", "", nil
	}
}

func budgetTotalTokensTx(ctx context.Context, tx *sql.Tx, budget Budget, recordedAt time.Time) (int, time.Time, time.Time, error) {
	windowStart, windowEnd := budgetWindow(budget, recordedAt.UTC())
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

func budgetWindow(budget Budget, recordedAt time.Time) (time.Time, time.Time) {
	if budget.WindowStart != nil && budget.WindowEnd != nil && budget.WindowStart.Before(*budget.WindowEnd) {
		return budget.WindowStart.UTC(), budget.WindowEnd.UTC()
	}

	switch budget.BudgetType {
	case "hourly":
		start := recordedAt.UTC().Truncate(time.Hour)
		return start, start.Add(time.Hour)
	case "daily":
		start := startOfDay(recordedAt.UTC())
		return start, start.Add(24 * time.Hour)
	case "weekly":
		start := startOfWeek(recordedAt.UTC())
		return start, start.AddDate(0, 0, 7)
	case "monthly":
		start := startOfMonth(recordedAt.UTC())
		return start, start.AddDate(0, 1, 0)
	case "total":
		return time.Time{}, maxBudgetWindowEnd()
	default:
		start := startOfDay(recordedAt.UTC())
		return start, start.Add(24 * time.Hour)
	}
}

func startOfDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func startOfWeek(t time.Time) time.Time {
	start := startOfDay(t.UTC())
	offset := (int(start.Weekday()) + 6) % 7
	return start.AddDate(0, 0, -offset)
}

func startOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func maxBudgetWindowEnd() time.Time {
	return time.Date(9999, time.December, 31, 23, 59, 59, 0, time.UTC)
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

func scanBudget(scanner budgetScanner) (Budget, error) {
	var budget Budget
	var thresholdJSON string
	var active int
	var windowStart sql.NullTime
	var windowEnd sql.NullTime
	var resetsAt sql.NullTime

	if err := scanner.Scan(
		&budget.ID,
		&budget.Name,
		&budget.BudgetType,
		&budget.LimitAmount,
		&budget.LimitUnit,
		&budget.ScopeType,
		&budget.ScopeTarget,
		&thresholdJSON,
		&active,
		&windowStart,
		&windowEnd,
		&resetsAt,
	); err != nil {
		return Budget{}, err
	}
	budget.IsActive = active == 1
	if thresholdJSON == "" {
		thresholdJSON = "[]"
	}
	if err := json.Unmarshal([]byte(thresholdJSON), &budget.AlertThresholds); err != nil {
		return Budget{}, fmt.Errorf("unmarshal budget thresholds: %w", err)
	}
	budget.AlertThresholds = sortedThresholds(budget.AlertThresholds)
	if windowStart.Valid {
		value := windowStart.Time.UTC()
		budget.WindowStart = &value
	}
	if windowEnd.Valid {
		value := windowEnd.Time.UTC()
		budget.WindowEnd = &value
	}
	if resetsAt.Valid {
		value := resetsAt.Time.UTC()
		budget.ResetsAt = &value
	}
	return budget, nil
}

func budgetHighestAlertThreshold(db queryRower, budgetID string, windowStart, windowEnd time.Time) (int, error) {
	row := db.QueryRow(
		`SELECT COALESCE(MAX(threshold_percent), 0)
		FROM budget_alerts
		WHERE budget_id = ? AND sent_at >= ? AND sent_at < ?`,
		budgetID,
		windowStart,
		windowEnd,
	)
	var threshold int
	if err := row.Scan(&threshold); err != nil {
		return 0, fmt.Errorf("query highest budget alert threshold: %w", err)
	}
	return threshold, nil
}

type queryRower interface {
	QueryRow(query string, args ...any) *sql.Row
}

func budgetActiveAt(budget Budget, now time.Time) bool {
	if !budget.IsActive {
		return false
	}
	if budget.WindowStart != nil && now.Before(budget.WindowStart.UTC()) {
		return false
	}
	if budget.WindowEnd != nil && !now.Before(budget.WindowEnd.UTC()) {
		return false
	}
	return true
}
