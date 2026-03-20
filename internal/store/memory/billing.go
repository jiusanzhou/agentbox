package memory

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.zoe.im/agentbox/internal/model"
)

// billingData holds in-memory billing state.
type billingData struct {
	mu              sync.RWMutex
	subscriptions   map[string]*model.Subscription
	usageRecords    []*model.UsageRecord
	payouts         map[string]*model.AuthorPayout // key: authorUserID:period
	costBreakdowns  map[string]*model.RunCostBreakdown
	stripeCustomers map[string]*model.StripeCustomer
	quotaUsage      map[string]*model.FreeQuotaUsage // key: userID:agentID:period
	platformUsage   []model.PlatformUsageRecord
	usageQuotas     map[string]*model.UsageQuota // key: userID
}

func newBillingData() *billingData {
	return &billingData{
		subscriptions:   make(map[string]*model.Subscription),
		payouts:         make(map[string]*model.AuthorPayout),
		costBreakdowns:  make(map[string]*model.RunCostBreakdown),
		stripeCustomers: make(map[string]*model.StripeCustomer),
		quotaUsage:      make(map[string]*model.FreeQuotaUsage),
		usageQuotas:     make(map[string]*model.UsageQuota),
	}
}

// --- Subscription ---

func (s *memoryStore) CreateSubscription(_ context.Context, sub *model.Subscription) error {
	s.billing.mu.Lock()
	defer s.billing.mu.Unlock()
	if _, exists := s.billing.subscriptions[sub.ID]; exists {
		return fmt.Errorf("subscription %s already exists", sub.ID)
	}
	s.billing.subscriptions[sub.ID] = sub
	return nil
}

func (s *memoryStore) GetSubscription(_ context.Context, id string) (*model.Subscription, error) {
	s.billing.mu.RLock()
	defer s.billing.mu.RUnlock()
	sub, ok := s.billing.subscriptions[id]
	if !ok {
		return nil, fmt.Errorf("subscription %s not found", id)
	}
	return sub, nil
}

func (s *memoryStore) GetActiveSubscription(_ context.Context, userID, agentID string) (*model.Subscription, error) {
	s.billing.mu.RLock()
	defer s.billing.mu.RUnlock()
	for _, sub := range s.billing.subscriptions {
		if sub.UserID == userID && sub.AgentID == agentID &&
			(sub.Status == model.SubscriptionActive || sub.Status == model.SubscriptionTrialing) {
			return sub, nil
		}
	}
	return nil, fmt.Errorf("no active subscription for user %s agent %s", userID, agentID)
}

func (s *memoryStore) GetSubscriptionByStripeSubID(_ context.Context, stripeSubID string) (*model.Subscription, error) {
	s.billing.mu.RLock()
	defer s.billing.mu.RUnlock()
	for _, sub := range s.billing.subscriptions {
		if sub.StripeSubID == stripeSubID {
			return sub, nil
		}
	}
	return nil, fmt.Errorf("no subscription with stripe_sub_id %s", stripeSubID)
}

func (s *memoryStore) UpdateSubscription(_ context.Context, sub *model.Subscription) error {
	s.billing.mu.Lock()
	defer s.billing.mu.Unlock()
	if _, exists := s.billing.subscriptions[sub.ID]; !exists {
		return fmt.Errorf("subscription %s not found", sub.ID)
	}
	s.billing.subscriptions[sub.ID] = sub
	return nil
}

func (s *memoryStore) ListSubscriptions(_ context.Context, userID string) ([]*model.Subscription, error) {
	s.billing.mu.RLock()
	defer s.billing.mu.RUnlock()
	var result []*model.Subscription
	for _, sub := range s.billing.subscriptions {
		if sub.UserID == userID {
			result = append(result, sub)
		}
	}
	return result, nil
}

// --- Usage Records ---

func (s *memoryStore) CreateUsageRecord(_ context.Context, rec *model.UsageRecord) error {
	s.billing.mu.Lock()
	defer s.billing.mu.Unlock()
	s.billing.usageRecords = append(s.billing.usageRecords, rec)
	return nil
}

