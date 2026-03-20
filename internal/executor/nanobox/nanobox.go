package nanobox

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.zoe.im/agentbox/internal/executor"
	"go.zoe.im/agentbox/pkg/runtime"
	"go.zoe.im/x"
)

// Config for the NanoBox sandbox executor.
type Config struct {
	APIURL   string `json:"api_url" yaml:"api_url"`   // e.g. http://localhost:9090
	APIKey   string `json:"api_key" yaml:"api_key"`
	Template string `json:"template" yaml:"template"` // sandbox template
	Timeout  int    `json:"timeout" yaml:"timeout"`   // seconds, default 300
}

func init() {
	executor.Register("nanobox", func(cfg x.TypedLazyConfig, opts ...any) (executor.Executor, error) {
		var c Config
		if len(cfg.Config) > 0 {
			if err := cfg.Unmarshal(&c); err != nil {
				return nil, err
			}
		}
		return New(c)
	})
}

// New creates a new NanoBox executor and verifies connectivity via health check.
func New(cfg Config) (executor.Executor, error) {
	if cfg.APIURL == "" {
		cfg.APIURL = "http://localhost:9090"
	}
	if cfg.Template == "" {
		cfg.Template = "default"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 300
	}

	e := &nanoboxExecutor{
		cfg:    cfg,
		client: &http.Client{Timeout: 30 * time.Second},
		sandboxes: make(map[string]*nanoboxSession),
		logger: slog.Default(),
	}

	// Health check to verify connectivity.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := e.healthCheck(ctx); err != nil {
		return nil, fmt.Errorf("nanobox health check failed: %w", err)
	}

	e.logger.Info("nanobox executor initialized", "api_url", cfg.APIURL, "template", cfg.Template)
	return e, nil
}

type nanoboxSession struct {
	sandboxID string
	runID     string
	runtime   runtime.Runtime
	msgCnt    int
	env       map[string]string
	logBuf    bytes.Buffer
	logMu     sync.Mutex
}

func (s *nanoboxSession) appendLog(data string) {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	s.logBuf.WriteString(data)
	s.logBuf.WriteByte('\n')
}

type nanoboxExecutor struct {
	cfg       Config
	client    *http.Client
	sandboxes map[string]*nanoboxSession // runID -> session
	mu        sync.RWMutex
	logger    *slog.Logger
}

// --- NanoBox API client methods ---

// doRequest makes an authenticated JSON request to the NanoBox API.
func (e *nanoboxExecutor) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, int, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, e.cfg.APIURL+path, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	if e.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.cfg.APIKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	return data, resp.StatusCode, nil
}

// doStreamRequest makes an authenticated request and returns the response body for streaming reads.
// The caller is responsible for closing the returned body.
func (e *nanoboxExecutor) doStreamRequest(ctx context.Context, method, path string, body interface{}) (io.ReadCloser, int, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, e.cfg.APIURL+path, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	if e.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.cfg.APIKey)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("http request: %w", err)
	}

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, resp.StatusCode, fmt.Errorf("nanobox request %s %s failed (%d): %s", method, path, resp.StatusCode, data)
	}

	return resp.Body, resp.StatusCode, nil
}

// healthCheck verifies the NanoBox API is reachable.
func (e *nanoboxExecutor) healthCheck(ctx context.Context) error {
	_, status, err := e.doRequest(ctx, "GET", "/api/v1/health", nil)
	if err != nil {
		return err
	}
	if status >= 400 {
		return fmt.Errorf("health endpoint returned status %d", status)
	}
	return nil
}

// createSandbox creates a new NanoBox sandbox and returns its ID.
func (e *nanoboxExecutor) createSandbox(ctx context.Context, env map[string]string, timeout int) (string, error) {
	template := e.cfg.Template
	if timeout <= 0 {
		timeout = e.cfg.Timeout
	}

	payload := map[string]interface{}{
		"template": template,
		"timeout":  timeout,
	}
	if len(env) > 0 {
		payload["env"] = env
	}

	data, status, err := e.doRequest(ctx, "POST", "/api/v1/sandboxes", payload)
	if err != nil {
		return "", fmt.Errorf("create sandbox: %w", err)
	}
	if status >= 400 {
		return "", fmt.Errorf("nanobox create sandbox failed (%d): %s", status, data)
	}

	var result struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("parse sandbox response: %w", err)
	}
	return result.ID, nil
}

