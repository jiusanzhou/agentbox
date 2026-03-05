package s3

import (
	"go.zoe.im/agentbox/internal/storage"
	"go.zoe.im/x"
)

// Config for S3-compatible object storage.
type Config struct {
	Endpoint  string `json:"endpoint" yaml:"endpoint"`
	Bucket    string `json:"bucket" yaml:"bucket"`
	Region    string `json:"region" yaml:"region"`
	AccessKey string `json:"access_key" yaml:"access_key"`
	SecretKey string `json:"secret_key" yaml:"secret_key"`
}

func init() {
	storage.Register("s3", func(cfg x.TypedLazyConfig, opts ...any) (storage.Storage, error) {
		var c Config
		if err := cfg.Unmarshal(&c); err != nil {
			return nil, err
		}
		return New(c)
	})
}

// New creates an S3-backed storage.
func New(cfg Config) (storage.Storage, error) {
	// TODO: implement with aws-sdk-go-v2
	panic("s3 storage not implemented")
}
