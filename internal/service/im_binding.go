package service

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.zoe.im/agentbox/internal/auth"
	"go.zoe.im/agentbox/internal/model"
)

// HandleBindCode handles POST /api/v1/im/bind (authenticated).
// Generates a 6-char uppercase alphanumeric binding code with a 10-minute TTL.
func (s *Service) HandleBindCode(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	code := generateBindingCode()
	bc := &model.BindingCode{
		Code:      code,
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}

	if err := s.store.CreateBindingCode(r.Context(), bc); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to create binding code: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"code":       code,
		"expires_in": 600,
	})
}

// HandleVerifyBinding handles POST /api/v1/im/verify (called by bots).
// Accepts a binding code and platform info, creates an IMBinding.
func (s *Service) HandleVerifyBinding(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Code             string `json:"code"`
		Platform         string `json:"platform"`
		PlatformUserID   string `json:"platform_user_id"`
		PlatformUsername string `json:"platform_username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Code == "" || req.Platform == "" || req.PlatformUserID == "" {
		http.Error(w, `{"error":"code, platform, and platform_user_id are required"}`, http.StatusBadRequest)
		return
	}

	// Look up the binding code.
	bc, err := s.store.GetBindingCode(r.Context(), req.Code)
	if err != nil {
		http.Error(w, `{"error":"invalid or expired binding code"}`, http.StatusNotFound)
		return
	}

	// Create the IM binding.
	binding := &model.IMBinding{
		ID:               genID(),
		UserID:           bc.UserID,
		Platform:         req.Platform,
		PlatformUserID:   req.PlatformUserID,
		PlatformUsername: req.PlatformUsername,
		CreatedAt:        time.Now(),
	}
	if err := s.store.CreateIMBinding(r.Context(), binding); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to create binding: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Delete the used code.
	_ = s.store.DeleteBindingCode(r.Context(), req.Code)

	// Look up the user for the response.
	user, err := s.store.GetUser(r.Context(), bc.UserID)
	if err != nil {
		http.Error(w, `{"error":"user not found"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"binding": binding,
		"user": map[string]any{
			"id":   user.ID,
			"name": user.Name,
		},
	})
}

// ListIMBindings handles GET /api/v1/im/bindings (authenticated).
func (s *Service) ListIMBindings(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	bindings, err := s.store.ListIMBindingsByUser(r.Context(), user.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to list bindings: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	if bindings == nil {
		bindings = []*model.IMBinding{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"bindings": bindings,
	})
}

// DeleteIMBinding handles DELETE /api/v1/im/bindings/{id} (authenticated).
func (s *Service) DeleteIMBinding(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	bindingID := r.PathValue("id")
	if bindingID == "" {
		http.Error(w, `{"error":"binding id required"}`, http.StatusBadRequest)
		return
	}

	// Verify the binding belongs to this user by listing all bindings.
	bindings, err := s.store.ListIMBindingsByUser(r.Context(), user.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to verify binding: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	found := false
	for _, b := range bindings {
		if b.ID == bindingID {
			found = true
			break
		}
	}
	if !found {
		http.Error(w, `{"error":"binding not found or not owned by user"}`, http.StatusNotFound)
		return
	}

	if err := s.store.DeleteIMBinding(r.Context(), bindingID); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"failed to delete binding: %s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

// generateBindingCode creates a 6-character uppercase alphanumeric code.
func generateBindingCode() string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 6)
	rand.Read(b)
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}
