package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.zoe.im/agentbox/internal/auth"
	"go.zoe.im/agentbox/pkg/agent"
)

// ListAgentExperience handles GET /api/v1/registry/agents/{id}/experience
// Returns experience packs for an agent.
func (s *Service) ListAgentExperience(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	dna, err := s.store.GetAgentDNA(r.Context(), id)
	if err != nil {
		dna, err = s.store.GetAgentDNABySlug(r.Context(), id)
		if err != nil {
			http.Error(w, `{"error":"agent not found"}`, http.StatusNotFound)
			return
		}
	}

	// Reconstruct experience from DNA memory field
	manifest := agent.FromDNA(dna)
	if manifest.Experience == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"agent_id":   dna.Slug,
			"level":      "",
			"packs":      0,
			"highlights": []any{},
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"agent_id":   dna.Slug,
		"level":      manifest.Experience.Level,
		"packs":      manifest.Experience.Packs,
		"domains":    manifest.Experience.Domains,
		"highlights": manifest.Experience.Highlights,
	})
}

// AddAgentExperience handles POST /api/v1/registry/agents/{id}/experience
// Adds experience packs to an agent (owner only).
func (s *Service) AddAgentExperience(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	id := r.PathValue("id")
	dna, err := s.store.GetAgentDNA(r.Context(), id)
	if err != nil {
		dna, err = s.store.GetAgentDNABySlug(r.Context(), id)
		if err != nil {
			http.Error(w, `{"error":"agent not found"}`, http.StatusNotFound)
			return
		}
	}

	if dna.UserID != user.ID {
		http.Error(w, `{"error":"forbidden: only the owner can add experience"}`, http.StatusForbidden)
		return
	}

	var req struct {
		Packs []agent.ExperiencePack `json:"packs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if len(req.Packs) == 0 {
		http.Error(w, `{"error":"at least one experience pack is required"}`, http.StatusBadRequest)
		return
	}

	// Validate and L1-sanitize each pack
	var sanitized []agent.ExperiencePack
	for _, p := range req.Packs {
		if err := p.Validate(); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"pack %s: %s"}`, p.ID, err.Error()), http.StatusBadRequest)
			return
		}
		agent.SanitizeExperiencePack(&p)
		sanitized = append(sanitized, p)
	}

	// Merge into DNA memory field
	manifest := agent.FromDNA(dna)
	if manifest.Experience == nil {
		manifest.Experience = &agent.Experience{}
	}

	// Add new highlights
	for _, p := range sanitized {
		manifest.Experience.Highlights = append(manifest.Experience.Highlights, agent.ExperienceHighlight{
			ID:         p.ID,
			Summary:    p.Summary,
			Difficulty: p.Difficulty,
		})
	}
	manifest.Experience.Packs = len(manifest.Experience.Highlights)

	// Update level based on pack count
	manifest.Experience.Level = computeLevel(manifest.Experience.Packs)

	// Merge tags into domains
	domainSet := make(map[string]bool)
	for _, d := range manifest.Experience.Domains {
		domainSet[d] = true
	}
	for _, p := range sanitized {
		if p.Domain != "" && !domainSet[p.Domain] {
			manifest.Experience.Domains = append(manifest.Experience.Domains, p.Domain)
			domainSet[p.Domain] = true
		}
	}

	// Write back to DNA
	if data, err := json.Marshal(manifest.Experience); err == nil {
		dna.Memory = data
	}
	dna.UpdatedAt = time.Now()

	if err := s.store.UpdateAgentDNA(r.Context(), dna); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"added":  len(sanitized),
		"total":  manifest.Experience.Packs,
		"level":  manifest.Experience.Level,
	})
}

// computeLevel determines agent level from experience pack count.
func computeLevel(packs int) string {
	switch {
	case packs >= 200:
		return "expert"
	case packs >= 51:
		return "senior"
	case packs >= 11:
		return "mid"
	case packs >= 1:
		return "junior"
	default:
		return ""
	}
}

// SanitizePreview handles POST /api/v1/experience/sanitize
// Applies L1 sanitization to text and returns preview (no storage).
func (s *Service) SanitizePreview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	sanitized, applied := agent.SanitizeL1Text(req.Text)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"original":   req.Text,
		"sanitized":  sanitized,
		"redactions": applied,
		"changed":    req.Text != sanitized,
	})
}
