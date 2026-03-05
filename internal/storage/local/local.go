package local

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.zoe.im/agentbox/internal/storage"
	"go.zoe.im/x"
)

// Config for local filesystem storage.
type Config struct {
	Root string `json:"root" yaml:"root"`
}

func init() {
	storage.Register("local", func(cfg x.TypedLazyConfig, opts ...any) (storage.Storage, error) {
		var c Config
		if len(cfg.Config) > 0 {
			if err := cfg.Unmarshal(&c); err != nil {
				return nil, err
			}
		}
		if c.Root == "" {
			c.Root = "./data/artifacts"
		}
		return New(c)
	})
}

type localStorage struct {
	root string
}

func New(cfg Config) (storage.Storage, error) {
	if err := os.MkdirAll(cfg.Root, 0755); err != nil {
		return nil, err
	}
	return &localStorage{root: cfg.Root}, nil
}

func (s *localStorage) Upload(_ context.Context, key string, reader io.Reader) error {
	path := filepath.Join(s.root, key)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, reader)
	return err
}

func (s *localStorage) Download(_ context.Context, key string) (io.ReadCloser, error) {
	return os.Open(filepath.Join(s.root, key))
}

func (s *localStorage) PresignedURL(_ context.Context, key string) (string, error) {
	return fmt.Sprintf("file://%s", filepath.Join(s.root, key)), nil
}

func (s *localStorage) Delete(_ context.Context, key string) error {
	return os.Remove(filepath.Join(s.root, key))
}

func (s *localStorage) List(_ context.Context, prefix string) ([]string, error) {
	var keys []string
	root := filepath.Join(s.root, prefix)
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			rel, _ := filepath.Rel(s.root, path)
			keys = append(keys, strings.ReplaceAll(rel, string(filepath.Separator), "/"))
		}
		return nil
	})
	return keys, err
}
