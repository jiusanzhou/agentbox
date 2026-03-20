package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"go.zoe.im/agentbox/internal/auth"
	"go.zoe.im/agentbox/internal/config"
	"go.zoe.im/agentbox/internal/model"
)

// ---------------------------------------------------------------------------
// Mock store -- full store.Store implementation (only agent DNA methods have
// real logic; everything else is a stub).
// ---------------------------------------------------------------------------

type registryMockStore struct {
	mu      sync.Mutex
	agents  map[string]*model.AgentDNA // by ID
	slugIdx map[string]*model.AgentDNA // by slug
}

func newRegistryMockStore() *registryMockStore {
	return &registryMockStore{
		agents:  make(map[string]*model.AgentDNA),
		slugIdx: make(map[string]*model.AgentDNA),
	}
}

var errRegistryNotFound = errors.New("not found")

func (s *registryMockStore) CreateAgentDNA(_ context.Context, a *model.AgentDNA) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agents[a.ID] = a
	s.slugIdx[a.Slug] = a
	return nil
}
func (s *registryMockStore) GetAgentDNA(_ context.Context, id string) (*model.AgentDNA, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.agents[id]
	if !ok {
		return nil, errRegistryNotFound
	}
	return a, nil
}
func (s *registryMockStore) GetAgentDNABySlug(_ context.Context, slug string) (*model.AgentDNA, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.slugIdx[slug]
	if !ok {
		return nil, errRegistryNotFound
	}
	return a, nil
}
func (s *registryMockStore) UpdateAgentDNA(_ context.Context, a *model.AgentDNA) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agents[a.ID] = a
	s.slugIdx[a.Slug] = a
	return nil
}
func (s *registryMockStore) DeleteAgentDNA(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a, ok := s.agents[id]; ok {
		delete(s.slugIdx, a.Slug)
	}
	delete(s.agents, id)
	return nil
}
func (s *registryMockStore) ListAgentDNAs(_ context.Context, _ model.AgentDNAListOptions) ([]*model.AgentDNA, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*model.AgentDNA
	for _, a := range s.agents {
		out = append(out, a)
	}
	return out, nil
}
func (s *registryMockStore) IncrementAgentDNADownloads(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if a, ok := s.agents[id]; ok {
		a.Downloads++
	}
	return nil
}

// --- stubs for the rest of store.Store ---
func (s *registryMockStore) CreateRun(context.Context, *model.Run) error   { return nil }
func (s *registryMockStore) GetRun(context.Context, string) (*model.Run, error) {
	return nil, errRegistryNotFound
}
func (s *registryMockStore) UpdateRun(context.Context, *model.Run) error   { return nil }
func (s *registryMockStore) ListRuns(context.Context, int, int) ([]*model.Run, error) {
	return nil, nil
}
func (s *registryMockStore) DeleteRun(context.Context, string) error       { return nil }
func (s *registryMockStore) CreateUser(context.Context, *model.User) error { return nil }
func (s *registryMockStore) GetUser(context.Context, string) (*model.User, error) {
	return nil, errRegistryNotFound
}
func (s *registryMockStore) GetUserByEmail(context.Context, string) (*model.User, error) {
	return nil, errRegistryNotFound
}
func (s *registryMockStore) GetUserByAPIKey(context.Context, string) (*model.User, error) {
	return nil, errRegistryNotFound
}
func (s *registryMockStore) UpdateUser(context.Context, *model.User) error { return nil }
func (s *registryMockStore) CreateIntegration(context.Context, *model.Integration) error {
	return nil
}
func (s *registryMockStore) GetIntegration(context.Context, string) (*model.Integration, error) {
	return nil, errRegistryNotFound
}
func (s *registryMockStore) ListIntegrations(context.Context, string) ([]*model.Integration, error) {
	return nil, nil
}
func (s *registryMockStore) UpdateIntegration(context.Context, *model.Integration) error { return nil }
func (s *registryMockStore) DeleteIntegration(context.Context, string) error             { return nil }
func (s *registryMockStore) ListAllEnabledIntegrations(context.Context) ([]*model.Integration, error) {
	return nil, nil
}
func (s *registryMockStore) CreateSubscription(context.Context, *model.Subscription) error {
	return nil
}
func (s *registryMockStore) GetSubscription(context.Context, string) (*model.Subscription, error) {
	return nil, errRegistryNotFound
}
func (s *registryMockStore) GetActiveSubscription(context.Context, string, string) (*model.Subscription, error) {
	return nil, errRegistryNotFound
}
func (s *registryMockStore) UpdateSubscription(context.Context, *model.Subscription) error {
	return nil
}
func (s *registryMockStore) ListSubscriptions(context.Context, string) ([]*model.Subscription, error) {
	return nil, nil
}
func (s *registryMockStore) CreateUsageRecord(context.Context, *model.UsageRecord) error { return nil }
func (s *registryMockStore) GetUsageSummary(context.Context, string, string, string) (*model.UsageSummary, error) {
	return nil, errRegistryNotFound
}
func (s *registryMockStore) ListUsageRecords(context.Context, model.BillingListOptions) ([]*model.UsageRecord, error) {
	return nil, nil
}
func (s *registryMockStore) CreateAuthorPayout(context.Context, *model.AuthorPayout) error {
	return nil
}
func (s *registryMockStore) GetAuthorPayout(context.Context, string, string) (*model.AuthorPayout, error) {
	return nil, errRegistryNotFound
}
func (s *registryMockStore) ListAuthorPayouts(context.Context, string) ([]*model.AuthorPayout, error) {
	return nil, nil
}
func (s *registryMockStore) UpsertRunCostBreakdown(context.Context, *model.RunCostBreakdown) error {
	return nil
}
func (s *registryMockStore) GetRunCostBreakdown(context.Context, string) (*model.RunCostBreakdown, error) {
	return nil, errRegistryNotFound
}
func (s *registryMockStore) UpsertStripeCustomer(context.Context, *model.StripeCustomer) error {
	return nil
}
func (s *registryMockStore) GetStripeCustomer(context.Context, string) (*model.StripeCustomer, error) {
	return nil, errRegistryNotFound
}
func (s *registryMockStore) GetFreeQuotaUsage(context.Context, string, string, string) (*model.FreeQuotaUsage, error) {
	return nil, errRegistryNotFound
}
func (s *registryMockStore) IncrementFreeQuotaUsage(context.Context, string, string, string, int64) error {
	return nil
}

