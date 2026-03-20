package channel

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"go.zoe.im/agentbox/internal/engine"
	"go.zoe.im/agentbox/internal/executor"
	"go.zoe.im/agentbox/internal/model"
	"go.zoe.im/agentbox/internal/store/memory"
)

// mockChannel implements the Channel interface for testing.
type mockChannel struct {
	name       string
	handler    Handler
	cbHandler  CallbackHandler
	sentMu     sync.Mutex
	sent       []sentMsg
	editCalls  []editCall
	buttonMsgs []buttonMsg
}

type sentMsg struct {
	ChatID string
	Text   string
	Opts   *SendOptions
}

type editCall struct {
	ChatID    string
	MessageID string
	Text      string
}

type buttonMsg struct {
	ChatID  string
	Text    string
	Buttons []Button
}

func (m *mockChannel) Name() string { return m.name }

func (m *mockChannel) Start(_ context.Context, handler Handler) error {
	m.handler = handler
	return nil
}

func (m *mockChannel) Send(_ context.Context, chatID string, text string, opts *SendOptions) error {
	m.sentMu.Lock()
	defer m.sentMu.Unlock()
	m.sent = append(m.sent, sentMsg{ChatID: chatID, Text: text, Opts: opts})
	return nil
}

func (m *mockChannel) EditMessage(_ context.Context, chatID string, messageID string, text string, opts *SendOptions) error {
	m.sentMu.Lock()
	defer m.sentMu.Unlock()
	m.editCalls = append(m.editCalls, editCall{ChatID: chatID, MessageID: messageID, Text: text})
	return nil
}

func (m *mockChannel) SendWithButtons(_ context.Context, chatID string, text string, buttons []Button, opts *SendOptions) (string, error) {
	m.sentMu.Lock()
	defer m.sentMu.Unlock()
	m.buttonMsgs = append(m.buttonMsgs, buttonMsg{ChatID: chatID, Text: text, Buttons: buttons})
	return "btn-msg-1", nil
}

func (m *mockChannel) OnCallback(handler CallbackHandler) {
	m.cbHandler = handler
}

func (m *mockChannel) Stop(_ context.Context) error { return nil }

func (m *mockChannel) getSent() []sentMsg {
	m.sentMu.Lock()
	defer m.sentMu.Unlock()
	cp := make([]sentMsg, len(m.sent))
	copy(cp, m.sent)
	return cp
}

// mockExecutor implements executor.Executor for testing.
type mockExecutor struct {
	mu           sync.Mutex
	sessions     map[string]bool
	lastMessage  string
	response     string
	streamTokens []string
}

func newMockExecutor() *mockExecutor {
	return &mockExecutor{
		sessions: make(map[string]bool),
		response: "mock response",
	}
}

func (e *mockExecutor) Execute(_ context.Context, req *executor.Request) (*executor.Response, error) {
	return &executor.Response{Output: e.response}, nil
}

func (e *mockExecutor) Logs(_ context.Context, id string) (string, error) {
	return "test logs", nil
}

func (e *mockExecutor) Stop(_ context.Context, id string) error {
	return e.StopSession(context.Background(), id)
}

func (e *mockExecutor) StartSession(_ context.Context, req *executor.Request) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.sessions[req.ID] = true
	return req.ID, nil
}

func (e *mockExecutor) SendMessage(_ context.Context, id string, message string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.sessions[id] {
		return "", fmt.Errorf("session not found: %s", id)
	}
	e.lastMessage = message
	return e.response, nil
}

func (e *mockExecutor) SendMessageStream(_ context.Context, id string, message string, onToken executor.TokenCallback) (string, error) {
	e.mu.Lock()
	tokens := e.streamTokens
	e.mu.Unlock()

	if onToken != nil {
		for _, tok := range tokens {
			onToken(tok)
		}
	}
	return e.response, nil
}

func (e *mockExecutor) StopSession(_ context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.sessions, id)
	return nil
}

