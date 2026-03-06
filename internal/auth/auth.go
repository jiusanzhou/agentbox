package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"go.zoe.im/agentbox/internal/model"
	"go.zoe.im/agentbox/internal/store"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserExists         = errors.New("user already exists")
	ErrInvalidToken       = errors.New("invalid token")
)

// Auth handles user authentication.
type Auth struct {
	store     store.Store
	jwtSecret []byte
}

// New creates an Auth instance.
func New(s store.Store, jwtSecret string) *Auth {
	secret := []byte(jwtSecret)
	if len(secret) == 0 {
		// Generate a random secret if not provided
		secret = make([]byte, 32)
		rand.Read(secret)
	}
	return &Auth{store: s, jwtSecret: secret}
}

// Register creates a new user.
func (a *Auth) Register(ctx context.Context, email, password, name string) (*model.User, error) {
	// Check if user exists
	if _, err := a.store.GetUserByEmail(ctx, email); err == nil {
		return nil, ErrUserExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	user := &model.User{
		ID:        generateID(),
		Email:     email,
		Name:      name,
		Password:  string(hash),
		Plan:      "free",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := a.store.CreateUser(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

// Login validates credentials and returns a JWT token.
func (a *Auth) Login(ctx context.Context, email, password string) (string, *model.User, error) {
	user, err := a.store.GetUserByEmail(ctx, email)
	if err != nil {
		return "", nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", nil, ErrInvalidCredentials
	}

	token, err := a.generateJWT(user)
	if err != nil {
		return "", nil, err
	}
	return token, user, nil
}

// ValidateToken parses a JWT and returns the user.
func (a *Auth) ValidateToken(ctx context.Context, tokenStr string) (*model.User, error) {
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return a.jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return nil, ErrInvalidToken
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, ErrInvalidToken
	}

	userID, _ := claims["sub"].(string)
	return a.store.GetUser(ctx, userID)
}

// ValidateAPIKey checks an API key (prefixed with "ak_") and returns the user.
func (a *Auth) ValidateAPIKey(ctx context.Context, apiKey string) (*model.User, error) {
	hash := hashAPIKey(apiKey)
	return a.store.GetUserByAPIKey(ctx, hash)
}

// GenerateAPIKey creates a new API key for a user. Returns the raw key (show once).
func (a *Auth) GenerateAPIKey(ctx context.Context, userID string) (string, error) {
	user, err := a.store.GetUser(ctx, userID)
	if err != nil {
		return "", err
	}

	raw := "ak_" + randomHex(32)
	user.APIKey = hashAPIKey(raw)
	user.UpdatedAt = time.Now()

	if err := a.store.UpdateUser(ctx, user); err != nil {
		return "", err
	}
	return raw, nil
}

func (a *Auth) generateJWT(user *model.User) (string, error) {
	claims := jwt.MapClaims{
		"sub":   user.ID,
		"email": user.Email,
		"name":  user.Name,
		"plan":  user.Plan,
		"exp":   time.Now().Add(7 * 24 * time.Hour).Unix(),
		"iat":   time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(a.jwtSecret)
}

func hashAPIKey(key string) string {
	h := sha256.Sum256([]byte(key))
	return hex.EncodeToString(h[:])
}

func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
