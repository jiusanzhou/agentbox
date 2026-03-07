package ratelimit

import (
	"net/http"
	"sync"
	"time"

	"go.zoe.im/agentbox/internal/config"
)

type bucket struct {
	tokens    float64
	lastCheck time.Time
	maxTokens float64
	rate      float64
}

// Limiter implements a per-key token bucket rate limiter.
type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	config  config.RateLimitConfig
}

// New creates a rate limiter from config.
func New(cfg config.RateLimitConfig) *Limiter {
	if cfg.RequestsPerMinute <= 0 {
		cfg.RequestsPerMinute = 60
	}
	if cfg.BurstSize <= 0 {
		cfg.BurstSize = 10
	}
	return &Limiter{
		buckets: make(map[string]*bucket),
		config:  cfg,
	}
}

// UpdateConfig applies new rate limit settings immediately.
func (l *Limiter) UpdateConfig(cfg config.RateLimitConfig) {
	if cfg.RequestsPerMinute <= 0 {
		cfg.RequestsPerMinute = 60
	}
	if cfg.BurstSize <= 0 {
		cfg.BurstSize = 10
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.config = cfg
	l.buckets = make(map[string]*bucket)
}

// Allow checks whether the given key is allowed to proceed.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	rate := float64(l.config.RequestsPerMinute) / 60.0

	b, ok := l.buckets[key]
	if !ok {
		l.buckets[key] = &bucket{
			tokens:    float64(l.config.BurstSize) - 1,
			lastCheck: now,
			rate:      rate,
			maxTokens: float64(l.config.BurstSize),
		}
		return true
	}

	elapsed := now.Sub(b.lastCheck).Seconds()
	b.tokens += elapsed * rate
	if b.tokens > b.maxTokens {
		b.tokens = b.maxTokens
	}
	b.lastCheck = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Middleware returns HTTP middleware that rate limits by user ID or IP.
func (l *Limiter) Middleware(getUserID func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := getUserID(r)
			if key == "" {
				key = r.RemoteAddr
			}

			if !l.Allow(key) {
				http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// Cleanup removes stale buckets older than maxAge.
func (l *Limiter) Cleanup(maxAge time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	for key, b := range l.buckets {
		if now.Sub(b.lastCheck) > maxAge {
			delete(l.buckets, key)
		}
	}
}
