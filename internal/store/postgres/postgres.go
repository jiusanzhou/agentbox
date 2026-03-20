package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"go.zoe.im/agentbox/internal/model"
	"go.zoe.im/agentbox/internal/store"
	"go.zoe.im/x"
)

// Config for postgres store.
type Config struct {
	DSN string `json:"dsn" yaml:"dsn"`
}

func init() {
	store.Register("postgres", func(cfg x.TypedLazyConfig, opts ...any) (store.Store, error) {
		var c Config
		if err := cfg.Unmarshal(&c); err != nil {
			return nil, err
		}
		return New(c)
	})
}

type pgStore struct {
	pool *pgxpool.Pool
}

// New creates a postgres-backed store.
func New(cfg Config) (store.Store, error) {
	pool, err := pgxpool.New(context.Background(), cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	if err := migrate(pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return &pgStore{pool: pool}, nil
}

func migrate(pool *pgxpool.Pool) error {
	ctx := context.Background()
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS runs (
			id         TEXT PRIMARY KEY,
			user_id    TEXT NOT NULL DEFAULT '',
			mode       TEXT NOT NULL DEFAULT 'run',
			name       TEXT NOT NULL DEFAULT '',
			status     TEXT NOT NULL DEFAULT 'pending',
			agent_file TEXT NOT NULL DEFAULT '',
			config     JSONB NOT NULL DEFAULT '{}',
			result     JSONB,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			started_at TIMESTAMPTZ,
			ended_at   TIMESTAMPTZ
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
			created_at TIMESTAMPTZ DEFAULT NOW(),
			updated_at TIMESTAMPTZ DEFAULT NOW()
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_users_email ON users(email);
		CREATE INDEX IF NOT EXISTS idx_users_api_key ON users(api_key);

		CREATE TABLE IF NOT EXISTS integrations (
			id         TEXT PRIMARY KEY,
			user_id    TEXT NOT NULL,
			type       TEXT NOT NULL,
			name       TEXT NOT NULL DEFAULT '',
			config     JSONB NOT NULL DEFAULT '{}',
			session_id TEXT DEFAULT '',
			enabled    BOOLEAN NOT NULL DEFAULT TRUE,
			status     TEXT NOT NULL DEFAULT 'disconnected',
			error      TEXT DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_integrations_user ON integrations(user_id);

		CREATE TABLE IF NOT EXISTS agent_dnas (
			id           TEXT PRIMARY KEY,
			user_id      TEXT NOT NULL,
			slug         TEXT UNIQUE NOT NULL,
			version      TEXT NOT NULL DEFAULT '0.1.0',
			identity     JSONB NOT NULL DEFAULT '{}',
			soul         JSONB,
			tools        JSONB,
			memory       JSONB,
			skills       JSONB,
			manifest     JSONB NOT NULL DEFAULT '{}',
			repo_url     TEXT DEFAULT '',
			repo_ref     TEXT DEFAULT '',
			status       TEXT NOT NULL DEFAULT 'draft',
			downloads    BIGINT NOT NULL DEFAULT 0,
			rating       DOUBLE PRECISION NOT NULL DEFAULT 0,
			created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			published_at TIMESTAMPTZ
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_agent_dnas_slug ON agent_dnas(slug);
		CREATE INDEX IF NOT EXISTS idx_agent_dnas_user ON agent_dnas(user_id);
		CREATE INDEX IF NOT EXISTS idx_agent_dnas_status ON agent_dnas(status);
	`)
	if err != nil {
		return err
	}

	// Billing tables
	_, err = pool.Exec(ctx, billingMigration)
	if err != nil {
		return err
	}

	// Workflow tables
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS workflows (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT DEFAULT '',
			steps JSONB NOT NULL DEFAULT '[]',
			status TEXT NOT NULL DEFAULT 'draft',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE INDEX IF NOT EXISTS idx_workflows_user ON workflows(user_id);

		CREATE TABLE IF NOT EXISTS workflow_runs (
			id TEXT PRIMARY KEY,
			workflow_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			steps JSONB NOT NULL DEFAULT '[]',
			started_at TIMESTAMPTZ,
			ended_at TIMESTAMPTZ
		);
		CREATE INDEX IF NOT EXISTS idx_workflow_runs_workflow ON workflow_runs(workflow_id);
	`)
	if err != nil {
		return err
	}

	// IM Binding tables
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS im_bindings (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			platform TEXT NOT NULL,
			platform_user_id TEXT NOT NULL,
			platform_username TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		);
		CREATE UNIQUE INDEX IF NOT EXISTS idx_im_bindings_platform ON im_bindings(platform, platform_user_id);
		CREATE INDEX IF NOT EXISTS idx_im_bindings_user ON im_bindings(user_id);

		CREATE TABLE IF NOT EXISTS binding_codes (
			code TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL
		);
	`)
	return err
}

// Init implements x.Lifecycle.
func (s *pgStore) Init(_ context.Context) error { return nil }

// Close implements x.Lifecycle.
func (s *pgStore) Close(_ context.Context) error {
	s.pool.Close()
	return nil
}

// Healthy implements x.HealthChecker.
func (s *pgStore) Healthy(_ context.Context) error {
	return s.pool.Ping(context.Background())
}

// --- Run methods ---

func (s *pgStore) CreateRun(ctx context.Context, run *model.Run) error {
	cfgJSON, _ := json.Marshal(run.Config)
	_, err := s.pool.Exec(ctx,
		`INSERT INTO runs (id, user_id, name, mode, status, agent_file, config, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		run.ID, run.UserID, run.Name, string(run.Mode), run.Status, run.AgentFile, cfgJSON, run.CreatedAt,
	)
	return err
}

func (s *pgStore) GetRun(ctx context.Context, id string) (*model.Run, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, user_id, mode, name, status, agent_file, config, result, created_at, started_at, ended_at FROM runs WHERE id = $1`, id,
	)
	return scanRun(row)
}

func (s *pgStore) UpdateRun(ctx context.Context, run *model.Run) error {
	var resultJSON []byte
	if run.Result != nil {
		resultJSON, _ = json.Marshal(run.Result)
	}
	_, err := s.pool.Exec(ctx,
		`UPDATE runs SET mode = $1, status = $2, result = $3, started_at = $4, ended_at = $5 WHERE id = $6`,
		string(run.Mode), run.Status, resultJSON, run.StartedAt, run.EndedAt, run.ID,
	)
	return err
}

func (s *pgStore) ListRuns(ctx context.Context, limit, offset int) ([]*model.Run, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, mode, name, status, agent_file, config, result, created_at, started_at, ended_at
		 FROM runs ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset,
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

func (s *pgStore) DeleteRun(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM runs WHERE id = $1`, id)
	return err
}

// --- User methods ---

func (s *pgStore) CreateUser(ctx context.Context, user *model.User) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO users (id, email, name, avatar, password, plan, api_key, github_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		user.ID, user.Email, user.Name, user.Avatar, user.Password,
		user.Plan, user.APIKey, user.GitHubID, user.CreatedAt, user.UpdatedAt,
	)
	return err
}

func (s *pgStore) GetUser(ctx context.Context, id string) (*model.User, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, email, name, avatar, password, plan, api_key, github_id, created_at, updated_at FROM users WHERE id = $1`, id,
	)
	return scanUser(row)
}

func (s *pgStore) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, email, name, avatar, password, plan, api_key, github_id, created_at, updated_at FROM users WHERE email = $1`, email,
	)
	return scanUser(row)
}

func (s *pgStore) GetUserByAPIKey(ctx context.Context, apiKeyHash string) (*model.User, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, email, name, avatar, password, plan, api_key, github_id, created_at, updated_at FROM users WHERE api_key = $1`, apiKeyHash,
	)
	return scanUser(row)
}

func (s *pgStore) UpdateUser(ctx context.Context, user *model.User) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE users SET email = $1, name = $2, avatar = $3, password = $4, plan = $5, api_key = $6, github_id = $7, updated_at = $8 WHERE id = $9`,
		user.Email, user.Name, user.Avatar, user.Password, user.Plan, user.APIKey, user.GitHubID, user.UpdatedAt, user.ID,
	)
	return err
}

