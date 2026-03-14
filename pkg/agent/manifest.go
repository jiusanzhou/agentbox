// Package agent provides types and utilities for the Agent Manifest (agent.yaml) spec.
package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"sigs.k8s.io/yaml"
)

// Manifest is the top-level agent.yaml structure.
type Manifest struct {
	ID          string        `json:"id" yaml:"id"`
	Name        string        `json:"name" yaml:"name"`
	Version     string        `json:"version" yaml:"version"`
	Description string        `json:"description" yaml:"description"`
	Emoji       string        `json:"emoji,omitempty" yaml:"emoji,omitempty"`
	Avatar      string        `json:"avatar,omitempty" yaml:"avatar,omitempty"`
	Author      string        `json:"author,omitempty" yaml:"author,omitempty"`
	License     string        `json:"license,omitempty" yaml:"license,omitempty"`
	Homepage    string        `json:"homepage,omitempty" yaml:"homepage,omitempty"`
	Repository  string        `json:"repository,omitempty" yaml:"repository,omitempty"`

	Persona       *Persona              `json:"persona" yaml:"persona"`
	Skills        []SkillRef            `json:"skills,omitempty" yaml:"skills,omitempty"`
	Adapters      *Adapters             `json:"adapters,omitempty" yaml:"adapters,omitempty"`
	Model         *ModelRequirements    `json:"model,omitempty" yaml:"model,omitempty"`
	Experience    *Experience           `json:"experience,omitempty" yaml:"experience,omitempty"`
	Collaboration *Collaboration        `json:"collaboration,omitempty" yaml:"collaboration,omitempty"`
	Runtime       *RuntimeRequirements  `json:"runtime,omitempty" yaml:"runtime,omitempty"`
	Marketplace   *Marketplace          `json:"marketplace,omitempty" yaml:"marketplace,omitempty"`
}

// Persona defines the agent's personality and communication style.
type Persona struct {
	Style      string   `json:"style" yaml:"style"`
	Language   []string `json:"language,omitempty" yaml:"language,omitempty"`
	Tone       string   `json:"tone" yaml:"tone"`
	Principles []string `json:"principles,omitempty" yaml:"principles,omitempty"`
}

// SkillRef references a skill with optional version constraint.
type SkillRef struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
}

// Adapters defines tool and framework compatibility.
type Adapters struct {
	Frameworks []FrameworkRef `json:"frameworks,omitempty" yaml:"frameworks,omitempty"`
	Tools      *ToolRequirements `json:"tools,omitempty" yaml:"tools,omitempty"`
	AgentApps  []AgentAppRef  `json:"agent_apps,omitempty" yaml:"agent_apps,omitempty"`
	Services   []ServiceDep   `json:"services,omitempty" yaml:"services,omitempty"`
}

// FrameworkRef references a supported agent framework.
type FrameworkRef struct {
	Name    string `json:"name" yaml:"name"`
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	Native  bool   `json:"native,omitempty" yaml:"native,omitempty"`
	Adapter string `json:"adapter,omitempty" yaml:"adapter,omitempty"`
}

// ToolRequirements categorizes tools by necessity.
type ToolRequirements struct {
	Required    []ToolRef `json:"required,omitempty" yaml:"required,omitempty"`
	Recommended []ToolRef `json:"recommended,omitempty" yaml:"recommended,omitempty"`
	Optional    []ToolRef `json:"optional,omitempty" yaml:"optional,omitempty"`
}

// ToolRef references a tool with optional reason.
type ToolRef struct {
	Name   string `json:"name" yaml:"name"`
	Reason string `json:"reason,omitempty" yaml:"reason,omitempty"`
}

// AgentAppRef references a sub-agent app.
type AgentAppRef struct {
	Name         string   `json:"name" yaml:"name"`
	Role         string   `json:"role" yaml:"role"`
	Required     bool     `json:"required,omitempty" yaml:"required,omitempty"`
	Alternatives []string `json:"alternatives,omitempty" yaml:"alternatives,omitempty"`
}

// ServiceDep describes an external service dependency.
type ServiceDep struct {
	Name    string `json:"name" yaml:"name"`
	Type    string `json:"type" yaml:"type"` // api, runtime, database, storage
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	Auth    string `json:"auth,omitempty" yaml:"auth,omitempty"`
}

// ModelRequirements describes LLM model needs.
type ModelRequirements struct {
	Minimum       string `json:"minimum,omitempty" yaml:"minimum,omitempty"`
	Recommended   string `json:"recommended,omitempty" yaml:"recommended,omitempty"`
	ContextWindow string `json:"context_window,omitempty" yaml:"context_window,omitempty"`
}

// Experience describes accumulated expertise.
type Experience struct {
	Level      string                `json:"level,omitempty" yaml:"level,omitempty"` // junior, mid, senior, expert
	Packs      int                   `json:"packs,omitempty" yaml:"packs,omitempty"`
	Domains    []string              `json:"domains,omitempty" yaml:"domains,omitempty"`
	Highlights []ExperienceHighlight `json:"highlights,omitempty" yaml:"highlights,omitempty"`
}

// ExperienceHighlight is a notable experience entry.
type ExperienceHighlight struct {
	ID         string `json:"id" yaml:"id"`
	Summary    string `json:"summary" yaml:"summary"`
	Difficulty string `json:"difficulty,omitempty" yaml:"difficulty,omitempty"` // basic, intermediate, advanced, expert
}

