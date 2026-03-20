package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

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
	trial_ends_at TIMESTAMPTZ,
	current_period_start TIMESTAMPTZ NOT NULL,
	current_period_end TIMESTAMPTZ NOT NULL,
	canceled_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
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
	input_tokens BIGINT NOT NULL DEFAULT 0,
	output_tokens BIGINT NOT NULL DEFAULT 0,
	total_tokens BIGINT NOT NULL DEFAULT 0,
	cost_micros BIGINT NOT NULL DEFAULT 0,
	duration_ms BIGINT NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_usage_user ON usage_records(user_id);
CREATE INDEX IF NOT EXISTS idx_usage_agent ON usage_records(agent_id);
CREATE INDEX IF NOT EXISTS idx_usage_created ON usage_records(created_at);

CREATE TABLE IF NOT EXISTS author_payouts (
	id TEXT PRIMARY KEY,
	author_user_id TEXT NOT NULL,
	agent_id TEXT NOT NULL,
	period TEXT NOT NULL,
	gross_revenue BIGINT NOT NULL DEFAULT 0,
	platform_fee BIGINT NOT NULL DEFAULT 0,
	author_earnings BIGINT NOT NULL DEFAULT 0,
	status TEXT NOT NULL DEFAULT 'pending',
	stripe_payout_id TEXT,
	paid_at TIMESTAMPTZ,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_payouts_author_period ON author_payouts(author_user_id, period);

CREATE TABLE IF NOT EXISTS run_cost_breakdowns (
	run_id TEXT PRIMARY KEY,
	pricing_model TEXT NOT NULL DEFAULT '',
	duration_ms BIGINT NOT NULL DEFAULT 0,
	input_tokens BIGINT NOT NULL DEFAULT 0,
	output_tokens BIGINT NOT NULL DEFAULT 0,
	compute_micros BIGINT NOT NULL DEFAULT 0,
	token_micros BIGINT NOT NULL DEFAULT 0,
	agent_fee_micros BIGINT NOT NULL DEFAULT 0,
	creator_earnings_micros BIGINT NOT NULL DEFAULT 0,
	platform_fee_micros BIGINT NOT NULL DEFAULT 0,
	total_micros BIGINT NOT NULL DEFAULT 0,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS stripe_customers (
	user_id TEXT PRIMARY KEY,
	stripe_customer_id TEXT NOT NULL UNIQUE,
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS free_quota_usage (
	user_id TEXT NOT NULL,
	agent_id TEXT NOT NULL DEFAULT '',
	period TEXT NOT NULL,
	quota_limit BIGINT NOT NULL DEFAULT 0,
	used BIGINT NOT NULL DEFAULT 0,
	updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
	PRIMARY KEY (user_id, agent_id, period)
);

CREATE TABLE IF NOT EXISTS platform_usage_records (
	id TEXT PRIMARY KEY,
	user_id TEXT NOT NULL,
	team_id TEXT NOT NULL DEFAULT '',
	type TEXT NOT NULL,
	amount DOUBLE PRECISION NOT NULL DEFAULT 0,
	unit TEXT NOT NULL DEFAULT '',
	run_id TEXT NOT NULL DEFAULT '',
	session_id TEXT NOT NULL DEFAULT '',
	description TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_platform_usage_user ON platform_usage_records(user_id);
CREATE INDEX IF NOT EXISTS idx_platform_usage_type ON platform_usage_records(type);
CREATE INDEX IF NOT EXISTS idx_platform_usage_created ON platform_usage_records(created_at);

CREATE TABLE IF NOT EXISTS usage_quotas (
	user_id TEXT PRIMARY KEY,
	plan TEXT NOT NULL DEFAULT 'free',
	compute_limit DOUBLE PRECISION NOT NULL DEFAULT 6000,
	token_limit BIGINT NOT NULL DEFAULT 100000,
	storage_limit BIGINT NOT NULL DEFAULT 104857600,
	api_call_limit BIGINT NOT NULL DEFAULT 1000
);
`

// --- Subscriptions ---

func (s *pgStore) CreateSubscription(ctx context.Context, sub *model.Subscription) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO subscriptions (id, user_id, agent_id, pricing_model, status, stripe_sub_id, stripe_price_id, trial_ends_at, current_period_start, current_period_end, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		sub.ID, sub.UserID, sub.AgentID, sub.PricingModel, sub.Status,
		sub.StripeSubID, sub.StripePriceID, sub.TrialEndsAt,
		sub.CurrentPeriodStart, sub.CurrentPeriodEnd, sub.CreatedAt, sub.UpdatedAt)
	return err
}

func (s *pgStore) GetSubscription(ctx context.Context, id string) (*model.Subscription, error) {
	var sub model.Subscription
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, agent_id, pricing_model, status, stripe_sub_id, stripe_price_id, trial_ends_at, current_period_start, current_period_end, canceled_at, created_at, updated_at FROM subscriptions WHERE id = $1`, id).
		Scan(&sub.ID, &sub.UserID, &sub.AgentID, &sub.PricingModel, &sub.Status,
			&sub.StripeSubID, &sub.StripePriceID, &sub.TrialEndsAt,
			&sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.CanceledAt, &sub.CreatedAt, &sub.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("subscription %s not found", id)
	}
	return &sub, err
}

func (s *pgStore) GetActiveSubscription(ctx context.Context, userID, agentID string) (*model.Subscription, error) {
	var sub model.Subscription
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, agent_id, pricing_model, status, stripe_sub_id, stripe_price_id, trial_ends_at, current_period_start, current_period_end, canceled_at, created_at, updated_at
		 FROM subscriptions WHERE user_id = $1 AND agent_id = $2 AND status IN ('active', 'trialing') LIMIT 1`,
		userID, agentID).
		Scan(&sub.ID, &sub.UserID, &sub.AgentID, &sub.PricingModel, &sub.Status,
			&sub.StripeSubID, &sub.StripePriceID, &sub.TrialEndsAt,
			&sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.CanceledAt, &sub.CreatedAt, &sub.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("no active subscription for user %s agent %s", userID, agentID)
	}
	return &sub, err
}

func (s *pgStore) GetSubscriptionByStripeSubID(ctx context.Context, stripeSubID string) (*model.Subscription, error) {
	var sub model.Subscription
	err := s.pool.QueryRow(ctx,
		`SELECT id, user_id, agent_id, pricing_model, status, stripe_sub_id, stripe_price_id, trial_ends_at, current_period_start, current_period_end, canceled_at, created_at, updated_at
		 FROM subscriptions WHERE stripe_sub_id = $1 LIMIT 1`,
		stripeSubID).
		Scan(&sub.ID, &sub.UserID, &sub.AgentID, &sub.PricingModel, &sub.Status,
			&sub.StripeSubID, &sub.StripePriceID, &sub.TrialEndsAt,
			&sub.CurrentPeriodStart, &sub.CurrentPeriodEnd, &sub.CanceledAt, &sub.CreatedAt, &sub.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("no subscription with stripe_sub_id %s", stripeSubID)
	}
	return &sub, err
}

func (s *pgStore) UpdateSubscription(ctx context.Context, sub *model.Subscription) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE subscriptions SET status=$1, stripe_sub_id=$2, current_period_start=$3, current_period_end=$4, canceled_at=$5, updated_at=$6 WHERE id=$7`,
		sub.Status, sub.StripeSubID, sub.CurrentPeriodStart, sub.CurrentPeriodEnd, sub.CanceledAt, sub.UpdatedAt, sub.ID)
	return err
}

func (s *pgStore) ListSubscriptions(ctx context.Context, userID string) ([]*model.Subscription, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, agent_id, pricing_model, status, stripe_sub_id, stripe_price_id, trial_ends_at, current_period_start, current_period_end, canceled_at, created_at, updated_at
		 FROM subscriptions WHERE user_id = $1 ORDER BY created_at DESC`, userID)
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

func (s *pgStore) CreateUsageRecord(ctx context.Context, rec *model.UsageRecord) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO usage_records (id, user_id, agent_id, run_id, model, input_tokens, output_tokens, total_tokens, cost_micros, duration_ms, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		rec.ID, rec.UserID, rec.AgentID, rec.RunID, rec.Model,
		rec.InputTokens, rec.OutputTokens, rec.TotalTokens, rec.CostMicros, rec.Duration, rec.CreatedAt)
	return err
}

func (s *pgStore) GetUsageSummary(ctx context.Context, userID, agentID, period string) (*model.UsageSummary, error) {
	summary := &model.UsageSummary{
		UserID:  userID,
		AgentID: agentID,
		Period:  period,
	}

	// Totals
	q := `SELECT COUNT(*), COALESCE(SUM(total_tokens),0), COALESCE(SUM(cost_micros),0)
		  FROM usage_records WHERE user_id = $1`
	args := []any{userID}
	argN := 1

	if agentID != "" {
		argN++
		q += fmt.Sprintf(" AND agent_id = $%d", argN)
		args = append(args, agentID)
	}
	if period != "" {
		argN++
		q += fmt.Sprintf(" AND to_char(created_at, 'YYYY-MM') = $%d", argN)
		args = append(args, period)
	}

	err := s.pool.QueryRow(ctx, q, args...).Scan(&summary.TotalRuns, &summary.TotalTokens, &summary.TotalCostMicros)
	if err != nil {
		return nil, err
	}

	// By model
	q2 := `SELECT model, COUNT(*), COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0), COALESCE(SUM(cost_micros),0)
		   FROM usage_records WHERE user_id = $1`
	args2 := []any{userID}
	argN2 := 1

	if agentID != "" {
		argN2++
		q2 += fmt.Sprintf(" AND agent_id = $%d", argN2)
		args2 = append(args2, agentID)
	}
	if period != "" {
		argN2++
		q2 += fmt.Sprintf(" AND to_char(created_at, 'YYYY-MM') = $%d", argN2)
		args2 = append(args2, period)
	}
	q2 += " GROUP BY model"

	rows, err := s.pool.Query(ctx, q2, args2...)
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

func (s *pgStore) ListUsageRecords(ctx context.Context, opts model.BillingListOptions) ([]*model.UsageRecord, error) {
	q := `SELECT id, user_id, agent_id, run_id, model, input_tokens, output_tokens, total_tokens, cost_micros, duration_ms, created_at
		  FROM usage_records WHERE 1=1`
	var args []any
	argN := 0

	nextArg := func() string {
		argN++
		return fmt.Sprintf("$%d", argN)
	}

	if opts.UserID != "" {
		q += " AND user_id = " + nextArg()
		args = append(args, opts.UserID)
	}
	if opts.AgentID != "" {
		q += " AND agent_id = " + nextArg()
		args = append(args, opts.AgentID)
	}
	if opts.Period != "" {
		q += " AND to_char(created_at, 'YYYY-MM') = " + nextArg()
		args = append(args, opts.Period)
	}

	q += " ORDER BY created_at DESC"
	if opts.Limit > 0 {
		q += " LIMIT " + nextArg()
		args = append(args, opts.Limit)
	}
	if opts.Offset > 0 {
		q += " OFFSET " + nextArg()
		args = append(args, opts.Offset)
	}

	rows, err := s.pool.Query(ctx, q, args...)
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

func (s *pgStore) CreateAuthorPayout(ctx context.Context, payout *model.AuthorPayout) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO author_payouts (id, author_user_id, agent_id, period, gross_revenue, platform_fee, author_earnings, status, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		payout.ID, payout.AuthorUserID, payout.AgentID, payout.Period,
		payout.GrossRevenue, payout.PlatformFee, payout.AuthorEarnings, payout.Status, payout.CreatedAt)
	return err
}

func (s *pgStore) GetAuthorPayout(ctx context.Context, authorUserID, period string) (*model.AuthorPayout, error) {
	var p model.AuthorPayout
	err := s.pool.QueryRow(ctx,
		`SELECT id, author_user_id, agent_id, period, gross_revenue, platform_fee, author_earnings, status, stripe_payout_id, paid_at, created_at
		 FROM author_payouts WHERE author_user_id = $1 AND period = $2`, authorUserID, period).
		Scan(&p.ID, &p.AuthorUserID, &p.AgentID, &p.Period,
			&p.GrossRevenue, &p.PlatformFee, &p.AuthorEarnings, &p.Status, &p.StripePayoutID, &p.PaidAt, &p.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("payout not found")
	}
	return &p, err
}

func (s *pgStore) ListAuthorPayouts(ctx context.Context, authorUserID string) ([]*model.AuthorPayout, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, author_user_id, agent_id, period, gross_revenue, platform_fee, author_earnings, status, stripe_payout_id, paid_at, created_at
		 FROM author_payouts WHERE author_user_id = $1 ORDER BY period DESC`, authorUserID)
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

