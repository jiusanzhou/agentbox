package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.zoe.im/agentbox/internal/auth"
	"go.zoe.im/agentbox/internal/model"
)

func genID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// --- Subscription endpoints ---

// SubscribeAgent handles POST /api/v1/billing/subscribe
// Creates a subscription for the user to a paid agent.
func (s *Service) SubscribeAgent(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		AgentID string `json:"agent_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	// Check agent exists
	dna, err := s.store.GetAgentDNABySlug(r.Context(), req.AgentID)
	if err != nil {
		dna, err = s.store.GetAgentDNA(r.Context(), req.AgentID)
		if err != nil {
			http.Error(w, `{"error":"agent not found"}`, http.StatusNotFound)
			return
		}
	}

	// Check for existing active subscription
	if existing, _ := s.store.GetActiveSubscription(r.Context(), user.ID, dna.ID); existing != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"subscription": existing,
			"message":      "already subscribed",
		})
		return
	}

	// Determine pricing
	pricingModel := model.PricingModel(dna.Manifest.PricingModel)
	if pricingModel == "" {
		pricingModel = model.PricingFree
	}

	now := time.Now()
	sub := &model.Subscription{
		ID:                 genID(),
		UserID:             user.ID,
		AgentID:            dna.ID,
		PricingModel:       pricingModel,
		Status:             model.SubscriptionActive,
		CurrentPeriodStart: now,
		CurrentPeriodEnd:   now.AddDate(0, 1, 0),
		CreatedAt:          now,
		UpdatedAt:          now,
	}

	// Free agents get free subscription; paid agents would go through Stripe
	// TODO: Stripe integration for paid agents
	if pricingModel != model.PricingFree {
		// For now, start as trial
		trialEnd := now.AddDate(0, 0, 14)
		sub.Status = model.SubscriptionTrialing
		sub.TrialEndsAt = &trialEnd
	}

	if err := s.store.CreateSubscription(r.Context(), sub); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sub)
}

// ListSubscriptions handles GET /api/v1/billing/subscriptions
func (s *Service) ListSubscriptions(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	subs, err := s.store.ListSubscriptions(r.Context(), user.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(subs)
}

// CancelSubscription handles POST /api/v1/billing/subscriptions/{id}/cancel
func (s *Service) CancelSubscription(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	id := r.PathValue("id")
	sub, err := s.store.GetSubscription(r.Context(), id)
	if err != nil {
		http.Error(w, `{"error":"subscription not found"}`, http.StatusNotFound)
		return
	}

	if sub.UserID != user.ID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	now := time.Now()
	sub.Status = model.SubscriptionCanceled
	sub.CanceledAt = &now
	sub.UpdatedAt = now

	if err := s.store.UpdateSubscription(r.Context(), sub); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sub)
}

// --- Usage endpoints ---

// RecordUsage handles POST /api/v1/billing/usage
// Records token usage for a run (called by the runtime after run completion).
func (s *Service) RecordUsage(w http.ResponseWriter, r *http.Request) {
	var req model.UsageRecord
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if req.ID == "" {
		req.ID = genID()
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now()
	}
	req.TotalTokens = req.InputTokens + req.OutputTokens

	if err := s.store.CreateUsageRecord(r.Context(), &req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(req)
}

// GetUsageSummary handles GET /api/v1/billing/usage/summary
func (s *Service) GetUsageSummary(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	agentID := r.URL.Query().Get("agent_id")
	period := r.URL.Query().Get("period")
	if period == "" {
		period = time.Now().Format("2006-01")
	}

	summary, err := s.store.GetUsageSummary(r.Context(), user.ID, agentID, period)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

// ListUsageRecords handles GET /api/v1/billing/usage/records
func (s *Service) ListUsageRecords(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	opts := model.BillingListOptions{
		UserID:  user.ID,
		AgentID: r.URL.Query().Get("agent_id"),
		Period:  r.URL.Query().Get("period"),
		Limit:   50,
	}

	records, err := s.store.ListUsageRecords(r.Context(), opts)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(records)
}

// --- Author Revenue endpoints ---

// GetAuthorRevenue handles GET /api/v1/billing/revenue
// Returns revenue summary for the authenticated agent author.
func (s *Service) GetAuthorRevenue(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	payouts, err := s.store.ListAuthorPayouts(r.Context(), user.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Compute totals
	var totalGross, totalEarnings, totalPaid int64
	for _, p := range payouts {
		totalGross += p.GrossRevenue
		totalEarnings += p.AuthorEarnings
		if p.Status == "paid" {
			totalPaid += p.AuthorEarnings
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"author_id":       user.ID,
		"total_gross":     totalGross,
		"total_earnings":  totalEarnings,
		"total_paid":      totalPaid,
		"pending":         totalEarnings - totalPaid,
		"share":           model.DefaultRevenueShare,
		"payouts":         payouts,
	})
}

// --- Access check middleware helper ---

// CheckAgentAccess verifies a user can use an agent (free, active subscription, or trial).
func (s *Service) CheckAgentAccess(userID, agentID string) (bool, string) {
	ctx := r2ctx()

	dna, err := s.store.GetAgentDNA(ctx, agentID)
	if err != nil {
		dna, err = s.store.GetAgentDNABySlug(ctx, agentID)
		if err != nil {
			return false, "agent not found"
		}
	}

	// Free agents → always allowed
	if dna.Manifest.PricingModel == "" || dna.Manifest.PricingModel == string(model.PricingFree) {
		return true, "free"
	}

	// Check subscription
	sub, err := s.store.GetActiveSubscription(ctx, userID, dna.ID)
	if err != nil {
		return false, "no active subscription"
	}

	// Check trial expiry
	if sub.Status == model.SubscriptionTrialing && sub.TrialEndsAt != nil && time.Now().After(*sub.TrialEndsAt) {
		return false, "trial expired"
	}

	return true, string(sub.Status)
}

func r2ctx() context.Context {
	return context.Background()
}
