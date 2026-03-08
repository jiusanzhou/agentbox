package engine

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go.zoe.im/agentbox/internal/executor"
	"go.zoe.im/agentbox/internal/model"
	"go.zoe.im/agentbox/internal/store"
)

// Engine orchestrates agent run lifecycle.
type Engine struct {
	store    store.Store
	executor executor.Executor
	logger   *slog.Logger

	// Optional tunnel executor for user-local execution via tunnel.
	tunnelExec executor.Executor

	mu     sync.Mutex
	active map[string]context.CancelFunc

	// Track which executor owns each session.
	sessExecMu sync.RWMutex
	sessExec   map[string]executor.Executor
}

func New(s store.Store, e executor.Executor, logger *slog.Logger) *Engine {
	if logger == nil {
		logger = slog.Default()
	}
	return &Engine{
		store:    s,
		executor: e,
		logger:   logger,
		active:   make(map[string]context.CancelFunc),
		sessExec: make(map[string]executor.Executor),
	}
}

// SetTunnelExecutor sets the optional tunnel executor for user-local execution.
func (e *Engine) SetTunnelExecutor(exec executor.Executor) {
	e.tunnelExec = exec
}

// Submit creates a new run and starts execution.
func (e *Engine) Submit(ctx context.Context, run *model.Run) error {
	run.Status = model.RunStatusPending
	run.CreatedAt = time.Now()

	if err := e.store.CreateRun(ctx, run); err != nil {
		return fmt.Errorf("create run: %w", err)
	}

	go e.execute(run)
	return nil
}

func (e *Engine) execute(run *model.Run) {
	timeout := run.Config.Timeout
	if timeout == 0 {
		timeout = 3600
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	e.mu.Lock()
	e.active[run.ID] = cancel
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.active, run.ID)
		e.mu.Unlock()
	}()

	now := time.Now()
	run.Status = model.RunStatusRunning
	run.StartedAt = &now
	_ = e.store.UpdateRun(ctx, run)

	e.logger.Info("executing run", "id", run.ID, "name", run.Name)

	result, err := e.executor.Execute(ctx, &executor.Request{
		ID:        run.ID,
		UserID:    run.UserID,
		AgentFile: run.AgentFile,
		Image:     run.Config.Image,
		Env:       run.Config.Env,
		Timeout:   timeout,
	})

	end := time.Now()
	run.EndedAt = &end

	if err != nil {
		run.Status = model.RunStatusFailed
		run.Result = &model.Result{ExitCode: 1, Error: err.Error()}
		e.logger.Error("run failed", "id", run.ID, "err", err)
	} else {
		run.Status = model.RunStatusCompleted
		run.Result = &model.Result{
			ExitCode:  result.ExitCode,
			Output:    result.Output,
			Artifacts: result.Artifacts,
		}
		e.logger.Info("run completed", "id", run.ID, "exit_code", result.ExitCode)
	}

	_ = e.store.UpdateRun(context.Background(), run)
}

// Cancel aborts a running execution.
func (e *Engine) Cancel(id string) error {
	e.mu.Lock()
	cancel, ok := e.active[id]
	e.mu.Unlock()
	if !ok {
		return fmt.Errorf("run %s not active", id)
	}
	cancel()
	return nil
}

// Get returns a run by ID.
func (e *Engine) Get(ctx context.Context, id string) (*model.Run, error) {
	return e.store.GetRun(ctx, id)
}

// List returns runs with pagination.
func (e *Engine) List(ctx context.Context, limit, offset int) ([]*model.Run, error) {
	return e.store.ListRuns(ctx, limit, offset)
}

// selectExecutor picks the tunnel executor if available and the run has a UserID,
// otherwise falls back to the default executor.
func (e *Engine) selectExecutor(run *model.Run) executor.Executor {
	switch run.Executor {
	case "tunnel":
		if e.tunnelExec != nil {
			return e.tunnelExec
		}
	case "local":
		// "local" explicitly means the default executor (which is local when configured)
		return e.executor
	case "docker", "k8s", "kubernetes":
		// These all use the default executor (which should be docker/k8s)
		return e.executor
	default:
		// Auto-select: prefer tunnel if user has an active connection
		if e.tunnelExec != nil && run.UserID != "" {
			return e.tunnelExec
		}
	}
	return e.executor
}

// getSessionExecutor returns the executor that owns a session, defaulting to the main one.
func (e *Engine) getSessionExecutor(id string) executor.Executor {
	e.sessExecMu.RLock()
	exec, ok := e.sessExec[id]
	e.sessExecMu.RUnlock()
	if ok {
		return exec
	}
	return e.executor
}

// StartSession creates and starts a persistent session container.
func (e *Engine) StartSession(ctx context.Context, run *model.Run) error {
	run.Mode = model.RunModeSession
	run.Status = model.RunStatusPending
	run.CreatedAt = time.Now()

	if err := e.store.CreateRun(ctx, run); err != nil {
		return fmt.Errorf("create run: %w", err)
	}

	req := &executor.Request{
		ID:        run.ID,
		UserID:    run.UserID,
		AgentFile: run.AgentFile,
		Image:     run.Config.Image,
		Runtime:   run.Runtime,
		Env:       run.Config.Env,
	}

	exec := e.selectExecutor(run)

	_, err := exec.StartSession(ctx, req)
	if err != nil {
		// If tunnel executor fails, fall back to default executor
		if exec != e.executor {
			e.logger.Warn("tunnel executor failed, falling back to default", "id", run.ID, "err", err)
			exec = e.executor
			_, err = exec.StartSession(ctx, req)
		}
		if err != nil {
			run.Status = model.RunStatusFailed
			run.Result = &model.Result{ExitCode: 1, Error: err.Error()}
			_ = e.store.UpdateRun(ctx, run)
			return fmt.Errorf("start session: %w", err)
		}
	}

	// Track which executor owns this session
	e.sessExecMu.Lock()
	e.sessExec[run.ID] = exec
	e.sessExecMu.Unlock()

	now := time.Now()
	run.Status = model.RunStatusRunning
	run.StartedAt = &now
	_ = e.store.UpdateRun(ctx, run)

	e.logger.Info("session started", "id", run.ID)
	return nil
}