// --- Run Cost Breakdowns ---

func (s *pgStore) UpsertRunCostBreakdown(ctx context.Context, b *model.RunCostBreakdown) error {
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now()
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO run_cost_breakdowns
		 (run_id, pricing_model, duration_ms, input_tokens, output_tokens,
		  compute_micros, token_micros, agent_fee_micros, creator_earnings_micros,
		  platform_fee_micros, total_micros, created_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		 ON CONFLICT(run_id) DO UPDATE SET
		  pricing_model=EXCLUDED.pricing_model,
		  duration_ms=EXCLUDED.duration_ms,
		  input_tokens=EXCLUDED.input_tokens,
		  output_tokens=EXCLUDED.output_tokens,
		  compute_micros=EXCLUDED.compute_micros,
		  token_micros=EXCLUDED.token_micros,
		  agent_fee_micros=EXCLUDED.agent_fee_micros,
		  creator_earnings_micros=EXCLUDED.creator_earnings_micros,
		  platform_fee_micros=EXCLUDED.platform_fee_micros,
		  total_micros=EXCLUDED.total_micros`,
		b.RunID, b.PricingModel, b.DurationMs, b.InputTokens, b.OutputTokens,
		b.ComputeMicros, b.TokenMicros, b.AgentFeeMicros, b.CreatorEarningsMicros,
		b.PlatformFeeMicros, b.TotalMicros, b.CreatedAt)
	return err
}

func (s *pgStore) GetRunCostBreakdown(ctx context.Context, runID string) (*model.RunCostBreakdown, error) {
	var b model.RunCostBreakdown
	err := s.pool.QueryRow(ctx,
		`SELECT run_id, pricing_model, duration_ms, input_tokens, output_tokens,
		        compute_micros, token_micros, agent_fee_micros, creator_earnings_micros,
		        platform_fee_micros, total_micros, created_at
		 FROM run_cost_breakdowns WHERE run_id = $1`, runID).
		Scan(&b.RunID, &b.PricingModel, &b.DurationMs, &b.InputTokens, &b.OutputTokens,
			&b.ComputeMicros, &b.TokenMicros, &b.AgentFeeMicros, &b.CreatorEarningsMicros,
			&b.PlatformFeeMicros, &b.TotalMicros, &b.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("run cost breakdown %s not found", runID)
	}
	return &b, err
}

// --- Stripe Customers ---

func (s *pgStore) UpsertStripeCustomer(ctx context.Context, c *model.StripeCustomer) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO stripe_customers (user_id, stripe_customer_id, created_at)
		 VALUES ($1, $2, $3)
		 ON CONFLICT(user_id) DO UPDATE SET stripe_customer_id=EXCLUDED.stripe_customer_id`,
		c.UserID, c.StripeCustomerID, c.CreatedAt)
	return err
}

