package agent

import (
	"encoding/json"
	"testing"

	"go.zoe.im/agentbox/internal/model"
)

// ---------------------------------------------------------------------------
// Helper: build a fully-populated manifest for conversion tests.
// ---------------------------------------------------------------------------

func fullManifest() *Manifest {
	return &Manifest{
		ID:          "test-agent",
		Name:        "Test Agent",
		Version:     "1.2.3",
		Description: "A fully-featured test agent",
		Author:      "zoe",
		License:     "MIT",
		Persona: &Persona{
			Style:      "INTJ",
			Tone:       "concise",
			Language:   []string{"en", "zh"},
			Principles: []string{"be precise", "stay focused"},
		},
		Skills: []SkillRef{
			{Name: "coding-agent", Version: "^1.0"},
			{Name: "review-agent"},
		},
		Adapters: &Adapters{
			Frameworks: []FrameworkRef{
				{Name: "openclaw", Native: true},
				{Name: "langchain"},
			},
			Tools: &ToolRequirements{
				Required: []ToolRef{{Name: "exec", Reason: "run code"}},
			},
		},
		Model: &ModelRequirements{
			Minimum:       "sonnet",
			Recommended:   "opus",
			ContextWindow: "200k",
		},
		Experience: &Experience{
			Level:   "senior",
			Packs:   10,
			Domains: []string{"Rust", "Go"},
		},
		Marketplace: &Marketplace{
			Tags:    []string{"rust", "dev"},
			Pricing: &Pricing{Model: "subscription"},
		},
		Runtime: &RuntimeRequirements{
			Platform: []string{"linux", "darwin"},
			Sandbox:  "required",
		},
	}
}

// ---------------------------------------------------------------------------
// TestToDNA
// ---------------------------------------------------------------------------

func TestToDNA(t *testing.T) {
	m := fullManifest()
	dna := m.ToDNA()

	// Basic identity mapping.
	if dna.Slug != "test-agent" {
		t.Errorf("Slug = %q, want test-agent", dna.Slug)
	}
	if dna.Version != "1.2.3" {
		t.Errorf("Version = %q, want 1.2.3", dna.Version)
	}
	if dna.Identity == nil {
		t.Fatal("Identity is nil")
	}
	if dna.Identity.Name != "Test Agent" {
		t.Errorf("Identity.Name = %q", dna.Identity.Name)
	}
	if dna.Identity.Description != "A fully-featured test agent" {
		t.Errorf("Identity.Description = %q", dna.Identity.Description)
	}

	// Manifest metadata.
	if dna.Manifest == nil {
		t.Fatal("Manifest is nil")
	}
	if dna.Manifest.Version != "1.2.3" {
		t.Errorf("Manifest.Version = %q", dna.Manifest.Version)
	}
	if dna.Manifest.Author != "zoe" {
		t.Errorf("Manifest.Author = %q", dna.Manifest.Author)
	}
	if dna.Manifest.License != "MIT" {
		t.Errorf("Manifest.License = %q", dna.Manifest.License)
	}

	// Soul from persona.
	if dna.Soul == nil {
		t.Fatal("Soul is nil")
	}
	if dna.Soul.Personality != "INTJ" {
		t.Errorf("Soul.Personality = %q", dna.Soul.Personality)
	}
	if dna.Soul.Tone != "concise" {
		t.Errorf("Soul.Tone = %q", dna.Soul.Tone)
	}
	if len(dna.Soul.Values) != 2 || dna.Soul.Values[0] != "be precise" {
		t.Errorf("Soul.Values = %v", dna.Soul.Values)
	}

	// Skills JSON.
	if len(dna.Skills) == 0 {
		t.Error("Skills JSON should not be empty")
	}
	var skills []SkillRef
	if err := json.Unmarshal(dna.Skills, &skills); err != nil {
		t.Fatalf("unmarshal skills: %v", err)
	}
	if len(skills) != 2 || skills[0].Name != "coding-agent" {
		t.Errorf("skills = %+v", skills)
	}

	// Tools JSON from adapters.
	if len(dna.Tools) == 0 {
		t.Error("Tools JSON should not be empty")
	}

	// Memory JSON from experience.
	if len(dna.Memory) == 0 {
		t.Error("Memory JSON should not be empty")
	}

	// Runtime from preferred framework.
	if dna.Manifest.Runtime != "openclaw" {
		t.Errorf("Manifest.Runtime = %q, want openclaw", dna.Manifest.Runtime)
	}

	// Marketplace tags and pricing.
	if len(dna.Manifest.Tags) != 2 || dna.Manifest.Tags[0] != "rust" {
		t.Errorf("Manifest.Tags = %v", dna.Manifest.Tags)
	}
	if dna.Manifest.PricingModel != "subscription" {
		t.Errorf("Manifest.PricingModel = %q", dna.Manifest.PricingModel)
	}
	if dna.Manifest.Currency != "USD" {
		t.Errorf("Manifest.Currency = %q", dna.Manifest.Currency)
	}

	// Model requirements.
	if dna.Manifest.Requirements == nil {
		t.Fatal("Manifest.Requirements is nil")
	}
	if dna.Manifest.Requirements["model_minimum"] != "sonnet" {
		t.Errorf("model_minimum = %q", dna.Manifest.Requirements["model_minimum"])
	}
	if dna.Manifest.Requirements["model_recommended"] != "opus" {
		t.Errorf("model_recommended = %q", dna.Manifest.Requirements["model_recommended"])
	}
	if dna.Manifest.Requirements["context_window"] != "200k" {
		t.Errorf("context_window = %q", dna.Manifest.Requirements["context_window"])
	}

	// Runtime requirements.
	if dna.Manifest.Requirements["sandbox"] != "required" {
		t.Errorf("sandbox = %q", dna.Manifest.Requirements["sandbox"])
	}
}

