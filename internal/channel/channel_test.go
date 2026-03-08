package channel

import (
	"sync"
	"testing"
	"time"
)

func assert(t *testing.T, cond bool, msgs ...string) {
	t.Helper()
	if !cond {
		msg := "assertion failed"
		if len(msgs) > 0 {
			msg = msgs[0]
		}
		t.Fatal(msg)
	}
}

func TestPermissionGateway_AllowFlow(t *testing.T) {
	pg := NewPermissionGateway()

	pg.Register("req-1", "bash", "chat-1")

	// Resolve in background
	go func() {
		time.Sleep(10 * time.Millisecond)
		err := pg.Resolve("req-1", true)
		assert(t, err == nil, "resolve should not error")
	}()

	result := pg.WaitFor("req-1")
	assert(t, result, "should be allowed")
}

func TestPermissionGateway_DenyFlow(t *testing.T) {
	pg := NewPermissionGateway()

	pg.Register("req-2", "bash", "chat-1")

	go func() {
		time.Sleep(10 * time.Millisecond)
		pg.Resolve("req-2", false)
	}()

	result := pg.WaitFor("req-2")
	assert(t, !result, "should be denied")
}

func TestPermissionGateway_WaitForUnknown(t *testing.T) {
	pg := NewPermissionGateway()

	// WaitFor on a non-existent request should return false immediately
	result := pg.WaitFor("nonexistent")
	assert(t, !result, "unknown request should return false")
}

func TestPermissionGateway_ResolveUnknown(t *testing.T) {
	pg := NewPermissionGateway()

	err := pg.Resolve("nonexistent", true)
	assert(t, err != nil, "resolve unknown should error")
}

func TestPermissionGateway_DenyAll(t *testing.T) {
	pg := NewPermissionGateway()

	pg.Register("r1", "tool1", "chat-1")
	pg.Register("r2", "tool2", "chat-2")
	pg.Register("r3", "tool3", "chat-3")

	var results [3]bool
	var wg sync.WaitGroup

	// Start waiting on all 3
	for i, id := range []string{"r1", "r2", "r3"} {
		wg.Add(1)
		go func(idx int, reqID string) {
			defer wg.Done()
			results[idx] = pg.WaitFor(reqID)
		}(i, id)
	}

	// Give goroutines time to start waiting
	time.Sleep(20 * time.Millisecond)

	// Deny all pending
	pg.DenyAll()

	wg.Wait()

	for i, r := range results {
		assert(t, !r, "request "+string(rune('1'+i))+" should be denied")
	}
}

func TestPermissionGateway_Concurrent(t *testing.T) {
	pg := NewPermissionGateway()

	const n = 20
	var wg sync.WaitGroup
	results := make([]bool, n)

	for i := 0; i < n; i++ {
		id := "perm-" + string(rune('a'+i))
		pg.Register(id, "tool", "chat")

		wg.Add(1)
		go func(idx int, reqID string) {
			defer wg.Done()
			results[idx] = pg.WaitFor(reqID)
		}(i, id)
	}

	time.Sleep(20 * time.Millisecond)

	// Resolve odd ones as allow, even ones as deny
	for i := 0; i < n; i++ {
		id := "perm-" + string(rune('a'+i))
		pg.Resolve(id, i%2 == 1)
	}

	wg.Wait()

	for i := 0; i < n; i++ {
		if i%2 == 1 {
			assert(t, results[i], "odd request should be allowed")
		} else {
			assert(t, !results[i], "even request should be denied")
		}
	}
}

func TestPermissionGateway_DoubleResolve(t *testing.T) {
	pg := NewPermissionGateway()
	pg.Register("double", "tool", "chat")

	go func() {
		time.Sleep(10 * time.Millisecond)
		pg.Resolve("double", true)
		// Second resolve should be a no-op (request already removed after WaitFor returns)
		err := pg.Resolve("double", false)
		// After WaitFor consumes the result, the request is removed
		_ = err
	}()

	result := pg.WaitFor("double")
	assert(t, result, "first resolve should take effect (allowed)")
}

func TestPermissionRequest_Fields(t *testing.T) {
	pg := NewPermissionGateway()
	req := pg.Register("id-1", "bash", "chat-42")

	assert(t, req.ID == "id-1", "ID mismatch")
	assert(t, req.Tool == "bash", "Tool mismatch")
	assert(t, req.ChatID == "chat-42", "ChatID mismatch")
}
