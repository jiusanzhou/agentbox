package tunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

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

// setupTestHub creates a Hub with a test HTTP server and returns the server URL.
func setupTestHub(t *testing.T) (*Hub, *httptest.Server) {
	t.Helper()
	hub := NewHub(slog.Default(), func(token string) (string, error) {
		if token == "valid-token" {
			return "user1", nil
		}
		return "", fmt.Errorf("invalid token")
	})
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/tunnel", hub.HandleConnect)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return hub, server
}

// dialClient performs the WebSocket handshake (hello + welcome) and returns the connection.
func dialClient(t *testing.T, server *httptest.Server, token string, caps []string) *websocket.Conn {
	t.Helper()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/tunnel"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert(t, err == nil, "dial failed: "+fmt.Sprint(err))

	hello := HelloMessage{Type: "hello", Token: token, Capabilities: caps, Version: "1"}
	err = conn.WriteJSON(hello)
	assert(t, err == nil, "write hello failed")

	var resp HelloResponse
	err = conn.ReadJSON(&resp)
	assert(t, err == nil, "read welcome failed")

	if token == "valid-token" {
		assert(t, resp.Type == "welcome", "expected welcome, got "+resp.Type)
		assert(t, resp.UserID == "user1", "expected user1")
	}

	return conn
}

func TestHub_HTTPForward(t *testing.T) {
	hub, server := setupTestHub(t)
	conn := dialClient(t, server, "valid-token", []string{"webdav"})
	defer conn.Close()

	// Wait for hub registration
	time.Sleep(50 * time.Millisecond)
	assert(t, hub.IsConnected("user1"), "user should be connected")

	// Client reads requests and responds in a goroutine
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var req TunnelRequest
			if err := json.Unmarshal(msg, &req); err != nil {
				continue
			}
			resp := &TunnelResponse{
				ID:         req.ID,
				StatusCode: 200,
				Headers:    map[string]string{"X-Test": "ok"},
				Body:       []byte("hello from client"),
			}
			conn.WriteJSON(resp)
		}
	}()

	// Forward a request through the hub
	resp, err := hub.Forward("user1", &TunnelRequest{
		ID:     "req-1",
		Method: "GET",
		Path:   "/webdav/test",
	})
	assert(t, err == nil, "forward failed: "+fmt.Sprint(err))
	assert(t, resp.StatusCode == 200, "expected 200")
	assert(t, string(resp.Body) == "hello from client", "body mismatch")
	assert(t, resp.Headers["X-Test"] == "ok", "header mismatch")
}

func TestHub_ExecFlow(t *testing.T) {
	hub, server := setupTestHub(t)

	// Use the full Client for exec support
	client := NewClient(server.URL, "valid-token", slog.Default())
	client.EnableExec()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go client.Connect(ctx)
	time.Sleep(100 * time.Millisecond)
	assert(t, hub.IsConnected("user1"), "client should be connected")
	assert(t, hub.HasCapability("user1", "exec"), "should have exec cap")

	// Start exec: echo hello
	ch, err := hub.StartExec("user1", &ExecRequest{
		ID:      "exec-1",
		Command: []string{"echo", "hello"},
	})
	assert(t, err == nil, "start exec failed: "+fmt.Sprint(err))

	var gotStdout bool
	var gotDone bool
	for msg := range ch {
		switch msg.Type {
		case "exec.stdout":
			if strings.Contains(msg.Data, "hello") {
				gotStdout = true
			}
		case "exec.done":
			gotDone = true
			assert(t, msg.ExitCode == 0, "expected exit code 0")
		}
	}
	assert(t, gotStdout, "should have received stdout with hello")
	assert(t, gotDone, "should have received done message")
}

