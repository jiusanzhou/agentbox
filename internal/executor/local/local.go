package local

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"go.zoe.im/agentbox/internal/executor"
	"go.zoe.im/agentbox/internal/runtime"
	"go.zoe.im/x"
)

// Config for local executor.
type Config struct {
	WorkDir string `json:"work_dir,omitempty" yaml:"work_dir"`
}

func init() {
	executor.Register("local", func(cfg x.TypedLazyConfig, opts ...any) (executor.Executor, error) {
		var c Config
		if len(cfg.Config) > 0 {
			if err := cfg.Unmarshal(&c); err != nil {
				return nil, err
			}
		}
		if c.WorkDir == "" {
			home, _ := os.UserHomeDir()
			c.WorkDir = filepath.Join(home, ".abox", "sessions")
		}
		return New(c)
	})
}

type localSession struct {
	runID   string
	workDir string
	runtime runtime.Runtime
	msgCnt  int
	env     map[string]string
	logBuf  bytes.Buffer
	logMu   sync.Mutex
}

type localExecutor struct {
	workDir  string
	logger   *slog.Logger
	sessions map[string]*localSession
	mu       sync.RWMutex
}

// New creates a local executor.
func New(cfg Config) (executor.Executor, error) {
	if err := os.MkdirAll(cfg.WorkDir, 0755); err != nil {
		return nil, fmt.Errorf("create work dir: %w", err)
	}
	return &localExecutor{
		workDir:  cfg.WorkDir,
		logger:   slog.Default(),
		sessions: make(map[string]*localSession),
	}, nil
}

func (e *localExecutor) Execute(ctx context.Context, req *executor.Request) (*executor.Response, error) {
	rt := e.getRuntime(req.Runtime)
	dir := filepath.Join(e.workDir, req.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}
	defer os.RemoveAll(dir)

	if req.AgentFile != "" {
		_ = os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(req.AgentFile), 0644)
	}

	args := rt.BuildExecArgs(req.AgentFile, false)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = buildEnv(req.Env)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	e.logger.Info("executing local process", "runtime", rt.Name(), "dir", dir)

	err := cmd.Run()

	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n--- stderr ---\n" + stderr.String()
	}

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
		}
	}

	return &executor.Response{
		ExitCode: exitCode,
		Output:   output,
		Logs:     output,
	}, nil
}

func (e *localExecutor) Logs(_ context.Context, id string) (string, error) {
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

func (e *localExecutor) Stop(ctx context.Context, id string) error {
	return e.StopSession(ctx, id)
}

func (e *localExecutor) StartSession(_ context.Context, req *executor.Request) (string, error) {
	rt := e.getRuntime(req.Runtime)
	dir := filepath.Join(e.workDir, req.ID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("create session dir: %w", err)
	}

	if req.AgentFile != "" {
		_ = os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(req.AgentFile), 0644)
	}

	sess := &localSession{
		runID:   req.ID,
		workDir: dir,
		runtime: rt,
		env:     req.Env,
	}

	e.mu.Lock()
	e.sessions[req.ID] = sess
	e.mu.Unlock()

	e.logger.Info("local session started", "id", req.ID, "runtime", rt.Name(), "dir", dir)
	return req.ID, nil
}

func (e *localExecutor) SendMessage(ctx context.Context, id string, message string) (string, error) {
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

	cmd := exec.CommandContext(ctx, filtered[0], filtered[1:]...)
	cmd.Dir = sess.workDir
	cmd.Env = buildEnv(sess.env)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	e.logger.Info("sending message to local session", "id", id, "runtime", rt.Name())

	if err := cmd.Run(); err != nil {
		output := stdout.String()
		if stderr.Len() > 0 {
			output += "\n" + stderr.String()
		}
		sess.appendLog(output)
		return output, fmt.Errorf("exec: %w", err)
	}

	output := strings.TrimSpace(stdout.String())
	sess.appendLog(output)
	sess.msgCnt++
	return output, nil
}

func (e *localExecutor) SendMessageStream(ctx context.Context, id string, message string, onToken executor.TokenCallback) (string, error) {
	e.mu.RLock()
	sess, ok := e.sessions[id]
	e.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("session not found: %s", id)
	}

	rt := sess.runtime
	args := rt.BuildExecArgs(message, sess.msgCnt > 0)

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = sess.workDir
	cmd.Env = buildEnv(sess.env)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}

	e.logger.Info("streaming message to local session", "id", id, "runtime", rt.Name())

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("start: %w", err)
	}

	var fullResponse strings.Builder
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
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

	_ = cmd.Wait()
	sess.appendLog(fullResponse.String())
	sess.msgCnt++
	return fullResponse.String(), nil
}

func (e *localExecutor) StopSession(_ context.Context, id string) error {
	e.mu.Lock()
	sess, ok := e.sessions[id]
	delete(e.sessions, id)
	e.mu.Unlock()

	if !ok {
		return fmt.Errorf("session not found: %s", id)
	}

	e.logger.Info("stopping local session", "id", id)

	// Clean up workspace
	_ = os.RemoveAll(sess.workDir)
	return nil
}

func (e *localExecutor) RecoverSessions(_ context.Context) ([]string, error) {
	// Local processes don't survive server restarts
	return nil, nil
}

func (e *localExecutor) UploadFile(_ context.Context, runID string, filename string, data []byte) error {
	e.mu.RLock()
	sess, ok := e.sessions[runID]
	e.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session not found: %s", runID)
	}

	uploadDir := filepath.Join(sess.workDir, "uploads")
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		return fmt.Errorf("create upload dir: %w", err)
	}
	return os.WriteFile(filepath.Join(uploadDir, filepath.Base(filename)), data, 0644)
}

func (e *localExecutor) StreamLogs(_ context.Context, id string) (<-chan string, error) {
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

func (e *localExecutor) getRuntime(name string) runtime.Runtime {
	if name != "" {
		if rt := runtime.Get(name); rt != nil {
			return rt
		}
	}
	return runtime.Default()
}

func buildEnv(env map[string]string) []string {
	result := os.Environ()
	for k, v := range env {
		result = append(result, k+"="+v)
	}
	return result
}

func (s *localSession) appendLog(data string) {
	s.logMu.Lock()
	defer s.logMu.Unlock()
	s.logBuf.WriteString(data)
	s.logBuf.WriteByte('\n')
}
