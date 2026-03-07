package sqlite

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"go.zoe.im/agentbox/internal/model"
	"go.zoe.im/agentbox/internal/store"
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

func setupSQLite(t *testing.T) store.Store {
	t.Helper()
	dir := t.TempDir()
	s, err := New(Config{Path: filepath.Join(dir, "test.db")})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSQLiteStore_RunCRUD(t *testing.T) {
	s := setupSQLite(t)
	ctx := context.Background()

	// Create
	run := &model.Run{
		ID:        "r1",
		Name:      "test",
		Mode:      model.RunModeRun,
		Status:    model.RunStatusPending,
		AgentFile: "Be brief.",
		CreatedAt: time.Now(),
	}
	err := s.CreateRun(ctx, run)
	assert(t, err == nil, "create should succeed")

	// Duplicate create
	err = s.CreateRun(ctx, run)
	assert(t, err != nil, "duplicate create should fail")

	// Get
	got, err := s.GetRun(ctx, "r1")
	assert(t, err == nil, "get should succeed")
	assert(t, got.Name == "test", "name should match")
	assert(t, got.Status == model.RunStatusPending, "status should match")
	assert(t, got.Mode == model.RunModeRun, "mode should match")

	// Get not found
	_, err = s.GetRun(ctx, "nonexistent")
	assert(t, err != nil, "get nonexistent should fail")

	// List
	runs, err := s.ListRuns(ctx, 10, 0)
	assert(t, err == nil, "list should succeed")
	assert(t, len(runs) == 1, "should have 1 run")

	// Update
	now := time.Now()
	run.Status = model.RunStatusRunning
	run.StartedAt = &now
	err = s.UpdateRun(ctx, run)
	assert(t, err == nil, "update should succeed")
	got, _ = s.GetRun(ctx, "r1")
	assert(t, got.Status == model.RunStatusRunning, "status should be updated")
	assert(t, got.StartedAt != nil, "started_at should be set")

	// Update with result
	end := time.Now()
	run.Status = model.RunStatusCompleted
	run.EndedAt = &end
	run.Result = &model.Result{ExitCode: 0, Output: "done"}
	err = s.UpdateRun(ctx, run)
	assert(t, err == nil, "update with result should succeed")
	got, _ = s.GetRun(ctx, "r1")
	assert(t, got.Status == model.RunStatusCompleted, "status should be completed")
	assert(t, got.Result != nil, "result should be set")
	assert(t, got.Result.Output == "done", "result output should match")

	// Delete
	err = s.DeleteRun(ctx, "r1")
	assert(t, err == nil, "delete should succeed")
	_, err = s.GetRun(ctx, "r1")
	assert(t, err != nil, "get after delete should fail")
}

func TestSQLiteStore_ListRuns_Pagination(t *testing.T) {
	s := setupSQLite(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		s.CreateRun(ctx, &model.Run{
			ID:        "r" + string(rune('a'+i)),
			Name:      "test",
			Mode:      model.RunModeRun,
			Status:    model.RunStatusPending,
			CreatedAt: time.Now().Add(time.Duration(i) * time.Second),
		})
	}

	runs, _ := s.ListRuns(ctx, 2, 0)
	assert(t, len(runs) == 2, "limit 2 should return 2")

	runs, _ = s.ListRuns(ctx, 10, 3)
	assert(t, len(runs) == 2, "offset 3 of 5 should return 2")

	runs, _ = s.ListRuns(ctx, 10, 10)
	assert(t, len(runs) == 0, "offset past end should return 0")
}

func TestSQLiteStore_UserCRUD(t *testing.T) {
	s := setupSQLite(t)
	ctx := context.Background()

	user := &model.User{
		ID:        "u1",
		Email:     "test@example.com",
		Name:      "Test User",
		Password:  "hashed",
		Plan:      "free",
		APIKey:    "ak_hash_123",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create
	err := s.CreateUser(ctx, user)
	assert(t, err == nil, "create user should succeed")

	// Duplicate email
	user2 := &model.User{
		ID: "u2", Email: "test@example.com", Name: "Other",
		Password: "x", Plan: "free",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	err = s.CreateUser(ctx, user2)
	assert(t, err != nil, "duplicate email should fail")

	// Get by ID
	got, err := s.GetUser(ctx, "u1")
	assert(t, err == nil, "get user should succeed")
	assert(t, got.Email == "test@example.com", "email should match")
	assert(t, got.Name == "Test User", "name should match")

	// Get not found
	_, err = s.GetUser(ctx, "nonexistent")
	assert(t, err != nil, "get nonexistent user should fail")

	// Get by email
	got, err = s.GetUserByEmail(ctx, "test@example.com")
	assert(t, err == nil, "get by email should succeed")
	assert(t, got.ID == "u1", "ID should match")

	// Get by email not found
	_, err = s.GetUserByEmail(ctx, "nope@example.com")
	assert(t, err != nil, "get by unknown email should fail")

	// Get by API key
	got, err = s.GetUserByAPIKey(ctx, "ak_hash_123")
	assert(t, err == nil, "get by API key should succeed")
	assert(t, got.ID == "u1", "ID should match")

	// Get by API key not found
	_, err = s.GetUserByAPIKey(ctx, "unknown")
	assert(t, err != nil, "get by unknown API key should fail")

	// Update
	user.Name = "Updated Name"
	user.UpdatedAt = time.Now()
	err = s.UpdateUser(ctx, user)
	assert(t, err == nil, "update user should succeed")
	got, _ = s.GetUser(ctx, "u1")
	assert(t, got.Name == "Updated Name", "name should be updated")
}

func TestSQLiteStore_IntegrationCRUD(t *testing.T) {
	s := setupSQLite(t)
	ctx := context.Background()

	intg := &model.Integration{
		ID:        "i1",
		UserID:    "u1",
		Type:      "webhook",
		Name:      "My Webhook",
		Config:    json.RawMessage(`{"secret":"s3cret"}`),
		Enabled:   true,
		Status:    "connected",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Create
	err := s.CreateIntegration(ctx, intg)
	assert(t, err == nil, "create integration should succeed")

	// Get
	got, err := s.GetIntegration(ctx, "i1")
	assert(t, err == nil, "get integration should succeed")
	assert(t, got.Name == "My Webhook", "name should match")
	assert(t, got.Type == "webhook", "type should match")
	assert(t, got.Enabled == true, "enabled should be true")

	// Get not found
	_, err = s.GetIntegration(ctx, "nonexistent")
	assert(t, err != nil, "get nonexistent integration should fail")

	// List by user
	intg2 := &model.Integration{
		ID: "i2", UserID: "u1", Type: "slack", Name: "Slack",
		Config: json.RawMessage(`{}`), Enabled: false,
		Status: "disconnected", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	s.CreateIntegration(ctx, intg2)
	intg3 := &model.Integration{
		ID: "i3", UserID: "u2", Type: "discord", Name: "Discord",
		Config: json.RawMessage(`{}`), Enabled: true,
		Status: "connected", CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	s.CreateIntegration(ctx, intg3)

	list, err := s.ListIntegrations(ctx, "u1")
	assert(t, err == nil, "list integrations should succeed")
	assert(t, len(list) == 2, "user u1 should have 2 integrations")

	list, err = s.ListIntegrations(ctx, "u2")
	assert(t, err == nil)
	assert(t, len(list) == 1, "user u2 should have 1 integration")

	list, err = s.ListIntegrations(ctx, "u99")
	assert(t, err == nil)
	assert(t, len(list) == 0, "unknown user should have 0 integrations")

	// ListAllEnabled
	enabled, err := s.ListAllEnabledIntegrations(ctx)
	assert(t, err == nil, "list enabled should succeed")
	assert(t, len(enabled) == 2, "should have 2 enabled integrations")

	// Update
	intg.Name = "Updated Webhook"
	intg.UpdatedAt = time.Now()
	err = s.UpdateIntegration(ctx, intg)
	assert(t, err == nil, "update integration should succeed")
	got, _ = s.GetIntegration(ctx, "i1")
	assert(t, got.Name == "Updated Webhook", "name should be updated")

	// Delete
	err = s.DeleteIntegration(ctx, "i1")
	assert(t, err == nil, "delete integration should succeed")
	_, err = s.GetIntegration(ctx, "i1")
	assert(t, err != nil, "get after delete should fail")
}
