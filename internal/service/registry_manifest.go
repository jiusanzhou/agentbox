package service

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"go.zoe.im/agentbox/internal/auth"
	"go.zoe.im/agentbox/internal/model"
	"go.zoe.im/agentbox/pkg/agent"
)

// UploadAgentManifest handles POST /api/v1/registry/agents/upload
// Accepts multipart form with agent.yaml and optional workspace files.
// Parses the manifest, converts to AgentDNA, and creates/updates in store.
func (s *Service) UploadAgentManifest(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	// Parse multipart form (max 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"parse form: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	// Read agent.yaml (required)
	manifestData, err := readFormFile(r, "agent.yaml")
	if err != nil {
		http.Error(w, `{"error":"agent.yaml is required"}`, http.StatusBadRequest)
		return
	}

	// Parse and validate manifest
	manifest, err := agent.Parse(manifestData)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid agent.yaml: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	// Convert to AgentDNA
	dna := manifest.ToDNA()
	dna.UserID = user.ID

	// Read optional files and merge into DNA
	if soulData, err := readFormFile(r, "SOUL.md"); err == nil {
		if dna.Soul == nil {
			dna.Soul = &model.AgentSoul{}
		}
		dna.Soul.Voice = string(soulData) // Store raw SOUL.md in voice field for rendering
	}

	if agentsData, err := readFormFile(r, "AGENTS.md"); err == nil {
		if dna.Identity == nil {
			dna.Identity = &model.AgentIdentity{Name: manifest.Name}
		}
		dna.Identity.Role = string(agentsData) // Store raw AGENTS.md in role field
	}

	now := time.Now()

	// Check if slug already exists for this user (update path)
	existing, err := s.store.GetAgentDNABySlug(r.Context(), dna.Slug)
	if err == nil && existing.UserID == user.ID {
		// Update existing
		existing.Version = dna.Version
		existing.Identity = dna.Identity
		existing.Soul = dna.Soul
		existing.Tools = dna.Tools
		existing.Memory = dna.Memory
		existing.Skills = dna.Skills
		existing.Manifest = dna.Manifest
		existing.Status = model.AgentDNAStatusPublished
		existing.UpdatedAt = now
		if existing.PublishedAt == nil {
			existing.PublishedAt = &now
		}

		if err := s.store.UpdateAgentDNA(r.Context(), existing); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":   "updated",
			"agent":    existing,
			"manifest": manifest,
		})
		return
	}

	// Create new
	dna.ID = shortID()
	dna.Status = model.AgentDNAStatusPublished
	dna.CreatedAt = now
	dna.UpdatedAt = now
	dna.PublishedAt = &now

	if err := s.store.CreateAgentDNA(r.Context(), dna); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"status":   "created",
		"agent":    dna,
		"manifest": manifest,
	})
}

// GetAgentManifest handles GET /api/v1/registry/agents/{id}/manifest
// Returns the agent in agent.yaml manifest format.
func (s *Service) GetAgentManifest(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	dna, err := s.store.GetAgentDNA(r.Context(), id)
	if err != nil {
		dna, err = s.store.GetAgentDNABySlug(r.Context(), id)
		if err != nil {
			http.Error(w, `{"error":"agent not found"}`, http.StatusNotFound)
			return
		}
	}

	manifest := agent.FromDNA(dna)

	format := r.URL.Query().Get("format")
	if format == "yaml" {
		w.Header().Set("Content-Type", "application/yaml")
		// Use sigs.k8s.io/yaml to marshal
		data, err := json.Marshal(manifest)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}
		w.Write(data)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(manifest)
}

// SearchRegistryAgents handles GET /api/v1/registry/agents/search
// Enhanced search with manifest-aware filtering.
func (s *Service) SearchRegistryAgents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	opts := model.AgentDNAListOptions{
		Query:   q.Get("q"),
		Runtime: q.Get("runtime"),
		Tag:     q.Get("tag"),
		Status:  model.AgentDNAStatusPublished,
	}

	agents, err := s.store.ListAgentDNAs(r.Context(), opts)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Post-filter by manifest fields
	level := q.Get("level")       // junior, mid, senior, expert
	framework := q.Get("framework") // openclaw, claude-code, etc.
	minPacks := q.Get("min_packs")   // minimum experience packs

	var results []map[string]any
	for _, dna := range agents {
		manifest := agent.FromDNA(dna)

		// Filter by experience level
		if level != "" && manifest.Experience != nil {
			if manifest.Experience.Level != level {
				continue
			}
		}

		// Filter by framework
		if framework != "" && manifest.Adapters != nil {
			found := false
			for _, fw := range manifest.Adapters.Frameworks {
				if strings.EqualFold(fw.Name, framework) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// Filter by minimum experience packs
		if minPacks != "" && manifest.Experience != nil {
			var min int
			fmt.Sscanf(minPacks, "%d", &min)
			if manifest.Experience.Packs < min {
				continue
			}
		}

		result := map[string]any{
			"id":          dna.ID,
			"slug":        dna.Slug,
			"name":        dna.Identity.Name,
			"version":     dna.Version,
			"description": dna.Identity.Description,
			"status":      dna.Status,
			"downloads":   dna.Downloads,
			"rating":      dna.Rating,
			"created_at":  dna.CreatedAt,
			"manifest":    manifest,
		}
		results = append(results, result)
	}

	if results == nil {
		results = []map[string]any{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// readFormFile reads a named file from multipart form data.
func readFormFile(r *http.Request, name string) ([]byte, error) {
	// Try exact name first
	file, _, err := r.FormFile(name)
	if err != nil {
		// Try without extension for flexibility
		base := strings.TrimSuffix(name, filepath.Ext(name))
		file, _, err = r.FormFile(base)
		if err != nil {
			return nil, err
		}
	}
	defer func(f multipart.File) { f.Close() }(file)
	return io.ReadAll(file)
}
