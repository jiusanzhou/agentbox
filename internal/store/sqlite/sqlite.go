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
			user_id    TEXT DEFAULT '',
			mode       TEXT NOT NULL DEFAULT 'run',
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
		CREATE INDEX IF NOT EXISTS idx_runs_user_id ON runs(user_id);

		CREATE TABLE IF NOT EXISTS users (
			id         TEXT PRIMARY KEY,
			email      TEXT UNIQUE NOT NULL,
			name       TEXT NOT NULL DEFAULT '',
			avatar     TEXT DEFAULT '',
			password   TEXT NOT NULL DEFAULT '',
			plan       TEXT NOT NULL DEFAULT 'free',
			api_key    TEXT DEFAULT '',
			github_id  TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email);
		CREATE INDEX IF NOT EXISTS idx_users_api_key ON users(api_key);
	`)
	if err != nil {
		return err
	}

	// Add user_id column to existing runs table (ignore error if already exists).
	db.Exec("ALTER TABLE runs ADD COLUMN user_id TEXT DEFAULT ''")

	// Add github_id column to existing users table (ignore error if already exists).
	db.Exec("ALTER TABLE users ADD COLUMN github_id TEXT DEFAULT ''")

	// Integrations table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS integrations (
			id         TEXT PRIMARY KEY,
			user_id    TEXT NOT NULL,
			type       TEXT NOT NULL,
			name       TEXT NOT NULL DEFAULT '',
			config     TEXT NOT NULL DEFAULT '{}',
			session_id TEXT DEFAULT '',
			enabled    INTEGER NOT NULL DEFAULT 1,
			status     TEXT NOT NULL DEFAULT 'disconnected',
			error      TEXT DEFAULT '',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_integrations_user ON integrations(user_id);
	`)

	if err != nil {
		return err
	}

	// Agent DNA registry table
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS agent_dnas (
			id           TEXT PRIMARY KEY,
			user_id      TEXT NOT NULL,
			slug         TEXT UNIQUE NOT NULL,
			version      TEXT NOT NULL DEFAULT '0.1.0',
			identity     TEXT NOT NULL DEFAULT '{}',
			soul         TEXT,
			tools        TEXT,
			memory       TEXT,
			skills       TEXT,
			manifest     TEXT NOT NULL DEFAULT '{}',
			repo_url     TEXT DEFAULT '',
			repo_ref     TEXT DEFAULT '',
			status       TEXT NOT NULL DEFAULT 'draft',
			downloads    INTEGER NOT NULL DEFAULT 0,
			rating       REAL NOT NULL DEFAULT 0,
			created_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at   DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			published_at DATETIME
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_dnas_slug ON agent_dnas(slug);
		CREATE INDEX IF NOT EXISTS idx_agent_dnas_user ON agent_dnas(user_id);
		CREATE INDEX IF NOT EXISTS idx_agent_dnas_status ON agent_dnas(status);
	`)
	if err != nil {
		return err
	}

	// Billing tables
	_, err = db.Exec(billingMigration)
	if err != nil {
		return err
	}

	// IM Binding tables
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS im_bindings (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			platform TEXT NOT NULL,
			platform_user_id TEXT NOT NULL,
			platform_username TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_im_bindings_platform ON im_bindings(platform, platform_user_id);
		CREATE INDEX IF NOT EXISTS idx_im_bindings_user ON im_bindings(user_id);

		CREATE TABLE IF NOT EXISTS binding_codes (
			code TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			expires_at DATETIME NOT NULL
		);
	`)
	return err
}

func (s *sqliteStore) CreateRun(ctx context.Context, run *model.Run) error {
	cfgJSON, _ := json.Marshal(run.Config)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO runs (id, user_id, name, mode, status, agent_file, config, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		run.ID, run.UserID, run.Name, string(run.Mode), run.Status, run.AgentFile, string(cfgJSON), run.CreatedAt,
	)
	return err
}