// --- Integration methods ---

func (s *pgStore) CreateIntegration(ctx context.Context, i *model.Integration) error {
	_, err := s.pool.Exec(ctx,
		`INSERT INTO integrations (id, user_id, type, name, config, session_id, enabled, status, error, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
		i.ID, i.UserID, i.Type, i.Name, []byte(i.Config), i.SessionID,
		i.Enabled, i.Status, i.Error, i.CreatedAt, i.UpdatedAt,
	)
	return err
}

func (s *pgStore) GetIntegration(ctx context.Context, id string) (*model.Integration, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT id, user_id, type, name, config, session_id, enabled, status, error, created_at, updated_at
		 FROM integrations WHERE id = $1`, id,
	)
	return scanIntegration(row)
}

func (s *pgStore) ListIntegrations(ctx context.Context, userID string) ([]*model.Integration, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, type, name, config, session_id, enabled, status, error, created_at, updated_at
		 FROM integrations WHERE user_id = $1 ORDER BY created_at DESC`, userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntegrationRows(rows)
}

func (s *pgStore) UpdateIntegration(ctx context.Context, i *model.Integration) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE integrations SET name = $1, config = $2, session_id = $3, enabled = $4, status = $5, error = $6, updated_at = $7 WHERE id = $8`,
		i.Name, []byte(i.Config), i.SessionID, i.Enabled, i.Status, i.Error, i.UpdatedAt, i.ID,
	)
	return err
}

