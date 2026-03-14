package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseExample(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("schema", "examples", "rust-proxy-expert.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	m, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}

	if m.ID != "rust-proxy-expert" {
		t.Errorf("id = %q, want rust-proxy-expert", m.ID)
	}
	if m.Name != "锈刃" {
		t.Errorf("name = %q, want 锈刃", m.Name)
	}
	if m.Version != "1.0.0" {
		t.Errorf("version = %q, want 1.0.0", m.Version)
	}
	if m.Persona == nil {
		t.Fatal("persona is nil")
	}
	if m.Persona.Style == "" {
		t.Error("persona.style is empty")
	}
	if len(m.Persona.Language) != 2 {
		t.Errorf("persona.language len = %d, want 2", len(m.Persona.Language))
	}
	if len(m.Skills) != 3 {
		t.Errorf("skills len = %d, want 3", len(m.Skills))
	}
	if m.Adapters == nil {
		t.Fatal("adapters is nil")
	}
	if len(m.Adapters.Frameworks) != 2 {
		t.Errorf("frameworks len = %d, want 2", len(m.Adapters.Frameworks))
	}
	if m.Adapters.Tools == nil || len(m.Adapters.Tools.Required) != 4 {
		t.Errorf("required tools len unexpected")
	}
	if len(m.Adapters.AgentApps) != 1 {
		t.Errorf("agent_apps len = %d, want 1", len(m.Adapters.AgentApps))
	}
	if m.Model == nil || m.Model.Recommended != "opus" {
		t.Errorf("model.recommended = %v, want opus", m.Model)
	}
	if m.Experience == nil || m.Experience.Level != "senior" {
		t.Errorf("experience.level unexpected")
	}
	if m.Experience.Packs != 47 {
		t.Errorf("experience.packs = %d, want 47", m.Experience.Packs)
	}
	if len(m.Experience.Highlights) != 3 {
		t.Errorf("highlights len = %d, want 3", len(m.Experience.Highlights))
	}
	if m.Marketplace == nil || m.Marketplace.Pricing == nil {
		t.Fatal("marketplace.pricing is nil")
	}
	if m.Marketplace.Pricing.Trial != 10 {
		t.Errorf("pricing.trial = %d, want 10", m.Marketplace.Pricing.Trial)
	}
}

func TestPreferredFramework(t *testing.T) {
	m := &Manifest{
		Adapters: &Adapters{
			Frameworks: []FrameworkRef{
				{Name: "langchain", Native: false},
				{Name: "openclaw", Native: true},
			},
		},
	}
	if got := m.PreferredFramework(); got != "openclaw" {
		t.Errorf("PreferredFramework = %q, want openclaw", got)
	}
}

func TestPreferredFrameworkNoNative(t *testing.T) {
	m := &Manifest{
		Adapters: &Adapters{
			Frameworks: []FrameworkRef{
				{Name: "langchain"},
				{Name: "crewai"},
			},
		},
	}
	if got := m.PreferredFramework(); got != "langchain" {
		t.Errorf("PreferredFramework = %q, want langchain (first)", got)
	}
}

func TestPreferredFrameworkEmpty(t *testing.T) {
	m := &Manifest{}
	if got := m.PreferredFramework(); got != "" {
		t.Errorf("PreferredFramework = %q, want empty", got)
	}
}

func TestRequiredTools(t *testing.T) {
	m := &Manifest{
		Adapters: &Adapters{
			Tools: &ToolRequirements{
				Required: []ToolRef{
					{Name: "exec"},
					{Name: "read"},
				},
			},
		},
	}
	tools := m.RequiredTools()
	if len(tools) != 2 || tools[0] != "exec" || tools[1] != "read" {
		t.Errorf("RequiredTools = %v, want [exec read]", tools)
	}
}

func TestValidateRequired(t *testing.T) {
	tests := []struct {
		name string
		m    Manifest
		err  bool
	}{
		{"empty id", Manifest{}, true},
		{"bad id", Manifest{ID: "UPPER"}, true},
		{"short id", Manifest{ID: "a"}, true},
		{"no name", Manifest{ID: "ab"}, true},
		{"no version", Manifest{ID: "ab", Name: "x"}, true},
		{"bad version", Manifest{ID: "ab", Name: "x", Version: "abc"}, true},
		{"no desc", Manifest{ID: "ab", Name: "x", Version: "1.0.0"}, true},
		{"no persona", Manifest{ID: "ab", Name: "x", Version: "1.0.0", Description: "d"}, true},
		{"no style", Manifest{ID: "ab", Name: "x", Version: "1.0.0", Description: "d", Persona: &Persona{Tone: "t"}}, true},
		{"no tone", Manifest{ID: "ab", Name: "x", Version: "1.0.0", Description: "d", Persona: &Persona{Style: "s"}}, true},
		{"valid", Manifest{ID: "ab", Name: "x", Version: "1.0.0", Description: "d", Persona: &Persona{Style: "s", Tone: "t"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.m.Validate()
			if (err != nil) != tt.err {
				t.Errorf("Validate() error = %v, want err = %v", err, tt.err)
			}
		})
	}
}

func TestValidateExperienceLevel(t *testing.T) {
	m := Manifest{
		ID: "ab", Name: "x", Version: "1.0.0", Description: "d",
		Persona:    &Persona{Style: "s", Tone: "t"},
		Experience: &Experience{Level: "invalid"},
	}
	if err := m.Validate(); err == nil {
		t.Error("expected error for invalid experience level")
	}
}

func TestValidatePricingModel(t *testing.T) {
	m := Manifest{
		ID: "ab", Name: "x", Version: "1.0.0", Description: "d",
		Persona:     &Persona{Style: "s", Tone: "t"},
		Marketplace: &Marketplace{Pricing: &Pricing{Model: "invalid"}},
	}
	if err := m.Validate(); err == nil {
		t.Error("expected error for invalid pricing model")
	}
}
