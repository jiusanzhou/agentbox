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
	mu            sync.RWMutex
	subscriptions map[string]*model.Subscription
	usageRecords  []*model.UsageRecord
	payouts       map[string]*model.AuthorPayout // key: authorUserID:period
}

func newBillingData() *billingData {
	return &billingData{
		subscriptions: make(map[string]*model.Subscription),
		payouts:       make(map[string]*model.AuthorPayout),
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