func (s *pgStore) DeleteIntegration(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM integrations WHERE id = $1`, id)
	return err
}

func (s *pgStore) ListAllEnabledIntegrations(ctx context.Context) ([]*model.Integration, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, user_id, type, name, config, session_id, enabled, status, error, created_at, updated_at
		 FROM integrations WHERE enabled = TRUE`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanIntegrationRows(rows)
}

// --- AgentDNA methods ---

func (s *pgStore) CreateAgentDNA(ctx context.Context, agent *model.AgentDNA) error {
	identityJSON, _ := json.Marshal(agent.Identity)
	soulJSON := marshalNullableBytes(agent.Soul)
	manifestJSON, _ := json.Marshal(agent.Manifest)
	_, err := s.pool.Exec(ctx,
		`INSERT INTO agent_dnas (id, user_id, slug, version, identity, soul, tools, memory, skills, manifest,
			repo_url, repo_ref, status, downloads, rating, created_at, updated_at, published_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)`,
		agent.ID, agent.UserID, agent.Slug, agent.Version,
		identityJSON, soulJSON, nullableJSONBytes(agent.Tools), nullableJSONBytes(agent.Memory), nullableJSONBytes(agent.Skills),
		manifestJSON, agent.RepoURL, agent.RepoRef,
		string(agent.Status), agent.Downloads, agent.Rating,
		agent.CreatedAt, agent.UpdatedAt, agent.PublishedAt,
	)
	return err
}

func (s *pgStore) GetAgentDNA(ctx context.Context, id string) (*model.AgentDNA, error) {
	row := s.pool.QueryRow(ctx, agentDNASelectSQL+` WHERE id = $1`, id)
	return scanAgentDNA(row)
}

func (s *pgStore) GetAgentDNABySlug(ctx context.Context, slug string) (*model.AgentDNA, error) {
	row := s.pool.QueryRow(ctx, agentDNASelectSQL+` WHERE slug = $1`, slug)
	return scanAgentDNA(row)
}

func (s *pgStore) UpdateAgentDNA(ctx context.Context, agent *model.AgentDNA) error {
	identityJSON, _ := json.Marshal(agent.Identity)
	soulJSON := marshalNullableBytes(agent.Soul)
	manifestJSON, _ := json.Marshal(agent.Manifest)
	_, err := s.pool.Exec(ctx,
		`UPDATE agent_dnas SET user_id = $1, slug = $2, version = $3,
			identity = $4, soul = $5, tools = $6, memory = $7, skills = $8, manifest = $9,
			repo_url = $10, repo_ref = $11, status = $12, downloads = $13, rating = $14,
			updated_at = $15, published_at = $16
		 WHERE id = $17`,
		agent.UserID, agent.Slug, agent.Version,
		identityJSON, soulJSON, nullableJSONBytes(agent.Tools), nullableJSONBytes(agent.Memory), nullableJSONBytes(agent.Skills),
		manifestJSON, agent.RepoURL, agent.RepoRef,
		string(agent.Status), agent.Downloads, agent.Rating,
		agent.UpdatedAt, agent.PublishedAt, agent.ID,
	)
	return err
}