// SendMessage sends a message to a running session and returns the response.
func (e *Engine) SendMessage(ctx context.Context, runID string, message string) (string, error) {
	run, err := e.store.GetRun(ctx, runID)
	if err != nil {
		return "", fmt.Errorf("get run: %w", err)
	}
	if run.Mode != model.RunModeSession {
		return "", fmt.Errorf("run %s is not a session", runID)
	}
	if run.Status != model.RunStatusRunning {
		return "", fmt.Errorf("session %s is not running (status: %s)", runID, run.Status)
	}

	resp, err := e.getSessionExecutor(runID).SendMessage(ctx, runID, message)
	if err != nil {
		return "", err
	}

	now := time.Now()
	run.LastActivityAt = &now
	_ = e.store.UpdateRun(ctx, run)

	return resp, nil
}

// StopSession stops a running session container.
func (e *Engine) StopSession(ctx context.Context, runID string) error {
	run, err := e.store.GetRun(ctx, runID)
	if err != nil {
		return fmt.Errorf("get run: %w", err)
	}
	if run.Mode != model.RunModeSession {
		return fmt.Errorf("run %s is not a session", runID)
	}

	if err := e.getSessionExecutor(runID).StopSession(ctx, runID); err != nil {
		e.logger.Error("stop session failed", "id", runID, "err", err)
	}

	// Clean up session executor tracking
	e.sessExecMu.Lock()
	delete(e.sessExec, runID)
	e.sessExecMu.Unlock()

	now := time.Now()
	run.Status = model.RunStatusCompleted
	run.EndedAt = &now
	_ = e.store.UpdateRun(ctx, run)

	e.logger.Info("session stopped", "id", runID)
	return nil
}

// SendMessageStream sends a message and streams tokens via callback.
func (e *Engine) SendMessageStream(ctx context.Context, runID string, message string, onToken executor.TokenCallback) (string, error) {
	run, err := e.store.GetRun(ctx, runID)
	if err != nil {
		return "", fmt.Errorf("get run: %w", err)
	}
	if run.Mode != model.RunModeSession {
		return "", fmt.Errorf("run %s is not a session", runID)
	}
	if run.Status != model.RunStatusRunning {
		return "", fmt.Errorf("session %s is not running (status: %s)", runID, run.Status)
	}
	resp, err := e.getSessionExecutor(runID).SendMessageStream(ctx, runID, message, onToken)
	if err != nil {
		return "", err
	}

	now := time.Now()
	run.LastActivityAt = &now
	_ = e.store.UpdateRun(ctx, run)

	return resp, nil
}

// StartCleanup periodically removes expired sessions.
func (e *Engine) StartCleanup(ctx context.Context, ttl, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.cleanupExpiredSessions(ctx, ttl)
		}
	}
}

func (e *Engine) cleanupExpiredSessions(ctx context.Context, ttl time.Duration) {
	runs, err := e.store.ListRuns(ctx, 1000, 0)
	if err != nil {
		e.logger.Error("cleanup: list runs failed", "err", err)
		return
	}

	now := time.Now()
	for _, run := range runs {
		if run.Mode != model.RunModeSession || run.Status != model.RunStatusRunning {
			continue
		}
		if run.StartedAt == nil {
			continue
		}
		lastActive := run.StartedAt
		if run.LastActivityAt != nil {
			lastActive = run.LastActivityAt
		}
		if now.Sub(*lastActive) > ttl {
			e.logger.Info("cleaning up expired session", "id", run.ID, "age", now.Sub(*lastActive))
			_ = e.StopSession(ctx, run.ID)
		}
	}
}

// RecoverSessions scans for running containers/pods and reconciles with the store.
func (e *Engine) RecoverSessions(ctx context.Context) error {
	ids, err := e.executor.RecoverSessions(ctx)
	if err != nil {
		e.logger.Warn("failed to recover sessions", "err", err)
		return err
	}

	recovered := 0
	for _, id := range ids {
		run, err := e.store.GetRun(ctx, id)
		if err != nil {
			e.logger.Warn("orphan container found (no store record)", "id", id)
			continue
		}
		if run.Status != model.RunStatusRunning {
			run.Status = model.RunStatusRunning
			_ = e.store.UpdateRun(ctx, run)
		}
		recovered++
		e.logger.Info("recovered session", "id", id)
	}

	e.logger.Info("session recovery complete", "recovered", recovered)
	return nil
}

// UploadFile copies a file into a running session container.
func (e *Engine) UploadFile(ctx context.Context, runID string, filename string, data []byte) error {
	return e.getSessionExecutor(runID).UploadFile(ctx, runID, filename, data)
}

// StreamLogs streams container logs line by line.
func (e *Engine) StreamLogs(ctx context.Context, runID string) (<-chan string, error) {
	return e.getSessionExecutor(runID).StreamLogs(ctx, runID)
}