func (s *sqliteStore) GetRun(ctx context.Context, id string) (*model.Run, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, mode, name, status, agent_file, config, result, created_at, started_at, ended_at FROM runs WHERE id = ?`, id,
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
		`UPDATE runs SET mode = ?, status = ?, result = ?, started_at = ?, ended_at = ? WHERE id = ?`,
		string(run.Mode), run.Status, resultJSON, run.StartedAt, run.EndedAt, run.ID,
	)
	return err
}

func (s *sqliteStore) ListRuns(ctx context.Context, limit, offset int) ([]*model.Run, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, mode, name, status, agent_file, config, result, created_at, started_at, ended_at
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
		&run.ID, &run.UserID, &run.Mode, &run.Name, &run.Status, &run.AgentFile,
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
		&run.ID, &run.UserID, &run.Mode, &run.Name, &run.Status, &run.AgentFile,
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

// --- User methods ---

func (s *sqliteStore) CreateUser(ctx context.Context, user *model.User) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (id, email, name, avatar, password, plan, api_key, github_id, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		user.ID, user.Email, user.Name, user.Avatar, user.Password,
		user.Plan, user.APIKey, user.GitHubID, user.CreatedAt, user.UpdatedAt,
	)
	return err
}

func (s *sqliteStore) GetUser(ctx context.Context, id string) (*model.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, email, name, avatar, password, plan, api_key, github_id, created_at, updated_at FROM users WHERE id = ?`, id,
	)
	return scanUser(row)
}

func (s *sqliteStore) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, email, name, avatar, password, plan, api_key, github_id, created_at, updated_at FROM users WHERE email = ?`, email,
	)
	return scanUser(row)
}

func (s *sqliteStore) GetUserByAPIKey(ctx context.Context, apiKeyHash string) (*model.User, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, email, name, avatar, password, plan, api_key, github_id, created_at, updated_at FROM users WHERE api_key = ?`, apiKeyHash,
	)
	return scanUser(row)
}

func (s *sqliteStore) UpdateUser(ctx context.Context, user *model.User) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE users SET email = ?, name = ?, avatar = ?, password = ?, plan = ?, api_key = ?, github_id = ?, updated_at = ? WHERE id = ?`,
		user.Email, user.Name, user.Avatar, user.Password, user.Plan, user.APIKey, user.GitHubID, user.UpdatedAt, user.ID,
	)
	return err
}

func scanUser(row scannable) (*model.User, error) {
	var user model.User
	err := row.Scan(
		&user.ID, &user.Email, &user.Name, &user.Avatar, &user.Password,
		&user.Plan, &user.APIKey, &user.GitHubID, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}
	return &user, nil
}

// unused but ensures compile-time check
var _ store.Store = (*sqliteStore)(nil)
var _ interface{ Close(context.Context) error } = (*sqliteStore)(nil)

// --- Integration methods ---

func (s *sqliteStore) CreateIntegration(ctx context.Context, i *model.Integration) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO integrations (id, user_id, type, name, config, session_id, enabled, status, error, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		i.ID, i.UserID, i.Type, i.Name, string(i.Config), i.SessionID,
		i.Enabled, i.Status, i.Error, i.CreatedAt, i.UpdatedAt,
	)
	return err
}

func (s *sqliteStore) GetIntegration(ctx context.Context, id string) (*model.Integration, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, user_id, type, name, config, session_id, enabled, status, error, created_at, updated_at
		 FROM integrations WHERE id = ?`, id,
	)
	return scanIntegration(row)
}

func (s *sqliteStore) ListIntegrations(ctx context.Context, userID string) ([]*model.Integration, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, type, name, config, session_id, enabled, status, error, created_at, updated_at
		 FROM integrations WHERE user_id = ? ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntegrationRows(rows)
}

func (s *sqliteStore) UpdateIntegration(ctx context.Context, i *model.Integration) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE integrations SET name = ?, config = ?, session_id = ?, enabled = ?, status = ?, error = ?, updated_at = ? WHERE id = ?`,
		i.Name, string(i.Config), i.SessionID, i.Enabled, i.Status, i.Error, i.UpdatedAt, i.ID,
	)
	return err
}

func (s *sqliteStore) DeleteIntegration(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM integrations WHERE id = ?`, id)
	return err
}

