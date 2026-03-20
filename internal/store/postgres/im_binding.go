package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"go.zoe.im/agentbox/internal/model"
)

// --- IM Binding methods ---

func (s *pgStore) CreateIMBinding(ctx context.Context, binding *model.IMBinding) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO im_bindings (id, user_id, platform, platform_user_id, platform_username, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		binding.ID, binding.UserID, binding.Platform, binding.PlatformUserID, binding.PlatformUsername, binding.CreatedAt,
	)
	return err
}

func (s *pgStore) GetIMBindingByPlatform(ctx context.Context, platform, platformUserID string) (*model.IMBinding, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, user_id, platform, platform_user_id, platform_username, created_at
		 FROM im_bindings WHERE platform = $1 AND platform_user_id = $2`,
		platform, platformUserID,
	)
	var b model.IMBinding
	err := row.Scan(&b.ID, &b.UserID, &b.Platform, &b.PlatformUserID, &b.PlatformUsername, &b.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("im binding not found")
		}
		return nil, err
	}
	return &b, nil
}

func (s *pgStore) ListIMBindingsByUser(ctx context.Context, userID string) ([]*model.IMBinding, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, platform, platform_user_id, platform_username, created_at
		 FROM im_bindings WHERE user_id = $1 ORDER BY created_at DESC`, userID,
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

func (s *pgStore) DeleteIMBinding(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM im_bindings WHERE id = $1`, id)
	return err
}

func (s *pgStore) CreateBindingCode(ctx context.Context, code *model.BindingCode) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO binding_codes (code, user_id, expires_at) VALUES ($1, $2, $3)`,
		code.Code, code.UserID, code.ExpiresAt,
	)
	return err
}

func (s *pgStore) GetBindingCode(ctx context.Context, code string) (*model.BindingCode, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT code, user_id, expires_at FROM binding_codes WHERE code = $1 AND expires_at > NOW()`, code,
	)
	var bc model.BindingCode
	err := row.Scan(&bc.Code, &bc.UserID, &bc.ExpiresAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("binding code not found or expired")
		}
		return nil, err
	}
	return &bc, nil
}

func (s *pgStore) DeleteBindingCode(ctx context.Context, code string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM binding_codes WHERE code = $1`, code)
	return err
}
