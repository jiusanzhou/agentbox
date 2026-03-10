package dna

import (
	"testing"

	"go.zoe.im/agentbox/internal/model"
)

func TestValidate_Valid(t *testing.T) {
	agent := &model.AgentDNA{
		Identity: &model.AgentIdentity{Name: "Test Agent"},
		Manifest: &model.AgentManifest{
			Version: "1.0.0",
			Author:  "test",
			Runtime: "claude",
		},
		Version: "1.0.0",
	}
	if err := Validate(agent); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestValidate_MissingIdentity(t *testing.T) {
	agent := &model.AgentDNA{
		Manifest: &model.AgentManifest{
			Version: "1.0.0",
			Author:  "test",
			Runtime: "claude",
		},
	}
	err := Validate(agent)
	if err == nil {
		t.Fatal("expected error for missing identity")
	}
	ve, ok := err.(*ValidationError)
	if !ok {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	if len(ve.Errors) == 0 {
		t.Fatal("expected at least one error")
	}
}

func TestValidate_MissingManifest(t *testing.T) {
	agent := &model.AgentDNA{
		Identity: &model.AgentIdentity{Name: "Test"},
	}
	err := Validate(agent)
	if err == nil {
		t.Fatal("expected error for missing manifest")
	}
}

func TestValidate_InvalidSemver(t *testing.T) {
	agent := &model.AgentDNA{
		Identity: &model.AgentIdentity{Name: "Test"},
		Manifest: &model.AgentManifest{
			Version: "not-semver",
			Author:  "test",
			Runtime: "claude",
		},
		Version: "not-semver",
	}
	err := Validate(agent)
	if err == nil {
		t.Fatal("expected error for invalid semver")
	}
}

func TestValidate_InvalidRuntime(t *testing.T) {
	agent := &model.AgentDNA{
		Identity: &model.AgentIdentity{Name: "Test"},
		Manifest: &model.AgentManifest{
			Version: "1.0.0",
			Author:  "test",
			Runtime: "unsupported",
		},
		Version: "1.0.0",
	}
	err := Validate(agent)
	if err == nil {
		t.Fatal("expected error for unsupported runtime")
	}
}

func TestValidate_MissingAuthor(t *testing.T) {
	agent := &model.AgentDNA{
		Identity: &model.AgentIdentity{Name: "Test"},
		Manifest: &model.AgentManifest{
			Version: "1.0.0",
			Runtime: "claude",
		},
		Version: "1.0.0",
	}
	err := Validate(agent)
	if err == nil {
		t.Fatal("expected error for missing author")
	}
}

func TestBumpVersion(t *testing.T) {
	tests := []struct {
		prev string
		want string
	}{
		{"", "0.1.0"},
		{"1.0.0", "1.0.1"},
		{"0.1.0", "0.1.1"},
		{"2.3.4", "2.3.5"},
	}
	for _, tt := range tests {
		got, err := BumpVersion(tt.prev)
		if err != nil {
			t.Errorf("BumpVersion(%q) error: %v", tt.prev, err)
			continue
		}
		if got != tt.want {
			t.Errorf("BumpVersion(%q) = %q, want %q", tt.prev, got, tt.want)
		}
	}
}

func TestBumpVersion_Invalid(t *testing.T) {
	_, err := BumpVersion("not-semver")
	if err == nil {
		t.Error("expected error for invalid semver")
	}
}
