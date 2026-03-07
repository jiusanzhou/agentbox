package e2b

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"go.zoe.im/agentbox/internal/executor"
	"go.zoe.im/agentbox/internal/runtime"
	"go.zoe.im/x"
)

// Config for E2B cloud sandbox executor.
type Config struct {
	APIKey     string `json:"api_key" yaml:"api_key"`
	TemplateID string `json:"template_id" yaml:"template_id"` // default: "base"
	BaseURL    string `json:"base_url" yaml:"base_url"`       // default: "https://api.e2b.dev"
	TimeoutMs  int    `json:"timeout_ms" yaml:"timeout_ms"`   // sandbox lifetime, default: 300000 (5min)
}

func init() {
	executor.Register("e2b", func(cfg x.TypedLazyConfig, opts ...any) (executor.Executor, error) {
		var c Config
		if len(cfg.Config) > 0 {
			if err := cfg.Unmarshal(&c); err != nil {
				return nil, err
			}
		}
		if c.BaseURL == "" {
			c.BaseURL = "https://api.e2b.dev"
		}
		if c.TemplateID == "" {
			c.TemplateID = "base"
		}
		if c.TimeoutMs == 0 {
			c.TimeoutMs = 300000
		}
		return &e2bExecutor{
			cfg:      c,
			client:   &http.Client{},
			sessions: make(map[string]*e2bSession),
			logger:   slog.Default(),
		}, nil
	})
}

type e2bSession struct {
	sandboxID string
	runID     string
	runtime   runtime.Runtime
	msgCnt    int
	env       map[string]string
	logBuf    bytes.Buffer
	logMu     sync.Mutex
}

func (s *e2bSession) appendLog(data string) {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	s.logBuf.WriteString(data)
	s.logBuf.WriteByte('\n')
}

type e2bExecutor struct {
	cfg      Config
	client   *http.Client
	sessions map[string]*e2bSession
	mu       sync.RWMutex
	logger   *slog.Logger
}

// --- E2B API client methods ---

// doRequest makes an authenticated request to the E2B API.
func (e *e2bExecutor) doRequest(ctx context.Context, method, path string, body interface{}) ([]byte, int, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, e.cfg.BaseURL+path, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("X-API-Key", e.cfg.APIKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	return data, resp.StatusCode, nil
}

// createSandbox creates a new E2B sandbox and returns its ID.
func (e *e2bExecutor) createSandbox(ctx context.Context, envs map[string]string, metadata map[string]string) (string, error) {
	payload := map[string]interface{}{
		"templateID": e.cfg.TemplateID,
		"timeoutMs":  e.cfg.TimeoutMs,
	}
	if len(envs) > 0 {
		payload["envs"] = envs
	}
	if len(metadata) > 0 {
		payload["metadata"] = metadata
	}

	data, status, err := e.doRequest(ctx, "POST", "/sandboxes", payload)
	if err != nil {
		return "", fmt.Errorf("create sandbox: %w", err)
	}
	if status >= 400 {
		return "", fmt.Errorf("e2b create sandbox failed (%d): %s", status, data)
	}

	var result struct {
		SandboxID string `json:"sandboxID"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("parse sandbox response: %w", err)
	}
	return result.SandboxID, nil
}

// runCommand executes a command in a sandbox and returns stdout, stderr, exit code.
func (e *e2bExecutor) runCommand(ctx context.Context, sandboxID string, cmd string, envs map[string]string) (string, string, int, error) {
	payload := map[string]interface{}{
		"cmd":       cmd,
		"timeoutMs": 120000,
	}
	if len(envs) > 0 {
		payload["envs"] = envs
	}

	data, status, err := e.doRequest(ctx, "POST", "/sandboxes/"+sandboxID+"/commands", payload)
	if err != nil {
		return "", "", -1, fmt.Errorf("run command: %w", err)
	}
	if status >= 400 {
		return "", "", -1, fmt.Errorf("e2b run command failed (%d): %s", status, data)
	}

	var result struct {
		Stdout   string `json:"stdout"`
		Stderr   string `json:"stderr"`
		ExitCode int    `json:"exitCode"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", "", -1, fmt.Errorf("parse command response: %w", err)
	}
	return result.Stdout, result.Stderr, result.ExitCode, nil
}

// writeFile writes content to a file inside the sandbox.
func (e *e2bExecutor) writeFile(ctx context.Context, sandboxID string, path string, content []byte) error {
	req, err := http.NewRequestWithContext(ctx, "POST", e.cfg.BaseURL+"/sandboxes/"+sandboxID+"/files", bytes.NewReader(content))
	if err != nil {
		return fmt.Errorf("create write request: %w", err)
	}
	req.Header.Set("X-API-Key", e.cfg.APIKey)
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("X-Path", path)

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("e2b write file failed (%d): %s", resp.StatusCode, data)
	}
	return nil
}

// deleteSandbox stops and removes a sandbox.
func (e *e2bExecutor) deleteSandbox(ctx context.Context, sandboxID string) error {
	_, status, err := e.doRequest(ctx, "DELETE", "/sandboxes/"+sandboxID, nil)
	if err != nil {
		return fmt.Errorf("delete sandbox: %w", err)
	}
	if status >= 400 {
		return fmt.Errorf("e2b delete sandbox failed: %d", status)
	}
	return nil
}

// --- Executor interface implementation ---

