package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.zoe.im/agentbox/internal/auth"
	"go.zoe.im/agentbox/internal/model"
)

// CreateWorkflow handles POST /api/v1/workflows
func (s *Service) CreateWorkflow(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var wf model.Workflow
	if err := json.NewDecoder(r.Body).Decode(&wf); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	wf.ID = shortID()
	wf.UserID = user.ID
	if wf.Status == "" {
		wf.Status = "draft"
	}
	wf.CreatedAt = time.Now()
	wf.UpdatedAt = time.Now()

	// Assign IDs to steps if not set
	for i := range wf.Steps {
		if wf.Steps[i].ID == "" {
			wf.Steps[i].ID = shortID()
		}
	}

	if err := s.store.CreateWorkflow(r.Context(), &wf); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(wf)
}

// ListWorkflows handles GET /api/v1/workflows
func (s *Service) ListWorkflows(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	workflows, err := s.store.ListWorkflows(r.Context(), user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if workflows == nil {
		workflows = []*model.Workflow{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workflows)
}

// GetWorkflow handles GET /api/v1/workflows/{id}
func (s *Service) GetWorkflow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	wf, err := s.store.GetWorkflow(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wf)
}

// UpdateWorkflow handles PUT /api/v1/workflows/{id}
func (s *Service) UpdateWorkflow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, err := s.store.GetWorkflow(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	var update model.Workflow
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	existing.Name = update.Name
	existing.Description = update.Description
	existing.Steps = update.Steps
	if update.Status != "" {
		existing.Status = update.Status
	}
	existing.UpdatedAt = time.Now()

	if err := s.store.UpdateWorkflow(r.Context(), existing); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(existing)
}

// DeleteWorkflowHandler handles DELETE /api/v1/workflows/{id}
func (s *Service) DeleteWorkflowHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteWorkflow(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RunWorkflow handles POST /api/v1/workflows/{id}/run
func (s *Service) RunWorkflow(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	wf, err := s.store.GetWorkflow(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	run := &model.WorkflowRun{
		ID:         shortID(),
		WorkflowID: wf.ID,
		Status:     "pending",
	}

	// Initialize step runs
	for _, step := range wf.Steps {
		run.Steps = append(run.Steps, model.WorkflowStepRun{
			StepID: step.ID,
			Status: "pending",
		})
	}

	if err := s.store.CreateWorkflowRun(r.Context(), run); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Execute workflow asynchronously
	go s.executeWorkflow(context.Background(), wf, run)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(run)
}

// ListWorkflowRuns handles GET /api/v1/workflows/{id}/runs
func (s *Service) ListWorkflowRuns(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	runs, err := s.store.ListWorkflowRuns(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if runs == nil {
		runs = []*model.WorkflowRun{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(runs)
}

// executeWorkflow runs workflow steps in dependency order.
func (s *Service) executeWorkflow(ctx context.Context, wf *model.Workflow, run *model.WorkflowRun) {
	now := time.Now()
	run.StartedAt = &now
	run.Status = "running"
	s.store.UpdateWorkflowRun(ctx, run)

	// Build step map and outputs
	stepMap := make(map[string]*model.WorkflowStep)
	for i := range wf.Steps {
		stepMap[wf.Steps[i].ID] = &wf.Steps[i]
	}
	outputs := make(map[string]string) // stepID -> output
	completed := make(map[string]bool)

	for {
		// Find next runnable steps (all dependencies completed)
		var ready []string
		allDone := true
		for _, step := range wf.Steps {
			if completed[step.ID] {
				continue
			}
			allDone = false
			depsOK := true
			for _, dep := range step.DependsOn {
				if !completed[dep] {
					depsOK = false
					break
				}
			}
			if depsOK {
				ready = append(ready, step.ID)
			}
		}

		if allDone {
			break
		}
		if len(ready) == 0 {
			// Deadlock -- deps can't be satisfied
			run.Status = "failed"
			end := time.Now()
			run.EndedAt = &end
			s.store.UpdateWorkflowRun(ctx, run)
			return
		}

		// Execute ready steps (sequentially for simplicity)
		for _, stepID := range ready {
			step := stepMap[stepID]
			output, err := s.executeWorkflowStep(ctx, wf, step, outputs)

			// Update step run status
			for i := range run.Steps {
				if run.Steps[i].StepID == stepID {
					stepNow := time.Now()
					run.Steps[i].StartedAt = &stepNow
					if err != nil {
						run.Steps[i].Status = "failed"
						run.Steps[i].Output = err.Error()
					} else {
						run.Steps[i].Status = "completed"
						run.Steps[i].Output = output
					}
					run.Steps[i].EndedAt = &stepNow
				}
			}

			if err != nil {
				s.logger.Error("workflow step failed", "workflow", wf.ID, "step", stepID, "err", err)
				run.Status = "failed"
				end := time.Now()
				run.EndedAt = &end
				s.store.UpdateWorkflowRun(ctx, run)
				return
			}

			outputs[stepID] = output
			completed[stepID] = true
			s.store.UpdateWorkflowRun(ctx, run)
		}
	}

	run.Status = "completed"
	end := time.Now()
	run.EndedAt = &end
	s.store.UpdateWorkflowRun(ctx, run)
}

// executeWorkflowStep runs a single workflow step.
func (s *Service) executeWorkflowStep(ctx context.Context, wf *model.Workflow, step *model.WorkflowStep, outputs map[string]string) (string, error) {
	// Resolve input template -- replace {{prev.output}} and {{step.STEPID.output}}
	input := step.Input
	for id, out := range outputs {
		input = strings.ReplaceAll(input, "{{step."+id+".output}}", out)
	}
	// Replace {{prev.output}} with the last completed step's output
	if len(step.DependsOn) > 0 {
		lastDep := step.DependsOn[len(step.DependsOn)-1]
		if out, ok := outputs[lastDep]; ok {
			input = strings.ReplaceAll(input, "{{prev.output}}", out)
		}
	}

	s.logger.Info("executing workflow step", "workflow", wf.ID, "step", step.ID, "agent", step.AgentID, "runtime", step.Runtime)

	rt := step.Runtime
	if rt == "" {
		rt = "claude"
	}

	run := &model.Run{
		ID:        shortID(),
		UserID:    wf.UserID,
		Name:      fmt.Sprintf("workflow-%s-step-%s", wf.ID, step.ID),
		Mode:      model.RunModeRun,
		Runtime:   rt,
		AgentFile: input,
		Config: model.RunConfig{
			Timeout: 3600,
		},
	}

	if err := s.engine.Submit(ctx, run); err != nil {
		return "", fmt.Errorf("submit step: %w", err)
	}

	// Wait for completion
	for {
		current, err := s.engine.Get(ctx, run.ID)
		if err != nil {
			return "", fmt.Errorf("get run: %w", err)
		}
		switch current.Status {
		case model.RunStatusCompleted:
			if current.Result != nil {
				return current.Result.Output, nil
			}
			return "", nil
		case model.RunStatusFailed:
			errMsg := "step failed"
			if current.Result != nil && current.Result.Error != "" {
				errMsg = current.Result.Error
			}
			return "", fmt.Errorf("%s", errMsg)
		case model.RunStatusCancelled:
			return "", fmt.Errorf("step cancelled")
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
			// poll again
		}
	}
}
