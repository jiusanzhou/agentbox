package billing

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/stripe/stripe-go/v82"
	"github.com/stripe/stripe-go/v82/checkout/session"
	"github.com/stripe/stripe-go/v82/customer"
	"github.com/stripe/stripe-go/v82/webhook"
	"go.zoe.im/agentbox/internal/model"
)

// StripeClient wraps Stripe API operations needed for the marketplace.
type StripeClient struct {
	secretKey     string
	webhookSecret string
	// successURL and cancelURL are absolute URLs to redirect after checkout.
	successURL string
	cancelURL  string
}

// NewStripeClient creates a Stripe client.
func NewStripeClient(secretKey, webhookSecret, successURL, cancelURL string) *StripeClient {
	stripe.Key = secretKey
	return &StripeClient{
		secretKey:     secretKey,
		webhookSecret: webhookSecret,
		successURL:    successURL,
		cancelURL:     cancelURL,
	}
}

// EnsureCustomer returns the Stripe customer ID for a user, creating one if needed.
func (c *StripeClient) EnsureCustomer(userID, email, name string) (string, error) {
	params := &stripe.CustomerParams{
		Email: stripe.String(email),
		Name:  stripe.String(name),
		Metadata: map[string]string{
			"platform_user_id": userID,
		},
	}
	cust, err := customer.New(params)
	if err != nil {
		return "", fmt.Errorf("create stripe customer: %w", err)
	}
	return cust.ID, nil
}

// CheckoutRequest contains params for creating a checkout session.
type CheckoutRequest struct {
	UserID           string
	StripeCustomerID string
	AgentID          string
	AgentName        string
	PricingModel     string
	StripePriceID    string // pre-configured in Stripe Dashboard
}

// CheckoutResponse contains the Stripe checkout URL.
type CheckoutResponse struct {
	SessionID  string `json:"session_id"`
	URL        string `json:"url"`
	ExpiresAt  int64  `json:"expires_at"`
}

// CreateCheckoutSession creates a Stripe Checkout session for subscribing to a paid agent.
func (c *StripeClient) CreateCheckoutSession(req CheckoutRequest) (*CheckoutResponse, error) {
	mode := stripe.CheckoutSessionModeSubscription
	if req.PricingModel == "one-time" || req.PricingModel == "per_task" {
		mode = stripe.CheckoutSessionModePayment
	}

	params := &stripe.CheckoutSessionParams{
		Customer:   stripe.String(req.StripeCustomerID),
		Mode:       stripe.String(string(mode)),
		SuccessURL: stripe.String(c.successURL + "?session_id={CHECKOUT_SESSION_ID}"),
		CancelURL:  stripe.String(c.cancelURL),
		LineItems: []*stripe.CheckoutSessionLineItemParams{
			{
				Price:    stripe.String(req.StripePriceID),
				Quantity: stripe.Int64(1),
			},
		},
		Metadata: map[string]string{
			"platform_user_id": req.UserID,
			"agent_id":         req.AgentID,
		},
	}

	sess, err := session.New(params)
	if err != nil {
		return nil, fmt.Errorf("create checkout session: %w", err)
	}

	return &CheckoutResponse{
		SessionID: sess.ID,
		URL:       sess.URL,
		ExpiresAt: sess.ExpiresAt,
	}, nil
}

// WebhookEvent is a parsed Stripe webhook event.
type WebhookEvent struct {
	Type             string
	StripeSubID      string
	StripeCustomerID string
	UserID           string // from metadata
	AgentID          string // from metadata
	PriceID          string
	Status           string
}

// ParseWebhook validates the Stripe webhook signature and returns a structured event.
func (c *StripeClient) ParseWebhook(r *http.Request) (*WebhookEvent, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	sig := r.Header.Get("Stripe-Signature")
	event, err := webhook.ConstructEvent(body, sig, c.webhookSecret)
	if err != nil {
		return nil, fmt.Errorf("webhook signature invalid: %w", err)
	}

	we := &WebhookEvent{Type: string(event.Type)}

	switch event.Type {
	case "checkout.session.completed":
		var sess stripe.CheckoutSession
		if err := json.Unmarshal(event.Data.Raw, &sess); err != nil {
			return nil, err
		}
		we.StripeCustomerID = sess.Customer.ID
		we.UserID = sess.Metadata["platform_user_id"]
		we.AgentID = sess.Metadata["agent_id"]

	case "customer.subscription.created",
		"customer.subscription.updated",
		"customer.subscription.deleted":
		var sub stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &sub); err != nil {
			return nil, err
		}
		we.StripeSubID = sub.ID
		we.StripeCustomerID = sub.Customer.ID
		we.Status = string(sub.Status)
		if len(sub.Items.Data) > 0 {
			we.PriceID = sub.Items.Data[0].Price.ID
		}

	case "invoice.payment_succeeded":
		var inv stripe.Invoice
		if err := json.Unmarshal(event.Data.Raw, &inv); err != nil {
			return nil, err
		}
		if inv.Parent != nil &&
			inv.Parent.SubscriptionDetails != nil &&
			inv.Parent.SubscriptionDetails.Subscription != nil {
			we.StripeSubID = inv.Parent.SubscriptionDetails.Subscription.ID
		}
		if inv.Customer != nil {
			we.StripeCustomerID = inv.Customer.ID
		}
	}

	return we, nil
}

// SubscriptionStatusFromStripe maps a Stripe subscription status to our model.
func SubscriptionStatusFromStripe(stripeStatus string) model.SubscriptionStatus {
	switch stripeStatus {
	case "active":
		return model.SubscriptionActive
	case "trialing":
		return model.SubscriptionTrialing
	case "past_due":
		return model.SubscriptionPastDue
	case "canceled", "cancelled":
		return model.SubscriptionCanceled
	default:
		return model.SubscriptionExpired
	}
}