// ---------------------------------------------------------------------------
// Test helper: build a minimal Service with just the store and config set.
// ---------------------------------------------------------------------------

func newTestService(ms *registryMockStore) *Service {
	return &Service{
		store: ms,
		cfg:   config.NewConfig(),
	}
}

// ctxWithUser returns a context carrying the given user (same key as auth middleware).
func ctxWithUser(ctx context.Context, u *model.User) context.Context {
	return context.WithValue(ctx, auth.UserContextKey, u)
}

// ---------------------------------------------------------------------------
// TestListRegistryAgents
// ---------------------------------------------------------------------------

func TestListRegistryAgents(t *testing.T) {
	t.Run("returns agents", func(t *testing.T) {
		ms := newRegistryMockStore()
		svc := newTestService(ms)

		_ = ms.CreateAgentDNA(context.Background(), &model.AgentDNA{
			ID: "a1", Slug: "agent-one",
			Identity: &model.AgentIdentity{Name: "One"},
			Manifest: &model.AgentManifest{Version: "1.0.0"},
			Status:   model.AgentDNAStatusPublished,
		})
		_ = ms.CreateAgentDNA(context.Background(), &model.AgentDNA{
			ID: "a2", Slug: "agent-two",
			Identity: &model.AgentIdentity{Name: "Two"},
			Manifest: &model.AgentManifest{Version: "1.0.0"},
			Status:   model.AgentDNAStatusPublished,
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/registry/agents", nil)
		w := httptest.NewRecorder()
		svc.ListRegistryAgents(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}

		var agents []*model.AgentDNA
		if err := json.NewDecoder(w.Body).Decode(&agents); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if len(agents) != 2 {
			t.Errorf("len(agents) = %d, want 2", len(agents))
		}
	})

	t.Run("empty list returns empty array", func(t *testing.T) {
		ms := newRegistryMockStore()
		svc := newTestService(ms)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/registry/agents", nil)
		w := httptest.NewRecorder()
		svc.ListRegistryAgents(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200", w.Code)
		}
		body := strings.TrimSpace(w.Body.String())
		if body != "[]" {
			t.Errorf("body = %q, want []", body)
		}
	})
}

// ---------------------------------------------------------------------------
// TestGetRegistryAgent
// ---------------------------------------------------------------------------

func TestGetRegistryAgent(t *testing.T) {
	ms := newRegistryMockStore()
	svc := newTestService(ms)

	_ = ms.CreateAgentDNA(context.Background(), &model.AgentDNA{
		ID: "a1", Slug: "my-agent",
		Identity: &model.AgentIdentity{Name: "My Agent"},
		Manifest: &model.AgentManifest{Version: "1.0.0"},
		Status:   model.AgentDNAStatusPublished,
	})

	t.Run("found by ID", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/registry/agents/a1", nil)
		req.SetPathValue("id", "a1")
		w := httptest.NewRecorder()
		svc.GetRegistryAgent(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
		var agent model.AgentDNA
		json.NewDecoder(w.Body).Decode(&agent)
		if agent.ID != "a1" {
			t.Errorf("ID = %q, want a1", agent.ID)
		}
	})

	t.Run("found by slug", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/registry/agents/my-agent", nil)
		req.SetPathValue("id", "my-agent")
		w := httptest.NewRecorder()
		svc.GetRegistryAgent(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200; body = %s", w.Code, w.Body.String())
		}
		var agent model.AgentDNA
		json.NewDecoder(w.Body).Decode(&agent)
		if agent.Slug != "my-agent" {
			t.Errorf("Slug = %q, want my-agent", agent.Slug)
		}
	})

	t.Run("not found", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/registry/agents/does-not-exist", nil)
		req.SetPathValue("id", "does-not-exist")
		w := httptest.NewRecorder()
		svc.GetRegistryAgent(w, req)

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", w.Code)
		}
	})
}