func (e *e2bExecutor) Execute(ctx context.Context, req *executor.Request) (*executor.Response, error) {
	sandboxID, err := e.createSandbox(ctx, req.Env, map[string]string{"abox_run_id": req.ID})
	if err != nil {
		return nil, err
	}
	defer e.deleteSandbox(ctx, sandboxID)

	rt := e.getRuntime(req.Runtime)

	if req.AgentFile != "" {
		if err := e.writeFile(ctx, sandboxID, "/workspace/AGENTS.md", []byte(req.AgentFile)); err != nil {
			return nil, fmt.Errorf("write agent file: %w", err)
		}
	}

	args := rt.BuildExecArgs(req.AgentFile, false)
	cmd := shellJoin(args)

	e.logger.Info("executing in e2b sandbox", "runtime", rt.Name(), "sandbox", sandboxID)

	stdout, stderr, exitCode, err := e.runCommand(ctx, sandboxID, cmd, nil)
	if err != nil {
		return nil, err
	}

	output := stdout
	if stderr != "" {
		output += "\n--- stderr ---\n" + stderr
	}

	return &executor.Response{
		ExitCode: exitCode,
		Output:   output,
		Logs:     output,
	}, nil
}

func (e *e2bExecutor) StartSession(ctx context.Context, req *executor.Request) (string, error) {
	sandboxID, err := e.createSandbox(ctx, req.Env, map[string]string{
		"abox_run_id": req.ID,
		"mode":        "session",
	})
	if err != nil {
		return "", err
	}

	rt := e.getRuntime(req.Runtime)

	if req.AgentFile != "" {
		if err := e.writeFile(ctx, sandboxID, "/workspace/AGENTS.md", []byte(req.AgentFile)); err != nil {
			_ = e.deleteSandbox(ctx, sandboxID)
			return "", fmt.Errorf("write agent file: %w", err)
		}
	}

	for _, cmd := range rt.SetupCommands() {
		if _, _, _, err := e.runCommand(ctx, sandboxID, cmd, nil); err != nil {
			e.logger.Warn("setup command failed", "cmd", cmd, "err", err)
		}
	}

	sess := &e2bSession{
		sandboxID: sandboxID,
		runID:     req.ID,
		runtime:   rt,
		env:       req.Env,
	}

	e.mu.Lock()
	e.sessions[req.ID] = sess
	e.mu.Unlock()

	e.logger.Info("e2b session started", "id", req.ID, "sandbox", sandboxID, "runtime", rt.Name())
	return req.ID, nil
}

func (e *e2bExecutor) SendMessage(ctx context.Context, id string, message string) (string, error) {
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

	cmd := shellJoin(filtered)

	e.logger.Info("sending message to e2b session", "id", id, "runtime", rt.Name())

	stdout, stderr, _, err := e.runCommand(ctx, sess.sandboxID, cmd, nil)
	if err != nil {
		output := stdout
		if stderr != "" {
			output += "\n" + stderr
		}
		sess.appendLog(output)
		return output, fmt.Errorf("exec: %w", err)
	}

	output := strings.TrimSpace(stdout)
	sess.appendLog(output)
	sess.msgCnt++
	return output, nil
}

func (e *e2bExecutor) SendMessageStream(ctx context.Context, id string, message string, onToken executor.TokenCallback) (string, error) {
	e.mu.RLock()
	sess, ok := e.sessions[id]
	e.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("session not found: %s", id)
	}

	rt := sess.runtime
	args := rt.BuildExecArgs(message, sess.msgCnt > 0)
	cmd := shellJoin(args)

	e.logger.Info("streaming message to e2b session", "id", id, "runtime", rt.Name())

	stdout, _, _, err := e.runCommand(ctx, sess.sandboxID, cmd, nil)
	if err != nil {
		return "", err
	}

	// Parse output line by line through the runtime, matching local executor logic.
	var fullResponse strings.Builder
	for _, line := range strings.Split(stdout, "\n") {
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

	sess.appendLog(fullResponse.String())
	sess.msgCnt++
	return fullResponse.String(), nil
}

func (e *e2bExecutor) StopSession(ctx context.Context, id string) error {
	e.mu.Lock()
	sess, ok := e.sessions[id]
	delete(e.sessions, id)
	e.mu.Unlock()

	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	e.logger.Info("stopping e2b session", "id", id, "sandbox", sess.sandboxID)
	return e.deleteSandbox(ctx, sess.sandboxID)
}

func (e *e2bExecutor) Stop(ctx context.Context, id string) error {
	return e.StopSession(ctx, id)
}

func (e *e2bExecutor) RecoverSessions(ctx context.Context) ([]string, error) {
	data, _, err := e.doRequest(ctx, "GET", "/sandboxes", nil)
	if err != nil {
		return nil, err
	}

	var sandboxes []struct {
		SandboxID string            `json:"sandboxID"`
		Metadata  map[string]string `json:"metadata"`
	}
	if err := json.Unmarshal(data, &sandboxes); err != nil {
		return nil, fmt.Errorf("parse sandboxes list: %w", err)
	}

	var ids []string
	for _, s := range sandboxes {
		if runID, ok := s.Metadata["abox_run_id"]; ok {
			ids = append(ids, runID)
		}
	}
	return ids, nil
}

func (e *e2bExecutor) Logs(_ context.Context, id string) (string, error) {
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

func (e *e2bExecutor) StreamLogs(_ context.Context, id string) (<-chan string, error) {
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

func (e *e2bExecutor) UploadFile(ctx context.Context, runID string, filename string, data []byte) error {
	e.mu.RLock()
	sess, ok := e.sessions[runID]
	e.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session not found: %s", runID)
	}
	return e.writeFile(ctx, sess.sandboxID, "/workspace/uploads/"+filename, data)
}

func (e *e2bExecutor) getRuntime(name string) runtime.Runtime {
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
