package dna

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.zoe.im/agentbox/internal/model"

	"gopkg.in/yaml.v3"
)

// ParseDir reads an agent DNA directory and returns the parsed AgentDNA.
// Expected structure:
//
//	dir/
//	├── AGENT.md          → Identity
//	├── SOUL.md           → Soul
//	├── TOOLS.md          → Tools
//	├── MEMORY/           → Memory files
//	├── SKILLS/           → Skill files
//	└── manifest.yaml     → Manifest
func ParseDir(dir string) (*model.AgentDNA, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("dna: stat %s: %w", dir, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("dna: %s is not a directory", dir)
	}

	agent := &model.AgentDNA{}

	// Parse manifest.yaml (required)
	manifest, err := parseManifest(filepath.Join(dir, "manifest.yaml"))
	if err != nil {
		// Try manifest.yml as fallback
		manifest, err = parseManifest(filepath.Join(dir, "manifest.yml"))
		if err != nil {
			return nil, fmt.Errorf("dna: manifest.yaml required: %w", err)
		}
	}
	agent.Manifest = manifest
	agent.Version = manifest.Version

	// Parse AGENT.md (required)
	identity, err := parseAgentMD(filepath.Join(dir, "AGENT.md"))
	if err != nil {
		return nil, fmt.Errorf("dna: AGENT.md required: %w", err)
	}
	agent.Identity = identity

	// Parse SOUL.md (optional)
	if soul, err := parseSoulMD(filepath.Join(dir, "SOUL.md")); err == nil {
		agent.Soul = soul
	}

	// Parse TOOLS.md (optional)
	if toolsContent, err := os.ReadFile(filepath.Join(dir, "TOOLS.md")); err == nil {
		tools := map[string]string{"content": string(toolsContent)}
		agent.Tools, _ = json.Marshal(tools)
	}

	// Collect MEMORY/ files (optional)
	if memFiles, err := collectDirFiles(filepath.Join(dir, "MEMORY")); err == nil && len(memFiles) > 0 {
		agent.Memory, _ = json.Marshal(memFiles)
	}

	// Collect SKILLS/ files (optional)
	if skillFiles, err := collectDirFiles(filepath.Join(dir, "SKILLS")); err == nil && len(skillFiles) > 0 {
		agent.Skills, _ = json.Marshal(skillFiles)
	}

	// Derive slug from directory name if not set
	agent.Slug = filepath.Base(dir)

	return agent, nil
}

func parseManifest(path string) (*model.AgentManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m model.AgentManifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if m.Version == "" {
		m.Version = "0.1.0"
	}
	return &m, nil
}

func parseAgentMD(path string) (*model.AgentIdentity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseIdentityFromMarkdown(string(data)), nil
}

// parseIdentityFromMarkdown extracts structured fields from AGENT.md markdown.
// Parses H1 as name, and H2 sections for role, description, capabilities, etc.
func parseIdentityFromMarkdown(content string) *model.AgentIdentity {
	id := &model.AgentIdentity{}
	lines := strings.Split(content, "\n")

	var currentSection string
	var descLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## ") {
			id.Name = strings.TrimPrefix(trimmed, "# ")
			currentSection = ""
			continue
		}

		if strings.HasPrefix(trimmed, "## ") {
			currentSection = strings.ToLower(strings.TrimPrefix(trimmed, "## "))
			continue
		}

		if trimmed == "" {
			continue
		}

		item := strings.TrimPrefix(trimmed, "- ")
		item = strings.TrimPrefix(item, "* ")

		switch {
		case strings.Contains(currentSection, "role"):
			if id.Role == "" {
				id.Role = item
			}
		case strings.Contains(currentSection, "description"):
			descLines = append(descLines, trimmed)
		case strings.Contains(currentSection, "capabilit"):
			id.Capabilities = append(id.Capabilities, item)
		case strings.Contains(currentSection, "constraint"):
			id.Constraints = append(id.Constraints, item)
		case strings.Contains(currentSection, "workflow"):
			id.Workflow = append(id.Workflow, item)
		case strings.Contains(currentSection, "guideline"):
			id.Guidelines = append(id.Guidelines, item)
		case strings.Contains(currentSection, "instruction"):
			descLines = append(descLines, trimmed)
		}
	}

	if len(descLines) > 0 {
		id.Description = strings.Join(descLines, "\n")
	}

	return id
}

func parseSoulMD(path string) (*model.AgentSoul, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseSoulFromMarkdown(string(data)), nil
}

func parseSoulFromMarkdown(content string) *model.AgentSoul {
	soul := &model.AgentSoul{}
	lines := strings.Split(content, "\n")

	var currentSection string
	var personalityLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "## ") {
			currentSection = strings.ToLower(strings.TrimPrefix(trimmed, "## "))
			continue
		}

		if trimmed == "" {
			continue
		}

		item := strings.TrimPrefix(trimmed, "- ")
		item = strings.TrimPrefix(item, "* ")

		switch {
		case strings.Contains(currentSection, "personalit"):
			personalityLines = append(personalityLines, trimmed)
		case strings.Contains(currentSection, "value"):
			soul.Values = append(soul.Values, item)
		case strings.Contains(currentSection, "communication") || strings.Contains(currentSection, "style"):
			if soul.CommunicationStyle == "" {
				soul.CommunicationStyle = item
			}
		case strings.Contains(currentSection, "voice"):
			if soul.Voice == "" {
				soul.Voice = item
			}
		case strings.Contains(currentSection, "tone"):
			if soul.Tone == "" {
				soul.Tone = item
			}
		}
	}

	if len(personalityLines) > 0 {
		soul.Personality = strings.Join(personalityLines, "\n")
	}

	return soul
}

// collectDirFiles reads all .md files in a directory and returns a map of filename → content.
func collectDirFiles(dir string) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	files := make(map[string]string)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		files[e.Name()] = string(data)
	}
	return files, nil
}
