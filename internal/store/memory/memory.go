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
	mu   sync.RWMutex
	runs map[string]*model.Run
}

func New() store.Store {
	return &memoryStore{
		runs: make(map[string]*model.Run),
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
