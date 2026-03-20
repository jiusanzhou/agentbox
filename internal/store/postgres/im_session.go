package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"go.zoe.im/agentbox/internal/model"
)

// --- IM Session methods ---

func (s *pgStore) CreateIMSession(ctx context.Context, session *model.IMSession) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO im_sessions (id, binding_id, user_id, platform, platform_chat_id, session_id, agent_id, active, last_message_at, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		session.ID, session.BindingID, session.UserID, session.Platform, session.PlatformChatID,
		session.SessionID, session.AgentID, session.Active, session.LastMessageAt, session.CreatedAt,
	)
	return err
}

func (s *pgStore) GetIMSession(ctx context.Context, id string) (*model.IMSession, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, binding_id, user_id, platform, platform_chat_id, session_id, agent_id, active, last_message_at, created_at
		 FROM im_sessions WHERE id = $1`, id,
	)
	var sess model.IMSession
	err := row.Scan(&sess.ID, &sess.BindingID, &sess.UserID, &sess.Platform, &sess.PlatformChatID,
		&sess.SessionID, &sess.AgentID, &sess.Active, &sess.LastMessageAt, &sess.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("im session %s not found", id)
		}
		return nil, err
	}
	return &sess, nil
}

func (s *pgStore) GetIMSessionByChat(ctx context.Context, platform, chatID string) (*model.IMSession, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, binding_id, user_id, platform, platform_chat_id, session_id, agent_id, active, last_message_at, created_at
		 FROM im_sessions WHERE platform = $1 AND platform_chat_id = $2 AND active = TRUE`,
		platform, chatID,
	)
	var sess model.IMSession
	err := row.Scan(&sess.ID, &sess.BindingID, &sess.UserID, &sess.Platform, &sess.PlatformChatID,
		&sess.SessionID, &sess.AgentID, &sess.Active, &sess.LastMessageAt, &sess.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("im session not found for %s/%s", platform, chatID)
		}
		return nil, err
	}
	return &sess, nil
}

func (s *pgStore) ListIMSessionsByUser(ctx context.Context, userID string) ([]*model.IMSession, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, binding_id, user_id, platform, platform_chat_id, session_id, agent_id, active, last_message_at, created_at
		 FROM im_sessions WHERE user_id = $1 ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*model.IMSession
	for rows.Next() {
		var sess model.IMSession
		if err := rows.Scan(&sess.ID, &sess.BindingID, &sess.UserID, &sess.Platform, &sess.PlatformChatID,
			&sess.SessionID, &sess.AgentID, &sess.Active, &sess.LastMessageAt, &sess.CreatedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, &sess)
	}
	return sessions, rows.Err()
}

func (s *pgStore) UpdateIMSession(ctx context.Context, session *model.IMSession) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE im_sessions SET binding_id = $1, user_id = $2, platform = $3, platform_chat_id = $4,
		 session_id = $5, agent_id = $6, active = $7, last_message_at = $8 WHERE id = $9`,
		session.BindingID, session.UserID, session.Platform, session.PlatformChatID,
		session.SessionID, session.AgentID, session.Active, session.LastMessageAt, session.ID,
	)
	return err
}

func (s *pgStore) DeleteIMSession(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM im_sessions WHERE id = $1`, id)
	return err
}
