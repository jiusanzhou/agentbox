package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"os"
	"path/filepath"

	"go.zoe.im/agentbox/internal/model"
	"go.zoe.im/agentbox/internal/store"
	"go.zoe.im/x"

	_ "github.com/mattn/go-sqlite3"
)

// Config for sqlite store.
type Config struct {
	Path string `json:"path" yaml:"path"`
}

func init() {
	store.Register("sqlite", func(cfg x.TypedLazyConfig, opts ...any) (store.Store, error) {
		var c Config
		if len(cfg.Config) > 0 {
			if err := cfg.Unmarshal(&c); err != nil {
				return nil, err
			}
		}
		if c.Path == "" {
			c.Path = "./data/agentbox.db"
		}
		return New(c)
	})
}

type sqliteStore struct {
	db *sql.DB
}

func New(cfg Config) (store.Store, error) {
	// Ensure parent directory exists
	if dir := filepath.Dir(cfg.Path); dir != "" {
		os.MkdirAll(dir, 0755)
	}

	db, err := sql.Open("sqlite3", cfg.Path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return &sqliteStore{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS runs (
			id         TEXT PRIMARY KEY,
			name       TEXT NOT NULL DEFAULT '',
			status     TEXT NOT NULL DEFAULT 'pending',
			agent_file TEXT NOT NULL DEFAULT '',
			config     TEXT NOT NULL DEFAULT '{}',
			result     TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			started_at DATETIME,
			ended_at   DATETIME
		);
		CREATE INDEX IF NOT EXISTS idx_runs_status ON runs(status);
		CREATE INDEX IF NOT EXISTS idx_runs_created ON runs(created_at DESC);
	`)
	return err
}

func (s *sqliteStore) CreateRun(ctx context.Context, run *model.Run) error {
	cfgJSON, _ := json.Marshal(run.Config)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO runs (id, name, status, agent_file, config, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		run.ID, run.Name, run.Status, run.AgentFile, string(cfgJSON), run.CreatedAt,
	)
	return err
}

func (s *sqliteStore) GetRun(ctx context.Context, id string) (*model.Run, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, status, agent_file, config, result, created_at, started_at, ended_at FROM runs WHERE id = ?`, id,
	)
	return scanRun(row)
}

func (s *sqliteStore) UpdateRun(ctx context.Context, run *model.Run) error {
	var resultJSON *string
	if run.Result != nil {
		b, _ := json.Marshal(run.Result)
		s := string(b)
		resultJSON = &s
	}
	_, err := s.db.ExecContext(ctx,
		`UPDATE runs SET status = ?, result = ?, started_at = ?, ended_at = ? WHERE id = ?`,
		run.Status, resultJSON, run.StartedAt, run.EndedAt, run.ID,
	)
	return err
}

func (s *sqliteStore) ListRuns(ctx context.Context, limit, offset int) ([]*model.Run, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, status, agent_file, config, result, created_at, started_at, ended_at
		 FROM runs ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var runs []*model.Run
	for rows.Next() {
		run, err := scanRunRows(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, run)
	}
	return runs, rows.Err()
}

func (s *sqliteStore) DeleteRun(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM runs WHERE id = ?`, id)
	return err
}

// Close implements x.Lifecycle
func (s *sqliteStore) Init(_ context.Context) error { return nil }
func (s *sqliteStore) Close(_ context.Context) error { return s.db.Close() }

// scanner helpers

type scannable interface {
	Scan(dest ...any) error
}

func scanRun(row scannable) (*model.Run, error) {
	var (
		run       model.Run
		cfgJSON   string
		resultJSON sql.NullString
		startedAt sql.NullTime
		endedAt   sql.NullTime
	)

	err := row.Scan(
		&run.ID, &run.Name, &run.Status, &run.AgentFile,
		&cfgJSON, &resultJSON, &run.CreatedAt, &startedAt, &endedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("run not found")
		}
		return nil, err
	}

	json.Unmarshal([]byte(cfgJSON), &run.Config)

	if resultJSON.Valid {
		run.Result = &model.Result{}
		json.Unmarshal([]byte(resultJSON.String), run.Result)
	}
	if startedAt.Valid {
		t := startedAt.Time
		run.StartedAt = &t
	}
	if endedAt.Valid {
		t := endedAt.Time
		run.EndedAt = &t
	}

	return &run, nil
}

func scanRunRows(rows *sql.Rows) (*model.Run, error) {
	var (
		run        model.Run
		cfgJSON    string
		resultJSON sql.NullString
		startedAt  sql.NullTime
		endedAt    sql.NullTime
	)

	err := rows.Scan(
		&run.ID, &run.Name, &run.Status, &run.AgentFile,
		&cfgJSON, &resultJSON, &run.CreatedAt, &startedAt, &endedAt,
	)
	if err != nil {
		return nil, err
	}

	json.Unmarshal([]byte(cfgJSON), &run.Config)

	if resultJSON.Valid {
		run.Result = &model.Result{}
		json.Unmarshal([]byte(resultJSON.String), run.Result)
	}
	if startedAt.Valid {
		t := startedAt.Time
		run.StartedAt = &t
	}
	if endedAt.Valid {
		t := endedAt.Time
		run.EndedAt = &t
	}

	return &run, nil
}

// Healthy implements x.HealthChecker
func (s *sqliteStore) Healthy(_ context.Context) error {
	return s.db.Ping()
}

// unused but ensures compile-time check
var _ store.Store = (*sqliteStore)(nil)
var _ interface{ Close(context.Context) error } = (*sqliteStore)(nil)
