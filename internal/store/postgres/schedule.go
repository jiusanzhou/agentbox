package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"go.zoe.im/agentbox/internal/model"
)

// --- Schedule methods ---

func (s *pgStore) CreateSchedule(ctx context.Context, sc *model.Schedule) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO schedules (id, user_id, name, agent_id, runtime, cron_expr, timezone, input, enabled, last_run_at, next_run_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		sc.ID, sc.UserID, sc.Name, sc.AgentID, sc.Runtime, sc.CronExpr,
		sc.Timezone, sc.Input, sc.Enabled, sc.LastRunAt, sc.NextRunAt, sc.CreatedAt,
	)
	return err
}

func (s *pgStore) GetSchedule(ctx context.Context, id string) (*model.Schedule, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, user_id, name, agent_id, runtime, cron_expr, timezone, input, enabled, last_run_at, next_run_at, created_at
		 FROM schedules WHERE id = $1`, id,
	)
	return scanSchedule(row)
}

func (s *pgStore) ListSchedules(ctx context.Context, userID string) ([]*model.Schedule, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, name, agent_id, runtime, cron_expr, timezone, input, enabled, last_run_at, next_run_at, created_at
		 FROM schedules WHERE user_id = $1 ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []*model.Schedule
	for rows.Next() {
		sc, err := scanScheduleRows(rows)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, sc)
	}
	return schedules, rows.Err()
}

func (s *pgStore) UpdateSchedule(ctx context.Context, sc *model.Schedule) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE schedules SET name = $1, agent_id = $2, runtime = $3, cron_expr = $4, timezone = $5,
			input = $6, enabled = $7, last_run_at = $8, next_run_at = $9
		 WHERE id = $10`,
		sc.Name, sc.AgentID, sc.Runtime, sc.CronExpr, sc.Timezone,
		sc.Input, sc.Enabled, sc.LastRunAt, sc.NextRunAt, sc.ID,
	)
	return err
}

func (s *pgStore) DeleteSchedule(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM schedules WHERE id = $1`, id)
	return err
}

func (s *pgStore) ListDueSchedules(ctx context.Context, now time.Time) ([]*model.Schedule, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, name, agent_id, runtime, cron_expr, timezone, input, enabled, last_run_at, next_run_at, created_at
		 FROM schedules WHERE enabled = true AND next_run_at IS NOT NULL AND next_run_at <= $1`, now,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []*model.Schedule
	for rows.Next() {
		sc, err := scanScheduleRows(rows)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, sc)
	}
	return schedules, rows.Err()
}

// --- Schedule scan helpers ---

func scanSchedule(row scannable) (*model.Schedule, error) {
	var sc model.Schedule
	err := row.Scan(
		&sc.ID, &sc.UserID, &sc.Name, &sc.AgentID, &sc.Runtime, &sc.CronExpr,
		&sc.Timezone, &sc.Input, &sc.Enabled, &sc.LastRunAt, &sc.NextRunAt, &sc.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("schedule not found")
		}
		return nil, err
	}
	return &sc, nil
}

func scanScheduleRows(rows pgx.Rows) (*model.Schedule, error) {
	var sc model.Schedule
	err := rows.Scan(
		&sc.ID, &sc.UserID, &sc.Name, &sc.AgentID, &sc.Runtime, &sc.CronExpr,
		&sc.Timezone, &sc.Input, &sc.Enabled, &sc.LastRunAt, &sc.NextRunAt, &sc.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &sc, nil
}