// execCommand runs a command in a sandbox and returns the output and exit code.
func (e *nanoboxExecutor) execCommand(ctx context.Context, sandboxID string, command string, timeout int) (string, int, error) {
	if timeout <= 0 {
		timeout = 60
	}

	payload := map[string]interface{}{
		"command": command,
		"timeout": timeout,
	}

	data, status, err := e.doRequest(ctx, "POST", "/api/v1/sandboxes/"+sandboxID+"/exec", payload)
	if err != nil {
		return "", -1, fmt.Errorf("exec command: %w", err)
	}
	if status >= 400 {
		return "", -1, fmt.Errorf("nanobox exec failed (%d): %s", status, data)
	}

	var result struct {
		ExitCode int    `json:"exit_code"`
		Output   string `json:"output"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", -1, fmt.Errorf("parse exec response: %w", err)
	}
	return result.Output, result.ExitCode, nil
}

// execCommandStream runs a command with streaming response, calling onLine for each output line.
// Returns the full output and the exit code.
func (e *nanoboxExecutor) execCommandStream(ctx context.Context, sandboxID string, command string, timeout int, onLine func(data string)) (string, int, error) {
	if timeout <= 0 {
		timeout = 60
	}

	payload := map[string]interface{}{
		"command": command,
		"timeout": timeout,
		"stream":  true,
	}

	body, _, err := e.doStreamRequest(ctx, "POST", "/api/v1/sandboxes/"+sandboxID+"/exec", payload)
	if err != nil {
		return "", -1, err
	}
	defer body.Close()

	var fullOutput strings.Builder
	exitCode := 0

	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var event struct {
			Type     string `json:"type"`
			Data     string `json:"data"`
			ExitCode int    `json:"exit_code"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		switch event.Type {
		case "stdout", "stderr":
			fullOutput.WriteString(event.Data)
			if onLine != nil {
				onLine(event.Data)
			}
		case "done":
			exitCode = event.ExitCode
		}
	}

	return fullOutput.String(), exitCode, scanner.Err()
}

// deleteSandbox stops and removes a sandbox.
func (e *nanoboxExecutor) deleteSandbox(ctx context.Context, sandboxID string) error {
	_, status, err := e.doRequest(ctx, "DELETE", "/api/v1/sandboxes/"+sandboxID, nil)
	if err != nil {
		return fmt.Errorf("delete sandbox: %w", err)
	}
	if status >= 400 {
		return fmt.Errorf("nanobox delete sandbox failed: %d", status)
	}
	return nil
}

// --- Executor interface implementation ---

func (e *nanoboxExecutor) Execute(ctx context.Context, req *executor.Request) (*executor.Response, error) {
	timeout := e.cfg.Timeout
	if req.Timeout > 0 {
		timeout = req.Timeout
	}

	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// Merge env with agent file.
	env := make(map[string]string)
	for k, v := range req.Env {
		env[k] = v
	}
	env["AGENTBOX_RUN_ID"] = req.ID
	if req.AgentFile != "" {
		env["AGENTBOX_AGENT_FILE"] = req.AgentFile
	}

	sandboxID, err := e.createSandbox(execCtx, env, timeout)
	if err != nil {
		return nil, err
	}
	defer e.deleteSandbox(ctx, sandboxID)

	rt := e.getRuntime(req.Runtime)

	// Write agent file if provided.
	if req.AgentFile != "" {
		writeCmd := fmt.Sprintf("cat > /workspace/AGENTS.md << 'AGENTBOX_EOF'\n%s\nAGENTBOX_EOF", req.AgentFile)
		if _, _, err := e.execCommand(execCtx, sandboxID, writeCmd, 30); err != nil {
			return nil, fmt.Errorf("write agent file: %w", err)
		}
	}

	args := rt.BuildExecArgs(req.AgentFile, false)
	cmd := shellJoin(args)

	e.logger.Info("executing in nanobox sandbox", "runtime", rt.Name(), "sandbox", sandboxID)

	output, exitCode, err := e.execCommand(execCtx, sandboxID, cmd, timeout)
	if err != nil {
		return nil, err
	}

	return &executor.Response{
		ExitCode: exitCode,
		Output:   output,
		Logs:     output,
	}, nil
}

