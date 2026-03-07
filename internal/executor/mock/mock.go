package mock

import (
	"context"
	"fmt"
	"sync"

	"go.zoe.im/agentbox/internal/executor"
)

// MockExecutor implements executor.Executor for testing.
type MockExecutor struct {
	mu        sync.Mutex
	started   map[string]bool
	responses map[string]string
}

// New creates a new MockExecutor.
func New() *MockExecutor {
	return &MockExecutor{
		started:   make(map[string]bool),
		responses: make(map[string]string),
	}
}

func (m *MockExecutor) Execute(ctx context.Context, req *executor.Request) (*executor.Response, error) {
	return &executor.Response{Output: "mock output", ExitCode: 0}, nil
}

func (m *MockExecutor) StartSession(ctx context.Context, req *executor.Request) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started[req.ID] = true
	return req.ID, nil
}

func (m *MockExecutor) SendMessage(ctx context.Context, id string, message string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.started[id] {
		return "", fmt.Errorf("session %s not started", id)
	}
	if resp, ok := m.responses[message]; ok {
		return resp, nil
	}
	return "mock response to: " + message, nil
}

func (m *MockExecutor) SendMessageStream(ctx context.Context, id string, message string, onToken executor.TokenCallback) (string, error) {
	m.mu.Lock()
	if !m.started[id] {
		m.mu.Unlock()
		return "", fmt.Errorf("session %s not started", id)
	}
	m.mu.Unlock()

	resp := "mock response to: " + message
	if onToken != nil {
		onToken(resp)
	}
	return resp, nil
}

func (m *MockExecutor) StopSession(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.started, id)
	return nil
}

func (m *MockExecutor) Stop(ctx context.Context, id string) error {
	return nil
}

func (m *MockExecutor) Logs(ctx context.Context, id string) (string, error) {
	return "mock logs for " + id, nil
}

func (m *MockExecutor) StreamLogs(ctx context.Context, id string) (<-chan string, error) {
	ch := make(chan string, 1)
	ch <- "mock log line"
	close(ch)
	return ch, nil
}

func (m *MockExecutor) RecoverSessions(ctx context.Context) ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var ids []string
	for id := range m.started {
		ids = append(ids, id)
	}
	return ids, nil
}

func (m *MockExecutor) UploadFile(ctx context.Context, runID string, filename string, data []byte) error {
	return nil
}

// SetResponse pre-configures a response for a given message.
func (m *MockExecutor) SetResponse(message, response string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[message] = response
}

// IsStarted checks if a session is currently started.
func (m *MockExecutor) IsStarted(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.started[id]
}
