package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.zoe.im/agentbox/internal/auth"
	"go.zoe.im/agentbox/internal/model"
)

// --- Agent Registry API endpoints ---

// ListRegistryAgents handles GET /api/v1/registry/agents
func (s *Service) ListRegistryAgents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	opts := model.AgentDNAListOptions{
		Query:   q.Get("q"),
		Runtime: q.Get("runtime"),
		Tag:     q.Get("tag"),
	}
	if status := q.Get("status"); status != "" {
		opts.Status = model.AgentDNAStatus(status)
	} else {
		opts.Status = model.AgentDNAStatusPublished // default to published
	}

	agents, err := s.store.ListAgentDNAs(r.Context(), opts)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	if agents == nil {
		agents = []*model.AgentDNA{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agents)
}

// GetRegistryAgent handles GET /api/v1/registry/agents/{id}
func (s *Service) GetRegistryAgent(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	// Try by ID first, then by slug
	agent, err := s.store.GetAgentDNA(r.Context(), id)
	if err != nil {
		agent, err = s.store.GetAgentDNABySlug(r.Context(), id)
		if err != nil {
			http.Error(w, `{"error":"agent not found"}`, http.StatusNotFound)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agent)
}

// CreateRegistryAgent handles POST /api/v1/registry/agents
func (s *Service) CreateRegistryAgent(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		Slug     string              `json:"slug"`
		Identity *model.AgentIdentity `json:"identity"`
		Soul     *model.AgentSoul     `json:"soul,omitempty"`
		Tools    json.RawMessage      `json:"tools,omitempty"`
		Memory   json.RawMessage      `json:"memory,omitempty"`
		Skills   json.RawMessage      `json:"skills,omitempty"`
		Manifest *model.AgentManifest `json:"manifest"`
		RepoURL  string               `json:"repo_url,omitempty"`
		RepoRef  string               `json:"repo_ref,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if req.Slug == "" {
		http.Error(w, `{"error":"slug is required"}`, http.StatusBadRequest)
		return
	}
	if req.Identity == nil || req.Identity.Name == "" {
		http.Error(w, `{"error":"identity.name is required"}`, http.StatusBadRequest)
		return
	}
	if req.Manifest == nil {
		http.Error(w, `{"error":"manifest is required"}`, http.StatusBadRequest)
		return
	}

	now := time.Now()
	version := req.Manifest.Version
	if version == "" {
		version = "0.1.0"
	}

	agent := &model.AgentDNA{
		ID:        shortID(),
		UserID:    user.ID,
		Slug:      req.Slug,
		Version:   version,
		Identity:  req.Identity,
		Soul:      req.Soul,
		Tools:     req.Tools,
		Memory:    req.Memory,
		Skills:    req.Skills,
		Manifest:  req.Manifest,
		RepoURL:   req.RepoURL,
		RepoRef:   req.RepoRef,
		Status:    model.AgentDNAStatusDraft,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.store.CreateAgentDNA(r.Context(), agent); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(agent)
}

// UpdateRegistryAgent handles PATCH /api/v1/registry/agents/{id}
func (s *Service) UpdateRegistryAgent(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	id := r.PathValue("id")
	agent, err := s.store.GetAgentDNA(r.Context(), id)
	if err != nil {
		agent, err = s.store.GetAgentDNABySlug(r.Context(), id)
		if err != nil {
			http.Error(w, `{"error":"agent not found"}`, http.StatusNotFound)
			return
		}
	}

	// Only the owner can update
	if agent.UserID != user.ID {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}

	var req struct {
		Identity *model.AgentIdentity  `json:"identity,omitempty"`
		Soul     *model.AgentSoul      `json:"soul,omitempty"`
		Tools    *json.RawMessage      `json:"tools,omitempty"`
		Memory   *json.RawMessage      `json:"memory,omitempty"`
		Skills   *json.RawMessage      `json:"skills,omitempty"`
		Manifest *model.AgentManifest  `json:"manifest,omitempty"`
		Status   *model.AgentDNAStatus `json:"status,omitempty"`
		RepoURL  *string               `json:"repo_url,omitempty"`
		RepoRef  *string               `json:"repo_ref,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if req.Identity != nil {
		agent.Identity = req.Identity
	}
	if req.Soul != nil {
		agent.Soul = req.Soul
	}
	if req.Tools != nil {
		agent.Tools = *req.Tools
	}
	if req.Memory != nil {
		agent.Memory = *req.Memory
	}
	if req.Skills != nil {
		agent.Skills = *req.Skills
	}
	if req.Manifest != nil {
		agent.Manifest = req.Manifest
		if req.Manifest.Version != "" {
			agent.Version = req.Manifest.Version
		}
	}
	if req.Status != nil {
		agent.Status = *req.Status
		if *req.Status == model.AgentDNAStatusPublished && agent.PublishedAt == nil {
			now := time.Now()
			agent.PublishedAt = &now
		}
	}
	if req.RepoURL != nil {
		agent.RepoURL = *req.RepoURL
	}
	if req.RepoRef != nil {
		agent.RepoRef = *req.RepoRef
	}

	agent.UpdatedAt = time.Now()

	if err := s.store.UpdateAgentDNA(r.Context(), agent); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agent)
}

