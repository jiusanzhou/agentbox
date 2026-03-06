package config

import (
	"encoding/json"

	"go.zoe.im/x"
)

var global = NewConfig()

// Global returns the global config singleton.
func Global() *Config { return global }

// Config is the root configuration for agentbox.
type Config struct {
	Debug bool   `json:"debug,omitempty" yaml:"debug" opts:"env"`
	Port  int32  `json:"port,omitempty" yaml:"port" opts:"env"`
	Addr  string `json:"addr,omitempty" yaml:"addr" opts:"env"`

	// Auth configuration
	Auth AuthConfig `json:"auth,omitempty" yaml:"auth"`

	// Store backend (memory, sqlite, postgres, etc.)
	Store x.TypedLazyConfig `json:"store,omitempty" yaml:"store" opts:"-"`

	// Storage backend for artifacts (local, s3, etc.)
	Storage x.TypedLazyConfig `json:"storage,omitempty" yaml:"storage" opts:"-"`

	// Executor backend (docker, kubernetes, etc.)
	Executor x.TypedLazyConfig `json:"executor,omitempty" yaml:"executor" opts:"-"`

	// Talk server transport config
	Server x.TypedLazyConfig `json:"server,omitempty" yaml:"server" opts:"-"`

	// Channel backends for IM integration (telegram, etc.)
	Channels []x.TypedLazyConfig `json:"channels,omitempty" yaml:"channels" opts:"-"`
}

func (c *Config) String() string {
	data, _ := json.Marshal(c)
	return string(data)
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	JWTSecret string `json:"jwt_secret,omitempty" yaml:"jwt_secret" opts:"env=ABOX_JWT_SECRET"`
	Enabled   bool   `json:"enabled" yaml:"enabled"`
}

func NewConfig() *Config {
	return &Config{
		Port: 8080,
		Addr: ":8080",
		Store: x.TypedLazyConfig{
			Type:   "sqlite",
			Config: json.RawMessage(`{"path":"./data/agentbox.db"}`),
		},
		Storage: x.TypedLazyConfig{
			Type:   "local",
			Config: json.RawMessage(`{"root":"./data/artifacts"}`),
		},
		Executor: x.TypedLazyConfig{
			Type:   "docker",
			Config: json.RawMessage(`{"image":"agentbox-sandbox:latest"}`),
		},
		Server: x.TypedLazyConfig{
			Type:   "http",
			Config: json.RawMessage(`{"addr":":8080"}`),
		},
	}
}