func (s *memoryStore) GetUsageSummary(_ context.Context, userID, agentID, period string) (*model.UsageSummary, error) {
	s.billing.mu.RLock()
	defer s.billing.mu.RUnlock()

	summary := &model.UsageSummary{
		UserID:  userID,
		AgentID: agentID,
		Period:  period,
	}

	modelMap := make(map[string]*model.ModelUsage)

	for _, rec := range s.billing.usageRecords {
		if rec.UserID != userID {
			continue
		}
		if agentID != "" && rec.AgentID != agentID {
			continue
		}
		recPeriod := rec.CreatedAt.Format("2006-01")
		if period != "" && recPeriod != period {
			continue
		}

		summary.TotalRuns++
		summary.TotalTokens += rec.TotalTokens
		summary.TotalCostMicros += rec.CostMicros

		mu, ok := modelMap[rec.Model]
		if !ok {
			mu = &model.ModelUsage{Model: rec.Model}
			modelMap[rec.Model] = mu
		}
		mu.Runs++
		mu.InputTokens += rec.InputTokens
		mu.OutputTokens += rec.OutputTokens
		mu.CostMicros += rec.CostMicros
	}

	for _, mu := range modelMap {
		summary.ByModel = append(summary.ByModel, *mu)
	}

	return summary, nil
}

func (s *memoryStore) ListUsageRecords(_ context.Context, opts model.BillingListOptions) ([]*model.UsageRecord, error) {
	s.billing.mu.RLock()
	defer s.billing.mu.RUnlock()

	var result []*model.UsageRecord
	for _, rec := range s.billing.usageRecords {
		if opts.UserID != "" && rec.UserID != opts.UserID {
			continue
		}
		if opts.AgentID != "" && rec.AgentID != opts.AgentID {
			continue
		}
		if opts.Period != "" && !strings.HasPrefix(rec.CreatedAt.Format(time.RFC3339), opts.Period) {
			continue
		}
		result = append(result, rec)
	}

	// Apply limit/offset
	if opts.Offset > 0 && opts.Offset < len(result) {
		result = result[opts.Offset:]
	}
	if opts.Limit > 0 && opts.Limit < len(result) {
		result = result[:opts.Limit]
	}

	return result, nil
}

// --- Author Payouts ---

func (s *memoryStore) CreateAuthorPayout(_ context.Context, payout *model.AuthorPayout) error {
	s.billing.mu.Lock()
	defer s.billing.mu.Unlock()
	key := payout.AuthorUserID + ":" + payout.Period
	s.billing.payouts[key] = payout
	return nil
}

func (s *memoryStore) GetAuthorPayout(_ context.Context, authorUserID, period string) (*model.AuthorPayout, error) {
	s.billing.mu.RLock()
	defer s.billing.mu.RUnlock()
	key := authorUserID + ":" + period
	p, ok := s.billing.payouts[key]
	if !ok {
		return nil, fmt.Errorf("payout for %s period %s not found", authorUserID, period)
	}
	return p, nil
}

func (s *memoryStore) ListAuthorPayouts(_ context.Context, authorUserID string) ([]*model.AuthorPayout, error) {
	s.billing.mu.RLock()
	defer s.billing.mu.RUnlock()
	var result []*model.AuthorPayout
	for _, p := range s.billing.payouts {
		if p.AuthorUserID == authorUserID {
			result = append(result, p)
		}
	}
	return result, nil
}

// --- Run Cost Breakdowns ---

func (s *memoryStore) UpsertRunCostBreakdown(_ context.Context, b *model.RunCostBreakdown) error {
	s.billing.mu.Lock()
	defer s.billing.mu.Unlock()
	cp := *b
	s.billing.costBreakdowns[b.RunID] = &cp
	return nil
}

func (s *memoryStore) GetRunCostBreakdown(_ context.Context, runID string) (*model.RunCostBreakdown, error) {
	s.billing.mu.RLock()
	defer s.billing.mu.RUnlock()
	b, ok := s.billing.costBreakdowns[runID]
	if !ok {
		return nil, fmt.Errorf("run cost breakdown %s not found", runID)
	}
	cp := *b
	return &cp, nil
}

// --- Stripe Customers ---

func (s *memoryStore) UpsertStripeCustomer(_ context.Context, c *model.StripeCustomer) error {
	s.billing.mu.Lock()
	defer s.billing.mu.Unlock()
	cp := *c
	s.billing.stripeCustomers[c.UserID] = &cp
	return nil
}

func (s *memoryStore) GetStripeCustomer(_ context.Context, userID string) (*model.StripeCustomer, error) {
	s.billing.mu.RLock()
	defer s.billing.mu.RUnlock()
	c, ok := s.billing.stripeCustomers[userID]
	if !ok {
		return nil, fmt.Errorf("stripe customer for user %s not found", userID)
	}
	cp := *c
	return &cp, nil
}

// --- Free Quota ---

func (s *memoryStore) GetFreeQuotaUsage(_ context.Context, userID, agentID, period string) (*model.FreeQuotaUsage, error) {
	s.billing.mu.RLock()
	defer s.billing.mu.RUnlock()
	key := userID + ":" + agentID + ":" + period
	q, ok := s.billing.quotaUsage[key]
	if !ok {
		return &model.FreeQuotaUsage{UserID: userID, AgentID: agentID, Period: period}, nil
	}
	cp := *q
	return &cp, nil
}

