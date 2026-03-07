package tunnel

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

const forwardTimeout = 30 * time.Second

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
	UserID  string
	Conn    *websocket.Conn
	Caps    []string
	pending map[string]chan *TunnelResponse
	mu      sync.Mutex
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
		UserID:  userID,
		Conn:    conn,
		Caps:    hello.Capabilities,
		pending: make(map[string]chan *TunnelResponse),
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

	// Read loop: receives TunnelResponse messages from client
	defer func() {
		h.mu.Lock()
		if h.clients[userID] == client {
			delete(h.clients, userID)
		}
		h.mu.Unlock()
		h.logger.Info("tunnel client disconnected", "user", userID)
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}

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
