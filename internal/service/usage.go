package service

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"go.zoe.im/agentbox/internal/auth"
	"go.zoe.im/agentbox/internal/model"
)

// GetPlatformUsageSummary handles GET /api/v1/usage/summary
func (s *Service) GetPlatformUsageSummary(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	period := r.URL.Query().Get("period")
	if period == "" {
		period = time.Now().Format("2006-01")
	}

	summary, err := s.store.GetPlatformUsageSummary(r.Context(), user.ID, period)
	if err != nil {
		http.Error(w, `{"error":"failed to get usage summary"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

// GetPlatformUsageHistory handles GET /api/v1/usage/history
func (s *Service) GetPlatformUsageHistory(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	limitStr := r.URL.Query().Get("limit")

	from := time.Now().AddDate(0, -1, 0)
	to := time.Now()
	limit := 50

	if fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			from = t
		} else if t, err := time.Parse("2006-01-02", fromStr); err == nil {
			from = t
		}
	}
	if toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			to = t
		} else if t, err := time.Parse("2006-01-02", toStr); err == nil {
			to = t.Add(24*time.Hour - time.Second)
		}
	}
	if limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 {
			limit = n
		}
	}

	records, err := s.store.GetPlatformUsageHistory(r.Context(), user.ID, from, to, limit)
	if err != nil {
		http.Error(w, `{"error":"failed to get usage history"}`, http.StatusInternalServerError)
		return
	}

	if records == nil {
		records = []model.PlatformUsageRecord{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(records)
}

// GetPlatformUsageQuota handles GET /api/v1/usage/quota
func (s *Service) GetPlatformUsageQuota(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	quota, err := s.store.GetUsageQuota(r.Context(), user.ID)
	if err != nil {
		http.Error(w, `{"error":"failed to get quota"}`, http.StatusInternalServerError)
		return
	}

	period := time.Now().Format("2006-01")
	summary, err := s.store.GetPlatformUsageSummary(r.Context(), user.ID, period)
	if err != nil {
		http.Error(w, `{"error":"failed to get usage summary"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"quota": quota,
		"usage": summary,
		"remaining": map[string]any{
			"compute_seconds": quota.ComputeLimit - summary.ComputeSeconds,
			"token_count":     quota.TokenLimit - summary.TokenCount,
			"storage_bytes":   quota.StorageLimit - summary.StorageBytes,
			"api_calls":       quota.APICallLimit - summary.APICalls,
		},
	})
}

// GetUsageDashboard handles GET /api/v1/usage/dashboard
func (s *Service) GetUsageDashboard(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	period := time.Now().Format("2006-01")

	daily, err := s.store.GetDailyUsage(r.Context(), user.ID, period)
	if err != nil {
		http.Error(w, `{"error":"failed to get daily usage"}`, http.StatusInternalServerError)
		return
	}

	if daily == nil {
		daily = []model.DailyUsage{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"period": period,
		"daily":  daily,
	})
}
