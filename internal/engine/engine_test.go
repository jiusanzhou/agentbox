package engine

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"go.zoe.im/agentbox/internal/executor"
	"go.zoe.im/agentbox/internal/model"
)

// ---------------------------------------------------------------------------
// Mock store (in-memory, no external deps)
// ---------------------------------------------------------------------------

type mockStore struct {
	mu   sync.Mutex
	runs map[string]*model.Run
}

func newMockStore() *mockStore {
	return &mockStore{runs: make(map[string]*model.Run)}
}

func (m *mockStore) CreateRun(_ context.Context, r *model.Run) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runs[r.ID] = r
	return nil
}
func (m *mockStore) GetRun(_ context.Context, id string) (*model.Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	r, ok := m.runs[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return r, nil
}
func (m *mockStore) UpdateRun(_ context.Context, r *model.Run) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.runs[r.ID] = r
	return nil
}
func (m *mockStore) ListRuns(_ context.Context, limit, offset int) ([]*model.Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []*model.Run
	for _, r := range m.runs {
		out = append(out, r)
	}
	if offset > len(out) {
		return nil, nil
	}
	out = out[offset:]
	if limit > 0 && limit < len(out) {
		out = out[:limit]
	}
	return out, nil
}
func (m *mockStore) DeleteRun(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.runs, id)
	return nil
}

// Unused Store interface methods -- stubs so we can pass *mockStore where
// store.Store is needed indirectly through engine.New (engine only uses run methods).
func (m *mockStore) CreateUser(context.Context, *model.User) error   { return nil }
func (m *mockStore) GetUser(context.Context, string) (*model.User, error) {
	return nil, errors.New("not implemented")
}
func (m *mockStore) GetUserByEmail(context.Context, string) (*model.User, error) {
	return nil, errors.New("not implemented")
}
func (m *mockStore) GetUserByAPIKey(context.Context, string) (*model.User, error) {
	return nil, errors.New("not implemented")
}
func (m *mockStore) UpdateUser(context.Context, *model.User) error { return nil }
func (m *mockStore) CreateIntegration(context.Context, *model.Integration) error { return nil }
func (m *mockStore) GetIntegration(context.Context, string) (*model.Integration, error) {
	return nil, errors.New("not implemented")
}
func (m *mockStore) ListIntegrations(context.Context, string) ([]*model.Integration, error) {
	return nil, nil
}
func (m *mockStore) UpdateIntegration(context.Context, *model.Integration) error { return nil }
func (m *mockStore) DeleteIntegration(context.Context, string) error             { return nil }
func (m *mockStore) ListAllEnabledIntegrations(context.Context) ([]*model.Integration, error) {
	return nil, nil
}
func (m *mockStore) CreateAgentDNA(context.Context, *model.AgentDNA) error { return nil }
func (m *mockStore) GetAgentDNA(context.Context, string) (*model.AgentDNA, error) {
	return nil, errors.New("not implemented")
}
func (m *mockStore) GetAgentDNABySlug(context.Context, string) (*model.AgentDNA, error) {
	return nil, errors.New("not implemented")
}
func (m *mockStore) UpdateAgentDNA(context.Context, *model.AgentDNA) error { return nil }
func (m *mockStore) DeleteAgentDNA(context.Context, string) error          { return nil }
func (m *mockStore) ListAgentDNAs(context.Context, model.AgentDNAListOptions) ([]*model.AgentDNA, error) {
	return nil, nil
}
func (m *mockStore) IncrementAgentDNADownloads(context.Context, string) error { return nil }
func (m *mockStore) CreateSubscription(context.Context, *model.Subscription) error { return nil }
func (m *mockStore) GetSubscription(context.Context, string) (*model.Subscription, error) {
	return nil, errors.New("not implemented")
}
func (m *mockStore) GetActiveSubscription(context.Context, string, string) (*model.Subscription, error) {
	return nil, errors.New("not implemented")
}
func (m *mockStore) GetSubscriptionByStripeSubID(context.Context, string) (*model.Subscription, error) {
	return nil, errors.New("not implemented")
}
func (m *mockStore) UpdateSubscription(context.Context, *model.Subscription) error { return nil }
func (m *mockStore) ListSubscriptions(context.Context, string) ([]*model.Subscription, error) {
	return nil, nil
}
func (m *mockStore) CreateUsageRecord(context.Context, *model.UsageRecord) error { return nil }
func (m *mockStore) GetUsageSummary(context.Context, string, string, string) (*model.UsageSummary, error) {
	return nil, errors.New("not implemented")
}
func (m *mockStore) ListUsageRecords(context.Context, model.BillingListOptions) ([]*model.UsageRecord, error) {
	return nil, nil
}
func (m *mockStore) CreateAuthorPayout(context.Context, *model.AuthorPayout) error { return nil }
func (m *mockStore) GetAuthorPayout(context.Context, string, string) (*model.AuthorPayout, error) {
	return nil, errors.New("not implemented")
}
func (m *mockStore) ListAuthorPayouts(context.Context, string) ([]*model.AuthorPayout, error) {
	return nil, nil
}
func (m *mockStore) UpsertRunCostBreakdown(context.Context, *model.RunCostBreakdown) error {
	return nil
}
func (m *mockStore) GetRunCostBreakdown(context.Context, string) (*model.RunCostBreakdown, error) {
	return nil, errors.New("not implemented")
}
func (m *mockStore) UpsertStripeCustomer(context.Context, *model.StripeCustomer) error { return nil }
func (m *mockStore) GetStripeCustomer(context.Context, string) (*model.StripeCustomer, error) {
	return nil, errors.New("not implemented")
}
func (m *mockStore) GetFreeQuotaUsage(context.Context, string, string, string) (*model.FreeQuotaUsage, error) {
	return nil, errors.New("not implemented")
}
func (m *mockStore) IncrementFreeQuotaUsage(context.Context, string, string, string, int64) error {
	return nil
}
func (m *mockStore) CreateIMBinding(context.Context, *model.IMBinding) error { return nil }
func (m *mockStore) GetIMBindingByPlatform(context.Context, string, string) (*model.IMBinding, error) {
	return nil, errors.New("not implemented")
}
func (m *mockStore) ListIMBindingsByUser(context.Context, string) ([]*model.IMBinding, error) {
	return nil, nil
}
func (m *mockStore) DeleteIMBinding(context.Context, string) error { return nil }
func (m *mockStore) CreateBindingCode(context.Context, *model.BindingCode) error {
	return nil
}
func (m *mockStore) GetBindingCode(context.Context, string) (*model.BindingCode, error) {
	return nil, errors.New("not implemented")
}
func (m *mockStore) DeleteBindingCode(context.Context, string) error { return nil }

