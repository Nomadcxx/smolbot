package usage

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func (s *Store) SaveQuotaSummary(ctx context.Context, summary QuotaSummary) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("usage store unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	summary.ProviderID = strings.TrimSpace(summary.ProviderID)
	if summary.ProviderID == "" {
		return fmt.Errorf("provider id is required")
	}
	if summary.State == "" {
		summary.State = QuotaStateUnavailable
	}
	if summary.FetchedAt.IsZero() {
		summary.FetchedAt = time.Now().UTC()
	} else {
		summary.FetchedAt = summary.FetchedAt.UTC()
	}
	if summary.ExpiresAt.IsZero() {
		summary.ExpiresAt = summary.FetchedAt
	} else {
		summary.ExpiresAt = summary.ExpiresAt.UTC()
	}

	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO quota_summaries (
			provider_id, account_name, account_email, plan_name,
			session_used_percent, session_resets_at, weekly_used_percent, weekly_resets_at,
			notify_usage_limits, state, source, fetched_at, expires_at,
			identity_state, identity_source, identity_account_name, identity_account_email, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(provider_id) DO UPDATE SET
			account_name = excluded.account_name,
			account_email = excluded.account_email,
			plan_name = excluded.plan_name,
			session_used_percent = excluded.session_used_percent,
			session_resets_at = excluded.session_resets_at,
			weekly_used_percent = excluded.weekly_used_percent,
			weekly_resets_at = excluded.weekly_resets_at,
			notify_usage_limits = excluded.notify_usage_limits,
			state = excluded.state,
			source = excluded.source,
			fetched_at = excluded.fetched_at,
			expires_at = excluded.expires_at,
			identity_state = excluded.identity_state,
			identity_source = excluded.identity_source,
			identity_account_name = excluded.identity_account_name,
			identity_account_email = excluded.identity_account_email,
			updated_at = excluded.updated_at`,
		summary.ProviderID,
		summary.AccountName,
		summary.AccountEmail,
		summary.PlanName,
		summary.SessionUsedPercent,
		nullableTime(summary.SessionResetsAt),
		summary.WeeklyUsedPercent,
		nullableTime(summary.WeeklyResetsAt),
		summary.NotifyUsageLimits,
		string(summary.State),
		string(summary.Source),
		summary.FetchedAt,
		summary.ExpiresAt,
		string(summary.IdentityState),
		string(summary.IdentitySource),
		summary.IdentityAccountName,
		summary.IdentityAccountEmail,
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("upsert quota summary: %w", err)
	}
	return nil
}

func (s *Store) LatestQuotaSummary(providerID string) (QuotaSummary, error) {
	if s == nil || s.db == nil {
		return QuotaSummary{}, fmt.Errorf("usage store unavailable")
	}

	var summary QuotaSummary
	var sessionResetsAt sql.NullTime
	var weeklyResetsAt sql.NullTime

	err := s.db.QueryRow(
		`SELECT
			provider_id, account_name, account_email, plan_name,
			session_used_percent, session_resets_at, weekly_used_percent, weekly_resets_at,
			notify_usage_limits, state, source, fetched_at, expires_at,
			identity_state, identity_source, identity_account_name, identity_account_email
		FROM quota_summaries
		WHERE provider_id = ?`,
		strings.TrimSpace(providerID),
	).Scan(
		&summary.ProviderID,
		&summary.AccountName,
		&summary.AccountEmail,
		&summary.PlanName,
		&summary.SessionUsedPercent,
		&sessionResetsAt,
		&summary.WeeklyUsedPercent,
		&weeklyResetsAt,
		&summary.NotifyUsageLimits,
		&summary.State,
		&summary.Source,
		&summary.FetchedAt,
		&summary.ExpiresAt,
		&summary.IdentityState,
		&summary.IdentitySource,
		&summary.IdentityAccountName,
		&summary.IdentityAccountEmail,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return QuotaSummary{}, err
		}
		return QuotaSummary{}, fmt.Errorf("query latest quota summary: %w", err)
	}

	summary.FetchedAt = summary.FetchedAt.UTC()
	summary.ExpiresAt = summary.ExpiresAt.UTC()
	if sessionResetsAt.Valid {
		t := sessionResetsAt.Time.UTC()
		summary.SessionResetsAt = &t
	}
	if weeklyResetsAt.Valid {
		t := weeklyResetsAt.Time.UTC()
		summary.WeeklyResetsAt = &t
	}
	return summary, nil
}

func nullableTime(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return t.UTC()
}
