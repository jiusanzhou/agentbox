package storage

import (
	"context"
	"io"

	"go.zoe.im/x"
	"go.zoe.im/x/factory"
)

var (
	storageFactory = factory.NewFactory[Storage, any]()

	// Create creates a Storage from config.
	Create = storageFactory.Create

	// Register registers a Storage implementation.
	Register = storageFactory.Register
)

// Storage provides artifact and volume persistence.
type Storage interface {
	Upload(ctx context.Context, key string, reader io.Reader) error
	Download(ctx context.Context, key string) (io.ReadCloser, error)
	PresignedURL(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, prefix string) ([]string, error)
}

// New creates a new Storage from a TypedLazyConfig.
func New(cfg x.TypedLazyConfig) (Storage, error) {
	return Create(cfg)
}
