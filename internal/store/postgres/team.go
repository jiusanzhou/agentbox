package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"go.zoe.im/agentbox/internal/model"
)

// --- Team methods ---

func (s *pgStore) CreateTeam(ctx context.Context, team *model.Team) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO teams (id, name, owner_id, created_at) VALUES ($1, $2, $3, $4)`,
		team.ID, team.Name, team.OwnerID, team.CreatedAt,
	)
	return err
}

func (s *pgStore) GetTeam(ctx context.Context, id string) (*model.Team, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, name, owner_id, created_at FROM teams WHERE id = $1`, id,
	)
	return scanTeam(row)
}

func (s *pgStore) ListTeamsByUser(ctx context.Context, userID string) ([]*model.Team, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT t.id, t.name, t.owner_id, t.created_at
		 FROM teams t
		 JOIN team_members tm ON t.id = tm.team_id
		 WHERE tm.user_id = $1
		 ORDER BY t.created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []*model.Team
	for rows.Next() {
		t, err := scanTeamRows(rows)
		if err != nil {
			return nil, err
		}
		teams = append(teams, t)
	}
	return teams, rows.Err()
}

func (s *pgStore) UpdateTeam(ctx context.Context, team *model.Team) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE teams SET name = $1, owner_id = $2 WHERE id = $3`,
		team.Name, team.OwnerID, team.ID,
	)
	return err
}

func (s *pgStore) DeleteTeam(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM teams WHERE id = $1`, id)
	return err
}

// --- TeamMember methods ---

func (s *pgStore) AddTeamMember(ctx context.Context, member *model.TeamMember) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO team_members (team_id, user_id, role, joined_at) VALUES ($1, $2, $3, $4)`,
		member.TeamID, member.UserID, member.Role, member.JoinedAt,
	)
	return err
}

func (s *pgStore) RemoveTeamMember(ctx context.Context, teamID, userID string) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM team_members WHERE team_id = $1 AND user_id = $2`, teamID, userID,
	)
	return err
}

func (s *pgStore) ListTeamMembers(ctx context.Context, teamID string) ([]*model.TeamMember, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT team_id, user_id, role, joined_at FROM team_members WHERE team_id = $1 ORDER BY joined_at`, teamID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []*model.TeamMember
	for rows.Next() {
		m, err := scanTeamMemberRows(rows)
		if err != nil {
			return nil, err
		}
		members = append(members, m)
	}
	return members, rows.Err()
}

func (s *pgStore) GetTeamMember(ctx context.Context, teamID, userID string) (*model.TeamMember, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT team_id, user_id, role, joined_at FROM team_members WHERE team_id = $1 AND user_id = $2`,
		teamID, userID,
	)
	return scanTeamMember(row)
}

// --- Team scan helpers ---

func scanTeam(row scannable) (*model.Team, error) {
	var t model.Team
	err := row.Scan(&t.ID, &t.Name, &t.OwnerID, &t.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("team not found")
		}
		return nil, err
	}
	return &t, nil
}

func scanTeamRows(rows pgx.Rows) (*model.Team, error) {
	var t model.Team
	err := rows.Scan(&t.ID, &t.Name, &t.OwnerID, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func scanTeamMember(row scannable) (*model.TeamMember, error) {
	var m model.TeamMember
	err := row.Scan(&m.TeamID, &m.UserID, &m.Role, &m.JoinedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("team member not found")
		}
		return nil, err
	}
	return &m, nil
}

func scanTeamMemberRows(rows pgx.Rows) (*model.TeamMember, error) {
	var m model.TeamMember
	err := rows.Scan(&m.TeamID, &m.UserID, &m.Role, &m.JoinedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}
