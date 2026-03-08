package auth

import (
	"net/http"
	"sync"
	"time"
)

// AuthRateLimiter provides per-IP rate limiting for auth endpoints.
type AuthRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*authBucket
}

type authBucket struct {
	tokens    float64
	lastCheck time.Time
}

// NewAuthRateLimiter creates a rate limiter with auto-cleanup every 5 minutes.
func NewAuthRateLimiter() *AuthRateLimiter {
	rl := &AuthRateLimiter{
		buckets: make(map[string]*authBucket),
	}
	go rl.cleanupLoop()
	return rl
}

func (rl *AuthRateLimiter) cleanupLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for ip, b := range rl.buckets {
			if now.Sub(b.lastCheck) > 5*time.Minute {
				delete(rl.buckets, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// allow checks if the IP is allowed given the rate (max attempts per minute).
func (rl *AuthRateLimiter) allow(ip string, maxPerMinute float64) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	b, ok := rl.buckets[ip]
	if !ok {
		rl.buckets[ip] = &authBucket{
			tokens:    maxPerMinute - 1,
			lastCheck: now,
		}
		return true
	}

	elapsed := now.Sub(b.lastCheck).Seconds()
	b.tokens += elapsed * (maxPerMinute / 60.0)
	if b.tokens > maxPerMinute {
		b.tokens = maxPerMinute
	}
	b.lastCheck = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// LimitLogin wraps an HTTP handler with login rate limiting (5/min per IP).
func (rl *AuthRateLimiter) LimitLogin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		if !rl.allow("login:"+ip, 5) {
			http.Error(w, `{"error":"too many login attempts, try again later"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// LimitRegister wraps an HTTP handler with register rate limiting (3/min per IP).
func (rl *AuthRateLimiter) LimitRegister(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		if !rl.allow("register:"+ip, 3) {
			http.Error(w, `{"error":"too many registration attempts, try again later"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func extractIP(r *http.Request) string {
	// Check X-Forwarded-For first (for reverse proxies)
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP (client IP)
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	// Fall back to RemoteAddr (strip port)
	addr := r.RemoteAddr
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}