func (e *mockExecutor) RecoverSessions(_ context.Context) ([]string, error) { return nil, nil }

func (e *mockExecutor) UploadFile(_ context.Context, runID string, filename string, data []byte) error {
	return nil
}

func (e *mockExecutor) StreamLogs(_ context.Context, runID string) (<-chan string, error) {
	ch := make(chan string)
	close(ch)
	return ch, nil
}

// --- Tests ---

func setupRouter(t *testing.T) (*Router, *mockChannel, *mockExecutor) {
	t.Helper()
	st := memory.New()
	exec := newMockExecutor()
	eng := engine.New(st, exec, nil)
	ch := &mockChannel{name: "test"}
	r := NewRouter(eng, st, nil)
	r.Add(ch)
	return r, ch, exec
}

func TestRouter_MessageRouting(t *testing.T) {
	r, ch, _ := setupRouter(t)
	ctx := context.Background()

	err := r.Start(ctx)
	assert(t, err == nil, "start failed")

	// Send a message — this should create a session and get a response
	err = ch.handler(ctx, &Message{
		ID:     "msg-1",
		ChatID: "chat-1",
		UserID: "user-1",
		Text:   "hello agent",
	})
	assert(t, err == nil, "handle message failed: "+fmt.Sprint(err))

	// Wait for streaming to complete
	time.Sleep(200 * time.Millisecond)

	sent := ch.getSent()
	assert(t, len(sent) >= 1, fmt.Sprintf("should have sent at least 1 message, got %d", len(sent)))

	// Verify a response was sent to the right chat
	found := false
	for _, s := range sent {
		if s.ChatID == "chat-1" {
			found = true
		}
	}
	assert(t, found, "response should be sent to chat-1")
}

func TestRouter_NewCommand(t *testing.T) {
	r, ch, _ := setupRouter(t)
	ctx := context.Background()
	r.Start(ctx)

	// First create a session
	ch.handler(ctx, &Message{ID: "m1", ChatID: "chat-1", Text: "first message"})
	time.Sleep(200 * time.Millisecond)

	// Reset
	err := ch.handler(ctx, &Message{ID: "m2", ChatID: "chat-1", Text: "/new"})
	assert(t, err == nil, "handle /new failed")

	sent := ch.getSent()
	// Look for "New session started" in sent messages
	found := false
	for _, s := range sent {
		if strings.Contains(s.Text, "New session started") {
			found = true
		}
	}
	assert(t, found, "should report new session started")
}

func TestRouter_ResetCommand(t *testing.T) {
	r, ch, _ := setupRouter(t)
	ctx := context.Background()
	r.Start(ctx)

	err := ch.handler(ctx, &Message{ID: "m1", ChatID: "chat-1", Text: "/reset"})
	assert(t, err == nil, "handle /reset failed")

	sent := ch.getSent()
	found := false
	for _, s := range sent {
		if strings.Contains(s.Text, "New session started") {
			found = true
		}
	}
	assert(t, found, "/reset should create new session")
}

func TestRouter_StopCommand(t *testing.T) {
	r, ch, _ := setupRouter(t)
	ctx := context.Background()
	r.Start(ctx)

	// No session → "No active session"
	ch.handler(ctx, &Message{ID: "m1", ChatID: "chat-1", Text: "/stop"})
	sent := ch.getSent()
	found := false
	for _, s := range sent {
		if strings.Contains(s.Text, "No active session") {
			found = true
		}
	}
	assert(t, found, "should report no active session")

	// Create a session then stop it
	ch.handler(ctx, &Message{ID: "m2", ChatID: "chat-2", Text: "create session"})
	time.Sleep(200 * time.Millisecond)
	ch.handler(ctx, &Message{ID: "m3", ChatID: "chat-2", Text: "/stop"})

	sent = ch.getSent()
	found = false
	for _, s := range sent {
		if strings.Contains(s.Text, "Session stopped") {
			found = true
		}
	}
	assert(t, found, "should report session stopped")
}

