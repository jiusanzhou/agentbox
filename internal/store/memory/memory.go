package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

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
	schedules     map[string]*model.Schedule
	teams         map[string]*model.Team
	teamMembers   map[string][]*model.TeamMember
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
		schedules:    make(map[string]*model.Schedule),
		teams:        make(map[string]*model.Team),
		teamMembers:  make(map[string][]*model.TeamMember),
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

// --- Schedule methods ---

func (s *memoryStore) CreateSchedule(_ context.Context, schedule *model.Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.schedules[schedule.ID]; exists {
		return fmt.Errorf("schedule %s already exists", schedule.ID)
	}
	s.schedules[schedule.ID] = schedule
	return nil
}

func (s *memoryStore) GetSchedule(_ context.Context, id string) (*model.Schedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	schedule, ok := s.schedules[id]
	if !ok {
		return nil, fmt.Errorf("schedule %s not found", id)
	}
	return schedule, nil
}

func (s *memoryStore) ListSchedules(_ context.Context, userID string) ([]*model.Schedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*model.Schedule
	for _, sc := range s.schedules {
		if sc.UserID == userID {
			result = append(result, sc)
		}
	}
	return result, nil
}

func (s *memoryStore) UpdateSchedule(_ context.Context, schedule *model.Schedule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.schedules[schedule.ID]; !exists {
		return fmt.Errorf("schedule %s not found", schedule.ID)
	}
	s.schedules[schedule.ID] = schedule
	return nil
}

func (s *memoryStore) DeleteSchedule(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.schedules, id)
	return nil
}

func (s *memoryStore) ListDueSchedules(_ context.Context, now time.Time) ([]*model.Schedule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*model.Schedule
	for _, sc := range s.schedules {
		if sc.Enabled && sc.NextRunAt != nil && !sc.NextRunAt.After(now) {
			result = append(result, sc)
		}
	}
	return result, nil
}

// --- Team methods ---

func (s *memoryStore) CreateTeam(_ context.Context, team *model.Team) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.teams[team.ID]; exists {
		return fmt.Errorf("team %s already exists", team.ID)
	}
	s.teams[team.ID] = team
	return nil
}

func (s *memoryStore) GetTeam(_ context.Context, id string) (*model.Team, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	team, ok := s.teams[id]
	if !ok {
		return nil, fmt.Errorf("team %s not found", id)
	}
	return team, nil
}

func (s *memoryStore) ListTeamsByUser(_ context.Context, userID string) ([]*model.Team, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*model.Team
	for teamID, members := range s.teamMembers {
		for _, m := range members {
			if m.UserID == userID {
				if team, ok := s.teams[teamID]; ok {
					result = append(result, team)
				}
				break
			}
		}
	}
	return result, nil
}

func (s *memoryStore) UpdateTeam(_ context.Context, team *model.Team) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.teams[team.ID]; !exists {
		return fmt.Errorf("team %s not found", team.ID)
	}
	s.teams[team.ID] = team
	return nil
}

func (s *memoryStore) DeleteTeam(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.teams, id)
	delete(s.teamMembers, id)
	return nil
}

func (s *memoryStore) AddTeamMember(_ context.Context, member *model.TeamMember) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range s.teamMembers[member.TeamID] {
		if m.UserID == member.UserID {
			return fmt.Errorf("user %s is already a member of team %s", member.UserID, member.TeamID)
		}
	}
	s.teamMembers[member.TeamID] = append(s.teamMembers[member.TeamID], member)
	return nil
}

func (s *memoryStore) RemoveTeamMember(_ context.Context, teamID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	members := s.teamMembers[teamID]
	for i, m := range members {
		if m.UserID == userID {
			s.teamMembers[teamID] = append(members[:i], members[i+1:]...)
			return nil
		}
	}
	return fmt.Errorf("member %s not found in team %s", userID, teamID)
}

func (s *memoryStore) ListTeamMembers(_ context.Context, teamID string) ([]*model.TeamMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	members := s.teamMembers[teamID]
	result := make([]*model.TeamMember, len(members))
	copy(result, members)
	return result, nil
}

func (s *memoryStore) GetTeamMember(_ context.Context, teamID, userID string) (*model.TeamMember, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, m := range s.teamMembers[teamID] {
		if m.UserID == userID {
			return m, nil
		}
	}
	return nil, fmt.Errorf("member %s not found in team %s", userID, teamID)
}
