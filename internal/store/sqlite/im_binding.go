package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"go.zoe.im/agentbox/internal/model"
)

// --- IM Binding methods ---

func (s *sqliteStore) CreateIMBinding(ctx context.Context, binding *model.IMBinding) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO im_bindings (id, user_id, platform, platform_user_id, platform_username, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		binding.ID, binding.UserID, binding.Platform, binding.PlatformUserID, binding.PlatformUsername, binding.CreatedAt,
	)
	return err
}

func (s *sqliteStore) GetIMBindingByPlatform(ctx context.Context, platform, platformUserID string) (*model.IMBinding, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, platform, platform_user_id, platform_username, created_at
		 FROM im_bindings WHERE platform = ? AND platform_user_id = ?`,
		platform, platformUserID,
	)
	var b model.IMBinding
	err := row.Scan(&b.ID, &b.UserID, &b.Platform, &b.PlatformUserID, &b.PlatformUsername, &b.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("im binding not found")
		}
		return nil, err
	}
	return &b, nil
}

func (s *sqliteStore) ListIMBindingsByUser(ctx context.Context, userID string) ([]*model.IMBinding, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, platform, platform_user_id, platform_username, created_at
		 FROM im_bindings WHERE user_id = ? ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bindings []*model.IMBinding
	for rows.Next() {
		var b model.IMBinding
		if err := rows.Scan(&b.ID, &b.UserID, &b.Platform, &b.PlatformUserID, &b.PlatformUsername, &b.CreatedAt); err != nil {
			return nil, err
		}
		bindings = append(bindings, &b)
	}
	return bindings, rows.Err()
}

func (s *sqliteStore) DeleteIMBinding(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM im_bindings WHERE id = ?`, id)
	return err
}

func (s *sqliteStore) CreateBindingCode(ctx context.Context, code *model.BindingCode) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO binding_codes (code, user_id, expires_at) VALUES (?, ?, ?)`,
		code.Code, code.UserID, code.ExpiresAt,
	)
	return err
}

func (s *sqliteStore) GetBindingCode(ctx context.Context, code string) (*model.BindingCode, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT code, user_id, expires_at FROM binding_codes WHERE code = ? AND expires_at > CURRENT_TIMESTAMP`, code,
	)
	var bc model.BindingCode
	err := row.Scan(&bc.Code, &bc.UserID, &bc.ExpiresAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("binding code not found or expired")
		}
		return nil, err
	}
	return &bc, nil
}

func (s *sqliteStore) DeleteBindingCode(ctx context.Context, code string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM binding_codes WHERE code = ?`, code)
	return err
}
