package tunnel

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// Client connects to the ABox server via WebSocket and handles forwarded requests.
type Client struct {
	serverURL  string
	token      string
	conn       *websocket.Conn
	providers  map[string]http.Handler
	logger     *slog.Logger
	caps       []string
}

func NewClient(serverURL, token string, logger *slog.Logger) *Client {
	if logger == nil {
		logger = slog.Default()
	}
	return &Client{
		serverURL: serverURL,
		token:     token,
		providers: make(map[string]http.Handler),
		logger:    logger,
	}
}

// AddProvider registers an http.Handler for a path prefix (e.g. "webdav", "mcp").
func (c *Client) AddProvider(name string, handler http.Handler) {
	c.providers[name] = handler
	c.caps = append(c.caps, name)
}

// Connect establishes the WebSocket tunnel with auto-reconnect.
func (c *Client) Connect(ctx context.Context) error {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		err := c.connectOnce(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		c.logger.Warn("tunnel disconnected, reconnecting", "err", err, "backoff", backoff)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (c *Client) connectOnce(ctx context.Context) error {
	// Build WebSocket URL
	u, err := url.Parse(c.serverURL)
	if err != nil {
		return err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "/api/v1/tunnel"

	c.logger.Info("connecting to tunnel", "url", u.String())

	conn, _, err := websocket.DefaultDialer.DialContext(ctx, u.String(), nil)
	if err != nil {
		return err
	}
	c.conn = conn
	defer func() {
		conn.Close()
		c.conn = nil
	}()

	// Send hello
	hello := HelloMessage{
		Type:         "hello",
		Token:        c.token,
		Capabilities: c.caps,
		Version:      "1",
	}
	if err := conn.WriteJSON(hello); err != nil {
		return err
	}

	// Read welcome
	var resp HelloResponse
	if err := conn.ReadJSON(&resp); err != nil {
		return err
	}
	if resp.Type == "error" {
		return &TunnelError{Message: resp.Error}
	}

	c.logger.Info("tunnel connected", "user_id", resp.UserID)

	// Read loop
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		var req TunnelRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			c.logger.Warn("invalid tunnel request", "err", err)
			continue
		}

		go func() {
			tunnelResp := c.handleRequest(&req)
			if writeErr := conn.WriteJSON(tunnelResp); writeErr != nil {
				c.logger.Error("failed to send response", "err", writeErr)
			}
		}()
	}
}

func (c *Client) handleRequest(req *TunnelRequest) *TunnelResponse {
	// Route based on first path segment: /webdav/... → "webdav" provider
	path := strings.TrimPrefix(req.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	providerName := parts[0]
	subPath := "/"
	if len(parts) > 1 {
		subPath = "/" + parts[1]
	}

	handler, ok := c.providers[providerName]
	if !ok {
		// Try serving from the first provider if no prefix match
		return &TunnelResponse{
			ID:         req.ID,
			StatusCode: http.StatusNotFound,
			Body:       []byte("unknown provider: " + providerName),
		}
	}

	// Build HTTP request for the handler
	httpReq, _ := http.NewRequest(req.Method, subPath, bytes.NewReader(req.Body))
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httpReq)

	result := recorder.Result()
	respHeaders := make(map[string]string)
	for k, v := range result.Header {
		if len(v) > 0 {
			respHeaders[k] = v[0]
		}
	}

	return &TunnelResponse{
		ID:         req.ID,
		StatusCode: result.StatusCode,
		Headers:    respHeaders,
		Body:       recorder.Body.Bytes(),
	}
}

// TunnelError represents an error from the tunnel protocol.
type TunnelError struct {
	Message string
}

func (e *TunnelError) Error() string {
	return "tunnel: " + e.Message
}