func (s *memoryStore) IncrementFreeQuotaUsage(_ context.Context, userID, agentID, period string, limit int64) error {
	s.billing.mu.Lock()
	defer s.billing.mu.Unlock()
	key := userID + ":" + agentID + ":" + period
	q, ok := s.billing.quotaUsage[key]
	if !ok {
		q = &model.FreeQuotaUsage{UserID: userID, AgentID: agentID, Period: period, Limit: limit}
		s.billing.quotaUsage[key] = q
	}
	q.Used++
	q.Limit = limit
	q.UpdatedAt = time.Now()
	return nil
}

// --- Platform Usage Tracking ---

func (s *memoryStore) RecordPlatformUsage(_ context.Context, rec *model.PlatformUsageRecord) error {
	s.billing.mu.Lock()
	defer s.billing.mu.Unlock()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = time.Now()
	}
	s.billing.platformUsage = append(s.billing.platformUsage, *rec)
	return nil
}

func (s *memoryStore) GetPlatformUsageSummary(_ context.Context, userID, period string) (*model.PlatformUsageSummary, error) {
	s.billing.mu.RLock()
	defer s.billing.mu.RUnlock()

	summary := &model.PlatformUsageSummary{UserID: userID, Period: period}

	for _, rec := range s.billing.platformUsage {
		if rec.UserID != userID {
			continue
		}
		recPeriod := rec.CreatedAt.Format("2006-01")
		recDay := rec.CreatedAt.Format("2006-01-02")
		if period != "" && recPeriod != period && recDay != period {
			continue
		}
		switch rec.Type {
		case "compute":
			summary.ComputeSeconds += rec.Amount
		case "tokens":
			summary.TokenCount += int64(rec.Amount)
		case "storage":
			summary.StorageBytes += int64(rec.Amount)
		case "api_call":
			summary.APICalls += int64(rec.Amount)
		}
	}

	summary.EstimatedCost = summary.ComputeSeconds*0.10/60 + float64(summary.TokenCount)*3.0/1_000_000
	return summary, nil
}

func (s *memoryStore) GetPlatformUsageHistory(_ context.Context, userID string, from, to time.Time, limit int) ([]model.PlatformUsageRecord, error) {
	s.billing.mu.RLock()
	defer s.billing.mu.RUnlock()

	var result []model.PlatformUsageRecord
	for i := len(s.billing.platformUsage) - 1; i >= 0; i-- {
		rec := s.billing.platformUsage[i]
		if rec.UserID != userID {
			continue
		}
		if rec.CreatedAt.Before(from) || rec.CreatedAt.After(to) {
			continue
		}
		result = append(result, rec)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (s *memoryStore) GetUsageQuota(_ context.Context, userID string) (*model.UsageQuota, error) {
	s.billing.mu.RLock()
	defer s.billing.mu.RUnlock()
	q, ok := s.billing.usageQuotas[userID]
	if !ok {
		return model.DefaultFreeQuota(userID), nil
	}
	cp := *q
	return &cp, nil
}

func (s *memoryStore) SetUsageQuota(_ context.Context, quota *model.UsageQuota) error {
	s.billing.mu.Lock()
	defer s.billing.mu.Unlock()
	cp := *quota
	s.billing.usageQuotas[quota.UserID] = &cp
	return nil
}

func (s *memoryStore) CheckQuota(ctx context.Context, userID, usageType string) (bool, float64, error) {
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

func (s *memoryStore) GetDailyUsage(_ context.Context, userID, period string) ([]model.DailyUsage, error) {
	s.billing.mu.RLock()
	defer s.billing.mu.RUnlock()

	dayMap := make(map[string]*model.DailyUsage)
	for _, rec := range s.billing.platformUsage {
		if rec.UserID != userID {
			continue
		}
		recPeriod := rec.CreatedAt.Format("2006-01")
		if period != "" && recPeriod != period {
			continue
		}
		day := rec.CreatedAt.Format("2006-01-02")
		d, ok := dayMap[day]
		if !ok {
			d = &model.DailyUsage{Date: day}
			dayMap[day] = d
		}
		switch rec.Type {
		case "compute":
			d.ComputeSeconds += rec.Amount
		case "tokens":
			d.TokenCount += int64(rec.Amount)
		case "api_call":
			d.APICalls += int64(rec.Amount)
		}
	}

	var result []model.DailyUsage
	for _, d := range dayMap {
		result = append(result, *d)
	}
	return result, nil
}
