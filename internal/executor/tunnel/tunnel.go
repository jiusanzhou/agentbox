package tunnel

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"go.zoe.im/agentbox/internal/executor"
	"go.zoe.im/agentbox/pkg/runtime"
	"go.zoe.im/agentbox/internal/tunnel"
	"go.zoe.im/x"
)

func init() {
	executor.Register("tunnel", func(cfg x.TypedLazyConfig, opts ...any) (executor.Executor, error) {
		// The tunnel executor requires a Hub passed via opts.
		var hub *tunnel.Hub
		for _, o := range opts {
			if h, ok := o.(*tunnel.Hub); ok {
				hub = h
			}
		}
		if hub == nil {
			return nil, fmt.Errorf("tunnel executor requires a *tunnel.Hub option")
		}
		return New(hub), nil
	})
}

type tunnelSession struct {
	runID     string
	userID    string
	runtime   runtime.Runtime
	msgCnt    int
	env       map[string]string
	agentFile string
	logBuf    bytes.Buffer
	logMu     sync.Mutex
}

type tunnelExecutor struct {
	hub      *tunnel.Hub
	logger   *slog.Logger
	sessions map[string]*tunnelSession
	mu       sync.RWMutex
}

// New creates a tunnel executor that delegates to user's local client via tunnel.
func New(hub *tunnel.Hub) executor.Executor {
	return &tunnelExecutor{
		hub:      hub,
		logger:   slog.Default(),
		sessions: make(map[string]*tunnelSession),
	}
}

func (e *tunnelExecutor) Execute(ctx context.Context, req *executor.Request) (*executor.Response, error) {
	rt := e.getRuntime(req.Runtime)

	args := rt.BuildExecArgs(req.AgentFile, false)

	execReq := &tunnel.ExecRequest{
		ID:      req.ID,
		Command: args,
		Env:     req.Env,
	}

	ch, err := e.hub.StartExec(req.UserID, execReq)
	if err != nil {
		return nil, fmt.Errorf("tunnel exec: %w", err)
	}

	return e.collectExecOutput(ch)
}

func (e *tunnelExecutor) Logs(_ context.Context, id string) (string, error) {
	e.mu.RLock()
	sess, ok := e.sessions[id]
	e.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("session not found: %s", id)
	}
	sess.logMu.Lock()
	defer sess.logMu.Unlock()
	return sess.logBuf.String(), nil
}

func (e *tunnelExecutor) Stop(ctx context.Context, id string) error {
	return e.StopSession(ctx, id)
}

func (e *tunnelExecutor) StartSession(_ context.Context, req *executor.Request) (string, error) {
	rt := e.getRuntime(req.Runtime)

	if !e.hub.IsConnected(req.UserID) {
		return "", fmt.Errorf("user %s has no active tunnel connection", req.UserID)
	}
	if !e.hub.HasCapability(req.UserID, "exec") {
		return "", fmt.Errorf("user %s tunnel does not support exec", req.UserID)
	}

	sess := &tunnelSession{
		runID:     req.ID,
		userID:    req.UserID,
		runtime:   rt,
		env:       req.Env,
		agentFile: req.AgentFile,
	}

	e.mu.Lock()
	e.sessions[req.ID] = sess
	e.mu.Unlock()

	e.logger.Info("tunnel session started", "id", req.ID, "user", req.UserID, "runtime", rt.Name())
	return req.ID, nil
}

func (e *tunnelExecutor) SendMessage(ctx context.Context, id string, message string) (string, error) {
	e.mu.RLock()
	sess, ok := e.sessions[id]
	e.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("session not found: %s", id)
	}

	rt := sess.runtime
	args := rt.BuildExecArgs(message, sess.msgCnt > 0)

	// Strip streaming flags for non-streaming call
	filtered := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--output-format" || a == "stream-json" || a == "--verbose" {
			continue
		}
		filtered = append(filtered, a)
	}

	execID := fmt.Sprintf("%s-msg-%d", id, sess.msgCnt)
	execReq := &tunnel.ExecRequest{
		ID:      execID,
		Command: filtered,
		Env:     sess.env,
	}

	ch, err := e.hub.StartExec(sess.userID, execReq)
	if err != nil {
		return "", fmt.Errorf("tunnel send message: %w", err)
	}

	resp, err := e.collectExecOutput(ch)
	if err != nil {
		sess.appendLog(err.Error())
		return "", err
	}

	sess.appendLog(resp.Output)
	sess.msgCnt++
	return resp.Output, nil
}

