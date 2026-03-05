package store

import (
	"context"

	"go.zoe.im/agentbox/internal/model"
	"go.zoe.im/x"
	"go.zoe.im/x/factory"
)

var (
	storeFactory = factory.NewFactory[Store, any]()

	// Create creates a Store from config.
	Create = storeFactory.Create

	// Register registers a Store implementation.
	Register = storeFactory.Register
)

// Store defines the persistence interface for runs.
type Store interface {
	CreateRun(ctx context.Context, run *model.Run) error
	GetRun(ctx context.Context, id string) (*model.Run, error)
	UpdateRun(ctx context.Context, run *model.Run) error
	ListRuns(ctx context.Context, limit, offset int) ([]*model.Run, error)
	DeleteRun(ctx context.Context, id string) error
}

// New creates a new Store from a TypedLazyConfig.
func New(cfg x.TypedLazyConfig) (Store, error) {
	return Create(cfg)
}