// --- Workflow stubs ---
func (m *mockStore) CreateWorkflow(context.Context, *model.Workflow) error   { return nil }
func (m *mockStore) GetWorkflow(context.Context, string) (*model.Workflow, error) {
	return nil, errors.New("not implemented")
}
func (m *mockStore) ListWorkflows(context.Context, string) ([]*model.Workflow, error) {
	return nil, nil
}
func (m *mockStore) UpdateWorkflow(context.Context, *model.Workflow) error { return nil }
func (m *mockStore) DeleteWorkflow(context.Context, string) error          { return nil }
func (m *mockStore) CreateWorkflowRun(context.Context, *model.WorkflowRun) error {
	return nil
}
func (m *mockStore) GetWorkflowRun(context.Context, string) (*model.WorkflowRun, error) {
	return nil, errors.New("not implemented")
}
func (m *mockStore) UpdateWorkflowRun(context.Context, *model.WorkflowRun) error { return nil }
func (m *mockStore) ListWorkflowRuns(context.Context, string) ([]*model.WorkflowRun, error) {
	return nil, nil
}

// ---------------------------------------------------------------------------
// Mock executor (in-memory, no external deps)
// ---------------------------------------------------------------------------

type mockExecutor struct {
	mu        sync.Mutex
	started   map[string]bool
	failStart bool // when true, StartSession returns an error
	blockExec chan struct{} // if non-nil, Execute blocks until this is closed or ctx is done
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{started: make(map[string]bool)}
}