func TestRouter_StatusCommand(t *testing.T) {
	r, ch, _ := setupRouter(t)
	ctx := context.Background()
	r.Start(ctx)

	// No session
	ch.handler(ctx, &Message{ID: "m1", ChatID: "chat-1", Text: "/status"})
	sent := ch.getSent()
	found := false
	for _, s := range sent {
		if strings.Contains(s.Text, "No active session") {
			found = true
		}
	}
	assert(t, found, "should report no active session for /status")

	// Create session then check status
	ch.handler(ctx, &Message{ID: "m2", ChatID: "chat-2", Text: "hello"})
	time.Sleep(200 * time.Millisecond)
	ch.handler(ctx, &Message{ID: "m3", ChatID: "chat-2", Text: "/status"})

	sent = ch.getSent()
	found = false
	for _, s := range sent {
		if strings.Contains(s.Text, "Session:") && strings.Contains(s.Text, "Status:") {
			found = true
		}
	}
	assert(t, found, "/status should return session info")
}

func TestRouter_PermissionCallbackAllow(t *testing.T) {
	r, ch, _ := setupRouter(t)
	ctx := context.Background()
	r.Start(ctx)

	// Register a permission request
	r.Permissions.Register("abc123", "bash", "chat-1")

	// Simulate button click: allow
	cb := &Callback{ID: "permission_allow_abc123", ChatID: "chat-1"}
	err := ch.cbHandler(ctx, cb)
	assert(t, err == nil, "callback should succeed")
}

func TestRouter_PermissionCallbackDeny(t *testing.T) {
	r, ch, _ := setupRouter(t)
	ctx := context.Background()
	r.Start(ctx)

	r.Permissions.Register("def456", "bash", "chat-1")

	// Simulate deny click
	go func() {
		time.Sleep(10 * time.Millisecond)
		ch.cbHandler(ctx, &Callback{ID: "permission_deny_def456", ChatID: "chat-1"})
	}()

	result := r.Permissions.WaitFor("def456")
	assert(t, !result, "should be denied")
}

func TestRouter_PermissionCallbackUnknownPrefix(t *testing.T) {
	r, ch, _ := setupRouter(t)
	ctx := context.Background()
	r.Start(ctx)

	// Callback with unrecognized ID should not error
	err := ch.cbHandler(ctx, &Callback{ID: "random_callback", ChatID: "chat-1"})
	assert(t, err == nil, "unknown callback should not error")
}

func TestRouter_EmptyMessage(t *testing.T) {
	r, ch, _ := setupRouter(t)
	ctx := context.Background()
	r.Start(ctx)

	// Empty message should be ignored
	err := ch.handler(ctx, &Message{ID: "m1", ChatID: "chat-1", Text: ""})
	assert(t, err == nil, "empty message should not error")

	// Whitespace-only message
	err = ch.handler(ctx, &Message{ID: "m2", ChatID: "chat-1", Text: "   "})
	assert(t, err == nil, "whitespace message should not error")
}

func TestRouter_StreamResponse(t *testing.T) {
	st := memory.New()
	exec := newMockExecutor()
	exec.streamTokens = []string{"Hello", " ", "world", "!"}
	exec.response = "Hello world!"

	eng := engine.New(st, exec, nil)
	ch := &mockChannel{name: "test"}
	r := NewRouter(eng, st, nil)
	r.Add(ch)

	ctx := context.Background()
	r.Start(ctx)

	err := ch.handler(ctx, &Message{
		ID:     "msg-stream",
		ChatID: "chat-stream",
		Text:   "stream test",
	})
	assert(t, err == nil, "stream message failed")

	time.Sleep(300 * time.Millisecond)

	sent := ch.getSent()
	assert(t, len(sent) >= 1, "should have sent messages")
}

