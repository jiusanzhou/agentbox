package model

import "time"

// --- Billing Models ---

// PricingModel defines how an agent is priced.
type PricingModel string

const (
	PricingFree         PricingModel = "free"
	PricingSubscription PricingModel = "subscription"
	PricingUsage        PricingModel = "usage"
	PricingOneTime      PricingModel = "one-time"
)

// SubscriptionStatus tracks the lifecycle of a subscription.
type SubscriptionStatus string

const (
	SubscriptionActive   SubscriptionStatus = "active"
	SubscriptionTrialing SubscriptionStatus = "trialing"
	SubscriptionPastDue  SubscriptionStatus = "past_due"
	SubscriptionCanceled SubscriptionStatus = "canceled"
	SubscriptionExpired  SubscriptionStatus = "expired"
)

// Subscription represents a user's subscription to a paid agent.
type Subscription struct {
	ID              string             `json:"id"`
	UserID          string             `json:"user_id"`
	AgentID         string             `json:"agent_id"`
	PricingModel    PricingModel       `json:"pricing_model"`
	Status          SubscriptionStatus `json:"status"`
	StripeSubID     string             `json:"stripe_sub_id,omitempty"`
	StripePriceID   string             `json:"stripe_price_id,omitempty"`
	TrialEndsAt     *time.Time         `json:"trial_ends_at,omitempty"`
	CurrentPeriodStart time.Time       `json:"current_period_start"`
	CurrentPeriodEnd   time.Time       `json:"current_period_end"`
	CanceledAt      *time.Time         `json:"canceled_at,omitempty"`
	CreatedAt       time.Time          `json:"created_at"`
	UpdatedAt       time.Time          `json:"updated_at"`
}

// UsageRecord tracks per-run token/model usage for billing.
type UsageRecord struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	AgentID       string    `json:"agent_id"`
	RunID         string    `json:"run_id"`
	Model         string    `json:"model"`
	InputTokens   int64     `json:"input_tokens"`
	OutputTokens  int64     `json:"output_tokens"`
	TotalTokens   int64     `json:"total_tokens"`
	CostMicros    int64     `json:"cost_micros"` // cost in 1/1,000,000 USD
	Duration      int64     `json:"duration_ms"`
	CreatedAt     time.Time `json:"created_at"`
}

// UsageSummary aggregates usage for a billing period.
type UsageSummary struct {
	UserID        string       `json:"user_id"`
	AgentID       string       `json:"agent_id,omitempty"`
	Period        string       `json:"period"` // YYYY-MM
	TotalRuns     int64        `json:"total_runs"`
	TotalTokens   int64        `json:"total_tokens"`
	TotalCostMicros int64      `json:"total_cost_micros"`
	ByModel       []ModelUsage `json:"by_model,omitempty"`
}

// ModelUsage breaks down usage by model.
type ModelUsage struct {
	Model        string `json:"model"`
	Runs         int64  `json:"runs"`
	InputTokens  int64  `json:"input_tokens"`
	OutputTokens int64  `json:"output_tokens"`
	CostMicros   int64  `json:"cost_micros"`
}

// --- Author Revenue ---

// RevenueShareConfig defines how revenue is split.
type RevenueShareConfig struct {
	AuthorPercent   int `json:"author_percent"`   // e.g. 70 = 70%
	PlatformPercent int `json:"platform_percent"` // e.g. 30 = 30%
}

// DefaultRevenueShare is the default split: 70% author, 30% platform.
var DefaultRevenueShare = RevenueShareConfig{
	AuthorPercent:   70,
	PlatformPercent: 30,
}

// AuthorPayout tracks accumulated earnings for an agent author.
type AuthorPayout struct {
	ID              string    `json:"id"`
	AuthorUserID    string    `json:"author_user_id"`
	AgentID         string    `json:"agent_id"`
	Period          string    `json:"period"` // YYYY-MM
	GrossRevenue    int64     `json:"gross_revenue"`    // micros
	PlatformFee     int64     `json:"platform_fee"`     // micros
	AuthorEarnings  int64     `json:"author_earnings"`  // micros
	Status          string    `json:"status"`           // pending, paid, held
	StripePayoutID  string    `json:"stripe_payout_id,omitempty"`
	PaidAt          *time.Time `json:"paid_at,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// --- Experience Licensing ---

// ExperienceLicense defines access control for experience packs.
type ExperienceLicense struct {
	ID              string    `json:"id"`
	ExperienceID    string    `json:"experience_id"`
	AgentID         string    `json:"agent_id"`
	LicenseType     string    `json:"license_type"` // open, paid, exclusive
	PriceMicros     int64     `json:"price_micros"` // one-time price (if paid)
	CreatedAt       time.Time `json:"created_at"`
}

// ExperiencePurchase records a user buying access to a paid experience pack.
type ExperiencePurchase struct {
	ID              string    `json:"id"`
	UserID          string    `json:"user_id"`
	ExperienceID    string    `json:"experience_id"`
	AgentID         string    `json:"agent_id"`
	PriceMicros     int64     `json:"price_micros"`
	StripePaymentID string    `json:"stripe_payment_id,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// BillingListOptions for querying usage records.
type BillingListOptions struct {
	UserID   string
	AgentID  string
	Period   string // YYYY-MM
	Limit    int
	Offset   int
}
