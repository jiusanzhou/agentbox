package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"go.zoe.im/agentbox/internal/model"
)

// --- Billing tables ---

const billingMigration = `
CREATE TABLE IF NOT EXISTS subscriptions (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	agent_id TEXT NOT NULL,
	pricing_model TEXT NOT NULL,
	status TEXT NOT NULL DEFAULT 'active',
	stripe_sub_id TEXT,
	stripe_price_id TEXT,
	trial_ends_at DATETIME,
	current_period_start DATETIME NOT NULL,
	current_period_end DATETIME NOT NULL,
	canceled_at DATETIME,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
	updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_subscriptions_user ON subscriptions(user_id);
CREATE INDEX IF NOT EXISTS idx_subscriptions_agent ON subscriptions(agent_id);
CREATE UNIQUE INDEX IF NOT EXISTS idx_subscriptions_user_agent_active ON subscriptions(user_id, agent_id) WHERE status IN ('active', 'trialing');

CREATE TABLE IF NOT EXISTS usage_records (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	agent_id TEXT NOT NULL,
	run_id TEXT,
	model TEXT NOT NULL,
	input_tokens INTEGER NOT NULL DEFAULT 0,
	output_tokens INTEGER NOT NULL DEFAULT 0,
	total_tokens INTEGER NOT NULL DEFAULT 0,
	cost_micros INTEGER NOT NULL DEFAULT 0,
	duration_ms INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_usage_user ON usage_records(user_id);
CREATE INDEX IF NOT EXISTS idx_usage_agent ON usage_records(agent_id);
CREATE INDEX IF NOT EXISTS idx_usage_created ON usage_records(created_at);

CREATE TABLE IF NOT EXISTS author_payouts (
	id TEXT PRIMARY KEY,
	author_user_id TEXT NOT NULL,
	agent_id TEXT NOT NULL,
	period TEXT NOT NULL,
	gross_revenue INTEGER NOT NULL DEFAULT 0,
	platform_fee INTEGER NOT NULL DEFAULT 0,
	author_earnings INTEGER NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'pending',
	stripe_payout_id TEXT,
	paid_at DATETIME,
	created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_payouts_author_period ON author_payouts(author_user_id, period);
`

func (s *sqliteStore) migrateBilling() error {
	_, err := s.db.Exec(billingMigration)
	return err
}

// --- Subscriptions ---

func (s *sqliteStore) CreateSubscription(ctx context.Context, sub *model.Subscription) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO subscriptions (id, user_id, agent_id, pricing_model, status, stripe_sub_id, stripe_price_id, trial_ends_at, current_period_start, current_period_end, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sub.ID, sub.UserID, sub.AgentID, sub.PricingModel, sub.Status,
		sub.StripeSubID, sub.StripePriceID, sub.TrialEndsAt,
		sub.CurrentPeriodStart, sub.CurrentPeriodEnd, sub.CreatedAt, sub.UpdatedAt)
	return err
}

func (s *sqliteStore) GetSubscription(ctx context.Context, id string) (*model.Subscription, error) {
	var sub model.Subscription
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, agent_id, pricing_model, status, stripe_sub_id, stripe_price_id, trial_ends_at, current_period_start, current_period_end, canceled_at, created_at, updated_at FROM subscriptions WHERE id = ?`, id).
		Scan(&sub.ID, &sub.UserID, &sub.AgentID, &sub.PricingModel, &sub.Status,
			&sub.StripeSubID, &sub.StripePriceID, &sub.TrialEndsAt,
			&sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.CanceledAt, &sub.CreatedAt, &sub.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("subscription %s not found", id)
	}
	return &sub, err
}

func (s *sqliteStore) GetActiveSubscription(ctx context.Context, userID, agentID string) (*model.Subscription, error) {
	var sub model.Subscription
	err := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, agent_id, pricing_model, status, stripe_sub_id, stripe_price_id, trial_ends_at, current_period_start, current_period_end, canceled_at, created_at, updated_at
		 FROM subscriptions WHERE user_id = ? AND agent_id = ? AND status IN ('active', 'trialing') LIMIT 1`,
		userID, agentID).
		Scan(&sub.ID, &sub.UserID, &sub.AgentID, &sub.PricingModel, &sub.Status,
			&sub.StripeSubID, &sub.StripePriceID, &sub.TrialEndsAt,
			&sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.CanceledAt, &sub.CreatedAt, &sub.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no active subscription for user %s agent %s", userID, agentID)
	}
	return &sub, err
}

