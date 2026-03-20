package service

import (
	"encoding/json"
	"net/http"
	"time"

	"go.zoe.im/agentbox/internal/auth"
	"go.zoe.im/agentbox/internal/model"
)

// CreateTeam handles POST /api/v1/teams
func (s *Service) CreateTeam(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var team model.Team
	if err := json.NewDecoder(r.Body).Decode(&team); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if team.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	team.ID = shortID()
	team.OwnerID = user.ID
	team.CreatedAt = time.Now()

	if err := s.store.CreateTeam(r.Context(), &team); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Add owner as a member with owner role
	member := &model.TeamMember{
		TeamID:   team.ID,
		UserID:   user.ID,
		Role:     "owner",
		JoinedAt: time.Now(),
	}
	if err := s.store.AddTeamMember(r.Context(), member); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(team)
}

// ListTeams handles GET /api/v1/teams
func (s *Service) ListTeams(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	teams, err := s.store.ListTeamsByUser(r.Context(), user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(teams)
}

// GetTeam handles GET /api/v1/teams/{id}
func (s *Service) GetTeam(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	teamID := r.PathValue("id")

	// Check membership
	_, err := s.store.GetTeamMember(r.Context(), teamID, user.ID)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	team, err := s.store.GetTeam(r.Context(), teamID)
	if err != nil {
		http.Error(w, "team not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(team)
}

// AddTeamMember handles POST /api/v1/teams/{id}/members
func (s *Service) AddTeamMember(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	teamID := r.PathValue("id")

	// Check that requester is owner or admin
	requesterMember, err := s.store.GetTeamMember(r.Context(), teamID, user.ID)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if requesterMember.Role != "owner" && requesterMember.Role != "admin" {
		http.Error(w, "only owner or admin can add members", http.StatusForbidden)
		return
	}

	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Email == "" {
		http.Error(w, "email is required", http.StatusBadRequest)
		return
	}

	// Look up user by email
	targetUser, err := s.store.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}

	role := req.Role
	if role == "" {
		role = "member"
	}

	member := &model.TeamMember{
		TeamID:   teamID,
		UserID:   targetUser.ID,
		Role:     role,
		JoinedAt: time.Now(),
	}

	if err := s.store.AddTeamMember(r.Context(), member); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(member)
}

// RemoveTeamMember handles DELETE /api/v1/teams/{id}/members/{user_id}
func (s *Service) RemoveTeamMember(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	teamID := r.PathValue("id")
	targetUserID := r.PathValue("user_id")

	// Check that requester is owner or admin
	requesterMember, err := s.store.GetTeamMember(r.Context(), teamID, user.ID)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if requesterMember.Role != "owner" && requesterMember.Role != "admin" {
		http.Error(w, "only owner or admin can remove members", http.StatusForbidden)
		return
	}

	// Cannot remove the owner
	team, err := s.store.GetTeam(r.Context(), teamID)
	if err != nil {
		http.Error(w, "team not found", http.StatusNotFound)
		return
	}
	if targetUserID == team.OwnerID {
		http.Error(w, "cannot remove team owner", http.StatusBadRequest)
		return
	}

	if err := s.store.RemoveTeamMember(r.Context(), teamID, targetUserID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListTeamMembers handles GET /api/v1/teams/{id}/members
func (s *Service) ListTeamMembers(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	teamID := r.PathValue("id")

	// Check membership
	_, err := s.store.GetTeamMember(r.Context(), teamID, user.ID)
	if err != nil {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	members, err := s.store.ListTeamMembers(r.Context(), teamID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(members)
}
