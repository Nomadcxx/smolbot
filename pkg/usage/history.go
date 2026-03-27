package usage

import (
	"context"
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
	if _, err := s.db.ExecContext(ctx, `DELETE FROM usage_records WHERE created_at < ?`, cutoff.UTC()); err != nil {
		return fmt.Errorf("prune usage records: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM historical_usage_samples WHERE sampled_at < ?`, cutoff.UTC()); err != nil {
		return fmt.Errorf("prune historical samples: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `DELETE FROM daily_usage_rollups WHERE date < ?`, cutoff.UTC().Format("2006-01-02")); err != nil {
		return fmt.Errorf("prune daily rollups: %w", err)
	}
	return nil
}
