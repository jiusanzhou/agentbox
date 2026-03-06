package auth

import (
	"context"
	"net/http"
	"strings"

	"go.zoe.im/agentbox/internal/model"
)

type contextKey string

const UserContextKey contextKey = "user"

// UserFromContext extracts the authenticated user from context.
func UserFromContext(ctx context.Context) *model.User {
	if u, ok := ctx.Value(UserContextKey).(*model.User); ok {
		return u
	}
	return nil
}

// Middleware returns HTTP middleware that extracts user from:
// 1. Authorization: Bearer <jwt>
// 2. Authorization: Bearer ak_<api_key>
// 3. Cookie: abox_token
func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var user *model.User

		// Try Authorization header
		if auth := r.Header.Get("Authorization"); auth != "" {
			token := strings.TrimPrefix(auth, "Bearer ")
			if strings.HasPrefix(token, "ak_") {
				user, _ = a.ValidateAPIKey(r.Context(), token)
			} else {
				user, _ = a.ValidateToken(r.Context(), token)
			}
		}

		// Try cookie
		if user == nil {
			if cookie, err := r.Cookie("abox_token"); err == nil {
				user, _ = a.ValidateToken(r.Context(), cookie.Value)
			}
		}

		if user != nil {
			ctx := context.WithValue(r.Context(), UserContextKey, user)
			r = r.WithContext(ctx)
		}

		next.ServeHTTP(w, r)
	})
}

// RequireAuth wraps a handler to require authentication.
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if UserFromContext(r.Context()) == nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}
