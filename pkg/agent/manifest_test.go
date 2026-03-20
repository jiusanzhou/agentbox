package agent

import (
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Helper: build a minimal valid manifest
// ---------------------------------------------------------------------------

func validManifest() Manifest {
	return Manifest{
		ID:          "my-agent",
		Name:        "My Agent",
		Version:     "1.0.0",
		Description: "A test agent",
		Persona: &Persona{
			Style: "professional",
			Tone:  "concise",
		},
	}
}

// ---------------------------------------------------------------------------
// TestParse
// ---------------------------------------------------------------------------

func TestParse(t *testing.T) {
	t.Run("valid yaml", func(t *testing.T) {
		yaml := `
id: test-agent
name: Test Agent
version: "1.0.0"
description: A simple test agent
persona:
  style: friendly
  tone: casual
`
		m, err := Parse([]byte(yaml))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if m.ID != "test-agent" {
			t.Errorf("ID = %q, want test-agent", m.ID)
		}
		if m.Name != "Test Agent" {
			t.Errorf("Name = %q, want Test Agent", m.Name)
		}
		if m.Version != "1.0.0" {
			t.Errorf("Version = %q, want 1.0.0", m.Version)
		}
		if m.Persona == nil || m.Persona.Style != "friendly" {
			t.Errorf("Persona.Style mismatch")
		}
	})

	t.Run("invalid yaml syntax", func(t *testing.T) {
		_, err := Parse([]byte(`{{{not yaml at all`))
		if err == nil {
			t.Fatal("expected error for invalid YAML")
		}
	})

	t.Run("yaml that fails validation", func(t *testing.T) {
		yaml := `
id: ""
name: ""
`
		_, err := Parse([]byte(yaml))
		if err == nil {
			t.Fatal("expected validation error")
		}
	})

	t.Run("valid yaml with all optional fields", func(t *testing.T) {
		yaml := `
id: full-agent
name: Full
version: "2.0.0"
description: Full agent
emoji: "🤖"
author: zoe
license: MIT
persona:
  style: analytical
  tone: formal
  language: [en, zh]
  principles: [be precise]
skills:
  - name: coding
    version: "^1.0"
adapters:
  frameworks:
    - name: openclaw
      native: true
  tools:
    required:
      - name: exec
model:
  minimum: sonnet
  recommended: opus
experience:
  level: senior
  packs: 10
marketplace:
  category: development
  tags: [rust, go]
  pricing:
    model: free
`
		m, err := Parse([]byte(yaml))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if m.Emoji != "🤖" {
			t.Errorf("Emoji = %q", m.Emoji)
		}
		if len(m.Skills) != 1 {
			t.Errorf("skills len = %d", len(m.Skills))
		}
		if m.Adapters == nil || len(m.Adapters.Frameworks) != 1 {
			t.Error("adapters.frameworks mismatch")
		}
	})
}

// ---------------------------------------------------------------------------
// TestValidate
// ---------------------------------------------------------------------------

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Manifest)
		wantErr string
	}{
		{"valid", func(m *Manifest) {}, ""},
		{"empty id", func(m *Manifest) { m.ID = "" }, "id is required"},
		{"invalid id (uppercase)", func(m *Manifest) { m.ID = "MyAgent" }, "invalid id"},
		{"invalid id (single char)", func(m *Manifest) { m.ID = "a" }, "invalid id"},
		{"empty name", func(m *Manifest) { m.Name = "" }, "name is required"},
		{"long name", func(m *Manifest) { m.Name = strings.Repeat("x", 65) }, "name too long"},
		{"name exactly 64", func(m *Manifest) { m.Name = strings.Repeat("x", 64) }, ""},
		{"empty version", func(m *Manifest) { m.Version = "" }, "version is required"},
		{"invalid version", func(m *Manifest) { m.Version = "abc" }, "invalid version"},
		{"empty description", func(m *Manifest) { m.Description = "" }, "description is required"},
		{"description too long", func(m *Manifest) { m.Description = strings.Repeat("x", 501) }, "description too long"},
		{"nil persona", func(m *Manifest) { m.Persona = nil }, "persona is required"},
		{"missing persona style", func(m *Manifest) { m.Persona.Style = "" }, "persona.style is required"},
		{"missing persona tone", func(m *Manifest) { m.Persona.Tone = "" }, "persona.tone is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := validManifest()
			tt.mutate(&m)
			err := m.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("expected error")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error = %q, want substring %q", err, tt.wantErr)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TestValidateExperience
// ---------------------------------------------------------------------------

func TestValidateExperience(t *testing.T) {
	validLevels := []string{"junior", "mid", "senior", "expert"}
	for _, lvl := range validLevels {
		t.Run("valid level "+lvl, func(t *testing.T) {
			m := validManifest()
			m.Experience = &Experience{Level: lvl}
			if err := m.Validate(); err != nil {
				t.Fatalf("unexpected error for level %q: %v", lvl, err)
			}
		})
	}

	t.Run("invalid level", func(t *testing.T) {
		m := validManifest()
		m.Experience = &Experience{Level: "wizard"}
		err := m.Validate()
		if err == nil || !strings.Contains(err.Error(), "invalid experience level") {
			t.Fatalf("error = %v, want 'invalid experience level'", err)
		}
	})

	t.Run("empty level is valid", func(t *testing.T) {
		m := validManifest()
		m.Experience = &Experience{Level: ""}
		if err := m.Validate(); err != nil {
			t.Fatalf("empty level should be valid: %v", err)
		}
	})

	t.Run("highlight without id", func(t *testing.T) {
		m := validManifest()
		m.Experience = &Experience{
			Highlights: []ExperienceHighlight{{ID: "", Summary: "did stuff"}},
		}
		err := m.Validate()
		if err == nil || !strings.Contains(err.Error(), "highlight requires id and summary") {
			t.Fatalf("error = %v, want highlight validation error", err)
		}
	})

	t.Run("highlight without summary", func(t *testing.T) {
		m := validManifest()
		m.Experience = &Experience{
			Highlights: []ExperienceHighlight{{ID: "h1", Summary: ""}},
		}
		err := m.Validate()
		if err == nil || !strings.Contains(err.Error(), "highlight requires id and summary") {
			t.Fatalf("error = %v", err)
		}
	})

	t.Run("valid highlight", func(t *testing.T) {
		m := validManifest()
		m.Experience = &Experience{
			Highlights: []ExperienceHighlight{{ID: "h1", Summary: "did stuff"}},
		}
		if err := m.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// TestValidatePricing
// ---------------------------------------------------------------------------

func TestValidatePricing(t *testing.T) {
	validModels := []string{"free", "one-time", "subscription", "usage"}
	for _, pm := range validModels {
		t.Run("valid model "+pm, func(t *testing.T) {
			m := validManifest()
			m.Marketplace = &Marketplace{Pricing: &Pricing{Model: pm}}
			if err := m.Validate(); err != nil {
				t.Fatalf("unexpected error for model %q: %v", pm, err)
			}
		})
	}

	t.Run("empty model is valid", func(t *testing.T) {
		m := validManifest()
		m.Marketplace = &Marketplace{Pricing: &Pricing{Model: ""}}
		if err := m.Validate(); err != nil {
			t.Fatalf("empty model should be valid: %v", err)
		}
	})

	t.Run("invalid model", func(t *testing.T) {
		m := validManifest()
		m.Marketplace = &Marketplace{Pricing: &Pricing{Model: "barter"}}
		err := m.Validate()
		if err == nil || !strings.Contains(err.Error(), "invalid pricing model") {
			t.Fatalf("error = %v, want 'invalid pricing model'", err)
		}
	})

	t.Run("nil pricing with marketplace is valid", func(t *testing.T) {
		m := validManifest()
		m.Marketplace = &Marketplace{Tags: []string{"dev"}}
		if err := m.Validate(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

// ---------------------------------------------------------------------------
// TestPreferredFramework
// ---------------------------------------------------------------------------

func TestPreferredFramework(t *testing.T) {
	t.Run("native framework returned", func(t *testing.T) {
		m := &Manifest{Adapters: &Adapters{
			Frameworks: []FrameworkRef{
				{Name: "langchain", Native: false},
				{Name: "openclaw", Native: true},
			},
		}}
		if got := m.PreferredFramework(); got != "openclaw" {
			t.Errorf("got %q, want openclaw", got)
		}
	})

	t.Run("no native returns first", func(t *testing.T) {
		m := &Manifest{Adapters: &Adapters{
			Frameworks: []FrameworkRef{
				{Name: "langchain"},
				{Name: "crewai"},
			},
		}}
		if got := m.PreferredFramework(); got != "langchain" {
			t.Errorf("got %q, want langchain", got)
		}
	})

	t.Run("nil adapters", func(t *testing.T) {
		m := &Manifest{}
		if got := m.PreferredFramework(); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})

	t.Run("empty frameworks list", func(t *testing.T) {
		m := &Manifest{Adapters: &Adapters{}}
		if got := m.PreferredFramework(); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}

// ---------------------------------------------------------------------------
// TestRequiredTools
// ---------------------------------------------------------------------------

func TestRequiredTools(t *testing.T) {
	t.Run("with tools", func(t *testing.T) {
		m := &Manifest{Adapters: &Adapters{Tools: &ToolRequirements{
			Required: []ToolRef{{Name: "exec"}, {Name: "read"}},
		}}}
		tools := m.RequiredTools()
		if len(tools) != 2 || tools[0] != "exec" || tools[1] != "read" {
			t.Errorf("RequiredTools = %v, want [exec read]", tools)
		}
	})

	t.Run("nil adapters", func(t *testing.T) {
		m := &Manifest{}
		if tools := m.RequiredTools(); tools != nil {
			t.Errorf("RequiredTools = %v, want nil", tools)
		}
	})

	t.Run("nil tools", func(t *testing.T) {
		m := &Manifest{Adapters: &Adapters{}}
		if tools := m.RequiredTools(); tools != nil {
			t.Errorf("RequiredTools = %v, want nil", tools)
		}
	})

	t.Run("empty required list", func(t *testing.T) {
		m := &Manifest{Adapters: &Adapters{Tools: &ToolRequirements{}}}
		tools := m.RequiredTools()
		if len(tools) != 0 {
			t.Errorf("RequiredTools = %v, want empty", tools)
		}
	})
}

// ---------------------------------------------------------------------------
// TestIDPattern / TestVersionPattern
// ---------------------------------------------------------------------------

func TestIDPattern(t *testing.T) {
	valid := []string{
		"ab", "my-agent", "agent.v2", "a0", "rust-proxy-expert",
		"a_b", "a-b.c", "a1234567890123456789012345678901234567890123456789012345678901b",
	}
	for _, id := range valid {
		t.Run("valid/"+id, func(t *testing.T) {
			if !idPattern.MatchString(id) {
				t.Errorf("idPattern should match %q", id)
			}
		})
	}

	invalid := []string{
		"", "a", "A", "UPPER", "-start", ".start", "_start",
		"end-", "end.", "end_",
		"has space", "has@symbol",
		strings.Repeat("a", 65), // > 64 chars
	}
	for _, id := range invalid {
		name := id
		if name == "" {
			name = "<empty>"
		}
		t.Run("invalid/"+name, func(t *testing.T) {
			if idPattern.MatchString(id) {
				t.Errorf("idPattern should NOT match %q", id)
			}
		})
	}
}

func TestVersionPattern(t *testing.T) {
	valid := []string{
		"0.0.1", "1.0.0", "1.2.3", "0.1.0",
		"1.0.0-alpha", "1.0.0-beta.1", "2.0.0-rc.1",
	}
	for _, v := range valid {
		t.Run("valid/"+v, func(t *testing.T) {
			if !versionPattern.MatchString(v) {
				t.Errorf("versionPattern should match %q", v)
			}
		})
	}

	invalid := []string{
		"", "abc", "1.0", "1", "v1.0.0", "1.0.0.0",
		"1.0.0-", "1.0.0-beta!",
	}
	for _, v := range invalid {
		name := v
		if name == "" {
			name = "<empty>"
		}
		t.Run("invalid/"+name, func(t *testing.T) {
			if versionPattern.MatchString(v) {
				t.Errorf("versionPattern should NOT match %q", v)
			}
		})
	}
}
