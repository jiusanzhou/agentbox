package postgres

import (
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

// New creates a postgres-backed store.
func New(cfg Config) (store.Store, error) {
	// TODO: implement with database/sql or pgx
	panic("postgres store not implemented")
}
