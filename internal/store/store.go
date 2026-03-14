package store

import (
	"context"

	"go.zoe.im/agentbox/internal/model"
	"go.zoe.im/x"
	"go.zoe.im/x/factory"
)

var (
	storeFactory = factory.NewFactory[Store, any]()

	// Create creates a Store from config.
	Create = storeFactory.Create

	// Register registers a Store implementation.
	Register = storeFactory.Register
)

// Store defines the persistence interface for runs and users.
type Store interface {
	// Run methods
	CreateRun(ctx context.Context, run *model.Run) error
	GetRun(ctx context.Context, id string) (*model.Run, error)
	UpdateRun(ctx context.Context, run *model.Run) error
	ListRuns(ctx context.Context, limit, offset int) ([]*model.Run, error)
	DeleteRun(ctx context.Context, id string) error

	// User methods
	CreateUser(ctx context.Context, user *model.User) error
	GetUser(ctx context.Context, id string) (*model.User, error)
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	GetUserByAPIKey(ctx context.Context, apiKeyHash string) (*model.User, error)
	UpdateUser(ctx context.Context, user *model.User) error

	// Integration methods
	CreateIntegration(ctx context.Context, i *model.Integration) error
	GetIntegration(ctx context.Context, id string) (*model.Integration, error)
	ListIntegrations(ctx context.Context, userID string) ([]*model.Integration, error)
	UpdateIntegration(ctx context.Context, i *model.Integration) error
	DeleteIntegration(ctx context.Context, id string) error
	ListAllEnabledIntegrations(ctx context.Context) ([]*model.Integration, error)

	// AgentDNA methods (marketplace registry)
	CreateAgentDNA(ctx context.Context, agent *model.AgentDNA) error
	GetAgentDNA(ctx context.Context, id string) (*model.AgentDNA, error)
	GetAgentDNABySlug(ctx context.Context, slug string) (*model.AgentDNA, error)
	UpdateAgentDNA(ctx context.Context, agent *model.AgentDNA) error
	DeleteAgentDNA(ctx context.Context, id string) error
	ListAgentDNAs(ctx context.Context, opts model.AgentDNAListOptions) ([]*model.AgentDNA, error)
	IncrementAgentDNADownloads(ctx context.Context, id string) error

	// Billing methods
	CreateSubscription(ctx context.Context, sub *model.Subscription) error
	GetSubscription(ctx context.Context, id string) (*model.Subscription, error)
	GetActiveSubscription(ctx context.Context, userID, agentID string) (*model.Subscription, error)
	UpdateSubscription(ctx context.Context, sub *model.Subscription) error
	ListSubscriptions(ctx context.Context, userID string) ([]*model.Subscription, error)

	CreateUsageRecord(ctx context.Context, rec *model.UsageRecord) error
	GetUsageSummary(ctx context.Context, userID, agentID, period string) (*model.UsageSummary, error)
	ListUsageRecords(ctx context.Context, opts model.BillingListOptions) ([]*model.UsageRecord, error)

	CreateAuthorPayout(ctx context.Context, payout *model.AuthorPayout) error
	GetAuthorPayout(ctx context.Context, authorUserID, period string) (*model.AuthorPayout, error)
	ListAuthorPayouts(ctx context.Context, authorUserID string) ([]*model.AuthorPayout, error)
}

// New creates a new Store from a TypedLazyConfig.
func New(cfg x.TypedLazyConfig) (Store, error) {
	return Create(cfg)
}