// Collaboration describes multi-agent interaction capabilities.
type Collaboration struct {
	CanDelegate bool     `json:"can_delegate,omitempty" yaml:"can_delegate,omitempty"`
	CanReceive  bool     `json:"can_receive,omitempty" yaml:"can_receive,omitempty"`
	Protocols   []string `json:"protocols,omitempty" yaml:"protocols,omitempty"`
}

// RuntimeRequirements describes platform and dependency needs.
type RuntimeRequirements struct {
	Platform     []string `json:"platform,omitempty" yaml:"platform,omitempty"`
	Dependencies []string `json:"dependencies,omitempty" yaml:"dependencies,omitempty"`
	Sandbox      string   `json:"sandbox,omitempty" yaml:"sandbox,omitempty"` // required, recommended, optional
}

// Marketplace contains market-facing metadata.
type Marketplace struct {
	Category string   `json:"category,omitempty" yaml:"category,omitempty"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Pricing  *Pricing `json:"pricing,omitempty" yaml:"pricing,omitempty"`
	Stats    *Stats   `json:"stats,omitempty" yaml:"stats,omitempty"`
}

// Pricing describes the agent's pricing model.
type Pricing struct {
	Model string `json:"model,omitempty" yaml:"model,omitempty"` // free, one-time, subscription, usage
	Base  string `json:"base,omitempty" yaml:"base,omitempty"`
	Trial int    `json:"trial,omitempty" yaml:"trial,omitempty"`
}

// Stats holds platform-managed statistics (read-only for authors).
type Stats struct {
	Users          int     `json:"users,omitempty" yaml:"users,omitempty"`
	Rating         float64 `json:"rating,omitempty" yaml:"rating,omitempty"`
	TasksCompleted int     `json:"tasks_completed,omitempty" yaml:"tasks_completed,omitempty"`
}

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,62}[a-z0-9]$`)
var versionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(-[a-zA-Z0-9.]+)?$`)

// ParseFile reads and parses an agent.yaml file.
func ParseFile(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read agent manifest: %w", err)
	}
	return Parse(data)
}

// Parse parses agent.yaml content from bytes.
func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse agent manifest: %w", err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Validate checks required fields and format constraints.
func (m *Manifest) Validate() error {
	if m.ID == "" {
		return fmt.Errorf("agent manifest: id is required")
	}
	if !idPattern.MatchString(m.ID) {
		return fmt.Errorf("agent manifest: invalid id %q (must match %s)", m.ID, idPattern.String())
	}
	if m.Name == "" {
		return fmt.Errorf("agent manifest: name is required")
	}
	if len(m.Name) > 64 {
		return fmt.Errorf("agent manifest: name too long (max 64)")
	}
	if m.Version == "" {
		return fmt.Errorf("agent manifest: version is required")
	}
	if !versionPattern.MatchString(m.Version) {
		return fmt.Errorf("agent manifest: invalid version %q (must be semver)", m.Version)
	}
	if m.Description == "" {
		return fmt.Errorf("agent manifest: description is required")
	}
	if len(m.Description) > 500 {
		return fmt.Errorf("agent manifest: description too long (max 500)")
	}
	if m.Persona == nil {
		return fmt.Errorf("agent manifest: persona is required")
	}
	if m.Persona.Style == "" {
		return fmt.Errorf("agent manifest: persona.style is required")
	}
	if m.Persona.Tone == "" {
		return fmt.Errorf("agent manifest: persona.tone is required")
	}
	if m.Experience != nil {
		if err := m.Experience.validate(); err != nil {
			return err
		}
	}
	if m.Marketplace != nil && m.Marketplace.Pricing != nil {
		if err := m.Marketplace.Pricing.validate(); err != nil {
			return err
		}
	}
	return nil
}

func (e *Experience) validate() error {
	validLevels := map[string]bool{"junior": true, "mid": true, "senior": true, "expert": true}
	if e.Level != "" && !validLevels[e.Level] {
		return fmt.Errorf("agent manifest: invalid experience level %q", e.Level)
	}
	for _, h := range e.Highlights {
		if h.ID == "" || h.Summary == "" {
			return fmt.Errorf("agent manifest: experience highlight requires id and summary")
		}
	}
	return nil
}

func (p *Pricing) validate() error {
	validModels := map[string]bool{"free": true, "one-time": true, "subscription": true, "usage": true}
	if p.Model != "" && !validModels[p.Model] {
		return fmt.Errorf("agent manifest: invalid pricing model %q", p.Model)
	}
	return nil
}

// PreferredFramework returns the first native framework, or the first listed.
func (m *Manifest) PreferredFramework() string {
	if m.Adapters == nil || len(m.Adapters.Frameworks) == 0 {
		return ""
	}
	for _, f := range m.Adapters.Frameworks {
		if f.Native {
			return f.Name
		}
	}
	return m.Adapters.Frameworks[0].Name
}

// RequiredTools returns all required tool names.
func (m *Manifest) RequiredTools() []string {
	if m.Adapters == nil || m.Adapters.Tools == nil {
		return nil
	}
	names := make([]string, len(m.Adapters.Tools.Required))
	for i, t := range m.Adapters.Tools.Required {
		names[i] = t.Name
	}
	return names
}

// FindWorkspaceFiles returns expected workspace file paths relative to a root dir.
func (m *Manifest) FindWorkspaceFiles(root string) []string {
	files := []string{
		filepath.Join(root, "agent.yaml"),
		filepath.Join(root, "SOUL.md"),
		filepath.Join(root, "AGENTS.md"),
	}
	if m.Avatar != "" {
		files = append(files, filepath.Join(root, m.Avatar))
	}
	return files
}
