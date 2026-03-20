package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"go.zoe.im/agentbox/internal/executor"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // allow all origins for development
	},
}

const (
	wsPingInterval  = 30 * time.Second
	wsReadDeadline  = 60 * time.Second
	wsWriteDeadline = 10 * time.Second
)

// wsIncoming represents a message sent by the client over the WebSocket.
type wsIncoming struct {
	Type    string `json:"type"`
	Content string `json:"content"`
}

// wsOutgoing represents a message sent by the server over the WebSocket.
type wsOutgoing struct {
	Type    string `json:"type"`
	Content string `json:"content,omitempty"`
}

// HandleWebSocket upgrades the HTTP connection to a WebSocket and provides
// real-time streaming chat for a session. The session ID is taken from the
// URL path and authentication is performed via a "token" query parameter.
//
// Route: GET /api/v1/ws/{session_id}
func (s *Service) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	sessionID := r.PathValue("session_id")
	if sessionID == "" {
		http.Error(w, `{"error":"session_id required"}`, http.StatusBadRequest)
		return
	}

	// Authenticate via query-string token.
	tokenStr := r.URL.Query().Get("token")
	if tokenStr == "" {
		http.Error(w, `{"error":"token query parameter required"}`, http.StatusUnauthorized)
		return
	}

	user, err := s.auth.ValidateToken(r.Context(), tokenStr)
	if err != nil {
		http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
		return
	}

	// Verify the session exists and belongs to the authenticated user.
	run, err := s.engine.Get(r.Context(), sessionID)
	if err != nil {
		http.Error(w, `{"error":"session not found"}`, http.StatusNotFound)
		return
	}
	if run.UserID != user.ID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	// Upgrade to WebSocket.
	conn, err := wsUpgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("websocket upgrade failed", slog.String("error", err.Error()))
		return
	}

	s.logger.Info("websocket connected",
		slog.String("session_id", sessionID),
		slog.String("user_id", user.ID),
	)

	s.serveWebSocket(conn, sessionID)
}

// serveWebSocket runs the read/write loops for a single WebSocket connection.
func (s *Service) serveWebSocket(conn *websocket.Conn, sessionID string) {
	var (
		writeMu sync.Mutex
		done    = make(chan struct{})
	)

	// writeJSON sends a JSON message to the client in a thread-safe manner.
	writeJSON := func(msg wsOutgoing) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(wsWriteDeadline))
		return conn.WriteJSON(msg)
	}

	// Cleanup on exit.
	defer func() {
		close(done)
		conn.Close()
		s.logger.Info("websocket disconnected", slog.String("session_id", sessionID))
	}()

	// Start the ping/pong keepalive goroutine.
	go func() {
		ticker := time.NewTicker(wsPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				writeMu.Lock()
				_ = conn.SetWriteDeadline(time.Now().Add(wsWriteDeadline))
				err := conn.WriteMessage(websocket.PingMessage, nil)
				writeMu.Unlock()
				if err != nil {
					return
				}
			}
		}
	}()

	// Configure pong handler to extend the read deadline.
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(wsReadDeadline))
	})

	// Read loop.
	for {
		_ = conn.SetReadDeadline(time.Now().Add(wsReadDeadline))

		_, raw, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseNormalClosure,
			) {
				s.logger.Warn("websocket read error",
					slog.String("session_id", sessionID),
					slog.String("error", err.Error()),
				)
			}
			return
		}

		var msg wsIncoming
		if err := json.Unmarshal(raw, &msg); err != nil {
			_ = writeJSON(wsOutgoing{Type: "error", Content: "invalid JSON"})
			continue
		}

		switch msg.Type {
		case "message":
			if msg.Content == "" {
				_ = writeJSON(wsOutgoing{Type: "error", Content: "empty message"})
				continue
			}
			s.handleStreamMessage(writeJSON, sessionID, msg.Content)

		default:
			_ = writeJSON(wsOutgoing{Type: "error", Content: "unknown message type"})
		}
	}
}

// handleStreamMessage sends a user message to the engine and streams tokens
// back over the WebSocket connection.
func (s *Service) handleStreamMessage(
	writeJSON func(wsOutgoing) error,
	sessionID string,
	content string,
) {
	onToken := executor.TokenCallback(func(token string) {
		if err := writeJSON(wsOutgoing{Type: "token", Content: token}); err != nil {
			s.logger.Warn("websocket token write error",
				slog.String("session_id", sessionID),
				slog.String("error", err.Error()),
			)
		}
	})

	// Use a background context because the WebSocket connection lifetime is
	// managed separately from the original HTTP request context.
	_, err := s.engine.SendMessageStream(context.Background(), sessionID, content, onToken)
	if err != nil {
		_ = writeJSON(wsOutgoing{Type: "error", Content: err.Error()})
		return
	}

	_ = writeJSON(wsOutgoing{Type: "done"})
}
