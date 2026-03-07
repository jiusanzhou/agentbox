package model

import (
	"encoding/json"
	"time"
)

// Integration represents a per-user IM channel binding.
type Integration struct {
	ID        string          `json:"id"`
	UserID    string          `json:"user_id"`
	Type      string          `json:"type"`                // "telegram", "discord", "slack", "wecom", "webhook"
	Name      string          `json:"name"`                // user-friendly name
	Config    json.RawMessage `json:"config"`              // type-specific config (token, etc.)
	SessionID string          `json:"session_id,omitempty"` // which agent session to route to (empty = auto-create)
	Enabled   bool            `json:"enabled"`
	Status    string          `json:"status"`              // "connected", "disconnected", "error"
	Error     string          `json:"error,omitempty"`
	CreatedAt time.Time       `json:"created_at"`
	UpdatedAt time.Time       `json:"updated_at"`
}