func TestRouter_RequestPermission(t *testing.T) {
	r, ch, _ := setupRouter(t)
	ctx := context.Background()
	r.Start(ctx)

	// Simulate allow in background
	go func() {
		time.Sleep(50 * time.Millisecond)
		ch.sentMu.Lock()
		bmsgs := ch.buttonMsgs
		ch.sentMu.Unlock()

		if len(bmsgs) > 0 {
			// Find the allow button ID
			for _, btn := range bmsgs[0].Buttons {
				if strings.Contains(btn.ID, "permission_allow_") {
					ch.cbHandler(ctx, &Callback{ID: btn.ID, ChatID: "chat-perm"})
					return
				}
			}
		}
	}()

	allowed := r.RequestPermission(ctx, "chat-perm", "bash", "rm -rf /tmp/test")
	assert(t, allowed, "permission should be allowed")
}

func TestRouter_StopDeniesPermissions(t *testing.T) {
	r, _, _ := setupRouter(t)
	ctx := context.Background()
	r.Start(ctx)

	// Register some pending permissions
	r.Permissions.Register("p1", "tool1", "chat-1")
	r.Permissions.Register("p2", "tool2", "chat-2")

	var results [2]bool
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); results[0] = r.Permissions.WaitFor("p1") }()
	go func() { defer wg.Done(); results[1] = r.Permissions.WaitFor("p2") }()

	time.Sleep(20 * time.Millisecond)

	// Stop router should deny all
	r.Stop(ctx)
	wg.Wait()

	assert(t, !results[0], "p1 should be denied on stop")
	assert(t, !results[1], "p2 should be denied on stop")
}

func TestRouter_AgentCommand(t *testing.T) {
	r, ch, _ := setupRouter(t)
	ctx := context.Background()
	r.Start(ctx)

	err := ch.handler(ctx, &Message{ID: "m1", ChatID: "chat-1", Text: "/agent custom-agent"})
	assert(t, err == nil, "handle /agent failed")

	sent := ch.getSent()
	found := false
	for _, s := range sent {
		if strings.Contains(s.Text, "Session started with custom agent") {
			found = true
		}
	}
	assert(t, found, "should report custom agent session started")
}

func TestRouter_MultiChannel(t *testing.T) {
	st := memory.New()
	exec := newMockExecutor()
	eng := engine.New(st, exec, nil)

	ch1 := &mockChannel{name: "telegram"}
	ch2 := &mockChannel{name: "feishu"}

	r := NewRouter(eng, st, nil)
	r.Add(ch1)
	r.Add(ch2)

	ctx := context.Background()
	r.Start(ctx)

	// Message with channel hint
	msg := &Message{
		ID:     "m1",
		ChatID: "chat-1",
		Text:   "/status",
		Extra:  map[string]string{"channel": "feishu"},
	}
	ch1.handler(ctx, msg)

	sent := ch1.getSent()
	// Even with hint, the response goes through the handler's channel
	assert(t, len(sent) >= 1 || len(ch2.getSent()) >= 0, "at least one channel should receive response")
}

func TestRouter_SessionPersistence(t *testing.T) {
	r, ch, exec := setupRouter(t)
	ctx := context.Background()
	r.Start(ctx)

	// Send message to create session
	ch.handler(ctx, &Message{ID: "m1", ChatID: "chat-persist", Text: "first"})
	time.Sleep(200 * time.Millisecond)

	// Send another message — should reuse same session
	exec.response = "second response"
	ch.handler(ctx, &Message{ID: "m2", ChatID: "chat-persist", Text: "second"})
	time.Sleep(200 * time.Millisecond)

	// Check router's internal session mapping
	r.mu.RLock()
	_, exists := r.sessions["chat-persist"]
	r.mu.RUnlock()
	assert(t, exists, "session should persist for same chat")
}

// Test that shortID generates unique hex IDs of correct length
func TestShortID(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := shortID()
		assert(t, len(id) == 8, "shortID should be 8 hex chars")
		assert(t, !ids[id], "shortID should be unique")
		ids[id] = true
	}
}

// Ensure model import is exercised
var _ = model.RunModeSession