func (s *pgStore) GetStripeCustomer(ctx context.Context, userID string) (*model.StripeCustomer, error) {
	var c model.StripeCustomer
	err := s.pool.QueryRow(ctx,
		`SELECT user_id, stripe_customer_id, created_at FROM stripe_customers WHERE user_id = $1`, userID).
		Scan(&c.UserID, &c.StripeCustomerID, &c.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("stripe customer for user %s not found", userID)
	}
	return &c, err
}

// --- Free Quota ---

func (s *pgStore) GetFreeQuotaUsage(ctx context.Context, userID, agentID, period string) (*model.FreeQuotaUsage, error) {
	var q model.FreeQuotaUsage
	err := s.pool.QueryRow(ctx,
		`SELECT user_id, agent_id, period, quota_limit, used, updated_at
		 FROM free_quota_usage WHERE user_id=$1 AND agent_id=$2 AND period=$3`,
		userID, agentID, period).
		Scan(&q.UserID, &q.AgentID, &q.Period, &q.Limit, &q.Used, &q.UpdatedAt)
	if err == pgx.ErrNoRows {
		return &model.FreeQuotaUsage{UserID: userID, AgentID: agentID, Period: period}, nil
	}
	return &q, err
}

func (s *pgStore) IncrementFreeQuotaUsage(ctx context.Context, userID, agentID, period string, limit int64) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO free_quota_usage (user_id, agent_id, period, quota_limit, used, updated_at)
		 VALUES ($1, $2, $3, $4, 1, NOW())
		 ON CONFLICT(user_id, agent_id, period) DO UPDATE SET
		  used = free_quota_usage.used + 1,
		  quota_limit = EXCLUDED.quota_limit,
		  updated_at = NOW()`,
		userID, agentID, period, limit)
	return err
}

// --- Platform Usage Tracking ---

func (s *pgStore) RecordPlatformUsage(ctx context.Context, rec *model.PlatformUsageRecord) error {
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now()
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO platform_usage_records (id, user_id, team_id, type, amount, unit, run_id, session_id, description, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		rec.ID, rec.UserID, rec.TeamID, rec.Type, rec.Amount, rec.Unit,
		rec.RunID, rec.SessionID, rec.Description, rec.CreatedAt)
	return err
}

func (s *pgStore) GetPlatformUsageSummary(ctx context.Context, userID, period string) (*model.PlatformUsageSummary, error) {
	summary := &model.PlatformUsageSummary{UserID: userID, Period: period}

	q := `SELECT
		COALESCE(SUM(CASE WHEN type='compute' THEN amount ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN type='tokens' THEN amount::bigint ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN type='storage' THEN amount::bigint ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN type='api_call' THEN amount::bigint ELSE 0 END), 0)
	FROM platform_usage_records WHERE user_id = $1`
	args := []any{userID}
	argN := 1

	if len(period) == 7 {
		argN++
		q += fmt.Sprintf(" AND to_char(created_at, 'YYYY-MM') = $%d", argN)
		args = append(args, period)
	} else if len(period) == 10 {
		argN++
		q += fmt.Sprintf(" AND to_char(created_at, 'YYYY-MM-DD') = $%d", argN)
		args = append(args, period)
	}

	err := s.pool.QueryRow(ctx, q, args...).Scan(
		&summary.ComputeSeconds, &summary.TokenCount, &summary.StorageBytes, &summary.APICalls)
	if err != nil {
		return nil, err
	}

	summary.EstimatedCost = summary.ComputeSeconds*0.10/60 + float64(summary.TokenCount)*3.0/1_000_000
	return summary, nil
}

func (s *pgStore) GetPlatformUsageHistory(ctx context.Context, userID string, from, to time.Time, limit int) ([]model.PlatformUsageRecord, error) {
	q := `SELECT id, user_id, team_id, type, amount, unit, run_id, session_id, description, created_at
		  FROM platform_usage_records WHERE user_id = $1 AND created_at >= $2 AND created_at <= $3
		  ORDER BY created_at DESC`
	args := []any{userID, from, to}
	argN := 3
	if limit > 0 {
		argN++
		q += fmt.Sprintf(" LIMIT $%d", argN)
		args = append(args, limit)
	}

	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.PlatformUsageRecord
	for rows.Next() {
		var rec model.PlatformUsageRecord
		if err := rows.Scan(&rec.ID, &rec.UserID, &rec.TeamID, &rec.Type, &rec.Amount, &rec.Unit,
			&rec.RunID, &rec.SessionID, &rec.Description, &rec.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, rec)
	}
	return result, rows.Err()
}

func (s *pgStore) GetUsageQuota(ctx context.Context, userID string) (*model.UsageQuota, error) {
	var q model.UsageQuota
	err := s.pool.QueryRow(ctx,
		`SELECT user_id, plan, compute_limit, token_limit, storage_limit, api_call_limit FROM usage_quotas WHERE user_id = $1`, userID).
		Scan(&q.UserID, &q.Plan, &q.ComputeLimit, &q.TokenLimit, &q.StorageLimit, &q.APICallLimit)
	if err == pgx.ErrNoRows {
		return model.DefaultFreeQuota(userID), nil
	}
	return &q, err
}

func (s *pgStore) SetUsageQuota(ctx context.Context, quota *model.UsageQuota) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO usage_quotas (user_id, plan, compute_limit, token_limit, storage_limit, api_call_limit)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT(user_id) DO UPDATE SET
		  plan=EXCLUDED.plan, compute_limit=EXCLUDED.compute_limit, token_limit=EXCLUDED.token_limit,
		  storage_limit=EXCLUDED.storage_limit, api_call_limit=EXCLUDED.api_call_limit`,
		quota.UserID, quota.Plan, quota.ComputeLimit, quota.TokenLimit, quota.StorageLimit, quota.APICallLimit)
	return err
}

