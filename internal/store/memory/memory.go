package memory

import (
	"context"
	"fmt"
	"sync"

	"go.zoe.im/agentbox/internal/model"
	"go.zoe.im/agentbox/internal/store"
	"go.zoe.im/x"
)

func init() {
	store.Register("memory", func(cfg x.TypedLazyConfig, opts ...any) (store.Store, error) {
		return New(), nil
	})
}

type memoryStore struct {
	mu            sync.RWMutex
	runs          map[string]*model.Run
	users         map[string]*model.User
	integrations  map[string]*model.Integration
	agentDNAs     map[string]*model.AgentDNA
	billing       *billingData
	imBindings    map[string]*model.IMBinding
	bindingCodes  map[string]*model.BindingCode
	workflows     map[string]*model.Workflow
	workflowRuns  map[string]*model.WorkflowRun
}

func New() store.Store {
	return &memoryStore{
		runs:         make(map[string]*model.Run),
		users:        make(map[string]*model.User),
		integrations: make(map[string]*model.Integration),
		agentDNAs:    make(map[string]*model.AgentDNA),
		billing:      newBillingData(),
		imBindings:   make(map[string]*model.IMBinding),
		bindingCodes: make(map[string]*model.BindingCode),
		workflows:    make(map[string]*model.Workflow),
		workflowRuns: make(map[string]*model.WorkflowRun),
	}
}

func (s *memoryStore) CreateRun(_ context.Context, run *model.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.runs[run.ID]; exists {
		return fmt.Errorf("run %s already exists", run.ID)
	}
	s.runs[run.ID] = run
	return nil
}

func (s *memoryStore) GetRun(_ context.Context, id string) (*model.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, ok := s.runs[id]
	if !ok {
		return nil, fmt.Errorf("run %s not found", id)
	}
	return run, nil
}

func (s *memoryStore) UpdateRun(_ context.Context, run *model.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.runs[run.ID]; !exists {
		return fmt.Errorf("run %s not found", run.ID)
	}
	s.runs[run.ID] = run
	return nil
}

func (s *memoryStore) ListRuns(_ context.Context, limit, offset int) ([]*model.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	runs := make([]*model.Run, 0, len(s.runs))
	for _, r := range s.runs {
		runs = append(runs, r)
	}
	// Simple pagination
	if offset >= len(runs) {
		return nil, nil
	}
	end := offset + limit
	if end > len(runs) {
		end = len(runs)
	}
	return runs[offset:end], nil
}

func (s *memoryStore) DeleteRun(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.runs, id)
	return nil
}

// --- User methods ---

func (s *memoryStore) CreateUser(_ context.Context, user *model.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.users {
		if u.Email == user.Email {
			return fmt.Errorf("user with email %s already exists", user.Email)
		}
	}
	s.users[user.ID] = user
	return nil
}

func (s *memoryStore) GetUser(_ context.Context, id string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[id]
	if !ok {
		return nil, fmt.Errorf("user %s not found", id)
	}
	return user, nil
}

func (s *memoryStore) GetUserByEmail(_ context.Context, email string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.Email == email {
			return u, nil
		}
	}
	return nil, fmt.Errorf("user with email %s not found", email)
}

func (s *memoryStore) GetUserByAPIKey(_ context.Context, apiKeyHash string) (*model.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.APIKey == apiKeyHash {
			return u, nil
		}
	}
	return nil, fmt.Errorf("user not found")
}

func (s *memoryStore) UpdateUser(_ context.Context, user *model.User) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.users[user.ID]; !exists {
		return fmt.Errorf("user %s not found", user.ID)
	}
	s.users[user.ID] = user
	return nil
}

// --- Integration methods ---

func (s *memoryStore) CreateIntegration(_ context.Context, i *model.Integration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.integrations[i.ID] = i
	return nil
}

func (s *memoryStore) GetIntegration(_ context.Context, id string) (*model.Integration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	i, ok := s.integrations[id]
	if !ok {
		return nil, fmt.Errorf("integration %s not found", id)
	}
	return i, nil
}

func (s *memoryStore) ListIntegrations(_ context.Context, userID string) ([]*model.Integration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*model.Integration
	for _, i := range s.integrations {
		if i.UserID == userID {
			result = append(result, i)
		}
	}
	return result, nil
}

func (s *memoryStore) UpdateIntegration(_ context.Context, i *model.Integration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.integrations[i.ID]; !exists {
		return fmt.Errorf("integration %s not found", i.ID)
	}
	s.integrations[i.ID] = i
	return nil
}

func (s *memoryStore) DeleteIntegration(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.integrations, id)
	return nil
}

func (s *memoryStore) ListAllEnabledIntegrations(_ context.Context) ([]*model.Integration, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*model.Integration
	for _, i := range s.integrations {
		if i.Enabled {
			result = append(result, i)
		}
	}
	return result, nil
}

// --- AgentDNA methods ---

func (s *memoryStore) CreateAgentDNA(_ context.Context, agent *model.AgentDNA) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.agentDNAs[agent.ID]; exists {
		return fmt.Errorf("agent %s already exists", agent.ID)
	}
	for _, a := range s.agentDNAs {
		if a.Slug == agent.Slug {
			return fmt.Errorf("agent with slug %s already exists", agent.Slug)
		}
	}
	s.agentDNAs[agent.ID] = agent
	return nil
}

func (s *memoryStore) GetAgentDNA(_ context.Context, id string) (*model.AgentDNA, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	a, ok := s.agentDNAs[id]
	if !ok {
		return nil, fmt.Errorf("agent %s not found", id)
	}
	return a, nil
}

func (s *memoryStore) GetAgentDNABySlug(_ context.Context, slug string) (*model.AgentDNA, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, a := range s.agentDNAs {
		if a.Slug == slug {
			return a, nil
		}
	}
	return nil, fmt.Errorf("agent with slug %s not found", slug)
}

func (s *memoryStore) UpdateAgentDNA(_ context.Context, agent *model.AgentDNA) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.agentDNAs[agent.ID]; !exists {
		return fmt.Errorf("agent %s not found", agent.ID)
	}
	s.agentDNAs[agent.ID] = agent
	return nil
}

func (s *memoryStore) DeleteAgentDNA(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.agentDNAs, id)
	return nil
}

func (s *memoryStore) ListAgentDNAs(_ context.Context, opts model.AgentDNAListOptions) ([]*model.AgentDNA, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*model.AgentDNA
	for _, a := range s.agentDNAs {
		if opts.Status != "" && a.Status != opts.Status {
			continue
		}
		if opts.Runtime != "" && a.Manifest != nil && a.Manifest.Runtime != opts.Runtime {
			continue
		}
		result = append(result, a)
	}
	// Simple pagination
	offset := opts.Offset
	if offset >= len(result) {
		return nil, nil
	}
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], nil
}

func (s *memoryStore) IncrementAgentDNADownloads(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	a, ok := s.agentDNAs[id]
	if !ok {
		return fmt.Errorf("agent %s not found", id)
	}
	a.Downloads++
	return nil
}