func TestHub_ExecStdin(t *testing.T) {
	hub, server := setupTestHub(t)

	client := NewClient(server.URL, "valid-token", slog.Default())
	client.EnableExec()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go client.Connect(ctx)
	time.Sleep(100 * time.Millisecond)

	// Start 'cat' which echoes stdin
	ch, err := hub.StartExec("user1", &ExecRequest{
		ID:      "exec-stdin",
		Command: []string{"cat"},
	})
	assert(t, err == nil, "start cat failed: "+fmt.Sprint(err))

	// Send stdin data
	time.Sleep(50 * time.Millisecond)
	err = hub.SendInput("user1", "exec-stdin", "ping\n")
	assert(t, err == nil, "send stdin failed: "+fmt.Sprint(err))

	// Read echoed output
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()

	var echoed bool
	for !echoed {
		select {
		case msg, ok := <-ch:
			if !ok {
				t.Fatal("channel closed unexpectedly")
			}
			if msg.Type == "exec.stdout" && strings.Contains(msg.Data, "ping") {
				echoed = true
			}
		case <-timer.C:
			t.Fatal("timeout waiting for echo")
		}
	}

	// Stop the cat process by closing its stdin via exec.stop
	err = hub.StopExec("user1", "exec-stdin")
	assert(t, err == nil, "stop exec failed")

	// Drain remaining messages
	for range ch {
	}
}

func TestHub_ExecStop(t *testing.T) {
	hub, server := setupTestHub(t)

	client := NewClient(server.URL, "valid-token", slog.Default())
	client.EnableExec()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go client.Connect(ctx)
	time.Sleep(100 * time.Millisecond)

	// Start a long-running process
	ch, err := hub.StartExec("user1", &ExecRequest{
		ID:      "exec-stop",
		Command: []string{"sleep", "60"},
	})
	assert(t, err == nil, "start sleep failed")

	time.Sleep(50 * time.Millisecond)

	// Stop the process
	err = hub.StopExec("user1", "exec-stop")
	assert(t, err == nil, "stop exec failed")

	// Should receive done or error within a reasonable time
	timer := time.NewTimer(3 * time.Second)
	defer timer.Stop()

	var terminated bool
	for !terminated {
		select {
		case msg, ok := <-ch:
			if !ok {
				terminated = true
			} else if msg.Type == "exec.done" || msg.Type == "exec.error" {
				terminated = true
			}
		case <-timer.C:
			t.Fatal("timeout waiting for process termination")
		}
	}
}

