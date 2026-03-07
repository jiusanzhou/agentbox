package engine

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.zoe.im/agentbox/internal/executor/mock"
	"go.zoe.im/agentbox/internal/model"
	"go.zoe.im/agentbox/internal/store/memory"
)

func assert(t *testing.T, condition bool, msgs ...string) {
	t.Helper()
	if !condition {
		msg := "assertion failed"
		if len(msgs) > 0 {
			msg = msgs[0]
		}
		t.Fatal(msg)
	}
}

func TestEngine_SessionLifecycle(t *testing.T) {
	s := memory.New()
	exec := mock.New()
	eng := New(s, exec, nil)
	ctx := context.Background()

	// Start session
	run := &model.Run{
		ID:        "test-1",
		Name:      "test session",
		Mode:      model.RunModeSession,
		AgentFile: "Be brief.",
	}
	err := eng.StartSession(ctx, run)
	assert(t, err == nil, "start session should succeed")

	// Verify session is in store
	got, err := eng.Get(ctx, "test-1")
	assert(t, err == nil, "get should succeed")
	assert(t, got.Status == model.RunStatusRunning, "status should be running")
	assert(t, got.Mode == model.RunModeSession, "mode should be session")
	assert(t, got.StartedAt != nil, "started_at should be set")

	// Verify executor has the session
	assert(t, exec.IsStarted("test-1"), "executor should have session started")

	// Send message
	resp, err := eng.SendMessage(ctx, "test-1", "hello")
	assert(t, err == nil, "send message should succeed")
	assert(t, strings.Contains(resp, "mock response"), "response should contain mock response")

	// Send message stream
	var tokens []string
	streamResp, err := eng.SendMessageStream(ctx, "test-1", "stream hello", func(token string) {
		tokens = append(tokens, token)
	})
	assert(t, err == nil, "stream message should succeed")
	assert(t, strings.Contains(streamResp, "mock response"), "stream response should contain mock response")
	assert(t, len(tokens) > 0, "should have received tokens")

	// Stop session
	err = eng.StopSession(ctx, "test-1")
	assert(t, err == nil, "stop session should succeed")

	got, _ = eng.Get(ctx, "test-1")
	assert(t, got.Status == model.RunStatusCompleted, "status should be completed after stop")
	assert(t, got.EndedAt != nil, "ended_at should be set")

	// Executor should have session removed
	assert(t, !exec.IsStarted("test-1"), "executor should have session stopped")
}

func TestEngine_Submit(t *testing.T) {
	s := memory.New()
	exec := mock.New()
	eng := New(s, exec, nil)
	ctx := context.Background()

	run := &model.Run{
		ID:        "run-1",
		Name:      "one-shot",
		Mode:      model.RunModeRun,
		AgentFile: "Do something.",
	}
	err := eng.Submit(ctx, run)
	assert(t, err == nil, "submit should succeed")

	// Run is created in store
	got, err := eng.Get(ctx, "run-1")
	assert(t, err == nil, "get should succeed")
	assert(t, got.Status == model.RunStatusPending || got.Status == model.RunStatusRunning || got.Status == model.RunStatusCompleted,
		"status should be pending, running, or completed")

	// Wait for async execution to complete
	time.Sleep(100 * time.Millisecond)

	got, _ = eng.Get(ctx, "run-1")
	assert(t, got.Status == model.RunStatusCompleted, "should be completed after execution")
	assert(t, got.Result != nil, "result should be set")
	assert(t, got.Result.Output == "mock output", "result output should be mock output")
}

func TestEngine_List(t *testing.T) {
	s := memory.New()
	exec := mock.New()
	eng := New(s, exec, nil)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		eng.StartSession(ctx, &model.Run{
			ID:        "s" + string(rune('0'+i)),
			Name:      "session",
			Mode:      model.RunModeSession,
			AgentFile: "test",
		})
	}

	runs, err := eng.List(ctx, 10, 0)
	assert(t, err == nil, "list should succeed")
	assert(t, len(runs) == 3, "should have 3 runs")
}

func TestEngine_SendMessage_NotSession(t *testing.T) {
	s := memory.New()
	exec := mock.New()
	eng := New(s, exec, nil)
	ctx := context.Background()

	// Create a non-session run directly in store
	s.CreateRun(ctx, &model.Run{
		ID:     "run-1",
		Name:   "one-shot",
		Mode:   model.RunModeRun,
		Status: model.RunStatusRunning,
	})

	_, err := eng.SendMessage(ctx, "run-1", "hello")
	assert(t, err != nil, "send to non-session should fail")
}

func TestEngine_SendMessage_NotRunning(t *testing.T) {
	s := memory.New()
	exec := mock.New()
	eng := New(s, exec, nil)
	ctx := context.Background()

	// Create a completed session in store
	s.CreateRun(ctx, &model.Run{
		ID:     "s1",
		Name:   "done session",
		Mode:   model.RunModeSession,
		Status: model.RunStatusCompleted,
	})

	_, err := eng.SendMessage(ctx, "s1", "hello")
	assert(t, err != nil, "send to completed session should fail")
}

func TestEngine_SendMessage_NotFound(t *testing.T) {
	s := memory.New()
	exec := mock.New()
	eng := New(s, exec, nil)
	ctx := context.Background()

	_, err := eng.SendMessage(ctx, "nonexistent", "hello")
	assert(t, err != nil, "send to nonexistent session should fail")
}

func TestEngine_StopSession_NotSession(t *testing.T) {
	s := memory.New()
	exec := mock.New()
	eng := New(s, exec, nil)
	ctx := context.Background()

	s.CreateRun(ctx, &model.Run{
		ID:     "run-1",
		Name:   "one-shot",
		Mode:   model.RunModeRun,
		Status: model.RunStatusRunning,
	})

	err := eng.StopSession(ctx, "run-1")
	assert(t, err != nil, "stop non-session should fail")
}

func TestEngine_Cancel(t *testing.T) {
	s := memory.New()
	exec := mock.New()
	eng := New(s, exec, nil)

	// Cancel nonexistent should fail
	err := eng.Cancel("nonexistent")
	assert(t, err != nil, "cancel nonexistent should fail")
}

func TestEngine_UploadFile(t *testing.T) {
	s := memory.New()
	exec := mock.New()
	eng := New(s, exec, nil)
	ctx := context.Background()

	err := eng.UploadFile(ctx, "test-1", "hello.txt", []byte("hello"))
	assert(t, err == nil, "upload file should succeed")
}

func TestEngine_StreamLogs(t *testing.T) {
	s := memory.New()
	exec := mock.New()
	eng := New(s, exec, nil)
	ctx := context.Background()

	ch, err := eng.StreamLogs(ctx, "test-1")
	assert(t, err == nil, "stream logs should succeed")

	line := <-ch
	assert(t, line == "mock log line", "should receive mock log line")
}
