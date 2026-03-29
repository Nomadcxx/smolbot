package usage

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func updateDailyRollupTx(ctx context.Context, tx *sql.Tx, record CompletionRecord, recordedAt time.Time) error {
	dateKey := recordedAt.UTC().Format("2006-01-02")
	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO daily_usage_rollups (
			date, provider_id, model_name, session_key,
			total_requests, total_prompt_tokens, total_completion_tokens, total_tokens
		) VALUES (?, ?, ?, ?, 1, ?, ?, ?)
		ON CONFLICT(date, provider_id, model_name, session_key) DO UPDATE SET
			total_requests = total_requests + 1,
			total_prompt_tokens = total_prompt_tokens + excluded.total_prompt_tokens,
			total_completion_tokens = total_completion_tokens + excluded.total_completion_tokens,
			total_tokens = total_tokens + excluded.total_tokens`,
		dateKey,
		record.ProviderID,
		record.ModelName,
		record.SessionKey,
		record.PromptTokens,
		record.CompletionTokens,
		record.TotalTokens,
	); err != nil {
		return fmt.Errorf("upsert daily rollup: %w", err)
	}
	return nil
}
