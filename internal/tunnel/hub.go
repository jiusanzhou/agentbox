package tunnel

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

const forwardTimeout = 30 * time.Second

// execTimeout is the maximum time to wait for an exec session to complete.
const execTimeout = 10 * time.Minute

// TokenValidator validates an auth token and returns the user ID.
type TokenValidator func(token string) (userID string, err error)

// Hub manages connected tunnel clients, one per user.
type Hub struct {
	mu       sync.RWMutex
	clients  map[string]*ClientConn
	logger   *slog.Logger
	validate TokenValidator
}

// ClientConn represents a connected tunnel client.
type ClientConn struct {
	UserID      string
	Conn        *websocket.Conn
	Caps        []string
	pending     map[string]chan *TunnelResponse
	execStreams map[string]chan *ExecStreamMsg
	mu          sync.Mutex
}

func NewHub(logger *slog.Logger, validate TokenValidator) *Hub {
	if logger == nil {
		logger = slog.Default()
	}
	return &Hub{
		clients:  make(map[string]*ClientConn),
		logger:   logger,
		validate: validate,
	}
}

// HandleConnect upgrades HTTP to WebSocket and registers the client.
func (h *Hub) HandleConnect(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Error("websocket upgrade failed", "err", err)
		return
	}
	defer conn.Close()

	// Read hello message
	var hello HelloMessage
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if err := conn.ReadJSON(&hello); err != nil {
		h.logger.Error("failed to read hello", "err", err)
		conn.WriteJSON(HelloResponse{Type: "error", Error: "invalid hello message"})
		return
	}
	conn.SetReadDeadline(time.Time{})

	if hello.Type != "hello" {
		conn.WriteJSON(HelloResponse{Type: "error", Error: "expected hello message"})
		return
	}

	// Validate token
	userID, err := h.validate(hello.Token)
	if err != nil {
		h.logger.Warn("tunnel auth failed", "err", err)
		conn.WriteJSON(HelloResponse{Type: "error", Error: "authentication failed"})
		return
	}

	// Register client
	client := &ClientConn{
		UserID:      userID,
		Conn:        conn,
		Caps:        hello.Capabilities,
		pending:     make(map[string]chan *TunnelResponse),
		execStreams: make(map[string]chan *ExecStreamMsg),
	}

	h.mu.Lock()
	old := h.clients[userID]
	h.clients[userID] = client
	h.mu.Unlock()

	if old != nil {
		old.Conn.Close()
	}

	h.logger.Info("tunnel client connected", "user", userID, "caps", hello.Capabilities)

	conn.WriteJSON(HelloResponse{Type: "welcome", UserID: userID})

	// Read loop: receives TunnelResponse and ExecStreamMsg messages from client
	defer func() {
		h.mu.Lock()
		if h.clients[userID] == client {
			delete(h.clients, userID)
		}
		h.mu.Unlock()

		// Close all pending exec streams on disconnect
		client.mu.Lock()
		for id, ch := range client.execStreams {
			close(ch)
			delete(client.execStreams, id)
		}
		client.mu.Unlock()

		h.logger.Info("tunnel client disconnected", "user", userID)
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}

		// Peek at the "type" field to determine message kind
		var peek struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(msg, &peek); err != nil {
			h.logger.Warn("invalid tunnel message", "err", err)
			continue
		}

		if strings.HasPrefix(peek.Type, "exec.") {
			// Exec stream message
			var execMsg ExecStreamMsg
			if err := json.Unmarshal(msg, &execMsg); err != nil {
				h.logger.Warn("invalid exec stream msg", "err", err)
				continue
			}

			client.mu.Lock()
			ch, ok := client.execStreams[execMsg.ID]
			client.mu.Unlock()

			if ok {
				ch <- &execMsg
				// Close channel on terminal messages
				if execMsg.Type == "exec.done" || execMsg.Type == "exec.error" {
					client.mu.Lock()
					delete(client.execStreams, execMsg.ID)
					client.mu.Unlock()
					close(ch)
				}
			}
		} else {
			// HTTP tunnel response (default)
			var resp TunnelResponse
			if err := json.Unmarshal(msg, &resp); err != nil {
				h.logger.Warn("invalid tunnel response", "err", err)
				continue
			}

			client.mu.Lock()
			ch, ok := client.pending[resp.ID]
			if ok {
				delete(client.pending, resp.ID)
			}
			client.mu.Unlock()

			if ok {
				ch <- &resp
			}
		}
	}
}

