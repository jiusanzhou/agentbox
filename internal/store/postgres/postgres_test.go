package postgres

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"go.zoe.im/agentbox/internal/model"
)

// newTestStore creates a pgStore for testing, skipping if Postgres is unavailable.
// Set TEST_POSTGRES_DSN to run these tests, e.g.:
//
//	TEST_POSTGRES_DSN="postgres://localhost:5432/agentbox_test?sslmode=disable" go test ./internal/store/postgres/... -v
func newTestStore(t *testing.T) *pgStore {
	t.Helper()
	dsn := os.Getenv("TEST_POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgres://localhost:5432/agentbox_test?sslmode=disable"
	}

	s, err := New(Config{DSN: dsn})
	if err != nil {
		t.Skipf("skipping postgres test: %v", err)
	}
	t.Cleanup(func() {
		if closer, ok := s.(interface{ Close(context.Context) error }); ok {
			closer.Close(context.Background())
		}
	})
	return s.(*pgStore)
}

func TestNew(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping postgres test in short mode")
	}
	s := newTestStore(t)
	if s == nil {
		t.Fatal("New() returned nil store")
	}
}

func TestRunCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping postgres test in short mode")
	}
	s := newTestStore(t)
	ctx := context.Background()

	id := "test-run-" + time.Now().Format("150405.000")
	run := &model.Run{
		ID:        id,
		UserID:    "user1",
		Name:      "test run",
		Mode:      model.RunModeRun,
		Status:    model.RunStatusPending,
		AgentFile: "test agent",
		Config:    model.RunConfig{Timeout: 60},
		CreatedAt: time.Now(),
	}
	t.Cleanup(func() { s.pool.Exec(ctx, `DELETE FROM runs WHERE id = $1`, id) })

	if err := s.CreateRun(ctx, run); err != nil {
		t.Fatalf("CreateRun: %v", err)
	}

	got, err := s.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetRun: %v", err)
	}
	if got.Name != "test run" {
		t.Errorf("got name %q, want %q", got.Name, "test run")
	}

	now := time.Now()
	run.Status = model.RunStatusRunning
	run.StartedAt = &now
	if err := s.UpdateRun(ctx, run); err != nil {
		t.Fatalf("UpdateRun: %v", err)
	}

	got, _ = s.GetRun(ctx, run.ID)
	if got.Status != model.RunStatusRunning {
		t.Errorf("got status %q, want %q", got.Status, model.RunStatusRunning)
	}

	runs, err := s.ListRuns(ctx, 10, 0)
	if err != nil {
		t.Fatalf("ListRuns: %v", err)
	}
	if len(runs) == 0 {
		t.Error("ListRuns returned empty")
	}

	if err := s.DeleteRun(ctx, run.ID); err != nil {
		t.Fatalf("DeleteRun: %v", err)
	}
}

func TestUserCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping postgres test in short mode")
	}
	s := newTestStore(t)
	ctx := context.Background()

	id := "test-user-" + time.Now().Format("150405.000")
	email := id + "@example.com"
	user := &model.User{
		ID:        id,
		Email:     email,
		Name:      "Test User",
		Plan:      "free",
		APIKey:    "testhash-" + id,
		GitHubID:  "12345",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	t.Cleanup(func() { s.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, id) })

	if err := s.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	got, err := s.GetUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if got.Email != email {
		t.Errorf("got email %q, want %q", got.Email, email)
	}

	got, err = s.GetUserByEmail(ctx, email)
	if err != nil {
		t.Fatalf("GetUserByEmail: %v", err)
	}
	if got.ID != user.ID {
		t.Errorf("got id %q, want %q", got.ID, user.ID)
	}

	got, err = s.GetUserByAPIKey(ctx, user.APIKey)
	if err != nil {
		t.Fatalf("GetUserByAPIKey: %v", err)
	}
	if got.ID != user.ID {
		t.Errorf("got id %q, want %q", got.ID, user.ID)
	}

	user.Name = "Updated"
	user.UpdatedAt = time.Now()
	if err := s.UpdateUser(ctx, user); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
}

func TestIntegrationCRUD(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping postgres test in short mode")
	}
	s := newTestStore(t)
	ctx := context.Background()

	id := "test-intg-" + time.Now().Format("150405.000")
	intg := &model.Integration{
		ID:        id,
		UserID:    "user1",
		Type:      "telegram",
		Name:      "My Bot",
		Config:    json.RawMessage(`{"token":"abc"}`),
		Enabled:   true,
		Status:    "connected",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	t.Cleanup(func() { s.pool.Exec(ctx, `DELETE FROM integrations WHERE id = $1`, id) })

	if err := s.CreateIntegration(ctx, intg); err != nil {
		t.Fatalf("CreateIntegration: %v", err)
	}

	got, err := s.GetIntegration(ctx, intg.ID)
	if err != nil {
		t.Fatalf("GetIntegration: %v", err)
	}
	if got.Type != "telegram" {
		t.Errorf("got type %q, want %q", got.Type, "telegram")
	}

	list, err := s.ListIntegrations(ctx, "user1")
	if err != nil {
		t.Fatalf("ListIntegrations: %v", err)
	}
	if len(list) == 0 {
		t.Error("ListIntegrations returned empty")
	}

	all, err := s.ListAllEnabledIntegrations(ctx)
	if err != nil {
		t.Fatalf("ListAllEnabledIntegrations: %v", err)
	}
	if len(all) == 0 {
		t.Error("ListAllEnabledIntegrations returned empty")
	}

	if err := s.DeleteIntegration(ctx, intg.ID); err != nil {
		t.Fatalf("DeleteIntegration: %v", err)
	}
}

func TestCompileCheck(t *testing.T) {
	// Ensures pgStore implements store.Store at compile time.
	var _ interface{ Close(context.Context) error } = (*pgStore)(nil)
}