// ---------------------------------------------------------------------------
// TestFromDNA
// ---------------------------------------------------------------------------

func TestFromDNA(t *testing.T) {
	dna := &model.AgentDNA{
		Slug:    "from-dna",
		Version: "2.0.0",
		Identity: &model.AgentIdentity{
			Name:        "From DNA",
			Description: "Reconstructed",
		},
		Manifest: &model.AgentManifest{
			Version:      "2.0.0",
			Author:       "zoe",
			License:      "Apache-2.0",
			Runtime:      "langchain",
			Tags:         []string{"python"},
			PricingModel: "usage",
			Requirements: map[string]string{
				"model_minimum":     "haiku",
				"model_recommended": "sonnet",
				"context_window":    "128k",
			},
		},
		Soul: &model.AgentSoul{
			Personality: "analytical",
			Tone:        "formal",
			Values:      []string{"accuracy"},
		},
		Downloads: 42,
		Rating:    4.8,
	}

	m := FromDNA(dna)

	if m.ID != "from-dna" {
		t.Errorf("ID = %q", m.ID)
	}
	if m.Version != "2.0.0" {
		t.Errorf("Version = %q", m.Version)
	}
	if m.Name != "From DNA" {
		t.Errorf("Name = %q", m.Name)
	}
	if m.Author != "zoe" {
		t.Errorf("Author = %q", m.Author)
	}
	if m.License != "Apache-2.0" {
		t.Errorf("License = %q", m.License)
	}
	if m.Persona == nil || m.Persona.Style != "analytical" {
		t.Error("Persona.Style mismatch")
	}
	if m.Persona.Tone != "formal" {
		t.Errorf("Persona.Tone = %q", m.Persona.Tone)
	}
	if len(m.Persona.Principles) != 1 || m.Persona.Principles[0] != "accuracy" {
		t.Errorf("Persona.Principles = %v", m.Persona.Principles)
	}

	// Adapters from runtime.
	if m.Adapters == nil || len(m.Adapters.Frameworks) != 1 {
		t.Fatal("Adapters.Frameworks mismatch")
	}
	if m.Adapters.Frameworks[0].Name != "langchain" || !m.Adapters.Frameworks[0].Native {
		t.Errorf("framework = %+v", m.Adapters.Frameworks[0])
	}

	// Marketplace.
	if m.Marketplace == nil {
		t.Fatal("Marketplace is nil")
	}
	if len(m.Marketplace.Tags) != 1 || m.Marketplace.Tags[0] != "python" {
		t.Errorf("Tags = %v", m.Marketplace.Tags)
	}
	if m.Marketplace.Pricing == nil || m.Marketplace.Pricing.Model != "usage" {
		t.Error("Pricing.Model mismatch")
	}
	if m.Marketplace.Stats == nil {
		t.Fatal("Stats is nil")
	}
	if m.Marketplace.Stats.Users != 42 {
		t.Errorf("Stats.Users = %d", m.Marketplace.Stats.Users)
	}
	if m.Marketplace.Stats.Rating != 4.8 {
		t.Errorf("Stats.Rating = %f", m.Marketplace.Stats.Rating)
	}

	// Model requirements.
	if m.Model == nil {
		t.Fatal("Model is nil")
	}
	if m.Model.Minimum != "haiku" {
		t.Errorf("Model.Minimum = %q", m.Model.Minimum)
	}
	if m.Model.Recommended != "sonnet" {
		t.Errorf("Model.Recommended = %q", m.Model.Recommended)
	}
}

// ---------------------------------------------------------------------------
// TestToDNA_NilFields
// ---------------------------------------------------------------------------

