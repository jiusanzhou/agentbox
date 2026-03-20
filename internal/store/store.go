package store

import (
	"context"
	"time"

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
	GetSubscriptionByStripeSubID(ctx context.Context, stripeSubID string) (*model.Subscription, error)
	UpdateSubscription(ctx context.Context, sub *model.Subscription) error
	ListSubscriptions(ctx context.Context, userID string) ([]*model.Subscription, error)

	CreateUsageRecord(ctx context.Context, rec *model.UsageRecord) error
	GetUsageSummary(ctx context.Context, userID, agentID, period string) (*model.UsageSummary, error)
	ListUsageRecords(ctx context.Context, opts model.BillingListOptions) ([]*model.UsageRecord, error)

	CreateAuthorPayout(ctx context.Context, payout *model.AuthorPayout) error
	GetAuthorPayout(ctx context.Context, authorUserID, period string) (*model.AuthorPayout, error)
	ListAuthorPayouts(ctx context.Context, authorUserID string) ([]*model.AuthorPayout, error)

	// Run cost breakdown
	UpsertRunCostBreakdown(ctx context.Context, b *model.RunCostBreakdown) error
	GetRunCostBreakdown(ctx context.Context, runID string) (*model.RunCostBreakdown, error)

	// Stripe customer mapping
	UpsertStripeCustomer(ctx context.Context, c *model.StripeCustomer) error
	GetStripeCustomer(ctx context.Context, userID string) (*model.StripeCustomer, error)

	// Free quota
	GetFreeQuotaUsage(ctx context.Context, userID, agentID, period string) (*model.FreeQuotaUsage, error)
	IncrementFreeQuotaUsage(ctx context.Context, userID, agentID, period string, limit int64) error

	// Platform usage tracking
	RecordPlatformUsage(ctx context.Context, record *model.PlatformUsageRecord) error
	GetPlatformUsageSummary(ctx context.Context, userID, period string) (*model.PlatformUsageSummary, error)
	GetPlatformUsageHistory(ctx context.Context, userID string, from, to time.Time, limit int) ([]model.PlatformUsageRecord, error)
	GetUsageQuota(ctx context.Context, userID string) (*model.UsageQuota, error)
	SetUsageQuota(ctx context.Context, quota *model.UsageQuota) error
	CheckQuota(ctx context.Context, userID, usageType string) (bool, float64, error)
	GetDailyUsage(ctx context.Context, userID, period string) ([]model.DailyUsage, error)

	// IM Binding methods
	CreateIMBinding(ctx context.Context, binding *model.IMBinding) error
	GetIMBindingByPlatform(ctx context.Context, platform, platformUserID string) (*model.IMBinding, error)
	ListIMBindingsByUser(ctx context.Context, userID string) ([]*model.IMBinding, error)
	DeleteIMBinding(ctx context.Context, id string) error
	CreateBindingCode(ctx context.Context, code *model.BindingCode) error
	GetBindingCode(ctx context.Context, code string) (*model.BindingCode, error)
	DeleteBindingCode(ctx context.Context, code string) error

	// IM Session methods
	CreateIMSession(ctx context.Context, session *model.IMSession) error
	GetIMSession(ctx context.Context, id string) (*model.IMSession, error)
	GetIMSessionByChat(ctx context.Context, platform, chatID string) (*model.IMSession, error)
	ListIMSessionsByUser(ctx context.Context, userID string) ([]*model.IMSession, error)
	UpdateIMSession(ctx context.Context, session *model.IMSession) error
	DeleteIMSession(ctx context.Context, id string) error

	// Workflow methods
	CreateWorkflow(ctx context.Context, w *model.Workflow) error
	GetWorkflow(ctx context.Context, id string) (*model.Workflow, error)
	ListWorkflows(ctx context.Context, userID string) ([]*model.Workflow, error)
	UpdateWorkflow(ctx context.Context, w *model.Workflow) error
	DeleteWorkflow(ctx context.Context, id string) error
	CreateWorkflowRun(ctx context.Context, r *model.WorkflowRun) error
	GetWorkflowRun(ctx context.Context, id string) (*model.WorkflowRun, error)
	UpdateWorkflowRun(ctx context.Context, r *model.WorkflowRun) error
	ListWorkflowRuns(ctx context.Context, workflowID string) ([]*model.WorkflowRun, error)

	// Schedule methods
	CreateSchedule(ctx context.Context, s *model.Schedule) error
	GetSchedule(ctx context.Context, id string) (*model.Schedule, error)
	ListSchedules(ctx context.Context, userID string) ([]*model.Schedule, error)
	UpdateSchedule(ctx context.Context, s *model.Schedule) error
	DeleteSchedule(ctx context.Context, id string) error
	ListDueSchedules(ctx context.Context, now time.Time) ([]*model.Schedule, error)

	// Team methods
	CreateTeam(ctx context.Context, team *model.Team) error
	GetTeam(ctx context.Context, id string) (*model.Team, error)
	ListTeamsByUser(ctx context.Context, userID string) ([]*model.Team, error)
	UpdateTeam(ctx context.Context, team *model.Team) error
	DeleteTeam(ctx context.Context, id string) error
	AddTeamMember(ctx context.Context, member *model.TeamMember) error
	RemoveTeamMember(ctx context.Context, teamID, userID string) error
	ListTeamMembers(ctx context.Context, teamID string) ([]*model.TeamMember, error)
	GetTeamMember(ctx context.Context, teamID, userID string) (*model.TeamMember, error)
}

// New creates a new Store from a TypedLazyConfig.
func New(cfg x.TypedLazyConfig) (Store, error) {
	return Create(cfg)
}
