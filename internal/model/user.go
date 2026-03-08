package model

import "time"

// User represents a registered user.
type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	Avatar    string    `json:"avatar,omitempty"`
	Password  string    `json:"-"`
	Plan      string    `json:"plan"`
	APIKey    string    `json:"-"`
	GitHubID  string    `json:"github_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
