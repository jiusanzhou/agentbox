package tunnel

// TunnelRequest is sent from server to client via WebSocket.
type TunnelRequest struct {
	ID      string            `json:"id"`
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    []byte            `json:"body,omitempty"`
}

// TunnelResponse is sent from client back to server via WebSocket.
type TunnelResponse struct {
	ID         string            `json:"id"`
	StatusCode int               `json:"status_code"`
	Headers    map[string]string `json:"headers,omitempty"`
	Body       []byte            `json:"body,omitempty"`
}

// HelloMessage is sent by the client after WebSocket connection.
type HelloMessage struct {
	Type         string   `json:"type"`
	Token        string   `json:"token"`
	Capabilities []string `json:"capabilities"`
	Version      string   `json:"version"`
}

// HelloResponse is sent by the server after validating the hello message.
type HelloResponse struct {
	Type   string `json:"type"`
	UserID string `json:"user_id,omitempty"`
	Error  string `json:"error,omitempty"`
}
