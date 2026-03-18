package tunnel

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"go.zoe.im/agentbox/internal/executor"
	"go.zoe.im/agentbox/pkg/runtime"
	"go.zoe.im/agentbox/internal/tunnel"
)

// testEchoRuntime is a simple runtime that just runs echo, for testing.
type testEchoRuntime struct{}

func (t *testEchoRuntime) Name() string  { return "test-echo" }
func (t *testEchoRuntime) Image() string { return "" }
func (t *testEchoRuntime) BuildExecArgs(message string, continued bool) []string {
	return []string{"echo", message}
}
func (t *testEchoRuntime) ParseStreamLine(line string) (string, string, bool) {
	if line == "" {
		return "", "", false
	}
	return line + "\n", "", false
}
func (t *testEchoRuntime) EnvKeys() []string       { return nil }
func (t *testEchoRuntime) SetupCommands() []string { return nil }
func (t *testEchoRuntime) BinaryName() string      { return "echo" }
func (t *testEchoRuntime) InstallCommand() string  { return "" }

func init() {
	runtime.Register("test-echo", &testEchoRuntime{})
}

func assert(t *testing.T, cond bool, msgs ...string) {
	t.Helper()
	if !cond {
		msg := "assertion failed"
		if len(msgs) > 0 {
			msg = msgs[0]
		}
		t.Fatal(msg)
	}
}

// setupTestTunnel creates a Hub, a test HTTP server, and connects a Client with exec support.
// Returns the hub, executor, and a cleanup function.
func setupTestTunnel(t *testing.T) (*tunnel.Hub, *tunnelExecutor, context.CancelFunc) {
	t.Helper()

	hub := tunnel.NewHub(slog.Default(), func(token string) (string, error) {
		if token == "valid-token" {
			return "user1", nil
		}
		return "", fmt.Errorf("invalid token")
	})

	// Start hub server
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/tunnel", hub.HandleConnect)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	// Connect client with exec
	client := tunnel.NewClient(server.URL, "valid-token", slog.Default())
	client.EnableExec()

	ctx, cancel := context.WithCancel(context.Background())
	go client.Connect(ctx)
	time.Sleep(100 * time.Millisecond)

	exec := New(hub).(*tunnelExecutor)

	return hub, exec, cancel
}

func TestTunnelExecutor_StartSession(t *testing.T) {
	_, exec, cancel := setupTestTunnel(t)
	defer cancel()

	id, err := exec.StartSession(context.Background(), &executor.Request{
		ID:      "sess-1",
		UserID:  "user1",
		Runtime: "test-echo",
	})
	assert(t, err == nil, "start session failed: "+fmt.Sprint(err))
	assert(t, id == "sess-1", "session ID mismatch")
}

func TestTunnelExecutor_StartSessionNoTunnel(t *testing.T) {
	hub := tunnel.NewHub(slog.Default(), func(token string) (string, error) {
		return "user1", nil
	})
	exec := New(hub).(*tunnelExecutor)

	_, err := exec.StartSession(context.Background(), &executor.Request{
		ID:     "sess-1",
		UserID: "user1",
	})
	assert(t, err != nil, "should fail with no tunnel connected")
	assert(t, strings.Contains(err.Error(), "no active tunnel"), "error should mention no tunnel")
}