func (e *tunnelExecutor) SendMessageStream(ctx context.Context, id string, message string, onToken executor.TokenCallback) (string, error) {
	e.mu.RLock()
	sess, ok := e.sessions[id]
	e.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("session not found: %s", id)
	}

	rt := sess.runtime
	args := rt.BuildExecArgs(message, sess.msgCnt > 0)

	execID := fmt.Sprintf("%s-msg-%d", id, sess.msgCnt)
	execReq := &tunnel.ExecRequest{
		ID:      execID,
		Command: args,
		Env:     sess.env,
	}

	ch, err := e.hub.StartExec(sess.userID, execReq)
	if err != nil {
		return "", fmt.Errorf("tunnel stream message: %w", err)
	}

	var fullResponse strings.Builder

	for msg := range ch {
		switch msg.Type {
		case "exec.stdout":
			line := msg.Data
			if line == "" {
				continue
			}

			token, result, done := rt.ParseStreamLine(line)
			if done && result != "" && fullResponse.Len() == 0 {
				fullResponse.WriteString(result)
				continue
			}
			if token != "" {
				if rt.Name() == "claude" {
					existing := fullResponse.String()
					if len(token) > len(existing) {
						delta := token[len(existing):]
						fullResponse.Reset()
						fullResponse.WriteString(token)
						if onToken != nil {
							onToken(delta)
						}
					} else if token != existing {
						fullResponse.Reset()
						fullResponse.WriteString(token)
					}
				} else {
					fullResponse.WriteString(token)
					if onToken != nil {
						onToken(token)
					}
				}
			}

		case "exec.stderr":
			// Log stderr but don't include in response tokens
			sess.appendLog("[stderr] " + msg.Data)

		case "exec.done":
			// Process completed

		case "exec.error":
			sess.appendLog("[error] " + msg.Data)
			return fullResponse.String(), fmt.Errorf("tunnel exec error: %s", msg.Data)
		}
	}

	sess.appendLog(fullResponse.String())
	sess.msgCnt++
	return fullResponse.String(), nil
}

func (e *tunnelExecutor) StopSession(_ context.Context, id string) error {
	e.mu.Lock()
	sess, ok := e.sessions[id]
	delete(e.sessions, id)
	e.mu.Unlock()

	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	// Attempt to stop any running process
	_ = e.hub.StopExec(sess.userID, id)

	e.logger.Info("tunnel session stopped", "id", id)
	return nil
}

func (e *tunnelExecutor) RecoverSessions(_ context.Context) ([]string, error) {
	// Can't recover remote sessions
	return nil, nil
}

func (e *tunnelExecutor) UploadFile(_ context.Context, runID string, filename string, data []byte) error {
	e.mu.RLock()
	sess, ok := e.sessions[runID]
	e.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session not found: %s", runID)
	}

	// Delegate upload to the tunnel HTTP forward (webdav provider)
	req := &tunnel.TunnelRequest{
		ID:     fmt.Sprintf("upload-%s-%s", runID, filename),
		Method: "PUT",
		Path:   "/webdav/uploads/" + filename,
		Headers: map[string]string{
			"Content-Type": "application/octet-stream",
		},
		Body: data,
	}

	resp, err := e.hub.Forward(sess.userID, req)
	if err != nil {
		return fmt.Errorf("tunnel upload: %w", err)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("tunnel upload failed: status %d", resp.StatusCode)
	}
	return nil
}

func (e *tunnelExecutor) StreamLogs(_ context.Context, id string) (<-chan string, error) {
	e.mu.RLock()
	sess, ok := e.sessions[id]
	e.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("session not found: %s", id)
	}

	ch := make(chan string, 64)
	go func() {
		defer close(ch)
		sess.logMu.Lock()
		data := sess.logBuf.String()
		sess.logMu.Unlock()
		for _, line := range strings.Split(data, "\n") {
			ch <- line
		}
	}()
	return ch, nil
}

// collectExecOutput reads all messages from an exec stream and returns a Response.
func (e *tunnelExecutor) collectExecOutput(ch <-chan *tunnel.ExecStreamMsg) (*executor.Response, error) {
	var stdout, stderr strings.Builder

	for msg := range ch {
		switch msg.Type {
		case "exec.stdout":
			stdout.WriteString(msg.Data)
			stdout.WriteByte('\n')
		case "exec.stderr":
			stderr.WriteString(msg.Data)
			stderr.WriteByte('\n')
		case "exec.done":
			output := strings.TrimSpace(stdout.String())
			if stderr.Len() > 0 {
				output += "\n--- stderr ---\n" + strings.TrimSpace(stderr.String())
			}
			return &executor.Response{
				ExitCode: msg.ExitCode,
				Output:   output,
				Logs:     output,
			}, nil
		case "exec.error":
			return nil, fmt.Errorf("tunnel exec error: %s", msg.Data)
		}
	}

	// Channel closed without done/error — likely disconnect
	output := strings.TrimSpace(stdout.String())
	return &executor.Response{
		ExitCode: 1,
		Output:   output,
		Logs:     output,
	}, fmt.Errorf("tunnel connection lost")
}

func (e *tunnelExecutor) getRuntime(name string) runtime.Runtime {
	if name != "" {
		if rt := runtime.Get(name); rt != nil {
			return rt
		}
	}
	return runtime.Default()
}

func (s *tunnelSession) appendLog(data string) {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	s.logBuf.WriteString(data)
	s.logBuf.WriteByte('\n')
}
