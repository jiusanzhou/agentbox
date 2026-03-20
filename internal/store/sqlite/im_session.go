package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"go.zoe.im/agentbox/internal/model"
)

// --- IM Session methods ---

func (s *sqliteStore) CreateIMSession(ctx context.Context, session *model.IMSession) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO im_sessions (id, binding_id, user_id, platform, platform_chat_id, session_id, agent_id, active, last_message_at, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.ID, session.BindingID, session.UserID, session.Platform, session.PlatformChatID,
		session.SessionID, session.AgentID, session.Active, session.LastMessageAt, session.CreatedAt,
	)
	return err
}

func (s *sqliteStore) GetIMSession(ctx context.Context, id string) (*model.IMSession, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, binding_id, user_id, platform, platform_chat_id, session_id, agent_id, active, last_message_at, created_at
		 FROM im_sessions WHERE id = ?`, id,
	)
	return scanIMSession(row)
}

func (s *sqliteStore) GetIMSessionByChat(ctx context.Context, platform, chatID string) (*model.IMSession, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, binding_id, user_id, platform, platform_chat_id, session_id, agent_id, active, last_message_at, created_at
		 FROM im_sessions WHERE platform = ? AND platform_chat_id = ? AND active = 1`,
		platform, chatID,
	)
	return scanIMSession(row)
}

func (s *sqliteStore) ListIMSessionsByUser(ctx context.Context, userID string) ([]*model.IMSession, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, binding_id, user_id, platform, platform_chat_id, session_id, agent_id, active, last_message_at, created_at
		 FROM im_sessions WHERE user_id = ? ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*model.IMSession
	for rows.Next() {
		var sess model.IMSession
		var lastMsg sql.NullTime
		if err := rows.Scan(&sess.ID, &sess.BindingID, &sess.UserID, &sess.Platform, &sess.PlatformChatID,
			&sess.SessionID, &sess.AgentID, &sess.Active, &lastMsg, &sess.CreatedAt); err != nil {
			return nil, err
		}
		if lastMsg.Valid {
			t := lastMsg.Time
			sess.LastMessageAt = &t
		}
		sessions = append(sessions, &sess)
	}
	return sessions, rows.Err()
}

func (s *sqliteStore) UpdateIMSession(ctx context.Context, session *model.IMSession) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE im_sessions SET binding_id = ?, user_id = ?, platform = ?, platform_chat_id = ?,
		 session_id = ?, agent_id = ?, active = ?, last_message_at = ? WHERE id = ?`,
		session.BindingID, session.UserID, session.Platform, session.PlatformChatID,
		session.SessionID, session.AgentID, session.Active, session.LastMessageAt, session.ID,
	)
	return err
}

func (s *sqliteStore) DeleteIMSession(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM im_sessions WHERE id = ?`, id)
	return err
}

func scanIMSession(row scannable) (*model.IMSession, error) {
	var sess model.IMSession
	var lastMsg sql.NullTime
	err := row.Scan(&sess.ID, &sess.BindingID, &sess.UserID, &sess.Platform, &sess.PlatformChatID,
		&sess.SessionID, &sess.AgentID, &sess.Active, &lastMsg, &sess.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("im session not found")
		}
		return nil, err
	}
	if lastMsg.Valid {
		t := lastMsg.Time
		sess.LastMessageAt = &t
	}
	return &sess, nil
}