func (s *sqliteStore) ListAllEnabledIntegrations(ctx context.Context) ([]*model.Integration, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, type, name, config, session_id, enabled, status, error, created_at, updated_at
		 FROM integrations WHERE enabled = 1`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntegrationRows(rows)
}

func scanIntegration(row scannable) (*model.Integration, error) {
	var i model.Integration
	var config string
	err := row.Scan(&i.ID, &i.UserID, &i.Type, &i.Name, &config, &i.SessionID,
		&i.Enabled, &i.Status, &i.Error, &i.CreatedAt, &i.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("integration not found")
		}
		return nil, err
	}
	i.Config = json.RawMessage(config)
	return &i, nil
}

func scanIntegrationRows(rows *sql.Rows) ([]*model.Integration, error) {
	var result []*model.Integration
	for rows.Next() {
		var i model.Integration
		var config string
		err := rows.Scan(&i.ID, &i.UserID, &i.Type, &i.Name, &config, &i.SessionID,
			&i.Enabled, &i.Status, &i.Error, &i.CreatedAt, &i.UpdatedAt)
		if err != nil {
			return nil, err
		}
		i.Config = json.RawMessage(config)
		result = append(result, &i)
	}
	return result, rows.Err()
}

// --- AgentDNA methods ---

func (s *sqliteStore) CreateAgentDNA(ctx context.Context, agent *model.AgentDNA) error {
	identityJSON, _ := json.Marshal(agent.Identity)
	soulJSON := marshalNullable(agent.Soul)
	manifestJSON, _ := json.Marshal(agent.Manifest)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO agent_dnas (id, user_id, slug, version, identity, soul, tools, memory, skills, manifest,
			repo_url, repo_ref, status, downloads, rating, created_at, updated_at, published_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		agent.ID, agent.UserID, agent.Slug, agent.Version,
		string(identityJSON), soulJSON, nullableJSON(agent.Tools), nullableJSON(agent.Memory), nullableJSON(agent.Skills),
		string(manifestJSON), agent.RepoURL, agent.RepoRef,
		string(agent.Status), agent.Downloads, agent.Rating,
		agent.CreatedAt, agent.UpdatedAt, agent.PublishedAt,
	)
	return err
}

func (s *sqliteStore) GetAgentDNA(ctx context.Context, id string) (*model.AgentDNA, error) {
	row := s.db.QueryRowContext(ctx, agentDNASelectSQL+` WHERE id = ?`, id)
	return scanAgentDNA(row)
}

func (s *sqliteStore) GetAgentDNABySlug(ctx context.Context, slug string) (*model.AgentDNA, error) {
	row := s.db.QueryRowContext(ctx, agentDNASelectSQL+` WHERE slug = ?`, slug)
	return scanAgentDNA(row)
}

func (s *sqliteStore) UpdateAgentDNA(ctx context.Context, agent *model.AgentDNA) error {
	identityJSON, _ := json.Marshal(agent.Identity)
	soulJSON := marshalNullable(agent.Soul)
	manifestJSON, _ := json.Marshal(agent.Manifest)
	_, err := s.db.ExecContext(ctx,
		`UPDATE agent_dnas SET user_id = ?, slug = ?, version = ?,
			identity = ?, soul = ?, tools = ?, memory = ?, skills = ?, manifest = ?,
			repo_url = ?, repo_ref = ?, status = ?, downloads = ?, rating = ?,
			updated_at = ?, published_at = ?
		 WHERE id = ?`,
		agent.UserID, agent.Slug, agent.Version,
		string(identityJSON), soulJSON, nullableJSON(agent.Tools), nullableJSON(agent.Memory), nullableJSON(agent.Skills),
		string(manifestJSON), agent.RepoURL, agent.RepoRef,
		string(agent.Status), agent.Downloads, agent.Rating,
		agent.UpdatedAt, agent.PublishedAt, agent.ID,
	)
	return err
}

func (s *sqliteStore) DeleteAgentDNA(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM agent_dnas WHERE id = ?`, id)
	return err
}