func (s *pgStore) DeleteAgentDNA(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM agent_dnas WHERE id = $1`, id)
	return err
}

func (s *pgStore) ListAgentDNAs(ctx context.Context, opts model.AgentDNAListOptions) ([]*model.AgentDNA, error) {
	query := agentDNASelectSQL + ` WHERE 1=1`
	var args []any
	argN := 0

	nextArg := func() string {
		argN++
		return fmt.Sprintf("$%d", argN)
	}

	if opts.Status != "" {
		query += ` AND status = ` + nextArg()
		args = append(args, string(opts.Status))
	}
	if opts.Runtime != "" {
		query += ` AND manifest->>'runtime' = ` + nextArg()
		args = append(args, opts.Runtime)
	}
	if opts.Tag != "" {
		query += ` AND manifest->'tags' ? ` + nextArg()
		args = append(args, opts.Tag)
	}
	if opts.Query != "" {
		p := nextArg()
		query += ` AND (slug ILIKE ` + p + ` OR identity->>'name' ILIKE ` + p + ` OR identity->>'description' ILIKE ` + p + `)`
		args = append(args, "%"+opts.Query+"%")
	}

	query += ` ORDER BY downloads DESC, created_at DESC`

	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	query += ` LIMIT ` + nextArg()
	args = append(args, limit)
	query += ` OFFSET ` + nextArg()
	args = append(args, opts.Offset)

	rows, err := s.pool.Query(ctx, query, args...)
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

func (s *pgStore) IncrementAgentDNADownloads(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE agent_dnas SET downloads = downloads + 1, updated_at = NOW() WHERE id = $1`, id)
	return err
}

// --- Scan helpers ---

type scannable interface {
	Scan(dest ...any) error
}

func scanRun(row scannable) (*model.Run, error) {
	var (
		run        model.Run
		cfgJSON    []byte
		resultJSON []byte
	)

	err := row.Scan(
		&run.ID, &run.UserID, &run.Mode, &run.Name, &run.Status, &run.AgentFile,
		&cfgJSON, &resultJSON, &run.CreatedAt, &run.StartedAt, &run.EndedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("run not found")
		}
		return nil, err
	}

	json.Unmarshal(cfgJSON, &run.Config)

	if len(resultJSON) > 0 {
		run.Result = &model.Result{}
		json.Unmarshal(resultJSON, run.Result)
	}

	return &run, nil
}

func scanRunRows(rows pgx.Rows) (*model.Run, error) {
	var (
		run        model.Run
		cfgJSON    []byte
		resultJSON []byte
	)

	err := rows.Scan(
		&run.ID, &run.UserID, &run.Mode, &run.Name, &run.Status, &run.AgentFile,
		&cfgJSON, &resultJSON, &run.CreatedAt, &run.StartedAt, &run.EndedAt,
	)
	if err != nil {
		return nil, err
	}

	json.Unmarshal(cfgJSON, &run.Config)

	if len(resultJSON) > 0 {
		run.Result = &model.Result{}
		json.Unmarshal(resultJSON, run.Result)
	}

	return &run, nil
}

func scanUser(row scannable) (*model.User, error) {
	var user model.User
	err := row.Scan(
		&user.ID, &user.Email, &user.Name, &user.Avatar, &user.Password,
		&user.Plan, &user.APIKey, &user.GitHubID, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}
	return &user, nil
}

func scanIntegration(row scannable) (*model.Integration, error) {
	var i model.Integration
	var config []byte
	err := row.Scan(&i.ID, &i.UserID, &i.Type, &i.Name, &config, &i.SessionID,
		&i.Enabled, &i.Status, &i.Error, &i.CreatedAt, &i.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("integration not found")
		}
		return nil, err
	}
	i.Config = json.RawMessage(config)
	return &i, nil
}