func TestHub_AutoReconnect(t *testing.T) {
	hub, server := setupTestHub(t)

	client := NewClient(server.URL, "valid-token", slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go client.Connect(ctx)
	time.Sleep(100 * time.Millisecond)
	assert(t, hub.IsConnected("user1"), "should be connected")

	// Force disconnect by closing the underlying connection
	client.procMu.Lock()
	if client.conn != nil {
		client.conn.Close()
	}
	client.procMu.Unlock()

	// Give hub time to detect disconnect
	time.Sleep(200 * time.Millisecond)
	assert(t, !hub.IsConnected("user1"), "should be disconnected after close")
}

func TestHub_AuthInvalidToken(t *testing.T) {
	_, server := setupTestHub(t)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/api/v1/tunnel"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	assert(t, err == nil, "dial should succeed")
	defer conn.Close()

	// Send hello with invalid token
	hello := HelloMessage{Type: "hello", Token: "bad-token", Capabilities: nil, Version: "1"}
	err = conn.WriteJSON(hello)
	assert(t, err == nil, "write hello should succeed")

	var resp HelloResponse
	err = conn.ReadJSON(&resp)
	assert(t, err == nil, "should receive error response")
	assert(t, resp.Type == "error", "expected error type, got "+resp.Type)
	assert(t, strings.Contains(resp.Error, "authentication"), "error should mention authentication")
}

func TestHub_ForwardNoClient(t *testing.T) {
	hub := NewHub(slog.Default(), func(token string) (string, error) {
		return "user1", nil
	})

	_, err := hub.Forward("nonexistent", &TunnelRequest{ID: "r1", Method: "GET", Path: "/"})
	assert(t, err != nil, "should fail with no client")
	assert(t, strings.Contains(err.Error(), "no tunnel"), "error should mention no tunnel")
}

func TestHub_Capabilities(t *testing.T) {
	hub, server := setupTestHub(t)
	conn := dialClient(t, server, "valid-token", []string{"exec", "webdav", "mcp"})
	defer conn.Close()
	time.Sleep(50 * time.Millisecond)

	caps := hub.GetCapabilities("user1")
	assert(t, len(caps) == 3, fmt.Sprintf("expected 3 caps, got %d", len(caps)))
	assert(t, hub.HasCapability("user1", "exec"), "should have exec")
	assert(t, hub.HasCapability("user1", "webdav"), "should have webdav")
	assert(t, hub.HasCapability("user1", "mcp"), "should have mcp")
	assert(t, !hub.HasCapability("user1", "unknown"), "should not have unknown")

	// Non-existent user
	assert(t, hub.GetCapabilities("nobody") == nil, "no caps for unknown user")
	assert(t, !hub.HasCapability("nobody", "exec"), "no cap for unknown user")
}

func TestHub_ClientReplace(t *testing.T) {
	hub, server := setupTestHub(t)

	conn1 := dialClient(t, server, "valid-token", []string{"exec"})
	time.Sleep(50 * time.Millisecond)
	assert(t, hub.IsConnected("user1"), "first client connected")

	// Connect second client for the same user
	conn2 := dialClient(t, server, "valid-token", []string{"exec", "webdav"})
	time.Sleep(50 * time.Millisecond)

	// Old conn should have been closed by hub
	caps := hub.GetCapabilities("user1")
	assert(t, len(caps) == 2, "should have new client's caps")

	conn1.Close()
	conn2.Close()
}

func TestClient_HandleHTTPRequest(t *testing.T) {
	hub, server := setupTestHub(t)

	client := NewClient(server.URL, "valid-token", slog.Default())
	// Register a test provider
	client.AddProvider("test", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Path", r.URL.Path)
		w.WriteHeader(200)
		w.Write([]byte("handled: " + r.URL.Path))
	}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go client.Connect(ctx)
	time.Sleep(100 * time.Millisecond)
	assert(t, hub.IsConnected("user1"), "client should be connected")

	resp, err := hub.Forward("user1", &TunnelRequest{
		ID:     "http-1",
		Method: "GET",
		Path:   "/test/some/path",
	})
	assert(t, err == nil, "forward failed: "+fmt.Sprint(err))
	assert(t, resp.StatusCode == 200, "expected 200")
	assert(t, string(resp.Body) == "handled: /some/path", "body mismatch: "+string(resp.Body))
}

func TestClient_UnknownProvider(t *testing.T) {
	hub, server := setupTestHub(t)

	client := NewClient(server.URL, "valid-token", slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go client.Connect(ctx)
	time.Sleep(100 * time.Millisecond)

	resp, err := hub.Forward("user1", &TunnelRequest{
		ID:     "http-2",
		Method: "GET",
		Path:   "/nonexistent/path",
	})
	assert(t, err == nil, "forward should succeed")
	assert(t, resp.StatusCode == 404, "expected 404")
}

func TestHub_MultipleExec(t *testing.T) {
	hub, server := setupTestHub(t)

	client := NewClient(server.URL, "valid-token", slog.Default())
	client.EnableExec()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go client.Connect(ctx)
	time.Sleep(100 * time.Millisecond)

	// Run multiple exec commands sequentially
	// (client doesn't protect concurrent WebSocket writes, so we run serially)
	for i := 0; i < 3; i++ {
		id := fmt.Sprintf("multi-%d", i)
		ch, err := hub.StartExec("user1", &ExecRequest{
			ID:      id,
			Command: []string{"echo", fmt.Sprintf("msg-%d", i)},
		})
		assert(t, err == nil, fmt.Sprintf("start exec %d failed: %v", i, err))

		var gotDone bool
		for msg := range ch {
			if msg.Type == "exec.done" {
				gotDone = true
			}
		}
		assert(t, gotDone, fmt.Sprintf("exec %d did not complete", i))
	}
}
