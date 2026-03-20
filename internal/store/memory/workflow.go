package memory

import (
	"context"
	"fmt"

	"go.zoe.im/agentbox/internal/model"
)

// --- Workflow methods ---

func (s *memoryStore) CreateWorkflow(_ context.Context, w *model.Workflow) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.workflows[w.ID]; exists {
		return fmt.Errorf("workflow %s already exists", w.ID)
	}
	s.workflows[w.ID] = w
	return nil
}

func (s *memoryStore) GetWorkflow(_ context.Context, id string) (*model.Workflow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	wf, ok := s.workflows[id]
	if !ok {
		return nil, fmt.Errorf("workflow %s not found", id)
	}
	return wf, nil
}

func (s *memoryStore) ListWorkflows(_ context.Context, userID string) ([]*model.Workflow, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*model.Workflow
	for _, wf := range s.workflows {
		if wf.UserID == userID {
			result = append(result, wf)
		}
	}
	return result, nil
}

func (s *memoryStore) UpdateWorkflow(_ context.Context, w *model.Workflow) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.workflows[w.ID]; !exists {
		return fmt.Errorf("workflow %s not found", w.ID)
	}
	s.workflows[w.ID] = w
	return nil
}

func (s *memoryStore) DeleteWorkflow(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.workflows, id)
	return nil
}

func (s *memoryStore) CreateWorkflowRun(_ context.Context, r *model.WorkflowRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.workflowRuns[r.ID]; exists {
		return fmt.Errorf("workflow run %s already exists", r.ID)
	}
	s.workflowRuns[r.ID] = r
	return nil
}

func (s *memoryStore) GetWorkflowRun(_ context.Context, id string) (*model.WorkflowRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.workflowRuns[id]
	if !ok {
		return nil, fmt.Errorf("workflow run %s not found", id)
	}
	return r, nil
}

func (s *memoryStore) UpdateWorkflowRun(_ context.Context, r *model.WorkflowRun) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.workflowRuns[r.ID]; !exists {
		return fmt.Errorf("workflow run %s not found", r.ID)
	}
	s.workflowRuns[r.ID] = r
	return nil
}

func (s *memoryStore) ListWorkflowRuns(_ context.Context, workflowID string) ([]*model.WorkflowRun, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []*model.WorkflowRun
	for _, r := range s.workflowRuns {
		if r.WorkflowID == workflowID {
			result = append(result, r)
		}
	}
	return result, nil
}
