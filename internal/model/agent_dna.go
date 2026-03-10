package model

import (
	"encoding/json"
	"time"
)

// AgentDNAStatus represents the lifecycle state of a registered agent.
type AgentDNAStatus string

const (
	AgentDNAStatusDraft     AgentDNAStatus = "draft"
	AgentDNAStatusPublished AgentDNAStatus = "published"
	AgentDNAStatusArchived  AgentDNAStatus = "archived"
)

// AgentDNA represents a publishable agent in the marketplace registry.
// It models the full agent DNA directory structure:
//
//	agent-dna/
//	├── AGENT.md        → Identity
//	├── SOUL.md         → Soul
//	├── TOOLS.md        → Tools (json)
//	├── MEMORY/         → Memory (json)
//	├── SKILLS/         → Skills (json)
//	└── manifest.yaml   → Manifest
type AgentDNA struct {
	ID      string `json:"id"`
	UserID  string `json:"user_id"`
	Slug    string `json:"slug"`    // unique URL-friendly identifier
	Version string `json:"version"` // semver

	// Core DNA components
	Identity *AgentIdentity  `json:"identity"`
	Soul     *AgentSoul      `json:"soul,omitempty"`
	Tools    json.RawMessage `json:"tools,omitempty"`
	Memory   json.RawMessage `json:"memory,omitempty"`
	Skills   json.RawMessage `json:"skills,omitempty"`
	Manifest *AgentManifest  `json:"manifest"`

	// Git backing
	RepoURL string `json:"repo_url,omitempty"`
	RepoRef string `json:"repo_ref,omitempty"` // branch, tag, or commit SHA

	// Registry metadata
	Status    AgentDNAStatus `json:"status"`
	Downloads int64          `json:"downloads"`
	Rating    float64        `json:"rating"`

	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
}

// AgentIdentity holds fields parsed from AGENT.md.
type AgentIdentity struct {
	Name         string   `json:"name" yaml:"name"`
	Role         string   `json:"role,omitempty" yaml:"role"`
	Description  string   `json:"description,omitempty" yaml:"description"`
	Capabilities []string `json:"capabilities,omitempty" yaml:"capabilities"`
	Constraints  []string `json:"constraints,omitempty" yaml:"constraints"`
	Workflow     []string `json:"workflow,omitempty" yaml:"workflow"`
	Guidelines   []string `json:"guidelines,omitempty" yaml:"guidelines"`
}

// AgentSoul holds fields parsed from SOUL.md.
type AgentSoul struct {
	Personality        string   `json:"personality,omitempty" yaml:"personality"`
	Values             []string `json:"values,omitempty" yaml:"values"`
	CommunicationStyle string   `json:"communication_style,omitempty" yaml:"communication_style"`
	Voice              string   `json:"voice,omitempty" yaml:"voice"`
	Tone               string   `json:"tone,omitempty" yaml:"tone"`
}

// AgentManifest holds metadata from manifest.yaml.
type AgentManifest struct {
	Version      string            `json:"version" yaml:"version"`
	Author       string            `json:"author" yaml:"author"`
	License      string            `json:"license,omitempty" yaml:"license"`
	Tags         []string          `json:"tags,omitempty" yaml:"tags"`
	Runtime      string            `json:"runtime" yaml:"runtime"`                 // claude, codex, gemini, etc.
	PricingModel string            `json:"pricing_model" yaml:"pricing_model"`     // per_task, per_minute, free
	PricePerUnit float64           `json:"price_per_unit" yaml:"price_per_unit"`   // in currency units
	Currency     string            `json:"currency,omitempty" yaml:"currency"`     // USD default
	Requirements map[string]string `json:"requirements,omitempty" yaml:"requirements"` // e.g., "memory": "2GB"
}

// AgentDNAListOptions holds query parameters for listing agents.
type AgentDNAListOptions struct {
	Status  AgentDNAStatus `json:"status,omitempty"`
	Runtime string         `json:"runtime,omitempty"`
	Tag     string         `json:"tag,omitempty"`
	Query   string         `json:"q,omitempty"` // free-text search
	Limit   int            `json:"limit,omitempty"`
	Offset  int            `json:"offset,omitempty"`
}
