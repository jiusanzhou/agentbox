package local

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go.zoe.im/agentbox/internal/executor"
	"go.zoe.im/agentbox/internal/runtime"
)

// testRuntime is a minimal runtime for testing that runs a shell echo.
type testRuntime struct{}

func (t *testRuntime) Name() string  { return "test" }
func (t *testRuntime) Image() string { return "" }

func (t *testRuntime) BuildExecArgs(message string, continued bool) []string {
	if continued {
		return []string{"sh", "-c", "echo '[continued] " + message + "'"}
	}
	return []string{"sh", "-c", "echo '" + message + "'"}
}

func (t *testRuntime) ParseStreamLine(line string) (string, string, bool) {
	if line == "" {
		return "", "", false
	}
	return line + "\n", "", false
}

func (t *testRuntime) EnvKeys() []string       { return nil }
func (t *testRuntime) SetupCommands() []string { return nil }

func init() {
	runtime.Register("test", &testRuntime{})
}

func newTestExecutor(t *testing.T) *localExecutor {
	t.Helper()
	dir := t.TempDir()
	e, err := New(Config{WorkDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	return e.(*localExecutor)
}

func TestLocalExecutor_Execute(t *testing.T) {
	e := newTestExecutor(t)
	ctx := context.Background()

	resp, err := e.Execute(ctx, &executor.Request{
		ID:      "exec-1",
		Runtime: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.ExitCode != 0 {
		t.Fatalf("expected exit code 0, got %d", resp.ExitCode)
	}

	// One-shot should clean up workspace
	dir := filepath.Join(e.workDir, "exec-1")
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("workspace should be cleaned up after one-shot execute")
	}
}

func TestLocalExecutor_Execute_WithMessage(t *testing.T) {
	e := newTestExecutor(t)
	ctx := context.Background()

	resp, err := e.Execute(ctx, &executor.Request{
		ID:        "exec-2",
		Runtime:   "test",
		AgentFile: "hello world",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Output, "hello world") {
		t.Fatalf("expected output to contain 'hello world', got %q", resp.Output)
	}
}

func TestLocalExecutor_SessionLifecycle(t *testing.T) {
	e := newTestExecutor(t)
	ctx := context.Background()

	// Start session
	id, err := e.StartSession(ctx, &executor.Request{
		ID:      "sess-1",
		Runtime: "test",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id != "sess-1" {
		t.Fatalf("expected session id sess-1, got %s", id)
	}

	// Verify workspace exists
	dir := filepath.Join(e.workDir, "sess-1")
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		t.Fatal("session workspace should exist")
	}

	// Send first message
	resp, err := e.SendMessage(ctx, "sess-1", "hello")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp, "hello") {
		t.Fatalf("expected response to contain 'hello', got %q", resp)
	}

	// Send continued message
	resp, err = e.SendMessage(ctx, "sess-1", "world")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp, "[continued]") {
		t.Fatalf("expected continued flag in response, got %q", resp)
	}

	// Stop session
	if err := e.StopSession(ctx, "sess-1"); err != nil {
		t.Fatal(err)
	}

	// Verify workspace cleaned up
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatal("session workspace should be cleaned up after stop")
	}

	// Send to stopped session should fail
	_, err = e.SendMessage(ctx, "sess-1", "fail")
	if err == nil {
		t.Fatal("expected error sending to stopped session")
	}
}

func TestLocalExecutor_SendMessageStream(t *testing.T) {
	e := newTestExecutor(t)
	ctx := context.Background()

	_, err := e.StartSession(ctx, &executor.Request{
		ID:      "stream-1",
		Runtime: "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	var tokens []string
	result, err := e.SendMessageStream(ctx, "stream-1", "streaming test", func(token string) {
		tokens = append(tokens, token)
	})
	if err != nil {
		t.Fatal(err)
	}
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	if len(tokens) == 0 {
		t.Fatal("expected at least one streaming token")
	}

	_ = e.StopSession(ctx, "stream-1")
}

func TestLocalExecutor_UploadFile(t *testing.T) {
	e := newTestExecutor(t)
	ctx := context.Background()

	_, err := e.StartSession(ctx, &executor.Request{
		ID:      "upload-1",
		Runtime: "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	data := []byte("file content here")
	if err := e.UploadFile(ctx, "upload-1", "test.txt", data); err != nil {
		t.Fatal(err)
	}

	// Verify file exists
	path := filepath.Join(e.workDir, "upload-1", "uploads", "test.txt")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("uploaded file not found: %v", err)
	}
	if string(got) != "file content here" {
		t.Fatalf("uploaded file content mismatch: got %q", string(got))
	}

	_ = e.StopSession(ctx, "upload-1")
}

func TestLocalExecutor_UploadFile_NotFound(t *testing.T) {
	e := newTestExecutor(t)
	ctx := context.Background()

	err := e.UploadFile(ctx, "nonexistent", "test.txt", []byte("data"))
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestLocalExecutor_Logs(t *testing.T) {
	e := newTestExecutor(t)
	ctx := context.Background()

	_, err := e.StartSession(ctx, &executor.Request{
		ID:      "logs-1",
		Runtime: "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Send a message to generate logs
	_, _ = e.SendMessage(ctx, "logs-1", "log test")

	logs, err := e.Logs(ctx, "logs-1")
	if err != nil {
		t.Fatal(err)
	}
	if logs == "" {
		t.Fatal("expected non-empty logs")
	}

	_ = e.StopSession(ctx, "logs-1")
}

func TestLocalExecutor_RecoverSessions(t *testing.T) {
	e := newTestExecutor(t)

	ids, err := e.RecoverSessions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 0 {
		t.Fatalf("expected no recovered sessions, got %d", len(ids))
	}
}

func TestLocalExecutor_StreamLogs(t *testing.T) {
	e := newTestExecutor(t)
	ctx := context.Background()

	_, err := e.StartSession(ctx, &executor.Request{
		ID:      "slog-1",
		Runtime: "test",
	})
	if err != nil {
		t.Fatal(err)
	}

	_, _ = e.SendMessage(ctx, "slog-1", "hello logs")

	ch, err := e.StreamLogs(ctx, "slog-1")
	if err != nil {
		t.Fatal(err)
	}

	var lines []string
	for line := range ch {
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		t.Fatal("expected log lines from StreamLogs")
	}

	_ = e.StopSession(ctx, "slog-1")
}

func TestLocalExecutor_AgentFile(t *testing.T) {
	e := newTestExecutor(t)
	ctx := context.Background()

	_, err := e.StartSession(ctx, &executor.Request{
		ID:        "agent-1",
		Runtime:   "test",
		AgentFile: "You are a helpful assistant.",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify AGENTS.md exists
	path := filepath.Join(e.workDir, "agent-1", "AGENTS.md")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("AGENTS.md not found: %v", err)
	}
	if string(got) != "You are a helpful assistant." {
		t.Fatalf("AGENTS.md content mismatch: got %q", string(got))
	}

	_ = e.StopSession(ctx, "agent-1")
}

func TestLocalExecutor_Env(t *testing.T) {
	e := newTestExecutor(t)
	ctx := context.Background()

	// Use a runtime that echoes an env var
	runtime.Register("test-env", &envTestRuntime{})

	_, err := e.StartSession(ctx, &executor.Request{
		ID:      "env-1",
		Runtime: "test-env",
		Env:     map[string]string{"MY_TEST_VAR": "secret_value"},
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := e.SendMessage(ctx, "env-1", "check")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp, "secret_value") {
		t.Fatalf("expected env var in output, got %q", resp)
	}

	_ = e.StopSession(ctx, "env-1")
}

// envTestRuntime echoes the MY_TEST_VAR env variable.
type envTestRuntime struct{}

func (t *envTestRuntime) Name() string  { return "test-env" }
func (t *envTestRuntime) Image() string { return "" }
func (t *envTestRuntime) BuildExecArgs(message string, continued bool) []string {
	return []string{"sh", "-c", "echo $MY_TEST_VAR"}
}
func (t *envTestRuntime) ParseStreamLine(line string) (string, string, bool) {
	if line == "" {
		return "", "", false
	}
	return line + "\n", "", false
}
func (t *envTestRuntime) EnvKeys() []string       { return []string{"MY_TEST_VAR"} }
func (t *envTestRuntime) SetupCommands() []string { return nil }