func (e *mockExecutor) Execute(ctx context.Context, req *executor.Request) (*executor.Response, error) {
	if e.blockExec != nil {
		select {
		case <-e.blockExec:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return &executor.Response{Output: "mock output", ExitCode: 0}, nil
}
func (e *mockExecutor) Logs(_ context.Context, id string) (string, error) {
	return "mock logs", nil
}
func (e *mockExecutor) Stop(_ context.Context, id string) error { return nil }

func (e *mockExecutor) StartSession(_ context.Context, req *executor.Request) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.failStart {
		return "", errors.New("start failed")
	}
	e.started[req.ID] = true
	return req.ID, nil
}
func (e *mockExecutor) SendMessage(_ context.Context, id, message string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.started[id] {
		return "", fmt.Errorf("session %s not started", id)
	}
	return "mock response to: " + message, nil
}
func (e *mockExecutor) SendMessageStream(_ context.Context, id, message string, cb executor.TokenCallback) (string, error) {
	e.mu.Lock()
	if !e.started[id] {
		e.mu.Unlock()
		return "", fmt.Errorf("session %s not started", id)
	}
	e.mu.Unlock()
	resp := "mock response to: " + message
	if cb != nil {
		cb(resp)
	}
	return resp, nil
}
func (e *mockExecutor) StopSession(_ context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.started, id)
	return nil
}
func (e *mockExecutor) RecoverSessions(_ context.Context) ([]string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	var ids []string
	for id := range e.started {
		ids = append(ids, id)
	}
	return ids, nil
}
func (e *mockExecutor) UploadFile(_ context.Context, _, _ string, _ []byte) error { return nil }
func (e *mockExecutor) StreamLogs(_ context.Context, _ string) (<-chan string, error) {
	ch := make(chan string, 1)
	ch <- "mock log line"
	close(ch)
	return ch, nil
}
func (e *mockExecutor) isStarted(id string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.started[id]
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestSubmit(t *testing.T) {
	s := newMockStore()
	exec := newMockExecutor()
	eng := New(s, exec, nil)
	ctx := context.Background()

	run := &model.Run{
		ID:        "run-1",
		Name:      "one-shot",
		Mode:      model.RunModeRun,
		AgentFile: "Do something.",
	}
	if err := eng.Submit(ctx, run); err != nil {
		t.Fatalf("Submit: %v", err)
	}

	got, err := s.GetRun(ctx, "run-1")
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.Status != model.RunStatusPending && got.Status != model.RunStatusRunning && got.Status != model.RunStatusCompleted {
		t.Errorf("status = %s, want pending|running|completed", got.Status)
	}

	// Wait for the goroutine to finish.
	time.Sleep(150 * time.Millisecond)

	got, _ = s.GetRun(ctx, "run-1")
	if got.Status != model.RunStatusCompleted {
		t.Errorf("after execution: status = %s, want completed", got.Status)
	}
	if got.Result == nil || got.Result.Output != "mock output" {
		t.Errorf("result mismatch: %+v", got.Result)
	}
}

func TestCancel(t *testing.T) {
	t.Run("cancel non-active returns error", func(t *testing.T) {
		eng := New(newMockStore(), newMockExecutor(), nil)
		err := eng.Cancel("does-not-exist")
		if err == nil {
			t.Fatal("expected error for non-active run")
		}
		if !strings.Contains(err.Error(), "not active") {
			t.Errorf("error = %q, want 'not active'", err)
		}
	})

	t.Run("cancel active run succeeds", func(t *testing.T) {
		s := newMockStore()
		exec := newMockExecutor()
		exec.blockExec = make(chan struct{}) // block Execute until we cancel
		eng := New(s, exec, nil)
		ctx := context.Background()

		run := &model.Run{
			ID: "run-cancel", Name: "slow", Mode: model.RunModeRun, AgentFile: "x",
			Config: model.RunConfig{Timeout: 60},
		}
		_ = eng.Submit(ctx, run)
		// Give goroutine a moment to register the cancel func.
		time.Sleep(50 * time.Millisecond)

		if err := eng.Cancel("run-cancel"); err != nil {
			t.Fatalf("Cancel: %v", err)
		}
	})
}

func TestStartSession(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		s := newMockStore()
		exec := newMockExecutor()
		eng := New(s, exec, nil)
		ctx := context.Background()

		run := &model.Run{ID: "sess-1", Name: "session", AgentFile: "test"}
		if err := eng.StartSession(ctx, run); err != nil {
			t.Fatalf("StartSession: %v", err)
		}
		if run.Status != model.RunStatusRunning {
			t.Errorf("status = %s, want running", run.Status)
		}
		if run.Mode != model.RunModeSession {
			t.Errorf("mode = %s, want session", run.Mode)
		}
		if !exec.isStarted("sess-1") {
			t.Error("executor should have session started")
		}
	})

	t.Run("executor failure", func(t *testing.T) {
		s := newMockStore()
		exec := newMockExecutor()
		exec.failStart = true
		eng := New(s, exec, nil)
		ctx := context.Background()

		run := &model.Run{ID: "sess-fail", Name: "session", AgentFile: "test"}
		err := eng.StartSession(ctx, run)
		if err == nil {
			t.Fatal("expected error when executor fails")
		}
		got, _ := s.GetRun(ctx, "sess-fail")
		if got.Status != model.RunStatusFailed {
			t.Errorf("status = %s, want failed", got.Status)
		}
	})
}