func (e *nanoboxExecutor) StartSession(ctx context.Context, req *executor.Request) (string, error) {
	rt := e.getRuntime(req.Runtime)

	env := make(map[string]string)
	for k, v := range req.Env {
		env[k] = v
	}
	env["AGENTBOX_RUN_ID"] = req.ID
	env["AGENTBOX_MODE"] = "session"

	sandboxID, err := e.createSandbox(ctx, env, e.cfg.Timeout)
	if err != nil {
		return "", err
	}

	// Write agent file if provided.
	if req.AgentFile != "" {
		writeCmd := fmt.Sprintf("cat > /workspace/AGENTS.md << 'AGENTBOX_EOF'\n%s\nAGENTBOX_EOF", req.AgentFile)
		if _, _, err := e.execCommand(ctx, sandboxID, writeCmd, 30); err != nil {
			_ = e.deleteSandbox(ctx, sandboxID)
			return "", fmt.Errorf("write agent file: %w", err)
		}
	}

	// Run setup commands.
	for _, cmd := range rt.SetupCommands() {
		if _, _, err := e.execCommand(ctx, sandboxID, cmd, 60); err != nil {
			e.logger.Warn("setup command failed", "cmd", cmd, "err", err)
		}
	}

	sess := &nanoboxSession{
		sandboxID: sandboxID,
		runID:     req.ID,
		runtime:   rt,
		env:       req.Env,
	}

	e.mu.Lock()
	e.sandboxes[req.ID] = sess
	e.mu.Unlock()

	e.logger.Info("nanobox session started", "id", req.ID, "sandbox", sandboxID, "runtime", rt.Name())
	return req.ID, nil
}

func (e *nanoboxExecutor) SendMessage(ctx context.Context, id string, message string) (string, error) {
	e.mu.RLock()
	sess, ok := e.sandboxes[id]
	e.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("session not found: %s", id)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	rt := sess.runtime
	args := rt.BuildExecArgs(message, sess.msgCnt > 0)

	// Strip streaming flags for non-streaming call.
	filtered := make([]string, 0, len(args))
	for _, a := range args {
		if a == "--output-format" || a == "stream-json" || a == "--verbose" {
			continue
		}
		filtered = append(filtered, a)
	}

	cmd := shellJoin(filtered)

	e.logger.Info("sending message to nanobox session", "id", id, "runtime", rt.Name())

	output, _, err := e.execCommand(cmdCtx, sess.sandboxID, cmd, 120)
	if err != nil {
		sess.appendLog(output)
		return output, fmt.Errorf("exec: %w", err)
	}

	output = strings.TrimSpace(output)
	sess.appendLog(output)
	sess.msgCnt++
	return output, nil
}

func (e *nanoboxExecutor) SendMessageStream(ctx context.Context, id string, message string, onToken executor.TokenCallback) (string, error) {
	e.mu.RLock()
	sess, ok := e.sandboxes[id]
	e.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("session not found: %s", id)
	}

	cmdCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	rt := sess.runtime
	args := rt.BuildExecArgs(message, sess.msgCnt > 0)
	cmd := shellJoin(args)

	e.logger.Info("streaming message to nanobox session", "id", id, "runtime", rt.Name())

	var fullResponse strings.Builder

	output, _, err := e.execCommandStream(cmdCtx, sess.sandboxID, cmd, 120, func(data string) {
		// Parse each line through the runtime for token extraction.
		for _, line := range strings.Split(data, "\n") {
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
		}
	})
	if err != nil {
		return "", err
	}

	// If the runtime parsing did not produce output, use the raw output.
	if fullResponse.Len() == 0 {
		fullResponse.WriteString(strings.TrimSpace(output))
	}

	sess.appendLog(fullResponse.String())
	sess.msgCnt++
	return fullResponse.String(), nil
}

func (e *nanoboxExecutor) StopSession(ctx context.Context, id string) error {
	e.mu.Lock()
	sess, ok := e.sandboxes[id]
	delete(e.sandboxes, id)
	e.mu.Unlock()

	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	e.logger.Info("stopping nanobox session", "id", id, "sandbox", sess.sandboxID)
	return e.deleteSandbox(ctx, sess.sandboxID)
}

func (e *nanoboxExecutor) Stop(ctx context.Context, id string) error {
	// Check if this is a tracked session first.
	e.mu.RLock()
	sess, ok := e.sandboxes[id]
	e.mu.RUnlock()

	if ok {
		return e.StopSession(ctx, id)
	}

	// Try to delete by sandbox ID directly (for Execute runs).
	_ = sess
	return e.deleteSandbox(ctx, id)
}