func scanIntegrationRows(rows pgx.Rows) ([]*model.Integration, error) {
	var result []*model.Integration
	for rows.Next() {
		var i model.Integration
		var config []byte
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

const agentDNASelectSQL = `SELECT id, user_id, slug, version, identity, soul, tools, memory, skills, manifest,
	repo_url, repo_ref, status, downloads, rating, created_at, updated_at, published_at
	FROM agent_dnas`

func scanAgentDNA(row scannable) (*model.AgentDNA, error) {
	var (
		a            model.AgentDNA
		identityJSON []byte
		soulJSON     []byte
		toolsJSON    []byte
		memoryJSON   []byte
		skillsJSON   []byte
		manifestJSON []byte
	)
	err := row.Scan(
		&a.ID, &a.UserID, &a.Slug, &a.Version,
		&identityJSON, &soulJSON, &toolsJSON, &memoryJSON, &skillsJSON,
		&manifestJSON, &a.RepoURL, &a.RepoRef,
		&a.Status, &a.Downloads, &a.Rating,
		&a.CreatedAt, &a.UpdatedAt, &a.PublishedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("agent not found")
		}
		return nil, err
	}

	a.Identity = &model.AgentIdentity{}
	json.Unmarshal(identityJSON, a.Identity)

	if len(soulJSON) > 0 {
		a.Soul = &model.AgentSoul{}
		json.Unmarshal(soulJSON, a.Soul)
	}
	if len(toolsJSON) > 0 {
		a.Tools = json.RawMessage(toolsJSON)
	}
	if len(memoryJSON) > 0 {
		a.Memory = json.RawMessage(memoryJSON)
	}
	if len(skillsJSON) > 0 {
		a.Skills = json.RawMessage(skillsJSON)
	}

	a.Manifest = &model.AgentManifest{}
	json.Unmarshal(manifestJSON, a.Manifest)

	return &a, nil
}

func scanAgentDNARows(rows pgx.Rows) (*model.AgentDNA, error) {
	var (
		a            model.AgentDNA
		identityJSON []byte
		soulJSON     []byte
		toolsJSON    []byte
		memoryJSON   []byte
		skillsJSON   []byte
		manifestJSON []byte
	)
	err := rows.Scan(
		&a.ID, &a.UserID, &a.Slug, &a.Version,
		&identityJSON, &soulJSON, &toolsJSON, &memoryJSON, &skillsJSON,
		&manifestJSON, &a.RepoURL, &a.RepoRef,
		&a.Status, &a.Downloads, &a.Rating,
		&a.CreatedAt, &a.UpdatedAt, &a.PublishedAt,
	)
	if err != nil {
		return nil, err
	}

	a.Identity = &model.AgentIdentity{}
	json.Unmarshal(identityJSON, a.Identity)

	if len(soulJSON) > 0 {
		a.Soul = &model.AgentSoul{}
		json.Unmarshal(soulJSON, a.Soul)
	}
	if len(toolsJSON) > 0 {
		a.Tools = json.RawMessage(toolsJSON)
	}
	if len(memoryJSON) > 0 {
		a.Memory = json.RawMessage(memoryJSON)
	}
	if len(skillsJSON) > 0 {
		a.Skills = json.RawMessage(skillsJSON)
	}

	a.Manifest = &model.AgentManifest{}
	json.Unmarshal(manifestJSON, a.Manifest)

	return &a, nil
}

// marshalNullableBytes returns JSON bytes or nil for nullable JSON columns.
func marshalNullableBytes(v any) []byte {
	if v == nil {
		return nil
	}
	b, _ := json.Marshal(v)
	return b
}

// nullableJSONBytes returns bytes or nil for json.RawMessage fields.
func nullableJSONBytes(raw json.RawMessage) []byte {
	if len(raw) == 0 {
		return nil
	}
	return []byte(raw)
}

// Compile-time interface check.
var _ store.Store = (*pgStore)(nil)
var _ interface{ Close(context.Context) error } = (*pgStore)(nil)