// Forward sends a request through the tunnel and waits for a response.
func (h *Hub) Forward(userID string, req *TunnelRequest) (*TunnelResponse, error) {
	h.mu.RLock()
	client, ok := h.clients[userID]
	h.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no tunnel for user %s", userID)
	}

	ch := make(chan *TunnelResponse, 1)

	client.mu.Lock()
	client.pending[req.ID] = ch
	client.mu.Unlock()

	client.mu.Lock()
	err := client.Conn.WriteJSON(req)
	client.mu.Unlock()
	if err != nil {
		client.mu.Lock()
		delete(client.pending, req.ID)
		client.mu.Unlock()
		return nil, fmt.Errorf("write to tunnel: %w", err)
	}

	select {
	case resp := <-ch:
		return resp, nil
	case <-time.After(forwardTimeout):
		client.mu.Lock()
		delete(client.pending, req.ID)
		client.mu.Unlock()
		return nil, fmt.Errorf("tunnel forward timeout for user %s", userID)
	}
}

// StartExec sends an exec.start request and returns a channel that receives stream messages.
func (h *Hub) StartExec(userID string, req *ExecRequest) (<-chan *ExecStreamMsg, error) {
	h.mu.RLock()
	client, ok := h.clients[userID]
	h.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("no tunnel for user %s", userID)
	}

	ch := make(chan *ExecStreamMsg, 64)

	client.mu.Lock()
	client.execStreams[req.ID] = ch
	client.mu.Unlock()

	req.Type = "exec.start"

	client.mu.Lock()
	err := client.Conn.WriteJSON(req)
	client.mu.Unlock()
	if err != nil {
		client.mu.Lock()
		delete(client.execStreams, req.ID)
		client.mu.Unlock()
		close(ch)
		return nil, fmt.Errorf("write exec request to tunnel: %w", err)
	}

	return ch, nil
}

// SendInput sends stdin data to a running exec process on the client.
func (h *Hub) SendInput(userID, execID, data string) error {
	h.mu.RLock()
	client, ok := h.clients[userID]
	h.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no tunnel for user %s", userID)
	}

	msg := &ExecInputMsg{
		ID:   execID,
		Type: "exec.stdin",
		Data: data,
	}

	client.mu.Lock()
	err := client.Conn.WriteJSON(msg)
	client.mu.Unlock()
	return err
}

// StopExec sends a kill signal for a running exec process on the client.
func (h *Hub) StopExec(userID, execID string) error {
	h.mu.RLock()
	client, ok := h.clients[userID]
	h.mu.RUnlock()

	if !ok {
		return fmt.Errorf("no tunnel for user %s", userID)
	}

	msg := &ExecStopMsg{
		ID:   execID,
		Type: "exec.stop",
	}

	client.mu.Lock()
	err := client.Conn.WriteJSON(msg)
	client.mu.Unlock()
	return err
}

// IsConnected returns true if the user has an active tunnel.
func (h *Hub) IsConnected(userID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.clients[userID]
	return ok
}

// GetCapabilities returns the capabilities of a connected client.
func (h *Hub) GetCapabilities(userID string) []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if c, ok := h.clients[userID]; ok {
		return c.Caps
	}
	return nil
}

// HasCapability returns true if the connected client has the given capability.
func (h *Hub) HasCapability(userID, cap string) bool {
	caps := h.GetCapabilities(userID)
	for _, c := range caps {
		if c == cap {
			return true
		}
	}
	return false
}
