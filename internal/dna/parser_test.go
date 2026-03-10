package dna

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseDir(t *testing.T) {
	// Create a temporary DNA directory
	dir := t.TempDir()

	// Write manifest.yaml
	manifestContent := `version: "1.0.0"
author: "test-author"
license: "MIT"
tags:
  - code-review
  - golang
runtime: claude
pricing_model: per_task
price_per_unit: 0.50
currency: USD
requirements:
  memory: "2GB"
`
	os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte(manifestContent), 0644)

	// Write AGENT.md
	agentContent := `# Code Reviewer

## Role
Senior code reviewer

## Description
Reviews code for quality, security, and best practices.

## Capabilities
- Static analysis
- Security scanning
- Performance review

## Constraints
- No code modification
- Read-only access

## Workflow
- Read the codebase
- Identify issues
- Generate report

## Guidelines
- Be constructive
- Cite specific lines
`
	os.WriteFile(filepath.Join(dir, "AGENT.md"), []byte(agentContent), 0644)

	// Write SOUL.md
	soulContent := `## Personality
Thorough and detail-oriented. Focuses on helping developers grow.

## Values
- Code quality
- Security first
- Developer experience

## Voice
Professional and encouraging

## Tone
Constructive and supportive
`
	os.WriteFile(filepath.Join(dir, "SOUL.md"), []byte(soulContent), 0644)

	// Write TOOLS.md
	os.WriteFile(filepath.Join(dir, "TOOLS.md"), []byte("# Tools\n\n- grep\n- ast-parser"), 0644)

	// Create MEMORY directory with files
	os.MkdirAll(filepath.Join(dir, "MEMORY"), 0755)
	os.WriteFile(filepath.Join(dir, "MEMORY", "KNOWLEDGE.md"), []byte("# Domain Knowledge\nGo best practices"), 0644)

	// Create SKILLS directory
	os.MkdirAll(filepath.Join(dir, "SKILLS"), 0755)
	os.WriteFile(filepath.Join(dir, "SKILLS", "lint.md"), []byte("# Lint Skill\nRun golangci-lint"), 0644)

	// Parse
	agent, err := ParseDir(dir)
	if err != nil {
		t.Fatalf("ParseDir: %v", err)
	}

	// Check manifest
	if agent.Manifest == nil {
		t.Fatal("manifest is nil")
	}
	if agent.Manifest.Version != "1.0.0" {
		t.Errorf("manifest version = %q, want %q", agent.Manifest.Version, "1.0.0")
	}
	if agent.Manifest.Author != "test-author" {
		t.Errorf("manifest author = %q, want %q", agent.Manifest.Author, "test-author")
	}
	if agent.Manifest.Runtime != "claude" {
		t.Errorf("manifest runtime = %q, want %q", agent.Manifest.Runtime, "claude")
	}
	if len(agent.Manifest.Tags) != 2 {
		t.Errorf("manifest tags count = %d, want 2", len(agent.Manifest.Tags))
	}
	if agent.Manifest.PricingModel != "per_task" {
		t.Errorf("manifest pricing_model = %q, want %q", agent.Manifest.PricingModel, "per_task")
	}

	// Check identity
	if agent.Identity == nil {
		t.Fatal("identity is nil")
	}
	if agent.Identity.Name != "Code Reviewer" {
		t.Errorf("identity name = %q, want %q", agent.Identity.Name, "Code Reviewer")
	}
	if agent.Identity.Role != "Senior code reviewer" {
		t.Errorf("identity role = %q, want %q", agent.Identity.Role, "Senior code reviewer")
	}
	if len(agent.Identity.Capabilities) != 3 {
		t.Errorf("capabilities count = %d, want 3", len(agent.Identity.Capabilities))
	}
	if len(agent.Identity.Constraints) != 2 {
		t.Errorf("constraints count = %d, want 2", len(agent.Identity.Constraints))
	}
	if len(agent.Identity.Workflow) != 3 {
		t.Errorf("workflow count = %d, want 3", len(agent.Identity.Workflow))
	}
	if len(agent.Identity.Guidelines) != 2 {
		t.Errorf("guidelines count = %d, want 2", len(agent.Identity.Guidelines))
	}

	// Check soul
	if agent.Soul == nil {
		t.Fatal("soul is nil")
	}
	if agent.Soul.Personality == "" {
		t.Error("soul personality is empty")
	}
	if len(agent.Soul.Values) != 3 {
		t.Errorf("soul values count = %d, want 3", len(agent.Soul.Values))
	}
	if agent.Soul.Voice != "Professional and encouraging" {
		t.Errorf("soul voice = %q, want %q", agent.Soul.Voice, "Professional and encouraging")
	}

	// Check tools, memory, skills are populated
	if len(agent.Tools) == 0 {
		t.Error("tools is empty")
	}
	if len(agent.Memory) == 0 {
		t.Error("memory is empty")
	}
	if len(agent.Skills) == 0 {
		t.Error("skills is empty")
	}

	// Check version propagated
	if agent.Version != "1.0.0" {
		t.Errorf("version = %q, want %q", agent.Version, "1.0.0")
	}
}

func TestParseDir_MissingManifest(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "AGENT.md"), []byte("# Test"), 0644)

	_, err := ParseDir(dir)
	if err == nil {
		t.Error("expected error for missing manifest.yaml")
	}
}

func TestParseDir_MissingAgentMD(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "manifest.yaml"), []byte("version: '1.0.0'\nauthor: test\nruntime: claude"), 0644)

	_, err := ParseDir(dir)
	if err == nil {
		t.Error("expected error for missing AGENT.md")
	}
}

func TestParseDir_NotADirectory(t *testing.T) {
	f, _ := os.CreateTemp("", "test")
	f.Close()
	defer os.Remove(f.Name())

	_, err := ParseDir(f.Name())
	if err == nil {
		t.Error("expected error for non-directory path")
	}
}
