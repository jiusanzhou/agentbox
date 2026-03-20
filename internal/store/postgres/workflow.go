package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"

	"go.zoe.im/agentbox/internal/model"
)

// --- Workflow methods ---

func (s *pgStore) CreateWorkflow(ctx context.Context, w *model.Workflow) error {
	stepsJSON, _ := json.Marshal(w.Steps)
	_, err := s.pool.Exec(ctx,
		`INSERT INTO workflows (id, user_id, name, description, steps, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		w.ID, w.UserID, w.Name, w.Description, stepsJSON, w.Status, w.CreatedAt, w.UpdatedAt,
	)
	return err
}

func (s *pgStore) GetWorkflow(ctx context.Context, id string) (*model.Workflow, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, user_id, name, description, steps, status, created_at, updated_at FROM workflows WHERE id = $1`, id,
	)
	return scanWorkflow(row)
}

func (s *pgStore) ListWorkflows(ctx context.Context, userID string) ([]*model.Workflow, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, name, description, steps, status, created_at, updated_at
		 FROM workflows WHERE user_id = $1 ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var workflows []*model.Workflow
	for rows.Next() {
		wf, err := scanWorkflowRows(rows)
		if err != nil {
			return nil, err
		}
		workflows = append(workflows, wf)
	}
	return workflows, rows.Err()
}

func (s *pgStore) UpdateWorkflow(ctx context.Context, w *model.Workflow) error {
	stepsJSON, _ := json.Marshal(w.Steps)
	_, err := s.pool.Exec(ctx,
		`UPDATE workflows SET name = $1, description = $2, steps = $3, status = $4, updated_at = $5 WHERE id = $6`,
		w.Name, w.Description, stepsJSON, w.Status, w.UpdatedAt, w.ID,
	)
	return err
}

func (s *pgStore) DeleteWorkflow(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM workflows WHERE id = $1`, id)
	return err
}

func (s *pgStore) CreateWorkflowRun(ctx context.Context, r *model.WorkflowRun) error {
	stepsJSON, _ := json.Marshal(r.Steps)
	_, err := s.pool.Exec(ctx,
		`INSERT INTO workflow_runs (id, workflow_id, status, steps, started_at, ended_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		r.ID, r.WorkflowID, r.Status, stepsJSON, r.StartedAt, r.EndedAt,
	)
	return err
}

func (s *pgStore) GetWorkflowRun(ctx context.Context, id string) (*model.WorkflowRun, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, workflow_id, status, steps, started_at, ended_at FROM workflow_runs WHERE id = $1`, id,
	)
	return scanWorkflowRun(row)
}

func (s *pgStore) UpdateWorkflowRun(ctx context.Context, r *model.WorkflowRun) error {
	stepsJSON, _ := json.Marshal(r.Steps)
	_, err := s.pool.Exec(ctx,
		`UPDATE workflow_runs SET status = $1, steps = $2, started_at = $3, ended_at = $4 WHERE id = $5`,
		r.Status, stepsJSON, r.StartedAt, r.EndedAt, r.ID,
	)
	return err
}

func (s *pgStore) ListWorkflowRuns(ctx context.Context, workflowID string) ([]*model.WorkflowRun, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, workflow_id, status, steps, started_at, ended_at
		 FROM workflow_runs WHERE workflow_id = $1 ORDER BY COALESCE(started_at, ended_at) DESC`, workflowID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*model.WorkflowRun
	for rows.Next() {
		r, err := scanWorkflowRunRows(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}
	return runs, rows.Err()
}

// --- Workflow scan helpers ---

func scanWorkflow(row scannable) (*model.Workflow, error) {
	var (
		wf        model.Workflow
		stepsJSON []byte
	)
	err := row.Scan(&wf.ID, &wf.UserID, &wf.Name, &wf.Description, &stepsJSON, &wf.Status, &wf.CreatedAt, &wf.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("workflow not found")
		}
		return nil, err
	}
	json.Unmarshal(stepsJSON, &wf.Steps)
	return &wf, nil
}

func scanWorkflowRows(rows pgx.Rows) (*model.Workflow, error) {
	var (
		wf        model.Workflow
		stepsJSON []byte
	)
	err := rows.Scan(&wf.ID, &wf.UserID, &wf.Name, &wf.Description, &stepsJSON, &wf.Status, &wf.CreatedAt, &wf.UpdatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(stepsJSON, &wf.Steps)
	return &wf, nil
}

func scanWorkflowRun(row scannable) (*model.WorkflowRun, error) {
	var (
		r         model.WorkflowRun
		stepsJSON []byte
	)
	err := row.Scan(&r.ID, &r.WorkflowID, &r.Status, &stepsJSON, &r.StartedAt, &r.EndedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("workflow run not found")
		}
		return nil, err
	}
	json.Unmarshal(stepsJSON, &r.Steps)
	return &r, nil
}

func scanWorkflowRunRows(rows pgx.Rows) (*model.WorkflowRun, error) {
	var (
		r         model.WorkflowRun
		stepsJSON []byte
	)
	err := rows.Scan(&r.ID, &r.WorkflowID, &r.Status, &stepsJSON, &r.StartedAt, &r.EndedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal(stepsJSON, &r.Steps)
	return &r, nil
}
