package channel

import (
	"fmt"
	"sync"
	"time"
)

const permissionTimeout = 5 * time.Minute

// PermissionRequest describes a pending tool-use permission request.
type PermissionRequest struct {
	ID       string // unique request ID (e.g. "permission_<shortid>")
	Tool     string // tool name
	ChatID   string
	resultCh chan bool
}

// PermissionGateway manages pending permission requests from agents.
// When an agent wants to use a tool, a request is created and the user
// is shown inline buttons to approve or deny. The agent goroutine blocks
// on WaitFor until the user responds or timeout (auto-deny after 5 min).
type PermissionGateway struct {
	mu      sync.Mutex
	pending map[string]*PermissionRequest
}

// NewPermissionGateway creates a new PermissionGateway.
func NewPermissionGateway() *PermissionGateway {
	return &PermissionGateway{
		pending: make(map[string]*PermissionRequest),
	}
}

// Register adds a permission request and returns it.
// The caller should then send buttons to the user and call WaitFor.
func (pg *PermissionGateway) Register(id, tool, chatID string) *PermissionRequest {
	req := &PermissionRequest{
		ID:       id,
		Tool:     tool,
		ChatID:   chatID,
		resultCh: make(chan bool, 1),
	}
	pg.mu.Lock()
	pg.pending[id] = req
	pg.mu.Unlock()
	return req
}

// WaitFor blocks until the user responds or timeout (auto-deny).
func (pg *PermissionGateway) WaitFor(id string) bool {
	pg.mu.Lock()
	req, ok := pg.pending[id]
	pg.mu.Unlock()
	if !ok {
		return false
	}

	timer := time.NewTimer(permissionTimeout)
	defer timer.Stop()

	select {
	case allowed := <-req.resultCh:
		pg.mu.Lock()
		delete(pg.pending, id)
		pg.mu.Unlock()
		return allowed
	case <-timer.C:
		pg.mu.Lock()
		delete(pg.pending, id)
		pg.mu.Unlock()
		return false
	}
}

// Resolve unblocks a waiting permission request.
func (pg *PermissionGateway) Resolve(id string, allowed bool) error {
	pg.mu.Lock()
	req, ok := pg.pending[id]
	pg.mu.Unlock()
	if !ok {
		return fmt.Errorf("permission request %s not found", id)
	}
	select {
	case req.resultCh <- allowed:
	default:
	}
	return nil
}

// DenyAll denies all pending requests (for cleanup).
func (pg *PermissionGateway) DenyAll() {
	pg.mu.Lock()
	defer pg.mu.Unlock()
	for id, req := range pg.pending {
		select {
		case req.resultCh <- false:
		default:
		}
		delete(pg.pending, id)
	}
}
