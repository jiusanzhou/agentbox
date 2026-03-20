package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type requestIDKey struct{}

// RequestID is an HTTP middleware that ensures every request has a unique
// request ID. If the incoming request carries an X-Request-ID header, that
// value is reused; otherwise a new random hex string is generated. The ID
// is stored in the request context and set as the X-Request-ID response header.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-ID")
		if id == "" {
			id = generateID()
		}

		// Set response header.
		w.Header().Set("X-Request-ID", id)

		// Store in context and continue.
		ctx := context.WithValue(r.Context(), requestIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFromContext extracts the request ID from the context. Returns an
// empty string if no request ID is present.
func RequestIDFromContext(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDKey{}).(string); ok {
		return id
	}
	return ""
}

// generateID produces a 16-byte (32 hex character) random identifier.
func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
