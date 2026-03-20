package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"go.zoe.im/agentbox/internal/model"
)

// --- Workflow methods ---

func (s *sqliteStore) CreateWorkflow(ctx context.Context, w *model.Workflow) error {
	stepsJSON, _ := json.Marshal(w.Steps)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO workflows (id, user_id, name, description, steps, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		w.ID, w.UserID, w.Name, w.Description, string(stepsJSON), w.Status, w.CreatedAt, w.UpdatedAt,
	)
	return err
}

func (s *sqliteStore) GetWorkflow(ctx context.Context, id string) (*model.Workflow, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, description, steps, status, created_at, updated_at FROM workflows WHERE id = ?`, id,
	)
	return scanWorkflow(row)
}

func (s *sqliteStore) ListWorkflows(ctx context.Context, userID string) ([]*model.Workflow, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, name, description, steps, status, created_at, updated_at
		 FROM workflows WHERE user_id = ? ORDER BY created_at DESC`, userID,
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

func (s *sqliteStore) UpdateWorkflow(ctx context.Context, w *model.Workflow) error {
	stepsJSON, _ := json.Marshal(w.Steps)
	_, err := s.db.ExecContext(ctx,
		`UPDATE workflows SET name = ?, description = ?, steps = ?, status = ?, updated_at = ? WHERE id = ?`,
		w.Name, w.Description, string(stepsJSON), w.Status, w.UpdatedAt, w.ID,
	)
	return err
}

func (s *sqliteStore) DeleteWorkflow(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM workflows WHERE id = ?`, id)
	return err
}

func (s *sqliteStore) CreateWorkflowRun(ctx context.Context, r *model.WorkflowRun) error {
	stepsJSON, _ := json.Marshal(r.Steps)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO workflow_runs (id, workflow_id, status, steps, started_at, ended_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		r.ID, r.WorkflowID, r.Status, string(stepsJSON), r.StartedAt, r.EndedAt,
	)
	return err
}

func (s *sqliteStore) GetWorkflowRun(ctx context.Context, id string) (*model.WorkflowRun, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, workflow_id, status, steps, started_at, ended_at FROM workflow_runs WHERE id = ?`, id,
	)
	return scanWorkflowRun(row)
}

func (s *sqliteStore) UpdateWorkflowRun(ctx context.Context, r *model.WorkflowRun) error {
	stepsJSON, _ := json.Marshal(r.Steps)
	_, err := s.db.ExecContext(ctx,
		`UPDATE workflow_runs SET status = ?, steps = ?, started_at = ?, ended_at = ? WHERE id = ?`,
		r.Status, string(stepsJSON), r.StartedAt, r.EndedAt, r.ID,
	)
	return err
}

func (s *sqliteStore) ListWorkflowRuns(ctx context.Context, workflowID string) ([]*model.WorkflowRun, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, workflow_id, status, steps, started_at, ended_at
		 FROM workflow_runs WHERE workflow_id = ? ORDER BY COALESCE(started_at, ended_at) DESC`, workflowID,
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
		stepsJSON string
	)
	err := row.Scan(&wf.ID, &wf.UserID, &wf.Name, &wf.Description, &stepsJSON, &wf.Status, &wf.CreatedAt, &wf.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("workflow not found")
		}
		return nil, err
	}
	json.Unmarshal([]byte(stepsJSON), &wf.Steps)
	return &wf, nil
}

func scanWorkflowRows(rows *sql.Rows) (*model.Workflow, error) {
	var (
		wf        model.Workflow
		stepsJSON string
	)
	err := rows.Scan(&wf.ID, &wf.UserID, &wf.Name, &wf.Description, &stepsJSON, &wf.Status, &wf.CreatedAt, &wf.UpdatedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(stepsJSON), &wf.Steps)
	return &wf, nil
}

func scanWorkflowRun(row scannable) (*model.WorkflowRun, error) {
	var (
		r         model.WorkflowRun
		stepsJSON string
		startedAt sql.NullTime
		endedAt   sql.NullTime
	)
	err := row.Scan(&r.ID, &r.WorkflowID, &r.Status, &stepsJSON, &startedAt, &endedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("workflow run not found")
		}
		return nil, err
	}
	json.Unmarshal([]byte(stepsJSON), &r.Steps)
	if startedAt.Valid {
		t := startedAt.Time
		r.StartedAt = &t
	}
	if endedAt.Valid {
		t := endedAt.Time
		r.EndedAt = &t
	}
	return &r, nil
}

func scanWorkflowRunRows(rows *sql.Rows) (*model.WorkflowRun, error) {
	var (
		r         model.WorkflowRun
		stepsJSON string
		startedAt sql.NullTime
		endedAt   sql.NullTime
	)
	err := rows.Scan(&r.ID, &r.WorkflowID, &r.Status, &stepsJSON, &startedAt, &endedAt)
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(stepsJSON), &r.Steps)
	if startedAt.Valid {
		t := startedAt.Time
		r.StartedAt = &t
	}
	if endedAt.Valid {
		t := endedAt.Time
		r.EndedAt = &t
	}
	return &r, nil
}