func (s *sqliteStore) ListAgentDNAs(ctx context.Context, opts model.AgentDNAListOptions) ([]*model.AgentDNA, error) {
	query := agentDNASelectSQL + ` WHERE 1=1`
	var args []any

	if opts.Status != "" {
		query += ` AND status = ?`
		args = append(args, string(opts.Status))
	}
	if opts.Runtime != "" {
		query += ` AND json_extract(manifest, '$.runtime') = ?`
		args = append(args, opts.Runtime)
	}
	if opts.Tag != "" {
		query += ` AND EXISTS (SELECT 1 FROM json_each(json_extract(manifest, '$.tags')) WHERE value = ?)`
		args = append(args, opts.Tag)
	}
	if opts.Query != "" {
		query += ` AND (slug LIKE ? OR json_extract(identity, '$.name') LIKE ? OR json_extract(identity, '$.description') LIKE ?)`
		q := "%" + opts.Query + "%"
		args = append(args, q, q, q)
	}

	query += ` ORDER BY downloads DESC, created_at DESC`

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	query += ` LIMIT ? OFFSET ?`
	args = append(args, limit, opts.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []*model.AgentDNA
	for rows.Next() {
		a, err := scanAgentDNARows(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func (s *sqliteStore) IncrementAgentDNADownloads(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE agent_dnas SET downloads = downloads + 1, updated_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

// AgentDNA SQL and scan helpers

const agentDNASelectSQL = `SELECT id, user_id, slug, version, identity, soul, tools, memory, skills, manifest,
	repo_url, repo_ref, status, downloads, rating, created_at, updated_at, published_at
	FROM agent_dnas`

func scanAgentDNA(row scannable) (*model.AgentDNA, error) {
	var (
		a            model.AgentDNA
		identityJSON string
		soulJSON     sql.NullString
		toolsJSON    sql.NullString
		memoryJSON   sql.NullString
		skillsJSON   sql.NullString
		manifestJSON string
		publishedAt  sql.NullTime
	)
	err := row.Scan(
		&a.ID, &a.UserID, &a.Slug, &a.Version,
		&identityJSON, &soulJSON, &toolsJSON, &memoryJSON, &skillsJSON,
		&manifestJSON, &a.RepoURL, &a.RepoRef,
		&a.Status, &a.Downloads, &a.Rating,
		&a.CreatedAt, &a.UpdatedAt, &publishedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("agent not found")
		}
		return nil, err
	}

	a.Identity = &model.AgentIdentity{}
	json.Unmarshal([]byte(identityJSON), a.Identity)

	if soulJSON.Valid {
		a.Soul = &model.AgentSoul{}
		json.Unmarshal([]byte(soulJSON.String), a.Soul)
	}
	if toolsJSON.Valid {
		a.Tools = json.RawMessage(toolsJSON.String)
	}
	if memoryJSON.Valid {
		a.Memory = json.RawMessage(memoryJSON.String)
	}
	if skillsJSON.Valid {
		a.Skills = json.RawMessage(skillsJSON.String)
	}

	a.Manifest = &model.AgentManifest{}
	json.Unmarshal([]byte(manifestJSON), a.Manifest)

	if publishedAt.Valid {
		t := publishedAt.Time
		a.PublishedAt = &t
	}

	return &a, nil
}

func scanAgentDNARows(rows *sql.Rows) (*model.AgentDNA, error) {
	var (
		a            model.AgentDNA
		identityJSON string
		soulJSON     sql.NullString
		toolsJSON    sql.NullString
		memoryJSON   sql.NullString
		skillsJSON   sql.NullString
		manifestJSON string
		publishedAt  sql.NullTime
	)
	err := rows.Scan(
		&a.ID, &a.UserID, &a.Slug, &a.Version,
		&identityJSON, &soulJSON, &toolsJSON, &memoryJSON, &skillsJSON,
		&manifestJSON, &a.RepoURL, &a.RepoRef,
		&a.Status, &a.Downloads, &a.Rating,
		&a.CreatedAt, &a.UpdatedAt, &publishedAt,
	)
	if err != nil {
		return nil, err
	}

	a.Identity = &model.AgentIdentity{}
	json.Unmarshal([]byte(identityJSON), a.Identity)

	if soulJSON.Valid {
		a.Soul = &model.AgentSoul{}
		json.Unmarshal([]byte(soulJSON.String), a.Soul)
	}
	if toolsJSON.Valid {
		a.Tools = json.RawMessage(toolsJSON.String)
	}
	if memoryJSON.Valid {
		a.Memory = json.RawMessage(memoryJSON.String)
	}
	if skillsJSON.Valid {
		a.Skills = json.RawMessage(skillsJSON.String)
	}

	a.Manifest = &model.AgentManifest{}
	json.Unmarshal([]byte(manifestJSON), a.Manifest)

	if publishedAt.Valid {
		t := publishedAt.Time
		a.PublishedAt = &t
	}

	return &a, nil
}

// marshalNullable returns a *string (JSON) or nil for nullable JSON columns.
func marshalNullable(v any) *string {
	if v == nil {
		return nil
	}
	b, _ := json.Marshal(v)
	s := string(b)
	return &s
}

// nullableJSON returns a *string or nil for json.RawMessage fields.
func nullableJSON(raw json.RawMessage) *string {
	if len(raw) == 0 {
		return nil
	}
	s := string(raw)
	return &s
}
