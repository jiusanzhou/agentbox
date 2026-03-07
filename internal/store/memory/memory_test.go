package memory

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.zoe.im/agentbox/internal/model"
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

func TestMemoryStore_RunCRUD(t *testing.T) {
	s := New()
	ctx := context.Background()

	// Create
	run := &model.Run{ID: "r1", Name: "test", Status: model.RunStatusPending, CreatedAt: time.Now()}
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

	// Get not found
	_, err = s.GetRun(ctx, "nonexistent")
	assert(t, err != nil, "get nonexistent should fail")

	// List
	runs, err := s.ListRuns(ctx, 10, 0)
	assert(t, err == nil, "list should succeed")
	assert(t, len(runs) == 1, "should have 1 run")

	// Update
	run.Status = model.RunStatusRunning
	err = s.UpdateRun(ctx, run)
	assert(t, err == nil, "update should succeed")
	got, _ = s.GetRun(ctx, "r1")
	assert(t, got.Status == model.RunStatusRunning, "status should be updated")

	// Update not found
	err = s.UpdateRun(ctx, &model.Run{ID: "nonexistent"})
	assert(t, err != nil, "update nonexistent should fail")

	// Delete
	err = s.DeleteRun(ctx, "r1")
	assert(t, err == nil, "delete should succeed")
	_, err = s.GetRun(ctx, "r1")
	assert(t, err != nil, "get after delete should fail")

	// List after delete
	runs, err = s.ListRuns(ctx, 10, 0)
	assert(t, err == nil, "list should succeed")
	assert(t, len(runs) == 0, "should have 0 runs")
}

func TestMemoryStore_ListRuns_Pagination(t *testing.T) {
	s := New()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		s.CreateRun(ctx, &model.Run{ID: "r" + string(rune('0'+i)), Name: "test"})
	}

	runs, _ := s.ListRuns(ctx, 2, 0)
	assert(t, len(runs) == 2, "limit 2 should return 2")

	runs, _ = s.ListRuns(ctx, 10, 3)
	assert(t, len(runs) == 2, "offset 3 of 5 should return 2")

	runs, _ = s.ListRuns(ctx, 10, 10)
	assert(t, runs == nil, "offset past end should return nil")
}

func TestMemoryStore_UserCRUD(t *testing.T) {
	s := New()
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
	user2 := &model.User{ID: "u2", Email: "test@example.com", Name: "Other"}
	err = s.CreateUser(ctx, user2)
	assert(t, err != nil, "duplicate email should fail")

	// Get by ID
	got, err := s.GetUser(ctx, "u1")
	assert(t, err == nil, "get user should succeed")
	assert(t, got.Email == "test@example.com", "email should match")

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
	err = s.UpdateUser(ctx, user)
	assert(t, err == nil, "update user should succeed")
	got, _ = s.GetUser(ctx, "u1")
	assert(t, got.Name == "Updated Name", "name should be updated")

	// Update not found
	err = s.UpdateUser(ctx, &model.User{ID: "nonexistent"})
	assert(t, err != nil, "update nonexistent user should fail")
}

func TestMemoryStore_IntegrationCRUD(t *testing.T) {
	s := New()
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

	// Get not found
	_, err = s.GetIntegration(ctx, "nonexistent")
	assert(t, err != nil, "get nonexistent integration should fail")

	// List by user
	intg2 := &model.Integration{ID: "i2", UserID: "u1", Type: "slack", Enabled: false}
	s.CreateIntegration(ctx, intg2)
	intg3 := &model.Integration{ID: "i3", UserID: "u2", Type: "discord", Enabled: true}
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
	err = s.UpdateIntegration(ctx, intg)
	assert(t, err == nil, "update integration should succeed")
	got, _ = s.GetIntegration(ctx, "i1")
	assert(t, got.Name == "Updated Webhook", "name should be updated")

	// Update not found
	err = s.UpdateIntegration(ctx, &model.Integration{ID: "nonexistent"})
	assert(t, err != nil, "update nonexistent integration should fail")

	// Delete
	err = s.DeleteIntegration(ctx, "i1")
	assert(t, err == nil, "delete integration should succeed")
	_, err = s.GetIntegration(ctx, "i1")
	assert(t, err != nil, "get after delete should fail")
}
