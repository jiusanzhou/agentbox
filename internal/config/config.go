package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"os"

	"go.zoe.im/agentbox/pkg/plugin"
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

	// Tunnel proxy address for sandbox containers
	TunnelProxyAddr string `json:"tunnel_proxy_addr,omitempty" yaml:"tunnel_proxy_addr" opts:"env=ABOX_TUNNEL_PROXY_ADDR"`

	// Session TTL: auto-cleanup idle sessions (e.g. "30m", "1h")
	SessionTTL      string `json:"session_ttl,omitempty" yaml:"session_ttl"`
	CleanupInterval string `json:"cleanup_interval,omitempty" yaml:"cleanup_interval"`

	// Rate limiting
	RateLimit RateLimitConfig `json:"rate_limit,omitempty" yaml:"rate_limit"`

	// CORS settings
	CORS CORSConfig `json:"cors,omitempty" yaml:"cors"`

	// Webhook secret for verifying git push hooks
	WebhookSecret string `json:"webhook_secret,omitempty" yaml:"webhook_secret" opts:"env=ABOX_WEBHOOK_SECRET"`

	// Stripe configuration for marketplace billing
	Stripe StripeConfig `json:"stripe,omitempty" yaml:"stripe"`

	// Plugins to load at startup
	Plugins []plugin.PluginConfig `json:"plugins,omitempty" yaml:"plugins"`
}

// StripeConfig holds Stripe API credentials and settings.
type StripeConfig struct {
	SecretKey      string `json:"secret_key,omitempty" yaml:"secret_key" opts:"env=STRIPE_SECRET_KEY"`
	WebhookSecret  string `json:"webhook_secret,omitempty" yaml:"webhook_secret" opts:"env=STRIPE_WEBHOOK_SECRET"`
	PublishableKey string `json:"publishable_key,omitempty" yaml:"publishable_key" opts:"env=STRIPE_PUBLISHABLE_KEY"`
	SuccessURL     string `json:"success_url,omitempty" yaml:"success_url"`
	CancelURL      string `json:"cancel_url,omitempty" yaml:"cancel_url"`
	// FreeRunsPerMonth is the number of free runs granted to each user per month (platform-wide).
	FreeRunsPerMonth int64 `json:"free_runs_per_month,omitempty" yaml:"free_runs_per_month"`
}

// RateLimitConfig holds rate limiting settings.
type RateLimitConfig struct {
	RequestsPerMinute int `json:"requests_per_minute,omitempty" yaml:"requests_per_minute"`
	BurstSize         int `json:"burst_size,omitempty" yaml:"burst_size"`
}

func (c *Config) String() string {
	data, _ := json.Marshal(c)
	return string(data)
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	JWTSecret string `json:"jwt_secret,omitempty" yaml:"jwt_secret" opts:"env=ABOX_JWT_SECRET"`
	Enabled   bool   `json:"enabled" yaml:"enabled"`

	// GitHub OAuth
	GitHubClientID     string `json:"github_client_id,omitempty" yaml:"github_client_id" opts:"env=ABOX_GITHUB_CLIENT_ID"`
	GitHubClientSecret string `json:"github_client_secret,omitempty" yaml:"github_client_secret" opts:"env=ABOX_GITHUB_CLIENT_SECRET"`
	GitHubCallbackURL  string `json:"github_callback_url,omitempty" yaml:"github_callback_url" opts:"env=ABOX_GITHUB_CALLBACK_URL"`

	// Google OAuth
	GoogleClientID     string `json:"google_client_id,omitempty" yaml:"google_client_id" opts:"env=ABOX_GOOGLE_CLIENT_ID"`
	GoogleClientSecret string `json:"google_client_secret,omitempty" yaml:"google_client_secret" opts:"env=ABOX_GOOGLE_CLIENT_SECRET"`
	GoogleCallbackURL  string `json:"google_callback_url,omitempty" yaml:"google_callback_url" opts:"env=ABOX_GOOGLE_CALLBACK_URL"`
}

// CORSConfig holds CORS settings.
type CORSConfig struct {
	AllowedOrigins   []string `json:"allowed_origins,omitempty" yaml:"allowed_origins"`
	AllowCredentials bool     `json:"allow_credentials,omitempty" yaml:"allow_credentials"`
	AllowedMethods   []string `json:"allowed_methods,omitempty" yaml:"allowed_methods"`
	AllowedHeaders   []string `json:"allowed_headers,omitempty" yaml:"allowed_headers"`
	MaxAge           int      `json:"max_age,omitempty" yaml:"max_age"`
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

// ResolveJWTSecret ensures a JWT secret is set.
// Priority: config value → JWT_SECRET env var → random generated secret.
func (c *Config) ResolveJWTSecret() {
	if c.Auth.JWTSecret != "" && c.Auth.JWTSecret != "abox-dev-secret-2026" {
		return
	}
	if env := os.Getenv("JWT_SECRET"); env != "" {
		c.Auth.JWTSecret = env
		return
	}
	// Generate a random 32-byte secret
	b := make([]byte, 32)
	rand.Read(b)
	c.Auth.JWTSecret = hex.EncodeToString(b)
	slog.Warn("no JWT secret configured, generated random secret (set JWT_SECRET env var for production)")
}