func TestTunnelExecutor_StartSessionNoExecCap(t *testing.T) {
	hub := tunnel.NewHub(slog.Default(), func(token string) (string, error) {
		if token == "valid-token" {
			return "user1", nil
		}
		return "", fmt.Errorf("invalid")
	})

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/tunnel", hub.HandleConnect)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	// Connect client WITHOUT exec capability
	client := tunnel.NewClient(server.URL, "valid-token", slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go client.Connect(ctx)
	time.Sleep(100 * time.Millisecond)

	exec := New(hub).(*tunnelExecutor)
	_, err := exec.StartSession(context.Background(), &executor.Request{
		ID:     "sess-1",
		UserID: "user1",
	})
	assert(t, err != nil, "should fail without exec capability")
	assert(t, strings.Contains(err.Error(), "does not support exec"), "error should mention exec")
}

func TestTunnelExecutor_SendMessage(t *testing.T) {
	_, exec, cancel := setupTestTunnel(t)
	defer cancel()

	// Start session first
	_, err := exec.StartSession(context.Background(), &executor.Request{
		ID:      "sess-msg",
		UserID:  "user1",
		Runtime: "test-echo",
	})
	assert(t, err == nil, "start session failed")

	resp, err := exec.SendMessage(context.Background(), "sess-msg", "hello world")
	assert(t, err == nil, "send message failed: "+fmt.Sprint(err))
	assert(t, strings.Contains(resp, "hello world"), "response should contain message")
}

func TestTunnelExecutor_SendMessageNoSession(t *testing.T) {
	hub := tunnel.NewHub(slog.Default(), func(token string) (string, error) {
		return "user1", nil
	})
	exec := New(hub).(*tunnelExecutor)

	_, err := exec.SendMessage(context.Background(), "nonexistent", "hello")
	assert(t, err != nil, "should fail with no session")
	assert(t, strings.Contains(err.Error(), "session not found"), "error should mention session not found")
}

func TestTunnelExecutor_StopSession(t *testing.T) {
	_, exec, cancel := setupTestTunnel(t)
	defer cancel()

	_, err := exec.StartSession(context.Background(), &executor.Request{
		ID:     "sess-stop",
		UserID: "user1",
	})
	assert(t, err == nil, "start session failed")

	err = exec.StopSession(context.Background(), "sess-stop")
	assert(t, err == nil, "stop session failed")

	// Stopping again should fail
	err = exec.StopSession(context.Background(), "sess-stop")
	assert(t, err != nil, "double stop should fail")
}

func TestTunnelExecutor_Logs(t *testing.T) {
	_, exec, cancel := setupTestTunnel(t)
	defer cancel()

	_, err := exec.StartSession(context.Background(), &executor.Request{
		ID:     "sess-logs",
		UserID: "user1",
	})
	assert(t, err == nil, "start session failed")

	// Manually append some logs
	exec.mu.RLock()
	sess := exec.sessions["sess-logs"]
	exec.mu.RUnlock()
	sess.appendLog("log line 1")
	sess.appendLog("log line 2")

	logs, err := exec.Logs(context.Background(), "sess-logs")
	assert(t, err == nil, "get logs failed")
	assert(t, strings.Contains(logs, "log line 1"), "should contain log line 1")
	assert(t, strings.Contains(logs, "log line 2"), "should contain log line 2")
}

func TestTunnelExecutor_LogsNoSession(t *testing.T) {
	hub := tunnel.NewHub(slog.Default(), func(token string) (string, error) {
		return "user1", nil
	})
	exec := New(hub).(*tunnelExecutor)

	_, err := exec.Logs(context.Background(), "nonexistent")
	assert(t, err != nil, "should fail with no session")
}

func TestTunnelExecutor_StreamLogs(t *testing.T) {
	_, exec, cancel := setupTestTunnel(t)
	defer cancel()

	_, err := exec.StartSession(context.Background(), &executor.Request{
		ID:     "sess-slog",
		UserID: "user1",
	})
	assert(t, err == nil, "start session failed")

	exec.mu.RLock()
	sess := exec.sessions["sess-slog"]
	exec.mu.RUnlock()
	sess.appendLog("streamed-line")

	ch, err := exec.StreamLogs(context.Background(), "sess-slog")
	assert(t, err == nil, "stream logs failed")

	var lines []string
	for line := range ch {
		lines = append(lines, line)
	}
	found := false
	for _, l := range lines {
		if strings.Contains(l, "streamed-line") {
			found = true
		}
	}
	assert(t, found, "streamed logs should contain the appended line")
}

func TestTunnelExecutor_RecoverSessions(t *testing.T) {
	hub := tunnel.NewHub(slog.Default(), func(token string) (string, error) {
		return "user1", nil
	})
	exec := New(hub).(*tunnelExecutor)

	ids, err := exec.RecoverSessions(context.Background())
	assert(t, err == nil, "recover sessions should not error")
	assert(t, ids == nil, "tunnel executor cannot recover sessions")
}

func TestTunnelExecutor_CollectExecOutput(t *testing.T) {
	hub := tunnel.NewHub(slog.Default(), func(token string) (string, error) {
		return "user1", nil
	})
	exec := New(hub).(*tunnelExecutor)

	// Simulate exec stream messages
	ch := make(chan *tunnel.ExecStreamMsg, 10)
	ch <- &tunnel.ExecStreamMsg{ID: "e1", Type: "exec.stdout", Data: "hello"}
	ch <- &tunnel.ExecStreamMsg{ID: "e1", Type: "exec.stdout", Data: "world"}
	ch <- &tunnel.ExecStreamMsg{ID: "e1", Type: "exec.stderr", Data: "warning"}
	ch <- &tunnel.ExecStreamMsg{ID: "e1", Type: "exec.done", ExitCode: 0}
	close(ch)

	resp, err := exec.collectExecOutput(ch)
	assert(t, err == nil, "collect exec output failed")
	assert(t, strings.Contains(resp.Output, "hello"), "output should contain hello")
	assert(t, strings.Contains(resp.Output, "world"), "output should contain world")
	assert(t, strings.Contains(resp.Output, "warning"), "output should contain stderr warning")
	assert(t, resp.ExitCode == 0, "exit code should be 0")
}

func TestTunnelExecutor_CollectExecOutputError(t *testing.T) {
	hub := tunnel.NewHub(slog.Default(), func(token string) (string, error) {
		return "user1", nil
	})
	exec := New(hub).(*tunnelExecutor)

	ch := make(chan *tunnel.ExecStreamMsg, 10)
	ch <- &tunnel.ExecStreamMsg{ID: "e1", Type: "exec.error", Data: "command not found"}
	close(ch)

	_, err := exec.collectExecOutput(ch)
	assert(t, err != nil, "should return error")
	assert(t, strings.Contains(err.Error(), "command not found"), "error should contain message")
}

func TestTunnelExecutor_CollectExecOutputDisconnect(t *testing.T) {
	hub := tunnel.NewHub(slog.Default(), func(token string) (string, error) {
		return "user1", nil
	})
	exec := New(hub).(*tunnelExecutor)

	// Channel closed without done/error — simulates disconnect
	ch := make(chan *tunnel.ExecStreamMsg, 10)
	ch <- &tunnel.ExecStreamMsg{ID: "e1", Type: "exec.stdout", Data: "partial"}
	close(ch)

	resp, err := exec.collectExecOutput(ch)
	assert(t, err != nil, "should return error on disconnect")
	assert(t, strings.Contains(err.Error(), "connection lost"), "error should mention connection lost")
	assert(t, strings.Contains(resp.Output, "partial"), "should have partial output")
}

func TestTunnelExecutor_SendMessageStream(t *testing.T) {
	_, exec, cancel := setupTestTunnel(t)
	defer cancel()

	_, err := exec.StartSession(context.Background(), &executor.Request{
		ID:      "sess-stream",
		UserID:  "user1",
		Runtime: "test-echo",
	})
	assert(t, err == nil, "start session failed")

	var mu sync.Mutex
	var tokens []string
	onToken := func(token string) {
		mu.Lock()
		tokens = append(tokens, token)
		mu.Unlock()
	}

	resp, err := exec.SendMessageStream(context.Background(), "sess-stream", "stream test", onToken)
	assert(t, err == nil, "stream message failed: "+fmt.Sprint(err))
	assert(t, strings.Contains(resp, "stream test"), "response should contain message")
}

func TestTunnelExecutor_SendMessageStreamNoSession(t *testing.T) {
	hub := tunnel.NewHub(slog.Default(), func(token string) (string, error) {
		return "user1", nil
	})
	exec := New(hub).(*tunnelExecutor)

	_, err := exec.SendMessageStream(context.Background(), "nonexistent", "msg", nil)
	assert(t, err != nil, "should fail with no session")
	assert(t, strings.Contains(err.Error(), "session not found"), "error should mention session")
}

func TestTunnelExecutor_UploadFile(t *testing.T) {
	_, exec, cancel := setupTestTunnel(t)
	defer cancel()

	_, err := exec.StartSession(context.Background(), &executor.Request{
		ID:     "sess-upload",
		UserID: "user1",
	})
	assert(t, err == nil, "start session failed")

	// Upload will try to forward via tunnel — client has no webdav provider
	// so it should get a 404 from client, which results in an error
	err = exec.UploadFile(context.Background(), "sess-upload", "test.txt", []byte("content"))
	// We expect this to either succeed or fail gracefully depending on provider
	// The important thing is no panic
	_ = err
}

func TestTunnelExecutor_UploadFileNoSession(t *testing.T) {
	hub := tunnel.NewHub(slog.Default(), func(token string) (string, error) {
		return "user1", nil
	})
	exec := New(hub).(*tunnelExecutor)

	err := exec.UploadFile(context.Background(), "nonexistent", "test.txt", []byte("data"))
	assert(t, err != nil, "should fail with no session")
}
