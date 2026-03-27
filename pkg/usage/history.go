package usage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func (s *Store) RecordHistoricalSample(ctx context.Context, sample HistoricalUsageSample) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, err := s.db.ExecContext(
		ctx,
		`INSERT INTO historical_usage_samples (
			provider_id, model_name, window_type, source, sampled_at, used_percent,
			resets_at, window_minutes, total_tokens
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sample.ProviderID,
		sample.ModelName,
		sample.WindowType,
		sample.Source,
		sample.SampledAt.UTC(),
		sample.UsedPercent,
		sample.ResetsAt.UTC(),
		sample.WindowMinutes,
		sample.TotalTokens,
	); err != nil {
		return fmt.Errorf("insert historical sample: %w", err)
	}
	return nil
}

func (s *Store) ListHistoricalSamples(providerID string) ([]HistoricalUsageSample, error) {
	rows, err := s.db.Query(
		`SELECT id, provider_id, model_name, window_type, source, sampled_at, used_percent, resets_at, window_minutes, total_tokens
		FROM historical_usage_samples
		WHERE provider_id = ?
		ORDER BY sampled_at ASC, id ASC`,
		providerID,
	)
	if err != nil {
		return nil, fmt.Errorf("query historical samples: %w", err)
	}
	defer rows.Close()

	var samples []HistoricalUsageSample
	for rows.Next() {
		var sample HistoricalUsageSample
		if err := rows.Scan(
			&sample.ID,
			&sample.ProviderID,
			&sample.ModelName,
			&sample.WindowType,
			&sample.Source,
			&sample.SampledAt,
			&sample.UsedPercent,
			&sample.ResetsAt,
			&sample.WindowMinutes,
			&sample.TotalTokens,
		); err != nil {
			return nil, fmt.Errorf("scan historical sample: %w", err)
		}
		samples = append(samples, sample)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate historical samples: %w", err)
	}
	return samples, nil
}

func (s *Store) PruneOlderThan(ctx context.Context, cutoff time.Time) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := s.pruneUsageRecordsOlderThan(ctx, cutoff.UTC()); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM historical_usage_samples WHERE sampled_at < ?`, cutoff.UTC()); err != nil {
		return fmt.Errorf("prune historical samples: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM daily_usage_rollups WHERE date < ?`, cutoff.UTC().Format("2006-01-02")); err != nil {
		return fmt.Errorf("prune daily rollups: %w", err)
	}
	if err := s.pruneBudgetAlertsOlderThan(ctx, cutoff.UTC()); err != nil {
		return err
	}
	return nil
}

func (s *Store) pruneUsageRecordsOlderThan(ctx context.Context, cutoff time.Time) error {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, provider_id, created_at
		FROM usage_records
		WHERE created_at < ?
		ORDER BY id ASC`,
		cutoff,
	)
	if err != nil {
		return fmt.Errorf("query usage records for pruning: %w", err)
	}
	defer rows.Close()

	type pruneCandidate struct {
		id         int64
		providerID string
		createdAt  time.Time
	}

	var candidates []pruneCandidate
	for rows.Next() {
		var candidate pruneCandidate
		if err := rows.Scan(&candidate.id, &candidate.providerID, &candidate.createdAt); err != nil {
			return fmt.Errorf("scan usage record prune candidate: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate usage records for pruning: %w", err)
	}

	budgets, err := s.ListBudgets(ctx)
	if err != nil {
		return fmt.Errorf("load budgets for usage pruning: %w", err)
	}

	for _, candidate := range candidates {
		if usageRecordNeededForActiveBudgetWindow(budgets, candidate.providerID, candidate.createdAt.UTC(), cutoff) {
			continue
		}
		if _, err := s.db.ExecContext(ctx, `DELETE FROM usage_records WHERE id = ?`, candidate.id); err != nil {
			return fmt.Errorf("delete usage record %d: %w", candidate.id, err)
		}
	}
	return nil
}

func (s *Store) pruneBudgetAlertsOlderThan(ctx context.Context, cutoff time.Time) error {
	rows, err := s.db.QueryContext(
		ctx,
		`SELECT id, budget_id, sent_at
		FROM budget_alerts
		WHERE sent_at < ?
		ORDER BY id ASC`,
		cutoff,
	)
	if err != nil {
		return fmt.Errorf("query budget alerts for pruning: %w", err)
	}
	defer rows.Close()

	type pruneCandidate struct {
		id       int64
		budgetID string
		sentAt   time.Time
	}

	var candidates []pruneCandidate
	for rows.Next() {
		var candidate pruneCandidate
		if err := rows.Scan(&candidate.id, &candidate.budgetID, &candidate.sentAt); err != nil {
			return fmt.Errorf("scan budget alert prune candidate: %w", err)
		}
		candidates = append(candidates, candidate)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate budget alerts for pruning: %w", err)
	}

	for _, candidate := range candidates {
		budget, err := s.GetBudget(ctx, candidate.budgetID)
		switch {
		case err == nil:
			_, windowEnd := budgetWindow(budget, candidate.sentAt.UTC())
			if windowEnd.After(cutoff) {
				continue
			}
		case err == sql.ErrNoRows:
			// Orphaned alerts are safe to prune.
		default:
			return fmt.Errorf("load budget for alert pruning: %w", err)
		}

		if _, err := s.db.ExecContext(ctx, `DELETE FROM budget_alerts WHERE id = ?`, candidate.id); err != nil {
			return fmt.Errorf("delete budget alert %d: %w", candidate.id, err)
		}
	}
	return nil
}

func usageRecordNeededForActiveBudgetWindow(budgets []Budget, providerID string, createdAt, cutoff time.Time) bool {
	for _, budget := range budgets {
		if budget.ScopeType != "provider" || budget.ScopeTarget != providerID || budget.LimitUnit != "tokens" {
			continue
		}
		if !budgetActiveAt(budget, createdAt) {
			continue
		}
		_, windowEnd := budgetWindow(budget, createdAt)
		if windowEnd.After(cutoff) {
			return true
		}
	}
	return false
}
