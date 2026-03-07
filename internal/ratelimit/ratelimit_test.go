package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.zoe.im/agentbox/internal/config"
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

func TestRateLimiter_Allow(t *testing.T) {
	l := New(config.RateLimitConfig{RequestsPerMinute: 60, BurstSize: 3})

	// First 3 should pass (burst)
	assert(t, l.Allow("user1"), "1st request should pass")
	assert(t, l.Allow("user1"), "2nd request should pass")
	assert(t, l.Allow("user1"), "3rd request should pass")

	// 4th should fail (burst exhausted)
	assert(t, !l.Allow("user1"), "4th request should be denied")

	// Different user should pass
	assert(t, l.Allow("user2"), "different user should pass")
}

func TestRateLimiter_Refill(t *testing.T) {
	// 6000 requests/min = 100/sec. BurstSize 1 means after 1 request, must wait.
	l := New(config.RateLimitConfig{RequestsPerMinute: 6000, BurstSize: 1})

	assert(t, l.Allow("u1"), "first request should pass")
	assert(t, !l.Allow("u1"), "immediate second should fail")

	time.Sleep(20 * time.Millisecond) // 100/sec = 1 per 10ms, wait 20ms for safety
	assert(t, l.Allow("u1"), "should pass after refill")
}

func TestRateLimiter_DefaultValues(t *testing.T) {
	// Zero values should use defaults
	l := New(config.RateLimitConfig{RequestsPerMinute: 0, BurstSize: 0})
	assert(t, l.config.RequestsPerMinute == 60, "default RPM should be 60")
	assert(t, l.config.BurstSize == 10, "default burst should be 10")
}

func TestRateLimiter_Cleanup(t *testing.T) {
	l := New(config.RateLimitConfig{RequestsPerMinute: 60, BurstSize: 3})

	l.Allow("old-user")
	l.Allow("new-user")

	// Manually age the "old-user" bucket
	l.mu.Lock()
	l.buckets["old-user"].lastCheck = time.Now().Add(-10 * time.Minute)
	l.mu.Unlock()

	l.Cleanup(5 * time.Minute)

	l.mu.Lock()
	_, oldExists := l.buckets["old-user"]
	_, newExists := l.buckets["new-user"]
	l.mu.Unlock()

	assert(t, !oldExists, "old-user should be cleaned up")
	assert(t, newExists, "new-user should still exist")
}

func TestRateLimiter_UpdateConfig(t *testing.T) {
	l := New(config.RateLimitConfig{RequestsPerMinute: 60, BurstSize: 3})
	l.Allow("user1")

	l.UpdateConfig(config.RateLimitConfig{RequestsPerMinute: 120, BurstSize: 5})
	assert(t, l.config.RequestsPerMinute == 120, "RPM should be updated")
	assert(t, l.config.BurstSize == 5, "burst should be updated")

	// Buckets should be cleared
	l.mu.Lock()
	assert(t, len(l.buckets) == 0, "buckets should be cleared after config update")
	l.mu.Unlock()
}

func TestRateLimiter_Middleware(t *testing.T) {
	l := New(config.RateLimitConfig{RequestsPerMinute: 60, BurstSize: 1})

	handler := l.Middleware(func(r *http.Request) string {
		return "test-user"
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// First request should pass
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert(t, rr.Code == http.StatusOK, "first request should return 200")

	// Second request should be rate limited
	rr = httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	assert(t, rr.Code == http.StatusTooManyRequests, "second request should return 429")
}