// HireRegistryAgent handles POST /api/v1/registry/agents/{id}/hire
// This creates a new run using the agent's DNA as the agent_file.
func (s *Service) HireRegistryAgent(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	id := r.PathValue("id")
	agent, err := s.store.GetAgentDNA(r.Context(), id)
	if err != nil {
		agent, err = s.store.GetAgentDNABySlug(r.Context(), id)
		if err != nil {
			http.Error(w, `{"error":"agent not found"}`, http.StatusNotFound)
			return
		}
	}

	if agent.Status != model.AgentDNAStatusPublished {
		http.Error(w, `{"error":"agent is not published"}`, http.StatusBadRequest)
		return
	}

	var req struct {
		Message  string          `json:"message"`
		Config   model.RunConfig `json:"config,omitempty"`
		Executor string          `json:"executor,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	// Build agent file from DNA identity
	agentFile := buildAgentFile(agent)

	// Determine runtime from manifest
	rt := "claude"
	if agent.Manifest != nil && agent.Manifest.Runtime != "" {
		rt = agent.Manifest.Runtime
	}

	run := &model.Run{
		ID:        shortID(),
		UserID:    user.ID,
		Name:      agent.Identity.Name,
		Mode:      model.RunModeSession,
		Runtime:   rt,
		Executor:  req.Executor,
		AgentFile: agentFile,
		Config:    req.Config,
	}
	if run.Config.Timeout == 0 {
		run.Config.Timeout = 3600
	}

	if err := s.engine.StartSession(r.Context(), run); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Increment download counter
	_ = s.store.IncrementAgentDNADownloads(r.Context(), agent.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(run)
}

// buildAgentFile constructs an AGENTS.md-compatible string from AgentDNA.
func buildAgentFile(agent *model.AgentDNA) string {
	var b []byte

	b = append(b, fmt.Sprintf("# %s\n\n", agent.Identity.Name)...)

	if agent.Identity.Description != "" {
		b = append(b, fmt.Sprintf("%s\n\n", agent.Identity.Description)...)
	}

	if agent.Identity.Role != "" {
		b = append(b, fmt.Sprintf("## Role\n%s\n\n", agent.Identity.Role)...)
	}

	if len(agent.Identity.Capabilities) > 0 {
		b = append(b, "## Capabilities\n"...)
		for _, c := range agent.Identity.Capabilities {
			b = append(b, fmt.Sprintf("- %s\n", c)...)
		}
		b = append(b, '\n')
	}

	if len(agent.Identity.Constraints) > 0 {
		b = append(b, "## Constraints\n"...)
		for _, c := range agent.Identity.Constraints {
			b = append(b, fmt.Sprintf("- %s\n", c)...)
		}
		b = append(b, '\n')
	}

	if len(agent.Identity.Workflow) > 0 {
		b = append(b, "## Workflow\n"...)
		for _, w := range agent.Identity.Workflow {
			b = append(b, fmt.Sprintf("- %s\n", w)...)
		}
		b = append(b, '\n')
	}

	if len(agent.Identity.Guidelines) > 0 {
		b = append(b, "## Guidelines\n"...)
		for _, g := range agent.Identity.Guidelines {
			b = append(b, fmt.Sprintf("- %s\n", g)...)
		}
		b = append(b, '\n')
	}

	// Append soul as a section
	if agent.Soul != nil {
		if agent.Soul.Personality != "" {
			b = append(b, fmt.Sprintf("## Personality\n%s\n\n", agent.Soul.Personality)...)
		}
		if agent.Soul.Voice != "" {
			b = append(b, fmt.Sprintf("## Voice\n%s\n\n", agent.Soul.Voice)...)
		}
		if agent.Soul.Tone != "" {
			b = append(b, fmt.Sprintf("## Tone\n%s\n\n", agent.Soul.Tone)...)
		}
	}

	return string(b)
}