// ---------------------------------------------------------------------------
// TestCreateRegistryAgent
// ---------------------------------------------------------------------------

func TestCreateRegistryAgent(t *testing.T) {
	testUser := &model.User{ID: "user-1", Email: "zoe@example.com", Name: "Zoe"}

	t.Run("valid creation", func(t *testing.T) {
		ms := newRegistryMockStore()
		svc := newTestService(ms)

		body := `{
			"slug": "new-agent",
			"identity": {"name": "New Agent"},
			"manifest": {"version": "1.0.0", "runtime": "claude"}
		}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/registry/agents", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(ctxWithUser(req.Context(), testUser))
		w := httptest.NewRecorder()

		svc.CreateRegistryAgent(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d, want 201; body = %s", w.Code, w.Body.String())
		}

		var agent model.AgentDNA
		json.NewDecoder(w.Body).Decode(&agent)
		if agent.Slug != "new-agent" {
			t.Errorf("Slug = %q, want new-agent", agent.Slug)
		}
		if agent.UserID != "user-1" {
			t.Errorf("UserID = %q, want user-1", agent.UserID)
		}
		if agent.Status != model.AgentDNAStatusDraft {
			t.Errorf("Status = %q, want draft", agent.Status)
		}
		if agent.Version != "1.0.0" {
			t.Errorf("Version = %q, want 1.0.0", agent.Version)
		}
	})

	t.Run("unauthorized (no user in context)", func(t *testing.T) {
		ms := newRegistryMockStore()
		svc := newTestService(ms)

		body := `{"slug":"x","identity":{"name":"X"},"manifest":{"version":"1.0.0"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/registry/agents", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		svc.CreateRegistryAgent(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", w.Code)
		}
	})

	t.Run("missing slug", func(t *testing.T) {
		ms := newRegistryMockStore()
		svc := newTestService(ms)

		body := `{"identity":{"name":"X"},"manifest":{"version":"1.0.0"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/registry/agents", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(ctxWithUser(req.Context(), testUser))
		w := httptest.NewRecorder()

		svc.CreateRegistryAgent(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "slug is required") {
			t.Errorf("body = %q, want 'slug is required'", w.Body.String())
		}
	})

	t.Run("missing identity name", func(t *testing.T) {
		ms := newRegistryMockStore()
		svc := newTestService(ms)

		body := `{"slug":"x","identity":{},"manifest":{"version":"1.0.0"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/registry/agents", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(ctxWithUser(req.Context(), testUser))
		w := httptest.NewRecorder()

		svc.CreateRegistryAgent(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "identity.name is required") {
			t.Errorf("body = %q", w.Body.String())
		}
	})

	t.Run("missing manifest", func(t *testing.T) {
		ms := newRegistryMockStore()
		svc := newTestService(ms)

		body := `{"slug":"x","identity":{"name":"X"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/registry/agents", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(ctxWithUser(req.Context(), testUser))
		w := httptest.NewRecorder()

		svc.CreateRegistryAgent(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400; body = %s", w.Code, w.Body.String())
		}
		if !strings.Contains(w.Body.String(), "manifest is required") {
			t.Errorf("body = %q", w.Body.String())
		}
	})

	t.Run("invalid json body", func(t *testing.T) {
		ms := newRegistryMockStore()
		svc := newTestService(ms)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/registry/agents", strings.NewReader(`{bad json`))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(ctxWithUser(req.Context(), testUser))
		w := httptest.NewRecorder()

		svc.CreateRegistryAgent(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400", w.Code)
		}
	})

	t.Run("default version when manifest.version empty", func(t *testing.T) {
		ms := newRegistryMockStore()
		svc := newTestService(ms)

		body := `{"slug":"x","identity":{"name":"X"},"manifest":{"runtime":"claude"}}`
		req := httptest.NewRequest(http.MethodPost, "/api/v1/registry/agents", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(ctxWithUser(req.Context(), testUser))
		w := httptest.NewRecorder()

		svc.CreateRegistryAgent(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("status = %d; body = %s", w.Code, w.Body.String())
		}
		var agent model.AgentDNA
		json.NewDecoder(w.Body).Decode(&agent)
		if agent.Version != "0.1.0" {
			t.Errorf("Version = %q, want 0.1.0", agent.Version)
		}
	})
}