func (s *pgStore) CheckQuota(ctx context.Context, userID, usageType string) (bool, float64, error) {
	quota, err := s.GetUsageQuota(ctx, userID)
	if err != nil {
		return false, 0, err
	}

	period := time.Now().Format("2006-01")
	summary, err := s.GetPlatformUsageSummary(ctx, userID, period)
	if err != nil {
		return false, 0, err
	}

	switch usageType {
	case "compute":
		remaining := quota.ComputeLimit - summary.ComputeSeconds
		return remaining > 0, remaining, nil
	case "tokens":
		remaining := float64(quota.TokenLimit - summary.TokenCount)
		return remaining > 0, remaining, nil
	case "storage":
		remaining := float64(quota.StorageLimit - summary.StorageBytes)
		return remaining > 0, remaining, nil
	case "api_call":
		remaining := float64(quota.APICallLimit - summary.APICalls)
		return remaining > 0, remaining, nil
	default:
		return true, 0, nil
	}
}

func (s *pgStore) GetDailyUsage(ctx context.Context, userID, period string) ([]model.DailyUsage, error) {
	q := `SELECT to_char(created_at, 'YYYY-MM-DD') as day,
		COALESCE(SUM(CASE WHEN type='compute' THEN amount ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN type='tokens' THEN amount::bigint ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN type='api_call' THEN amount::bigint ELSE 0 END), 0)
	FROM platform_usage_records WHERE user_id = $1 AND to_char(created_at, 'YYYY-MM') = $2
	GROUP BY day ORDER BY day`

	rows, err := s.pool.Query(ctx, q, userID, period)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []model.DailyUsage
	for rows.Next() {
		var d model.DailyUsage
		if err := rows.Scan(&d.Date, &d.ComputeSeconds, &d.TokenCount, &d.APICalls); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}