func (s *sqliteStore) UpdateSubscription(ctx context.Context, sub *model.Subscription) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE subscriptions SET status=?, stripe_sub_id=?, current_period_start=?, current_period_end=?, canceled_at=?, updated_at=? WHERE id=?`,
		sub.Status, sub.StripeSubID, sub.CurrentPeriodStart, sub.CurrentPeriodEnd, sub.CanceledAt, sub.UpdatedAt, sub.ID)
	return err
}

func (s *sqliteStore) ListSubscriptions(ctx context.Context, userID string) ([]*model.Subscription, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, agent_id, pricing_model, status, stripe_sub_id, stripe_price_id, trial_ends_at, current_period_start, current_period_end, canceled_at, created_at, updated_at
		 FROM subscriptions WHERE user_id = ? ORDER BY created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*model.Subscription
	for rows.Next() {
		var sub model.Subscription
		if err := rows.Scan(&sub.ID, &sub.UserID, &sub.AgentID, &sub.PricingModel, &sub.Status,
			&sub.StripeSubID, &sub.StripePriceID, &sub.TrialEndsAt,
			&sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.CanceledAt, &sub.CreatedAt, &sub.UpdatedAt); err != nil {
			return nil, err
		}
		result = append(result, &sub)
	}
	return result, rows.Err()
}

// --- Usage Records ---

func (s *sqliteStore) CreateUsageRecord(ctx context.Context, rec *model.UsageRecord) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO usage_records (id, user_id, agent_id, run_id, model, input_tokens, output_tokens, total_tokens, cost_micros, duration_ms, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.ID, rec.UserID, rec.AgentID, rec.RunID, rec.Model,
		rec.InputTokens, rec.OutputTokens, rec.TotalTokens, rec.CostMicros, rec.Duration, rec.CreatedAt)
	return err
}

func (s *sqliteStore) GetUsageSummary(ctx context.Context, userID, agentID, period string) (*model.UsageSummary, error) {
	summary := &model.UsageSummary{
		UserID:  userID,
		AgentID: agentID,
		Period:  period,
	}

	// Totals
	q := `SELECT COUNT(*), COALESCE(SUM(total_tokens),0), COALESCE(SUM(cost_micros),0)
		  FROM usage_records WHERE user_id = ?`
	args := []any{userID}
	if agentID != "" {
		q += " AND agent_id = ?"
		args = append(args, agentID)
	}
	if period != "" {
		q += " AND strftime('%Y-%m', created_at) = ?"
		args = append(args, period)
	}

	err := s.db.QueryRowContext(ctx, q, args...).Scan(&summary.TotalRuns, &summary.TotalTokens, &summary.TotalCostMicros)
	if err != nil {
		return nil, err
	}

	// By model
	q2 := `SELECT model, COUNT(*), COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0), COALESCE(SUM(cost_micros),0)
		   FROM usage_records WHERE user_id = ?`
	args2 := []any{userID}
	if agentID != "" {
		q2 += " AND agent_id = ?"
		args2 = append(args2, agentID)
	}
	if period != "" {
		q2 += " AND strftime('%Y-%m', created_at) = ?"
		args2 = append(args2, period)
	}
	q2 += " GROUP BY model"

	rows, err := s.db.QueryContext(ctx, q2, args2...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var mu model.ModelUsage
		if err := rows.Scan(&mu.Model, &mu.Runs, &mu.InputTokens, &mu.OutputTokens, &mu.CostMicros); err != nil {
			return nil, err
		}
		summary.ByModel = append(summary.ByModel, mu)
	}

	return summary, rows.Err()
}

func (s *sqliteStore) ListUsageRecords(ctx context.Context, opts model.BillingListOptions) ([]*model.UsageRecord, error) {
	q := `SELECT id, user_id, agent_id, run_id, model, input_tokens, output_tokens, total_tokens, cost_micros, duration_ms, created_at
		  FROM usage_records WHERE 1=1`
	var args []any

	if opts.UserID != "" {
		q += " AND user_id = ?"
		args = append(args, opts.UserID)
	}
	if opts.AgentID != "" {
		q += " AND agent_id = ?"
		args = append(args, opts.AgentID)
	}
	if opts.Period != "" {
		q += " AND strftime('%Y-%m', created_at) = ?"
		args = append(args, opts.Period)
	}

	q += " ORDER BY created_at DESC"
	if opts.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", opts.Limit)
	}
	if opts.Offset > 0 {
		q += fmt.Sprintf(" OFFSET %d", opts.Offset)
	}

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*model.UsageRecord
	for rows.Next() {
		var rec model.UsageRecord
		if err := rows.Scan(&rec.ID, &rec.UserID, &rec.AgentID, &rec.RunID, &rec.Model,
			&rec.InputTokens, &rec.OutputTokens, &rec.TotalTokens, &rec.CostMicros, &rec.Duration, &rec.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, &rec)
	}
	return result, rows.Err()
}

// --- Author Payouts ---

func (s *sqliteStore) CreateAuthorPayout(ctx context.Context, payout *model.AuthorPayout) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO author_payouts (id, author_user_id, agent_id, period, gross_revenue, platform_fee, author_earnings, status, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		payout.ID, payout.AuthorUserID, payout.AgentID, payout.Period,
		payout.GrossRevenue, payout.PlatformFee, payout.AuthorEarnings, payout.Status, payout.CreatedAt)
	return err
}

func (s *sqliteStore) GetAuthorPayout(ctx context.Context, authorUserID, period string) (*model.AuthorPayout, error) {
	var p model.AuthorPayout
	err := s.db.QueryRowContext(ctx,
		`SELECT id, author_user_id, agent_id, period, gross_revenue, platform_fee, author_earnings, status, stripe_payout_id, paid_at, created_at
		 FROM author_payouts WHERE author_user_id = ? AND period = ?`, authorUserID, period).
		Scan(&p.ID, &p.AuthorUserID, &p.AgentID, &p.Period,
			&p.GrossRevenue, &p.PlatformFee, &p.AuthorEarnings, &p.Status, &p.StripePayoutID, &p.PaidAt, &p.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("payout not found")
	}
	return &p, err
}

func (s *sqliteStore) ListAuthorPayouts(ctx context.Context, authorUserID string) ([]*model.AuthorPayout, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, author_user_id, agent_id, period, gross_revenue, platform_fee, author_earnings, status, stripe_payout_id, paid_at, created_at
		 FROM author_payouts WHERE author_user_id = ? ORDER BY period DESC`, authorUserID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []*model.AuthorPayout
	for rows.Next() {
		var p model.AuthorPayout
		if err := rows.Scan(&p.ID, &p.AuthorUserID, &p.AgentID, &p.Period,
			&p.GrossRevenue, &p.PlatformFee, &p.AuthorEarnings, &p.Status, &p.StripePayoutID, &p.PaidAt, &p.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, &p)
	}
	return result, rows.Err()
}
