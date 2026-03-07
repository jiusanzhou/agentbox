package config

import (
	"encoding/json"
	"os"
	"strings"

	"go.zoe.im/x"
	"gopkg.in/yaml.v3"
)

// LoadConfig reads a YAML config file into a Config struct.
func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	cfg := NewConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// SaveConfig writes config to a YAML file.
func SaveConfig(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// SanitizedConfig is a JSON-friendly view of Config with secrets masked.
type SanitizedConfig struct {
	Debug           bool                  `json:"debug"`
	Addr            string                `json:"addr"`
	SessionTTL      string                `json:"session_ttl"`
	CleanupInterval string                `json:"cleanup_interval"`
	RateLimit       RateLimitConfig       `json:"rate_limit"`
	Auth            SanitizedAuth         `json:"auth"`
	Channels        []SanitizedChannel    `json:"channels"`
	Store           x.TypedLazyConfig     `json:"store"`
	Storage         x.TypedLazyConfig     `json:"storage"`
	Executor        x.TypedLazyConfig     `json:"executor"`
}

// SanitizedAuth shows auth config with secrets masked.
type SanitizedAuth struct {
	Enabled   bool   `json:"enabled"`
	JWTSecret string `json:"jwt_secret"`
}

// SanitizedChannel is a channel config with secrets masked.
type SanitizedChannel struct {
	Type   string         `json:"type"`
	Name   string         `json:"name,omitempty"`
	Config map[string]any `json:"config"`
}

// secretKeys are config keys whose values should be masked.
var secretKeys = map[string]bool{
	"token":     true,
	"bot_token": true,
	"secret":    true,
	"app_secret": true,
	"signing_secret": true,
}

// maskString masks a string, showing first 10 chars + "..."
func maskString(s string) string {
	if len(s) <= 10 {
		return strings.Repeat("*", len(s))
	}
	return s[:10] + "..."
}

// Sanitize returns a sanitized copy of the config for API responses.
func Sanitize(cfg *Config) *SanitizedConfig {
	sc := &SanitizedConfig{
		Debug:           cfg.Debug,
		Addr:            cfg.Addr,
		SessionTTL:      cfg.SessionTTL,
		CleanupInterval: cfg.CleanupInterval,
		RateLimit:       cfg.RateLimit,
		Auth: SanitizedAuth{
			Enabled:   cfg.Auth.Enabled,
			JWTSecret: maskString(cfg.Auth.JWTSecret),
		},
		Store:    cfg.Store,
		Storage:  cfg.Storage,
		Executor: cfg.Executor,
	}

	for _, ch := range cfg.Channels {
		sanitized := SanitizedChannel{
			Type: ch.Type,
			Name: ch.Name,
		}

		// Parse channel config and mask secrets
		var raw map[string]any
		if len(ch.Config) > 0 {
			if err := json.Unmarshal(ch.Config, &raw); err == nil {
				for k, v := range raw {
					if secretKeys[k] {
						if s, ok := v.(string); ok {
							raw[k] = maskString(s)
						}
					}
				}
			}
		}
		if raw == nil {
			raw = map[string]any{}
		}
		sanitized.Config = raw
		sc.Channels = append(sc.Channels, sanitized)
	}

	return sc
}