func TestToDNA_NilFields(t *testing.T) {
	t.Run("nil persona", func(t *testing.T) {
		m := &Manifest{ID: "x", Version: "1.0.0"}
		dna := m.ToDNA()
		if dna.Soul != nil {
			t.Error("Soul should be nil when persona is nil")
		}
	})

	t.Run("nil adapters", func(t *testing.T) {
		m := &Manifest{ID: "x", Version: "1.0.0"}
		dna := m.ToDNA()
		if len(dna.Tools) != 0 {
			t.Error("Tools should be empty when adapters is nil")
		}
		if dna.Manifest.Runtime != "" {
			t.Errorf("Runtime = %q, want empty", dna.Manifest.Runtime)
		}
	})

	t.Run("nil experience", func(t *testing.T) {
		m := &Manifest{ID: "x", Version: "1.0.0"}
		dna := m.ToDNA()
		if len(dna.Memory) != 0 {
			t.Error("Memory should be empty when experience is nil")
		}
	})

	t.Run("nil marketplace", func(t *testing.T) {
		m := &Manifest{ID: "x", Version: "1.0.0"}
		dna := m.ToDNA()
		if len(dna.Manifest.Tags) != 0 {
			t.Error("Tags should be empty when marketplace is nil")
		}
	})

	t.Run("nil model", func(t *testing.T) {
		m := &Manifest{ID: "x", Version: "1.0.0"}
		dna := m.ToDNA()
		if dna.Manifest.Requirements != nil {
			t.Error("Requirements should be nil when model is nil")
		}
	})

	t.Run("nil runtime", func(t *testing.T) {
		m := &Manifest{ID: "x", Version: "1.0.0"}
		dna := m.ToDNA()
		// Requirements should only be created if model or runtime is non-nil.
		if dna.Manifest.Requirements != nil {
			t.Errorf("Requirements = %v, want nil", dna.Manifest.Requirements)
		}
	})
}

// ---------------------------------------------------------------------------
// TestFromDNA_NilFields
// ---------------------------------------------------------------------------

func TestFromDNA_NilFields(t *testing.T) {
	t.Run("nil identity", func(t *testing.T) {
		dna := &model.AgentDNA{Slug: "x", Version: "1.0.0"}
		m := FromDNA(dna)
		if m.Name != "" {
			t.Errorf("Name = %q, want empty", m.Name)
		}
	})

	t.Run("nil manifest", func(t *testing.T) {
		dna := &model.AgentDNA{
			Slug:    "x",
			Version: "1.0.0",
			Identity: &model.AgentIdentity{
				Name: "X",
			},
		}
		m := FromDNA(dna)
		if m.Author != "" {
			t.Errorf("Author = %q, want empty", m.Author)
		}
		if m.Adapters != nil {
			t.Error("Adapters should be nil when manifest.runtime is empty")
		}
	})

	t.Run("nil soul", func(t *testing.T) {
		dna := &model.AgentDNA{Slug: "x", Version: "1.0.0"}
		m := FromDNA(dna)
		if m.Persona != nil {
			t.Error("Persona should be nil when soul is nil")
		}
	})
}

// ---------------------------------------------------------------------------
// TestRoundTrip
// ---------------------------------------------------------------------------

func TestRoundTrip(t *testing.T) {
	orig := fullManifest()
	dna := orig.ToDNA()
	rebuilt := FromDNA(dna)

	// Key fields that should survive the round trip.
	checks := []struct {
		name string
		got  string
		want string
	}{
		{"ID", rebuilt.ID, orig.ID},
		{"Version", rebuilt.Version, orig.Version},
		{"Name", rebuilt.Name, orig.Name},
		{"Author", rebuilt.Author, orig.Author},
		{"License", rebuilt.License, orig.License},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}

	// Persona survives.
	if rebuilt.Persona == nil {
		t.Fatal("Persona is nil after round trip")
	}
	if rebuilt.Persona.Style != orig.Persona.Style {
		t.Errorf("Persona.Style = %q, want %q", rebuilt.Persona.Style, orig.Persona.Style)
	}
	if rebuilt.Persona.Tone != orig.Persona.Tone {
		t.Errorf("Persona.Tone = %q, want %q", rebuilt.Persona.Tone, orig.Persona.Tone)
	}

	// Model requirements survive.
	if rebuilt.Model == nil {
		t.Fatal("Model is nil after round trip")
	}
	if rebuilt.Model.Minimum != "sonnet" {
		t.Errorf("Model.Minimum = %q", rebuilt.Model.Minimum)
	}
	if rebuilt.Model.Recommended != "opus" {
		t.Errorf("Model.Recommended = %q", rebuilt.Model.Recommended)
	}

	// Marketplace tags survive.
	if rebuilt.Marketplace == nil {
		t.Fatal("Marketplace is nil after round trip")
	}
	if len(rebuilt.Marketplace.Tags) != 2 {
		t.Errorf("Marketplace.Tags = %v", rebuilt.Marketplace.Tags)
	}
}