func (e *nanoboxExecutor) Logs(ctx context.Context, id string) (string, error) {
	// Check local session log buffer first.
	e.mu.RLock()
	sess, ok := e.sandboxes[id]
	e.mu.RUnlock()

	if ok {
		sess.logMu.Lock()
		defer sess.logMu.Unlock()
		return sess.logBuf.String(), nil
	}

	// Fall back to API.
	data, status, err := e.doRequest(ctx, "GET", "/api/v1/sandboxes/"+id+"/logs", nil)
	if err != nil {
		return "", fmt.Errorf("get logs: %w", err)
	}
	if status >= 400 {
		return "", fmt.Errorf("nanobox get logs failed (%d): %s", status, data)
	}
	return string(data), nil
}

func (e *nanoboxExecutor) RecoverSessions(ctx context.Context) ([]string, error) {
	data, status, err := e.doRequest(ctx, "GET", "/api/v1/sandboxes?status=running", nil)
	if err != nil {
		return nil, fmt.Errorf("list sandboxes: %w", err)
	}
	if status >= 400 {
		return nil, fmt.Errorf("nanobox list sandboxes failed (%d): %s", status, data)
	}

	var sandboxes []struct {
		ID     string            `json:"id"`
		Status string            `json:"status"`
		Env    map[string]string `json:"env"`
	}
	if err := json.Unmarshal(data, &sandboxes); err != nil {
		return nil, fmt.Errorf("parse sandboxes list: %w", err)
	}

	var ids []string
	e.mu.Lock()
	defer e.mu.Unlock()

	for _, s := range sandboxes {
		runID := s.Env["AGENTBOX_RUN_ID"]
		if runID == "" {
			runID = s.ID
		}
		ids = append(ids, runID)

		// Restore session if not already tracked and is in session mode.
		if _, exists := e.sandboxes[runID]; exists {
			continue
		}
		if s.Env["AGENTBOX_MODE"] != "session" {
			continue
		}

		runtimeName := s.Env["AGENTBOX_RUNTIME"]
		rt := e.getRuntime(runtimeName)

		e.sandboxes[runID] = &nanoboxSession{
			sandboxID: s.ID,
			runID:     runID,
			runtime:   rt,
			env:       s.Env,
		}

		e.logger.Info("recovered nanobox session", "id", runID, "sandbox", s.ID, "runtime", rt.Name())
	}

	return ids, nil
}

func (e *nanoboxExecutor) UploadFile(ctx context.Context, runID string, filename string, data []byte) error {
	e.mu.RLock()
	sess, ok := e.sandboxes[runID]
	e.mu.RUnlock()

	sandboxID := runID
	if ok {
		sandboxID = sess.sandboxID
	}

	// Build multipart form body.
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return fmt.Errorf("write form data: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close multipart writer: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/sandboxes/%s/files?path=%s", e.cfg.APIURL, sandboxID, "/workspace/uploads/"+filename)
	req, err := http.NewRequestWithContext(ctx, "POST", url, &buf)
	if err != nil {
		return fmt.Errorf("create upload request: %w", err)
	}
	if e.cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.cfg.APIKey)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("upload file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("nanobox upload file failed (%d): %s", resp.StatusCode, body)
	}

	e.logger.Info("uploaded file to nanobox sandbox", "sandbox", sandboxID, "file", filename, "size", len(data))
	return nil
}

func (e *nanoboxExecutor) StreamLogs(ctx context.Context, runID string) (<-chan string, error) {
	e.mu.RLock()
	sess, ok := e.sandboxes[runID]
	e.mu.RUnlock()

	sandboxID := runID
	if ok {
		sandboxID = sess.sandboxID
	}

	body, _, err := e.doStreamRequest(ctx, "GET", "/api/v1/sandboxes/"+sandboxID+"/logs?stream=true", nil)
	if err != nil {
		return nil, fmt.Errorf("stream logs: %w", err)
	}

	ch := make(chan string, 64)
	go func() {
		defer close(ch)
		defer body.Close()
		scanner := bufio.NewScanner(body)
		scanner.Buffer(make([]byte, 64*1024), 64*1024)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			case ch <- scanner.Text():
			}
		}
	}()

	return ch, nil
}

// getRuntime returns the runtime for the given name, falling back to default.
func (e *nanoboxExecutor) getRuntime(name string) runtime.Runtime {
	if name != "" {
		if rt := runtime.Get(name); rt != nil {
			return rt
		}
	}
	return runtime.Default()
}

// shellJoin quotes arguments for shell execution.
func shellJoin(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		if strings.ContainsAny(a, " \t\n\"'\\$`!#&|;(){}") {
			quoted[i] = "'" + strings.ReplaceAll(a, "'", "'\"'\"'") + "'"
		} else {
			quoted[i] = a
		}
	}
	return strings.Join(quoted, " ")
}