func TestSendMessage(t *testing.T) {
	t.Run("to running session", func(t *testing.T) {
		s := newMockStore()
		exec := newMockExecutor()
		eng := New(s, exec, nil)
		ctx := context.Background()

		run := &model.Run{ID: "msg-1", Name: "s", AgentFile: "a"}
		_ = eng.StartSession(ctx, run)

		resp, err := eng.SendMessage(ctx, "msg-1", "hello")
		if err != nil {
			t.Fatalf("SendMessage: %v", err)
		}
		if !strings.Contains(resp, "mock response") {
			t.Errorf("response = %q, want mock response", resp)
		}
	})

	t.Run("to non-session run", func(t *testing.T) {
		s := newMockStore()
		exec := newMockExecutor()
		eng := New(s, exec, nil)
		ctx := context.Background()

		_ = s.CreateRun(ctx, &model.Run{
			ID: "run-1", Mode: model.RunModeRun, Status: model.RunStatusRunning,
		})
		_, err := eng.SendMessage(ctx, "run-1", "hello")
		if err == nil {
			t.Fatal("expected error for non-session")
		}
		if !strings.Contains(err.Error(), "not a session") {
			t.Errorf("error = %q, want 'not a session'", err)
		}
	})

	t.Run("to completed session", func(t *testing.T) {
		s := newMockStore()
		exec := newMockExecutor()
		eng := New(s, exec, nil)
		ctx := context.Background()

		_ = s.CreateRun(ctx, &model.Run{
			ID: "done-1", Mode: model.RunModeSession, Status: model.RunStatusCompleted,
		})
		_, err := eng.SendMessage(ctx, "done-1", "hello")
		if err == nil {
			t.Fatal("expected error for completed session")
		}
		if !strings.Contains(err.Error(), "not running") {
			t.Errorf("error = %q, want 'not running'", err)
		}
	})

	t.Run("to non-existent run", func(t *testing.T) {
		eng := New(newMockStore(), newMockExecutor(), nil)
		_, err := eng.SendMessage(context.Background(), "nope", "hello")
		if err == nil {
			t.Fatal("expected error for missing run")
		}
	})
}

func TestStopSession(t *testing.T) {
	t.Run("stop running session", func(t *testing.T) {
		s := newMockStore()
		exec := newMockExecutor()
		eng := New(s, exec, nil)
		ctx := context.Background()

		run := &model.Run{ID: "stop-1", Name: "s", AgentFile: "a"}
		_ = eng.StartSession(ctx, run)

		if err := eng.StopSession(ctx, "stop-1"); err != nil {
			t.Fatalf("StopSession: %v", err)
		}
		got, _ := s.GetRun(ctx, "stop-1")
		if got.Status != model.RunStatusCompleted {
			t.Errorf("status = %s, want completed", got.Status)
		}
		if got.EndedAt == nil {
			t.Error("ended_at should be set")
		}
		if exec.isStarted("stop-1") {
			t.Error("executor should not have session after stop")
		}
	})

	t.Run("stop non-session run", func(t *testing.T) {
		s := newMockStore()
		eng := New(s, newMockExecutor(), nil)
		ctx := context.Background()

		_ = s.CreateRun(ctx, &model.Run{
			ID: "run-2", Mode: model.RunModeRun, Status: model.RunStatusRunning,
		})
		err := eng.StopSession(ctx, "run-2")
		if err == nil {
			t.Fatal("expected error for non-session")
		}
	})
}

