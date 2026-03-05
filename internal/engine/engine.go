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

	mu     sync.Mutex
	active map[string]context.CancelFunc
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
	}
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
