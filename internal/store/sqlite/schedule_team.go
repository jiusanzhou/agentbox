package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.zoe.im/agentbox/internal/model"
)

// --- Schedule methods ---

func (s *sqliteStore) CreateSchedule(ctx context.Context, sched *model.Schedule) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO schedules (id, user_id, name, agent_id, runtime, cron_expr, timezone, input, enabled, last_run_at, next_run_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		sched.ID, sched.UserID, sched.Name, sched.AgentID, sched.Runtime,
		sched.CronExpr, sched.Timezone, sched.Input, sched.Enabled,
		sched.LastRunAt, sched.NextRunAt, sched.CreatedAt,
	)
	return err
}

func (s *sqliteStore) GetSchedule(ctx context.Context, id string) (*model.Schedule, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, name, agent_id, runtime, cron_expr, timezone, input, enabled, last_run_at, next_run_at, created_at
		 FROM schedules WHERE id = ?`, id,
	)
	return scanSchedule(row)
}

func (s *sqliteStore) ListSchedules(ctx context.Context, userID string) ([]*model.Schedule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, name, agent_id, runtime, cron_expr, timezone, input, enabled, last_run_at, next_run_at, created_at
		 FROM schedules WHERE user_id = ? ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []*model.Schedule
	for rows.Next() {
		sched, err := scanScheduleRow(rows)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, sched)
	}
	return schedules, rows.Err()
}

func (s *sqliteStore) UpdateSchedule(ctx context.Context, sched *model.Schedule) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE schedules SET name = ?, agent_id = ?, runtime = ?, cron_expr = ?, timezone = ?, input = ?, enabled = ?, last_run_at = ?, next_run_at = ? WHERE id = ?`,
		sched.Name, sched.AgentID, sched.Runtime, sched.CronExpr, sched.Timezone,
		sched.Input, sched.Enabled, sched.LastRunAt, sched.NextRunAt, sched.ID,
	)
	return err
}

func (s *sqliteStore) DeleteSchedule(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM schedules WHERE id = ?`, id)
	return err
}

func (s *sqliteStore) ListDueSchedules(ctx context.Context, now time.Time) ([]*model.Schedule, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, name, agent_id, runtime, cron_expr, timezone, input, enabled, last_run_at, next_run_at, created_at
		 FROM schedules WHERE enabled = 1 AND next_run_at IS NOT NULL AND next_run_at <= ?`, now,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []*model.Schedule
	for rows.Next() {
		sched, err := scanScheduleRow(rows)
		if err != nil {
			return nil, err
		}
		schedules = append(schedules, sched)
	}
	return schedules, rows.Err()
}

func scanSchedule(row scannable) (*model.Schedule, error) {
	var (
		sched     model.Schedule
		lastRunAt sql.NullTime
		nextRunAt sql.NullTime
	)
	err := row.Scan(
		&sched.ID, &sched.UserID, &sched.Name, &sched.AgentID, &sched.Runtime,
		&sched.CronExpr, &sched.Timezone, &sched.Input, &sched.Enabled,
		&lastRunAt, &nextRunAt, &sched.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("schedule not found")
		}
		return nil, err
	}
	if lastRunAt.Valid {
		t := lastRunAt.Time
		sched.LastRunAt = &t
	}
	if nextRunAt.Valid {
		t := nextRunAt.Time
		sched.NextRunAt = &t
	}
	return &sched, nil
}

func scanScheduleRow(rows *sql.Rows) (*model.Schedule, error) {
	var (
		sched     model.Schedule
		lastRunAt sql.NullTime
		nextRunAt sql.NullTime
	)
	err := rows.Scan(
		&sched.ID, &sched.UserID, &sched.Name, &sched.AgentID, &sched.Runtime,
		&sched.CronExpr, &sched.Timezone, &sched.Input, &sched.Enabled,
		&lastRunAt, &nextRunAt, &sched.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if lastRunAt.Valid {
		t := lastRunAt.Time
		sched.LastRunAt = &t
	}
	if nextRunAt.Valid {
		t := nextRunAt.Time
		sched.NextRunAt = &t
	}
	return &sched, nil
}

// --- Team methods ---

func (s *sqliteStore) CreateTeam(ctx context.Context, team *model.Team) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO teams (id, name, owner_id, created_at) VALUES (?, ?, ?, ?)`,
		team.ID, team.Name, team.OwnerID, team.CreatedAt,
	)
	return err
}

func (s *sqliteStore) GetTeam(ctx context.Context, id string) (*model.Team, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, owner_id, created_at FROM teams WHERE id = ?`, id,
	)
	var team model.Team
	err := row.Scan(&team.ID, &team.Name, &team.OwnerID, &team.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("team not found")
		}
		return nil, err
	}
	return &team, nil
}

func (s *sqliteStore) ListTeamsByUser(ctx context.Context, userID string) ([]*model.Team, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, owner_id, created_at FROM teams
		 WHERE id IN (SELECT team_id FROM team_members WHERE user_id = ?)
		 ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []*model.Team
	for rows.Next() {
		var team model.Team
		if err := rows.Scan(&team.ID, &team.Name, &team.OwnerID, &team.CreatedAt); err != nil {
			return nil, err
		}
		teams = append(teams, &team)
	}
	return teams, rows.Err()
}

func (s *sqliteStore) UpdateTeam(ctx context.Context, team *model.Team) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE teams SET name = ? WHERE id = ?`,
		team.Name, team.ID,
	)
	return err
}

func (s *sqliteStore) DeleteTeam(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM teams WHERE id = ?`, id)
	return err
}

func (s *sqliteStore) AddTeamMember(ctx context.Context, member *model.TeamMember) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO team_members (team_id, user_id, role, joined_at) VALUES (?, ?, ?, ?)`,
		member.TeamID, member.UserID, member.Role, member.JoinedAt,
	)
	return err
}

func (s *sqliteStore) RemoveTeamMember(ctx context.Context, teamID, userID string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM team_members WHERE team_id = ? AND user_id = ?`, teamID, userID,
	)
	return err
}

func (s *sqliteStore) ListTeamMembers(ctx context.Context, teamID string) ([]*model.TeamMember, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT team_id, user_id, role, joined_at FROM team_members WHERE team_id = ? ORDER BY joined_at`, teamID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*model.TeamMember
	for rows.Next() {
		var m model.TeamMember
		if err := rows.Scan(&m.TeamID, &m.UserID, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		members = append(members, &m)
	}
	return members, rows.Err()
}

func (s *sqliteStore) GetTeamMember(ctx context.Context, teamID, userID string) (*model.TeamMember, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT team_id, user_id, role, joined_at FROM team_members WHERE team_id = ? AND user_id = ?`,
		teamID, userID,
	)
	var m model.TeamMember
	err := row.Scan(&m.TeamID, &m.UserID, &m.Role, &m.JoinedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("team member not found")
		}
		return nil, err
	}
	return &m, nil
}