func TestRecoverSessions(t *testing.T) {
	t.Run("recovers alive sessions and marks interrupted", func(t *testing.T) {
		s := newMockStore()
		exec := newMockExecutor()
		eng := New(s, exec, nil)
		ctx := context.Background()

		// Simulate a session that is still running in the executor.
		exec.mu.Lock()
		exec.started["alive-1"] = true
		exec.mu.Unlock()

		now := time.Now()
		_ = s.CreateRun(ctx, &model.Run{
			ID: "alive-1", Mode: model.RunModeSession, Status: model.RunStatusRunning,
			StartedAt: &now,
		})
		// This one has no backing process.
		_ = s.CreateRun(ctx, &model.Run{
			ID: "orphan-1", Mode: model.RunModeSession, Status: model.RunStatusRunning,
			StartedAt: &now,
		})

		if err := eng.RecoverSessions(ctx); err != nil {
			t.Fatalf("RecoverSessions: %v", err)
		}

		alive, _ := s.GetRun(ctx, "alive-1")
		if alive.Status != model.RunStatusRunning {
			t.Errorf("alive status = %s, want running", alive.Status)
		}

		orphan, _ := s.GetRun(ctx, "orphan-1")
		if orphan.Status != model.RunStatusInterrupted {
			t.Errorf("orphan status = %s, want interrupted", orphan.Status)
		}
		if orphan.EndedAt == nil {
			t.Error("orphan ended_at should be set")
		}
	})
}

func TestSelectExecutor(t *testing.T) {
	defaultExec := newMockExecutor()
	tunnelExec := newMockExecutor()

	eng := New(newMockStore(), defaultExec, nil)
	eng.SetTunnelExecutor(tunnelExec)

	cases := []struct {
		name     string
		run      *model.Run
		wantExec *mockExecutor
	}{
		{
			name:     "explicit tunnel with tunnel executor set",
			run:      &model.Run{Executor: "tunnel", UserID: "u1"},
			wantExec: tunnelExec,
		},
		{
			name:     "explicit local",
			run:      &model.Run{Executor: "local"},
			wantExec: defaultExec,
		},
		{
			name:     "explicit docker",
			run:      &model.Run{Executor: "docker"},
			wantExec: defaultExec,
		},
		{
			name:     "explicit k8s",
			run:      &model.Run{Executor: "k8s"},
			wantExec: defaultExec,
		},
		{
			name:     "explicit kubernetes",
			run:      &model.Run{Executor: "kubernetes"},
			wantExec: defaultExec,
		},
		{
			name:     "auto-select with userID prefers tunnel",
			run:      &model.Run{UserID: "u1"},
			wantExec: tunnelExec,
		},
		{
			name:     "auto-select without userID falls back to default",
			run:      &model.Run{},
			wantExec: defaultExec,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := eng.selectExecutor(tc.run)
			// Compare by pointer identity.
			if got != tc.wantExec {
				t.Errorf("selectExecutor returned wrong executor")
			}
		})
	}

	t.Run("tunnel type without tunnel executor falls to default", func(t *testing.T) {
		eng2 := New(newMockStore(), defaultExec, nil)
		// No tunnel executor set.
		got := eng2.selectExecutor(&model.Run{Executor: "tunnel"})
		if got != defaultExec {
			t.Error("expected default executor when tunnel not set")
		}
	})
}

func TestCleanupExpiredSessions(t *testing.T) {
	s := newMockStore()
	exec := newMockExecutor()
	eng := New(s, exec, nil)
	ctx := context.Background()

	// Start a session that will look expired.
	run := &model.Run{ID: "old-1", Name: "s", AgentFile: "a"}
	_ = eng.StartSession(ctx, run)

	// Backdate the started_at to exceed TTL.
	got, _ := s.GetRun(ctx, "old-1")
	old := time.Now().Add(-2 * time.Hour)
	got.StartedAt = &old
	got.LastActivityAt = nil
	_ = s.UpdateRun(ctx, got)

	// Start a fresh session (should not be cleaned up).
	run2 := &model.Run{ID: "new-1", Name: "s", AgentFile: "a"}
	_ = eng.StartSession(ctx, run2)

	eng.cleanupExpiredSessions(ctx, 1*time.Hour)

	cleaned, _ := s.GetRun(ctx, "old-1")
	if cleaned.Status != model.RunStatusCompleted {
		t.Errorf("old session status = %s, want completed", cleaned.Status)
	}

	fresh, _ := s.GetRun(ctx, "new-1")
	if fresh.Status != model.RunStatusRunning {
		t.Errorf("fresh session status = %s, want running", fresh.Status)
	}
}
