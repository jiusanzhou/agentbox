package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.zoe.im/agentbox/internal/auth"
	"go.zoe.im/agentbox/internal/dna"
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

// PublishRegistryAgent handles POST /api/v1/registry/agents/publish
// Accepts a full DNA payload, validates it, and creates or updates the agent.
// If the slug already exists for this user, bumps the version and updates.
// Sets status to published.
func (s *Service) PublishRegistryAgent(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		Slug     string               `json:"slug"`
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

	// Build AgentDNA for validation
	agent := &model.AgentDNA{
		Slug:     req.Slug,
		Identity: req.Identity,
		Soul:     req.Soul,
		Tools:    req.Tools,
		Memory:   req.Memory,
		Skills:   req.Skills,
		Manifest: req.Manifest,
	}
	if req.Manifest != nil {
		agent.Version = req.Manifest.Version
	}

	// Validate
	if err := dna.Validate(agent); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusBadRequest)
		return
	}

	now := time.Now()

	// Check if slug already exists for this user (update path)
	existing, err := s.store.GetAgentDNABySlug(r.Context(), req.Slug)
	if err == nil && existing.UserID == user.ID {
		// Update existing: bump version if same as existing
		if existing.Version == agent.Version {
			bumped, bErr := dna.BumpVersion(existing.Version)
			if bErr != nil {
				http.Error(w, fmt.Sprintf(`{"error":"version bump failed: %s"}`, bErr.Error()), http.StatusInternalServerError)
				return
			}
			agent.Version = bumped
			agent.Manifest.Version = bumped
		}

		existing.Identity = agent.Identity
		existing.Soul = agent.Soul
		existing.Tools = agent.Tools
		existing.Memory = agent.Memory
		existing.Skills = agent.Skills
		existing.Manifest = agent.Manifest
		existing.Version = agent.Version
		existing.RepoURL = req.RepoURL
		existing.RepoRef = req.RepoRef
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
		json.NewEncoder(w).Encode(existing)
		return
	}

	// Create new
	agent.ID = shortID()
	agent.UserID = user.ID
	agent.Status = model.AgentDNAStatusPublished
	agent.CreatedAt = now
	agent.UpdatedAt = now
	agent.PublishedAt = &now
	agent.RepoURL = req.RepoURL
	agent.RepoRef = req.RepoRef

	if err := s.store.CreateAgentDNA(r.Context(), agent); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(agent)
}

// WebhookGitPush handles POST /api/v1/registry/webhooks/git
// Receives GitHub/generic git push webhook events and auto-publishes agent DNA.
// The webhook payload must include the repository URL and ref.
// Expects a webhook secret configured via AGENTBOX_WEBHOOK_SECRET env var.
func (s *Service) WebhookGitPush(w http.ResponseWriter, r *http.Request) {
	// Read body
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20)) // 1MB limit
	if err != nil {
		http.Error(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
		return
	}

	// Verify webhook signature if secret is configured
	if secret := s.cfg.WebhookSecret; secret != "" {
		sig := r.Header.Get("X-Hub-Signature-256")
		if sig == "" {
			http.Error(w, `{"error":"missing signature"}`, http.StatusUnauthorized)
			return
		}
		if !verifyGitHubSignature(secret, body, sig) {
			http.Error(w, `{"error":"invalid signature"}`, http.StatusUnauthorized)
			return
		}
	}

	// Parse webhook payload (GitHub push event format)
	var payload struct {
		Ref        string `json:"ref"`         // refs/heads/main, refs/tags/v1.0.0
		Repository struct {
			CloneURL string `json:"clone_url"`
			FullName string `json:"full_name"`
		} `json:"repository"`
		HeadCommit struct {
			ID string `json:"id"`
		} `json:"head_commit"`
		// Agent DNA metadata embedded by convention in the push payload
		// or parsed from the repo after clone.
		AgentDNA *struct {
			Slug     string               `json:"slug"`
			Identity *model.AgentIdentity `json:"identity"`
			Soul     *model.AgentSoul     `json:"soul,omitempty"`
			Tools    json.RawMessage      `json:"tools,omitempty"`
			Memory   json.RawMessage      `json:"memory,omitempty"`
			Skills   json.RawMessage      `json:"skills,omitempty"`
			Manifest *model.AgentManifest `json:"manifest"`
		} `json:"agent_dna,omitempty"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if payload.Repository.CloneURL == "" {
		http.Error(w, `{"error":"repository.clone_url is required"}`, http.StatusBadRequest)
		return
	}

	// Extract version from tag if this is a tag push
	var tagVersion string
	if strings.HasPrefix(payload.Ref, "refs/tags/") {
		tag := strings.TrimPrefix(payload.Ref, "refs/tags/")
		tag = strings.TrimPrefix(tag, "v") // v1.0.0 -> 1.0.0
		tagVersion = tag
	}

	// If agent_dna is provided inline, use it directly
	if payload.AgentDNA != nil && payload.AgentDNA.Slug != "" {
		agent := &model.AgentDNA{
			Slug:     payload.AgentDNA.Slug,
			Identity: payload.AgentDNA.Identity,
			Soul:     payload.AgentDNA.Soul,
			Tools:    payload.AgentDNA.Tools,
			Memory:   payload.AgentDNA.Memory,
			Skills:   payload.AgentDNA.Skills,
			Manifest: payload.AgentDNA.Manifest,
			RepoURL:  payload.Repository.CloneURL,
			RepoRef:  payload.HeadCommit.ID,
		}

		if tagVersion != "" && agent.Manifest != nil {
			agent.Manifest.Version = tagVersion
		}
		if agent.Manifest != nil {
			agent.Version = agent.Manifest.Version
		}

		if err := dna.Validate(agent); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusBadRequest)
			return
		}

		now := time.Now()

		// Try to find existing by slug
		existing, err := s.store.GetAgentDNABySlug(r.Context(), agent.Slug)
		if err == nil {
			// Update existing
			if existing.Version == agent.Version {
				bumped, _ := dna.BumpVersion(existing.Version)
				agent.Version = bumped
				agent.Manifest.Version = bumped
			}
			existing.Identity = agent.Identity
			existing.Soul = agent.Soul
			existing.Tools = agent.Tools
			existing.Memory = agent.Memory
			existing.Skills = agent.Skills
			existing.Manifest = agent.Manifest
			existing.Version = agent.Version
			existing.RepoURL = agent.RepoURL
			existing.RepoRef = agent.RepoRef
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
				"status":  "updated",
				"agent":   existing,
				"version": existing.Version,
			})
			return
		}

		// Create new (no user context for webhook — use repo name as user_id placeholder)
		agent.ID = shortID()
		agent.UserID = payload.Repository.FullName
		agent.Status = model.AgentDNAStatusPublished
		agent.CreatedAt = now
		agent.UpdatedAt = now
		agent.PublishedAt = &now

		if err := s.store.CreateAgentDNA(r.Context(), agent); err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "created",
			"agent":   agent,
			"version": agent.Version,
		})
		return
	}

	// No inline agent_dna — acknowledge the webhook for future processing
	// (full git clone + parse would be async)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status":  "acknowledged",
		"message": "webhook received; include agent_dna in payload for auto-publish, or use aboxctl publish locally",
	})
}

// verifyGitHubSignature verifies a GitHub webhook signature (sha256).
func verifyGitHubSignature(secret string, body []byte, signature string) bool {
	sig := strings.TrimPrefix(signature, "sha256=")
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sig))
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
