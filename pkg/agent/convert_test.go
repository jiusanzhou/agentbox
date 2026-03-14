package agent

import (
	"testing"
)

func TestToDNARoundTrip(t *testing.T) {
	m := &Manifest{
		ID:          "test-agent",
		Name:        "Test",
		Version:     "1.0.0",
		Description: "A test agent",
		Author:      "zoe",
		License:     "MIT",
		Persona: &Persona{
			Style:      "INTJ",
			Tone:       "concise",
			Language:   []string{"zh", "en"},
			Principles: []string{"be precise"},
		},
		Skills: []SkillRef{
			{Name: "coding-agent", Version: "^1.0"},
		},
		Adapters: &Adapters{
			Frameworks: []FrameworkRef{
				{Name: "openclaw", Native: true},
			},
			Tools: &ToolRequirements{
				Required: []ToolRef{{Name: "exec"}},
			},
		},
		Model: &ModelRequirements{
			Minimum:     "sonnet",
			Recommended: "opus",
		},
		Experience: &Experience{
			Level:   "senior",
			Packs:   10,
			Domains: []string{"Rust"},
		},
		Marketplace: &Marketplace{
			Tags:    []string{"rust", "dev"},
			Pricing: &Pricing{Model: "subscription"},
		},
	}

	dna := m.ToDNA()

	// Verify DNA fields
	if dna.Slug != "test-agent" {
		t.Errorf("slug = %q, want test-agent", dna.Slug)
	}
	if dna.Version != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", dna.Version)
	}
	if dna.Identity == nil || dna.Identity.Name != "Test" {
		t.Error("identity.name mismatch")
	}
	if dna.Soul == nil || dna.Soul.Personality != "INTJ" {
		t.Error("soul.personality mismatch")
	}
	if dna.Manifest == nil || dna.Manifest.Runtime != "openclaw" {
		t.Errorf("manifest.runtime = %q, want openclaw", dna.Manifest.Runtime)
	}
	if dna.Manifest.PricingModel != "subscription" {
		t.Errorf("pricing_model = %q, want subscription", dna.Manifest.PricingModel)
	}
	if len(dna.Skills) == 0 {
		t.Error("skills JSON is empty")
	}
	if len(dna.Tools) == 0 {
		t.Error("tools JSON is empty")
	}
	if len(dna.Memory) == 0 {
		t.Error("memory JSON is empty")
	}

	// Round trip back
	m2 := FromDNA(dna)
	if m2.ID != "test-agent" {
		t.Errorf("round trip id = %q, want test-agent", m2.ID)
	}
	if m2.Name != "Test" {
		t.Errorf("round trip name = %q, want Test", m2.Name)
	}
	if m2.Persona == nil || m2.Persona.Style != "INTJ" {
		t.Error("round trip persona mismatch")
	}
	if m2.Model == nil || m2.Model.Recommended != "opus" {
		t.Error("round trip model mismatch")
	}
}

func TestToDNAMinimal(t *testing.T) {
	m := &Manifest{
		ID:          "minimal",
		Name:        "Min",
		Version:     "0.1.0",
		Description: "Minimal agent",
		Persona: &Persona{
			Style: "friendly",
			Tone:  "warm",
		},
	}

	dna := m.ToDNA()
	if dna.Slug != "minimal" {
		t.Errorf("slug = %q", dna.Slug)
	}
	if dna.Soul == nil {
		t.Error("soul should not be nil")
	}

	// Round trip
	m2 := FromDNA(dna)
	if m2.ID != "minimal" {
		t.Errorf("round trip id = %q", m2.ID)
	}
}
