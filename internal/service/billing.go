package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/stripe/stripe-go/v82"
	portalsession "github.com/stripe/stripe-go/v82/billingportal/session"

	"go.zoe.im/agentbox/internal/auth"
	billingpkg "go.zoe.im/agentbox/internal/billing"
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

// --- Stripe Checkout ---

// CreateCheckoutSession handles POST /api/v1/billing/checkout
// Creates a Stripe Checkout session for subscribing to a paid agent.
func (s *Service) CreateCheckoutSession(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	if s.stripe == nil {
		http.Error(w, `{"error":"stripe not configured"}`, http.StatusServiceUnavailable)
		return
	}

	var req struct {
		AgentID       string `json:"agent_id"`
		StripePriceID string `json:"stripe_price_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	dna, err := s.store.GetAgentDNA(r.Context(), req.AgentID)
	if err != nil {
		dna, err = s.store.GetAgentDNABySlug(r.Context(), req.AgentID)
		if err != nil {
			http.Error(w, `{"error":"agent not found"}`, http.StatusNotFound)
			return
		}
	}

	// Get or create Stripe customer
	var customerID string
	if sc, err := s.store.GetStripeCustomer(r.Context(), user.ID); err == nil {
		customerID = sc.StripeCustomerID
	} else {
		customerID, err = s.stripe.EnsureCustomer(user.ID, user.Email, user.Name)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}
		_ = s.store.UpsertStripeCustomer(r.Context(), &model.StripeCustomer{
			UserID:           user.ID,
			StripeCustomerID: customerID,
			CreatedAt:        time.Now(),
		})
	}

	priceID := req.StripePriceID
	if priceID == "" {
		priceID = dna.Manifest.Requirements["stripe_price_id"]
	}
	if priceID == "" {
		http.Error(w, `{"error":"no stripe_price_id for this agent"}`, http.StatusBadRequest)
		return
	}

	resp, err := s.stripe.CreateCheckoutSession(billingpkg.CheckoutRequest{
		UserID:           user.ID,
		StripeCustomerID: customerID,
		AgentID:          dna.ID,
		AgentName:        dna.Slug,
		PricingModel:     dna.Manifest.PricingModel,
		StripePriceID:    priceID,
	})
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// HandleStripeWebhook handles POST /api/v1/billing/stripe/webhook
// Processes Stripe webhook events to update subscription state.
func (s *Service) HandleStripeWebhook(w http.ResponseWriter, r *http.Request) {
	if s.stripe == nil {
		http.Error(w, `{"error":"stripe not configured"}`, http.StatusServiceUnavailable)
		return
	}

	event, err := s.stripe.ParseWebhook(r)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	switch event.Type {
	case "checkout.session.completed":
		if event.UserID == "" || event.AgentID == "" {
			break
		}
		// Activate an existing subscription (e.g. from trial) or mark it as paid.
		sub, _ := s.store.GetActiveSubscription(ctx, event.UserID, event.AgentID)
		if sub == nil {
			break
		}
		sub.Status = model.SubscriptionActive
		sub.UpdatedAt = time.Now()
		_ = s.store.UpdateSubscription(ctx, sub)

	case "customer.subscription.created", "customer.subscription.updated":
		if event.UserID == "" {
			// Look up user by stripe customer ID (best effort)
			break
		}
		sub, _ := s.store.GetActiveSubscription(ctx, event.UserID, event.AgentID)
		if sub == nil {
			break
		}
		sub.Status = billingpkg.SubscriptionStatusFromStripe(event.Status)
		sub.StripeSubID = event.StripeSubID
		sub.StripePriceID = event.PriceID
		sub.UpdatedAt = time.Now()
		_ = s.store.UpdateSubscription(ctx, sub)

	case "customer.subscription.deleted":
		if event.UserID == "" {
			break
		}
		sub, _ := s.store.GetActiveSubscription(ctx, event.UserID, event.AgentID)
		if sub == nil {
			break
		}
		now := time.Now()
		sub.Status = model.SubscriptionCanceled
		sub.CanceledAt = &now
		sub.UpdatedAt = now
		_ = s.store.UpdateSubscription(ctx, sub)

	case "invoice.payment_succeeded":
		if event.StripeSubID == "" {
			break
		}
		sub, _ := s.store.GetSubscriptionByStripeSubID(ctx, event.StripeSubID)
		if sub == nil {
			break
		}
		now := time.Now()
		sub.Status = model.SubscriptionActive
		sub.CurrentPeriodStart = now
		sub.CurrentPeriodEnd = now.AddDate(0, 1, 0)
		sub.UpdatedAt = now
		_ = s.store.UpdateSubscription(ctx, sub)

	case "invoice.payment_failed":
		if event.StripeSubID == "" {
			break
		}
		sub, _ := s.store.GetSubscriptionByStripeSubID(ctx, event.StripeSubID)
		if sub == nil {
			break
		}
		sub.Status = model.SubscriptionPastDue
		sub.UpdatedAt = time.Now()
		_ = s.store.UpdateSubscription(ctx, sub)
	}

	w.WriteHeader(http.StatusOK)
}

// --- Billing Portal ---

// HandleBillingPortal handles GET /api/v1/billing/portal
// Creates a Stripe billing portal session for the authenticated user.
func (s *Service) HandleBillingPortal(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	if s.stripe == nil {
		http.Error(w, `{"error":"stripe not configured"}`, http.StatusServiceUnavailable)
		return
	}

	sc, err := s.store.GetStripeCustomer(r.Context(), user.ID)
	if err != nil {
		http.Error(w, `{"error":"no billing account found"}`, http.StatusNotFound)
		return
	}

	returnURL := s.cfg.Stripe.SuccessURL
	if returnURL == "" {
		returnURL = s.cfg.Stripe.CancelURL
	}

	params := &stripe.BillingPortalSessionParams{
		Customer:  stripe.String(sc.StripeCustomerID),
		ReturnURL: stripe.String(returnURL),
	}
	sess, err := portalsession.New(params)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"url": sess.URL,
	})
}

// --- Run Cost Breakdown ---

// GetRunCost handles GET /api/v1/billing/runs/{runId}/cost
// Returns the itemised cost breakdown for a completed run.
func (s *Service) GetRunCost(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	runID := r.PathValue("runId")
	breakdown, err := s.store.GetRunCostBreakdown(r.Context(), runID)
	if err != nil {
		http.Error(w, `{"error":"run cost not found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(breakdown)
}

// --- Free Quota Status ---

// GetQuotaStatus handles GET /api/v1/billing/quota
// Returns free run quota usage for the current user.
func (s *Service) GetQuotaStatus(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	agentID := r.URL.Query().Get("agent_id")
	period := time.Now().Format("2006-01")
	limit := s.cfg.Stripe.FreeRunsPerMonth
	if limit == 0 {
		limit = 10 // default: 10 free runs/month
	}

	usage, err := s.store.GetFreeQuotaUsage(r.Context(), user.ID, agentID, period)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	if usage.Limit == 0 {
		usage.Limit = limit
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"user_id":   user.ID,
		"agent_id":  agentID,
		"period":    period,
		"limit":     usage.Limit,
		"used":      usage.Used,
		"remaining": max(0, usage.Limit-usage.Used),
		"exhausted": usage.Used >= usage.Limit,
	})
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
