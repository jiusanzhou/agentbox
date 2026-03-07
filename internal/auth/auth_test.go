package auth

import (
	"context"
	"strings"
	"testing"

	"go.zoe.im/agentbox/internal/store/memory"
)

func assert(t *testing.T, condition bool, msgs ...string) {
	t.Helper()
	if !condition {
		msg := "assertion failed"
		if len(msgs) > 0 {
			msg = msgs[0]
		}
		t.Fatal(msg)
	}
}

func TestAuth_RegisterAndLogin(t *testing.T) {
	s := memory.New()
	a := New(s, "test-secret-key")
	ctx := context.Background()

	// Register
	user, err := a.Register(ctx, "test@example.com", "password123", "Test User")
	assert(t, err == nil, "register should succeed")
	assert(t, user.Email == "test@example.com", "email should match")
	assert(t, user.Name == "Test User", "name should match")
	assert(t, user.ID != "", "ID should be set")
	assert(t, user.Plan == "free", "plan should default to free")

	// Login
	token, loggedIn, err := a.Login(ctx, "test@example.com", "password123")
	assert(t, err == nil, "login should succeed")
	assert(t, loggedIn.ID == user.ID, "user ID should match")
	assert(t, token != "", "token should not be empty")

	// Validate token
	validated, err := a.ValidateToken(ctx, token)
	assert(t, err == nil, "validate token should succeed")
	assert(t, validated.ID == user.ID, "validated user ID should match")

	// Wrong password
	_, _, err = a.Login(ctx, "test@example.com", "wrong")
	assert(t, err != nil, "wrong password should fail")
	assert(t, err == ErrInvalidCredentials, "error should be ErrInvalidCredentials")

	// Wrong email
	_, _, err = a.Login(ctx, "nobody@example.com", "password123")
	assert(t, err != nil, "wrong email should fail")
	assert(t, err == ErrInvalidCredentials, "error should be ErrInvalidCredentials")

	// Invalid token
	_, err = a.ValidateToken(ctx, "invalid-token")
	assert(t, err != nil, "invalid token should fail")

	// API Key
	key, err := a.GenerateAPIKey(ctx, user.ID)
	assert(t, err == nil, "generate API key should succeed")
	assert(t, strings.HasPrefix(key, "ak_"), "key should start with ak_")

	// Validate API key
	keyUser, err := a.ValidateAPIKey(ctx, key)
	assert(t, err == nil, "validate API key should succeed")
	assert(t, keyUser.ID == user.ID, "API key user ID should match")

	// Invalid API key
	_, err = a.ValidateAPIKey(ctx, "ak_invalid")
	assert(t, err != nil, "invalid API key should fail")
}

func TestAuth_DuplicateEmail(t *testing.T) {
	s := memory.New()
	a := New(s, "test-secret-key")
	ctx := context.Background()

	_, err := a.Register(ctx, "dup@example.com", "pass1", "User 1")
	assert(t, err == nil, "first register should succeed")

	_, err = a.Register(ctx, "dup@example.com", "pass2", "User 2")
	assert(t, err != nil, "duplicate email should fail")
	assert(t, err == ErrUserExists, "error should be ErrUserExists")
}

func TestAuth_GenerateAPIKey_UserNotFound(t *testing.T) {
	s := memory.New()
	a := New(s, "test-secret-key")
	ctx := context.Background()

	_, err := a.GenerateAPIKey(ctx, "nonexistent")
	assert(t, err != nil, "generate API key for nonexistent user should fail")
}

func TestAuth_EmptySecret(t *testing.T) {
	s := memory.New()
	a := New(s, "")
	ctx := context.Background()

	// Should still work with auto-generated secret
	user, err := a.Register(ctx, "test@example.com", "pass", "Test")
	assert(t, err == nil, "register with empty secret should succeed")

	token, _, err := a.Login(ctx, "test@example.com", "pass")
	assert(t, err == nil, "login should succeed")

	validated, err := a.ValidateToken(ctx, token)
	assert(t, err == nil, "validate should succeed")
	assert(t, validated.ID == user.ID, "user ID should match")
}
