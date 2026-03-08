package tunnel

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"sync"
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
	execEnabled bool

	procMu    sync.Mutex
	processes map[string]*exec.Cmd
	stdinMap  map[string]io.WriteCloser
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
		processes: make(map[string]*exec.Cmd),
		stdinMap:  make(map[string]io.WriteCloser),
	}
}

// AddProvider registers an http.Handler for a path prefix (e.g. "webdav", "mcp").
func (c *Client) AddProvider(name string, handler http.Handler) {
	c.providers[name] = handler
	c.caps = append(c.caps, name)
}

// EnableExec opts in to remote exec capability. Must be called before Connect.
func (c *Client) EnableExec() {
	c.execEnabled = true
	c.caps = append(c.caps, "exec")
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
		// Kill all running processes on disconnect
		c.killAllProcesses()
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

		// Peek at type to dispatch
		var peek struct {
			Type string `json:"type"`
		}
		if err := json.Unmarshal(msg, &peek); err != nil {
			c.logger.Warn("invalid tunnel message", "err", err)
			continue
		}

		switch {
		case peek.Type == "exec.start":
			if !c.execEnabled {
				c.logger.Warn("received exec.start but exec not enabled")
				continue
			}
			var req ExecRequest
			if err := json.Unmarshal(msg, &req); err != nil {
				c.logger.Warn("invalid exec request", "err", err)
				continue
			}
			go c.handleExecStart(conn, &req)

		case peek.Type == "exec.stdin":
			var input ExecInputMsg
			if err := json.Unmarshal(msg, &input); err != nil {
				c.logger.Warn("invalid exec stdin msg", "err", err)
				continue
			}
			c.handleExecStdin(&input)

		case peek.Type == "exec.stop":
			var stop ExecStopMsg
			if err := json.Unmarshal(msg, &stop); err != nil {
				c.logger.Warn("invalid exec stop msg", "err", err)
				continue
			}
			c.handleExecStop(&stop)

		default:
			// HTTP tunnel request (existing behavior)
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

// handleExecStart spawns a local process and streams output back to server.
func (c *Client) handleExecStart(conn *websocket.Conn, req *ExecRequest) {
	if len(req.Command) == 0 {
		c.sendExecMsg(conn, &ExecStreamMsg{
			ID:   req.ID,
			Type: "exec.error",
			Data: "empty command",
		})
		return
	}

	cmd := exec.Command(req.Command[0], req.Command[1:]...)
	if req.Dir != "" {
		cmd.Dir = req.Dir
	}

	// Build env
	cmd.Env = os.Environ()
	for k, v := range req.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.sendExecMsg(conn, &ExecStreamMsg{
			ID:   req.ID,
			Type: "exec.error",
			Data: "stdout pipe: " + err.Error(),
		})
		return
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		c.sendExecMsg(conn, &ExecStreamMsg{
			ID:   req.ID,
			Type: "exec.error",
			Data: "stderr pipe: " + err.Error(),
		})
		return
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		c.sendExecMsg(conn, &ExecStreamMsg{
			ID:   req.ID,
			Type: "exec.error",
			Data: "stdin pipe: " + err.Error(),
		})
		return
	}

	if err := cmd.Start(); err != nil {
		c.sendExecMsg(conn, &ExecStreamMsg{
			ID:   req.ID,
			Type: "exec.error",
			Data: "start: " + err.Error(),
		})
		return
	}

	c.logger.Info("exec started", "id", req.ID, "cmd", req.Command)

	// Track the process
	c.procMu.Lock()
	c.processes[req.ID] = cmd
	c.stdinMap[req.ID] = stdin
	c.procMu.Unlock()

	// Stream stdout and stderr concurrently
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			c.sendExecMsg(conn, &ExecStreamMsg{
				ID:   req.ID,
				Type: "exec.stdout",
				Data: scanner.Text(),
			})
		}
	}()

	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			c.sendExecMsg(conn, &ExecStreamMsg{
				ID:   req.ID,
				Type: "exec.stderr",
				Data: scanner.Text(),
			})
		}
	}()

	wg.Wait()
	err = cmd.Wait()

	// Cleanup tracking
	c.procMu.Lock()
	delete(c.processes, req.ID)
	delete(c.stdinMap, req.ID)
	c.procMu.Unlock()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			c.sendExecMsg(conn, &ExecStreamMsg{
				ID:   req.ID,
				Type: "exec.error",
				Data: err.Error(),
			})
			return
		}
	}

	c.sendExecMsg(conn, &ExecStreamMsg{
		ID:       req.ID,
		Type:     "exec.done",
		ExitCode: exitCode,
	})

	c.logger.Info("exec finished", "id", req.ID, "exit_code", exitCode)
}

// handleExecStdin writes data to a running process's stdin.
func (c *Client) handleExecStdin(input *ExecInputMsg) {
	c.procMu.Lock()
	stdin, ok := c.stdinMap[input.ID]
	c.procMu.Unlock()

	if !ok {
		c.logger.Warn("exec stdin: process not found", "id", input.ID)
		return
	}

	if _, err := io.WriteString(stdin, input.Data); err != nil {
		c.logger.Warn("exec stdin write failed", "id", input.ID, "err", err)
	}
}

// handleExecStop kills a running process.
func (c *Client) handleExecStop(stop *ExecStopMsg) {
	c.procMu.Lock()
	cmd, ok := c.processes[stop.ID]
	c.procMu.Unlock()

	if !ok {
		c.logger.Warn("exec stop: process not found", "id", stop.ID)
		return
	}

	if cmd.Process != nil {
		c.logger.Info("killing exec process", "id", stop.ID)
		_ = cmd.Process.Kill()
	}
}

// killAllProcesses terminates all tracked processes (called on disconnect).
func (c *Client) killAllProcesses() {
	c.procMu.Lock()
	defer c.procMu.Unlock()

	for id, cmd := range c.processes {
		if cmd.Process != nil {
			c.logger.Info("killing orphaned exec process", "id", id)
			_ = cmd.Process.Kill()
		}
	}
	c.processes = make(map[string]*exec.Cmd)
	c.stdinMap = make(map[string]io.WriteCloser)
}

// sendExecMsg writes an exec stream message to the WebSocket connection.
func (c *Client) sendExecMsg(conn *websocket.Conn, msg *ExecStreamMsg) {
	if err := conn.WriteJSON(msg); err != nil {
		c.logger.Error("failed to send exec msg", "id", msg.ID, "type", msg.Type, "err", err)
	}
}

// TunnelError represents an error from the tunnel protocol.
type TunnelError struct {
	Message string
}

func (e *TunnelError) Error() string {
	return "tunnel: " + e.Message
}
