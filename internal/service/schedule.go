package service

import (
	"encoding/json"
	"net/http"
	"time"

	"go.zoe.im/agentbox/internal/auth"
	"go.zoe.im/agentbox/internal/model"
)

// CreateSchedule handles POST /api/v1/schedules
func (s *Service) CreateSchedule(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	var sched model.Schedule
	if err := json.NewDecoder(r.Body).Decode(&sched); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if sched.CronExpr == "" {
		http.Error(w, "cron_expr is required", http.StatusBadRequest)
		return
	}

	sched.ID = shortID()
	sched.UserID = user.ID
	sched.CreatedAt = time.Now()
	if sched.Timezone == "" {
		sched.Timezone = "UTC"
	}

	// Compute next run time
	if s.scheduler != nil {
		next, err := s.scheduler.ComputeNextRun(sched.CronExpr, sched.Timezone)
		if err != nil {
			http.Error(w, "invalid cron expression: "+err.Error(), http.StatusBadRequest)
			return
		}
		sched.NextRunAt = &next
	}

	if err := s.store.CreateSchedule(r.Context(), &sched); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(sched)
}

// ListSchedules handles GET /api/v1/schedules
func (s *Service) ListSchedules(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	schedules, err := s.store.ListSchedules(r.Context(), user.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(schedules)
}

// GetSchedule handles GET /api/v1/schedules/{id}
func (s *Service) GetSchedule(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	sched, err := s.store.GetSchedule(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "schedule not found", http.StatusNotFound)
		return
	}

	if sched.UserID != user.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sched)
}

// UpdateSchedule handles PUT /api/v1/schedules/{id}
func (s *Service) UpdateSchedule(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	existing, err := s.store.GetSchedule(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "schedule not found", http.StatusNotFound)
		return
	}

	if existing.UserID != user.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	var update model.Schedule
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Apply updates
	if update.Name != "" {
		existing.Name = update.Name
	}
	if update.CronExpr != "" {
		existing.CronExpr = update.CronExpr
	}
	if update.Timezone != "" {
		existing.Timezone = update.Timezone
	}
	if update.Input != "" {
		existing.Input = update.Input
	}
	if update.AgentID != "" {
		existing.AgentID = update.AgentID
	}
	if update.Runtime != "" {
		existing.Runtime = update.Runtime
	}
	existing.Enabled = update.Enabled

	// Recompute next run
	if s.scheduler != nil {
		next, err := s.scheduler.ComputeNextRun(existing.CronExpr, existing.Timezone)
		if err != nil {
			http.Error(w, "invalid cron expression: "+err.Error(), http.StatusBadRequest)
			return
		}
		existing.NextRunAt = &next
	}

	if err := s.store.UpdateSchedule(r.Context(), existing); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(existing)
}

// DeleteSchedule handles DELETE /api/v1/schedules/{id}
func (s *Service) DeleteSchedule(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	existing, err := s.store.GetSchedule(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "schedule not found", http.StatusNotFound)
		return
	}

	if existing.UserID != user.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if err := s.store.DeleteSchedule(r.Context(), r.PathValue("id")); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// TriggerSchedule handles POST /api/v1/schedules/{id}/trigger
func (s *Service) TriggerSchedule(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	sched, err := s.store.GetSchedule(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, "schedule not found", http.StatusNotFound)
		return
	}

	if sched.UserID != user.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	if s.scheduler == nil {
		http.Error(w, "scheduler not available", http.StatusServiceUnavailable)
		return
	}

	run, err := s.scheduler.TriggerSchedule(r.Context(), sched)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(run)
}
