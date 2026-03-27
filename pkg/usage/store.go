package usage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

const schema = `
CREATE TABLE IF NOT EXISTS usage_records (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_key TEXT NOT NULL,
	provider_id TEXT NOT NULL,
	model_name TEXT NOT NULL,
	request_type TEXT NOT NULL,
	prompt_tokens INTEGER NOT NULL DEFAULT 0,
	completion_tokens INTEGER NOT NULL DEFAULT 0,
	total_tokens INTEGER NOT NULL DEFAULT 0,
	duration_ms INTEGER NOT NULL DEFAULT 0,
	status TEXT NOT NULL,
	usage_source TEXT NOT NULL,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_usage_records_session_created ON usage_records(session_key, created_at, id);
CREATE INDEX IF NOT EXISTS idx_usage_records_provider_created ON usage_records(provider_id, created_at, id);
CREATE INDEX IF NOT EXISTS idx_usage_records_model_created ON usage_records(model_name, created_at, id);

CREATE TABLE IF NOT EXISTS daily_usage_rollups (
	date TEXT NOT NULL,
	provider_id TEXT NOT NULL,
	model_name TEXT NOT NULL DEFAULT '',
	session_key TEXT NOT NULL DEFAULT '',
	total_requests INTEGER NOT NULL DEFAULT 0,
	total_prompt_tokens INTEGER NOT NULL DEFAULT 0,
	total_completion_tokens INTEGER NOT NULL DEFAULT 0,
	total_tokens INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (date, provider_id, model_name, session_key)
);

CREATE TABLE IF NOT EXISTS budgets (
	id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	budget_type TEXT NOT NULL,
	limit_amount REAL NOT NULL,
	limit_unit TEXT NOT NULL,
	scope_type TEXT NOT NULL,
	scope_target TEXT NOT NULL DEFAULT '',
	account_key TEXT NOT NULL DEFAULT '',
	alert_thresholds TEXT NOT NULL DEFAULT '[]',
	alert_channels TEXT NOT NULL DEFAULT '[]',
	webhook_url TEXT NOT NULL DEFAULT '',
	current_spend REAL NOT NULL DEFAULT 0,
	current_tokens INTEGER NOT NULL DEFAULT 0,
	is_active INTEGER NOT NULL DEFAULT 1,
	window_start DATETIME,
	window_end DATETIME,
	resets_at DATETIME,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	last_alert_sent_at DATETIME,
	alert_count INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS budget_alerts (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	budget_id TEXT NOT NULL,
	alert_type TEXT NOT NULL,
	threshold_percent INTEGER,
	spend_at_alert REAL,
	tokens_at_alert INTEGER,
	message TEXT,
	sent_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	channel TEXT
);

CREATE INDEX IF NOT EXISTS idx_budget_alerts_budget_sent ON budget_alerts(budget_id, sent_at, id);

CREATE TABLE IF NOT EXISTS historical_usage_samples (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	provider_id TEXT NOT NULL,
	model_name TEXT NOT NULL DEFAULT '',
	account_key TEXT NOT NULL DEFAULT '',
	schema_version INTEGER NOT NULL DEFAULT 1,
	window_type TEXT NOT NULL,
	source TEXT NOT NULL,
	sampled_at DATETIME NOT NULL,
	used_percent REAL NOT NULL,
	resets_at DATETIME NOT NULL,
	window_minutes INTEGER NOT NULL,
	total_tokens INTEGER NOT NULL DEFAULT 0,
	total_cost REAL NOT NULL DEFAULT 0,
	provider_data TEXT NOT NULL DEFAULT '{}',
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_historical_usage_samples_provider_sampled ON historical_usage_samples(provider_id, sampled_at, id);
`

type Store struct {
	db *sql.DB
}

func NewStore(dsn string) (*Store, error) {
	db, err := sql.Open("sqlite3", sqliteDSN(dsn))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if dsn == ":memory:" {
		// SQLite in-memory databases are scoped to a single connection.
		// Constrain the pool so schema creation and later queries share it.
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
	}

	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) RecordCompletion(ctx context.Context, record CompletionRecord) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("usage store unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	record.normalize()
	if err := record.validate(); err != nil {
		return err
	}
	recordedAt := record.RecordedAt.UTC()
	if recordedAt.IsZero() {
		recordedAt = time.Now().UTC()
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO usage_records (
			session_key, provider_id, model_name, request_type,
			prompt_tokens, completion_tokens, total_tokens, duration_ms,
			status, usage_source, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.SessionKey,
		record.ProviderID,
		record.ModelName,
		record.RequestType,
		record.PromptTokens,
		record.CompletionTokens,
		record.TotalTokens,
		record.DurationMS,
		record.Status,
		record.UsageSource,
		recordedAt,
	); err != nil {
		return fmt.Errorf("insert usage record: %w", err)
	}

	if err := updateDailyRollupTx(ctx, tx, record, recordedAt); err != nil {
		return err
	}

	if err := s.processBudgetAlertsTx(ctx, tx, record, recordedAt); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func (s *Store) ListUsageRecords(sessionKey string) ([]UsageRecord, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("usage store unavailable")
	}

	query := `
		SELECT id, session_key, provider_id, model_name, request_type,
			prompt_tokens, completion_tokens, total_tokens, duration_ms,
			status, usage_source, created_at
		FROM usage_records
	`
	args := make([]any, 0, 1)
	if strings.TrimSpace(sessionKey) != "" {
		query += " WHERE session_key = ?"
		args = append(args, sessionKey)
	}
	query += " ORDER BY id ASC"

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query usage records: %w", err)
	}
	defer rows.Close()

	records := make([]UsageRecord, 0)
	for rows.Next() {
		var record UsageRecord
		if err := rows.Scan(
			&record.ID,
			&record.SessionKey,
			&record.ProviderID,
			&record.ModelName,
			&record.RequestType,
			&record.PromptTokens,
			&record.CompletionTokens,
			&record.TotalTokens,
			&record.DurationMS,
			&record.Status,
			&record.UsageSource,
			&record.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan usage record: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate usage records: %w", err)
	}
	return records, nil
}

func sqliteDSN(dsn string) string {
	if dsn == ":memory:" {
		return dsn
	}

	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	return dsn + separator + "_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on"
}

func (r *CompletionRecord) normalize() {
	if r.TotalTokens <= 0 {
		r.TotalTokens = r.PromptTokens + r.CompletionTokens
	}
	if r.RequestType == "" {
		r.RequestType = "chat"
	}
	if r.Status == "" {
		r.Status = "success"
	}
	if r.UsageSource == "" {
		r.UsageSource = "reported"
	}
}

func (r CompletionRecord) validate() error {
	switch {
	case strings.TrimSpace(r.SessionKey) == "":
		return fmt.Errorf("session key is required")
	case strings.TrimSpace(r.ProviderID) == "":
		return fmt.Errorf("provider id is required")
	case strings.TrimSpace(r.ModelName) == "":
		return fmt.Errorf("model name is required")
	case strings.TrimSpace(r.RequestType) == "":
		return fmt.Errorf("request type is required")
	case strings.TrimSpace(r.Status) == "":
		return fmt.Errorf("status is required")
	case strings.TrimSpace(r.UsageSource) == "":
		return fmt.Errorf("usage source is required")
	}
	return nil
}
