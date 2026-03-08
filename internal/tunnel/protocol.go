package tunnel

// TunnelRequest is sent from server to client via WebSocket.
type TunnelRequest struct {
	ID      string            `json:"id"`
	Type    string            `json:"type,omitempty"` // "http" (default) for HTTP forwarding
	Method  string            `json:"method"`
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    []byte            `json:"body,omitempty"`
}

// TunnelResponse is sent from client back to server via WebSocket.
type TunnelResponse struct {
	ID         string            `json:"id"`
	Type       string            `json:"type,omitempty"` // "http" (default) for HTTP responses
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

// --- Exec protocol messages ---

// ExecRequest is sent from server to client to start a process.
type ExecRequest struct {
	ID      string            `json:"id"`
	Type    string            `json:"type"` // "exec.start"
	Command []string          `json:"command"`
	Dir     string            `json:"dir,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// ExecStreamMsg is sent from client to server with stdout/stderr chunks or completion.
type ExecStreamMsg struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // "exec.stdout" | "exec.stderr" | "exec.done" | "exec.error"
	Data     string `json:"data,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
}

// ExecInputMsg is sent from server to client to write to stdin.
type ExecInputMsg struct {
	ID   string `json:"id"`
	Type string `json:"type"` // "exec.stdin"
	Data string `json:"data"`
}

// ExecStopMsg is sent from server to client to kill a process.
type ExecStopMsg struct {
	ID   string `json:"id"`
	Type string `json:"type"` // "exec.stop"
}
