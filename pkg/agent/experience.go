package agent

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"sigs.k8s.io/yaml"
)

// ExperiencePack represents a single sanitized experience unit.
type ExperiencePack struct {
	ID            string            `json:"id" yaml:"id"`
	Domain        string            `json:"domain" yaml:"domain"`
	Summary       string            `json:"summary" yaml:"summary"`
	Detail        string            `json:"detail" yaml:"detail"`
	Tags          []string          `json:"tags,omitempty" yaml:"tags,omitempty"`
	Difficulty    string            `json:"difficulty,omitempty" yaml:"difficulty,omitempty"`
	Verified      bool              `json:"verified,omitempty" yaml:"verified,omitempty"`
	AccumulatedAt string            `json:"accumulated_at,omitempty" yaml:"accumulated_at,omitempty"`
	Source        *ExperienceSource `json:"source,omitempty" yaml:"source,omitempty"`
	Sanitization  *SanitizationInfo `json:"sanitization,omitempty" yaml:"sanitization,omitempty"`
}

// ExperienceSource tracks provenance.
type ExperienceSource struct {
	Type        string `json:"type,omitempty" yaml:"type,omitempty"` // memory, manual, imported
	MemoryFile  string `json:"memory_file,omitempty" yaml:"memory_file,omitempty"`
	MemoryLines string `json:"memory_lines,omitempty" yaml:"memory_lines,omitempty"`
}

// SanitizationInfo records what sanitization was applied.
type SanitizationInfo struct {
	Level       string `json:"level" yaml:"level"` // L1, L2, L3
	SanitizedAt string `json:"sanitized_at,omitempty" yaml:"sanitized_at,omitempty"`
	Reviewed    bool   `json:"reviewed,omitempty" yaml:"reviewed,omitempty"`
	Reviewer    string `json:"reviewer,omitempty" yaml:"reviewer,omitempty"`
}

// ExperienceIndex is the index.yaml in an experience directory.
type ExperienceIndex struct {
	Agent   string            `json:"agent" yaml:"agent"`
	Version string            `json:"version" yaml:"version"`
	Packs   []ExperiencePack  `json:"packs" yaml:"packs"`
}

var expIDPattern = regexp.MustCompile(`^exp-[a-z0-9-]+$`)

// ValidateExperiencePack checks required fields.
func (e *ExperiencePack) Validate() error {
	if e.ID == "" {
		return fmt.Errorf("experience: id is required")
	}
	if !expIDPattern.MatchString(e.ID) {
		return fmt.Errorf("experience: invalid id %q (must match exp-xxx-nnn)", e.ID)
	}
	if e.Domain == "" {
		return fmt.Errorf("experience %s: domain is required", e.ID)
	}
	if e.Summary == "" {
		return fmt.Errorf("experience %s: summary is required", e.ID)
	}
	if len(e.Summary) > 200 {
		return fmt.Errorf("experience %s: summary too long (max 200)", e.ID)
	}
	if e.Detail == "" {
		return fmt.Errorf("experience %s: detail is required", e.ID)
	}
	validDiff := map[string]bool{"basic": true, "intermediate": true, "advanced": true, "expert": true, "": true}
	if !validDiff[e.Difficulty] {
		return fmt.Errorf("experience %s: invalid difficulty %q", e.ID, e.Difficulty)
	}
	return nil
}

// ParseExperiencePack parses a single experience pack from YAML bytes.
func ParseExperiencePack(data []byte) (*ExperiencePack, error) {
	var e ExperiencePack
	if err := yaml.Unmarshal(data, &e); err != nil {
		return nil, fmt.Errorf("parse experience pack: %w", err)
	}
	if err := e.Validate(); err != nil {
		return nil, err
	}
	return &e, nil
}

// ParseExperienceDir reads all experience packs from a directory.
func ParseExperienceDir(dir string) (*ExperienceIndex, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read experience dir: %w", err)
	}

	idx := &ExperienceIndex{}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if name == "index.yaml" || name == "index.json" {
			// Parse index
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err == nil {
				yaml.Unmarshal(data, idx)
			}
			continue
		}
		if !strings.HasSuffix(name, ".md") && !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			continue
		}

		// Try YAML parse first, then Markdown parse
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			pack, err := ParseExperiencePack(data)
			if err == nil {
				idx.Packs = append(idx.Packs, *pack)
			}
		} else {
			// Parse markdown experience
			pack, err := parseExperienceMarkdown(data, name)
			if err == nil {
				idx.Packs = append(idx.Packs, *pack)
			}
		}
	}

	return idx, nil
}

// parseExperienceMarkdown extracts experience from markdown format.
func parseExperienceMarkdown(data []byte, filename string) (*ExperiencePack, error) {
	content := string(data)
	lines := strings.Split(content, "\n")

	pack := &ExperiencePack{}

	var currentSection string
	var sectionContent []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "# ") {
			pack.Summary = strings.TrimPrefix(trimmed, "# ")
			continue
		}

		if strings.HasPrefix(trimmed, "## ") {
			// Flush previous section
			if currentSection != "" {
				flushExperienceSection(pack, currentSection, sectionContent)
			}
			currentSection = strings.ToLower(strings.TrimPrefix(trimmed, "## "))
			sectionContent = nil
			continue
		}

		if currentSection != "" && trimmed != "" {
			sectionContent = append(sectionContent, trimmed)
		}
	}
	// Flush last section
	if currentSection != "" {
		flushExperienceSection(pack, currentSection, sectionContent)
	}

	// Generate ID from filename if not set
	if pack.ID == "" {
		base := strings.TrimSuffix(filename, filepath.Ext(filename))
		if expIDPattern.MatchString(base) {
			pack.ID = base
		} else {
			pack.ID = "exp-" + sanitizeToID(base)
		}
	}

	// Generate detail from all sections if not set
	if pack.Detail == "" {
		pack.Detail = content
	}

	return pack, pack.Validate()
}

func flushExperienceSection(pack *ExperiencePack, section string, lines []string) {
	text := strings.Join(lines, "\n")

	switch {
	case strings.Contains(section, "domain") || strings.Contains(section, "领域"):
		pack.Domain = text
	case strings.Contains(section, "summary") || strings.Contains(section, "摘要"):
		if pack.Summary == "" {
			pack.Summary = text
		}
	case strings.Contains(section, "difficult") || strings.Contains(section, "难度"):
		pack.Difficulty = strings.ToLower(strings.TrimSpace(text))
	case strings.Contains(section, "tag") || strings.Contains(section, "标签"):
		for _, line := range lines {
			tag := strings.TrimLeft(line, "- ")
			if tag != "" {
				pack.Tags = append(pack.Tags, tag)
			}
		}
	default:
		// Accumulate into detail
		if pack.Detail != "" {
			pack.Detail += "\n\n## " + section + "\n" + text
		} else {
			pack.Detail = "## " + section + "\n" + text
		}
	}
}

func sanitizeToID(s string) string {
	s = strings.ToLower(s)
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 60 {
		s = s[:60]
	}
	return s
}

// GenerateExperienceID creates a deterministic ID from domain and summary.
func GenerateExperienceID(domain, summary string) string {
	h := sha256.Sum256([]byte(domain + ":" + summary))
	short := fmt.Sprintf("%x", h[:4])
	slug := sanitizeToID(summary)
	if len(slug) > 30 {
		slug = slug[:30]
	}
	return "exp-" + slug + "-" + short
}

// --- Sanitization Pipeline ---

// SanitizeLevel defines which sanitization level to apply.
type SanitizeLevel int

const (
	SanitizeL1 SanitizeLevel = 1 // Regex-based PII removal
	SanitizeL2 SanitizeLevel = 2 // AI-assisted abstraction (requires LLM call)
	SanitizeL3 SanitizeLevel = 3 // Human review
)

// L1 sanitization patterns
var l1Patterns = []struct {
	re   *regexp.Regexp
	repl string
	desc string
}{
	// Usernames and mentions
	{regexp.MustCompile(`@[a-zA-Z0-9_-]+`), "@[redacted]", "mentions"},
	// Email addresses
	{regexp.MustCompile(`[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}`), "[email]", "emails"},
	// IP addresses (v4)
	{regexp.MustCompile(`\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`), "[ip]", "ipv4"},
	// API keys / tokens (long hex/base64 strings)
	{regexp.MustCompile(`(?i)(api[_-]?key|token|secret|password|auth)[:\s=]+["']?[a-zA-Z0-9_\-/.+=]{20,}["']?`), "$1=[redacted]", "secrets"},
	// Generic long secrets (standalone 32+ char hex/alphanum)
	{regexp.MustCompile(`\b[a-f0-9]{32,}\b`), "[hash]", "long hex"},
	{regexp.MustCompile(`\b[A-Za-z0-9+/]{40,}={0,2}\b`), "[base64]", "long base64"},
	// File paths with usernames
	{regexp.MustCompile(`(/Users/|/home/|C:\\Users\\)[a-zA-Z0-9._-]+`), "$1[user]", "user paths"},
	// Home directory shorthand
	{regexp.MustCompile(`~/Projects/[a-zA-Z0-9._-]+/[a-zA-Z0-9._-]+`), "~/Projects/[org]/[project]", "project paths"},
	{regexp.MustCompile(`~/[a-zA-Z0-9._-]+/`), "~/[dir]/", "home paths"},
	// GitHub URLs with owner
	{regexp.MustCompile(`github\.com/[a-zA-Z0-9_-]+/[a-zA-Z0-9_.-]+`), "github.com/[owner]/[repo]", "github urls"},
	// GitHub-style owner/repo references (without github.com prefix)
	{regexp.MustCompile(`(?:GitHub|github|仓库|repo):\s*[a-zA-Z0-9_-]+/[a-zA-Z0-9_.-]+`), "[repo-ref]", "repo refs"},
	// owner/repo in monorepo context
	{regexp.MustCompile(`monorepo.*?:\s*[a-zA-Z0-9_-]+/[a-zA-Z0-9_.-]+`), "monorepo: [owner]/[repo]", "monorepo refs"},
	// Custom domains (multi-part, likely personal)
	{regexp.MustCompile(`\b[a-zA-Z0-9-]+\.[a-zA-Z0-9-]+\.(im|io|dev|me|cc|co)\b`), "[domain]", "personal domains"},
	// Domain names (internal)
	{regexp.MustCompile(`\b[a-zA-Z0-9-]+\.(internal|local|corp|company)\.[a-zA-Z]{2,}\b`), "[internal-domain]", "internal domains"},
	// SSH hosts
	{regexp.MustCompile(`ssh\s+[a-zA-Z0-9@._-]+`), "ssh [redacted]", "ssh hosts"},
	// Docker image with registry
	{regexp.MustCompile(`[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}/[a-zA-Z0-9/_.-]+:[a-zA-Z0-9._-]+`), "[registry]/[image]:[tag]", "docker images"},
}

// SanitizeL1Text applies regex-based sanitization to text.
// Returns sanitized text and list of redactions applied.
func SanitizeL1Text(text string) (string, []string) {
	var applied []string
	result := text

	for _, p := range l1Patterns {
		if p.re.MatchString(result) {
			result = p.re.ReplaceAllString(result, p.repl)
			applied = append(applied, p.desc)
		}
	}

	return result, applied
}

// SanitizeExperiencePack applies L1 sanitization to all text fields.
func SanitizeExperiencePack(pack *ExperiencePack) []string {
	var allApplied []string

	pack.Summary, _ = SanitizeL1Text(pack.Summary)
	detail, applied := SanitizeL1Text(pack.Detail)
	pack.Detail = detail
	allApplied = append(allApplied, applied...)
	pack.Domain, _ = SanitizeL1Text(pack.Domain)

	pack.Sanitization = &SanitizationInfo{
		Level:       "L1",
		SanitizedAt: time.Now().UTC().Format(time.RFC3339),
	}

	return allApplied
}

// MemoryToExperiences extracts experience pack candidates from a MEMORY.md file.
// This is L1 extraction — identifies sections that look like project experiences.
func MemoryToExperiences(memoryContent string) []ExperiencePack {
	lines := strings.Split(memoryContent, "\n")
	var packs []ExperiencePack

	var currentH2, currentH3 string
	var sectionLines []string
	var sectionStart int

	flushSection := func() {
		if currentH3 == "" || len(sectionLines) < 3 {
			return
		}

		content := strings.Join(sectionLines, "\n")

		// Skip TODOs, personal info, team sections
		lower := strings.ToLower(currentH3)
		if strings.Contains(lower, "todo") || strings.Contains(lower, "初始化") ||
			strings.Contains(lower, "团队") || strings.Contains(lower, "工作流偏好") {
			return
		}

		// Looks like a technical experience
		summary := currentH3
		if currentH2 != "" {
			summary = currentH2 + " — " + currentH3
		}

		// Sanitize
		sanitizedContent, _ := SanitizeL1Text(content)
		sanitizedSummary, _ := SanitizeL1Text(summary)

		id := GenerateExperienceID(currentH2, currentH3)

		pack := ExperiencePack{
			ID:      id,
			Domain:  currentH2,
			Summary: sanitizedSummary,
			Detail:  sanitizedContent,
			Source: &ExperienceSource{
				Type:        "memory",
				MemoryFile:  "MEMORY.md",
				MemoryLines: fmt.Sprintf("%d-%d", sectionStart, sectionStart+len(sectionLines)),
			},
			Sanitization: &SanitizationInfo{
				Level:       "L1",
				SanitizedAt: time.Now().UTC().Format(time.RFC3339),
			},
		}

		// Try to extract tags from content
		pack.Tags = extractTags(content)

		packs = append(packs, pack)
	}

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "## ") {
			flushSection()
			currentH2 = strings.TrimPrefix(trimmed, "## ")
			currentH3 = ""
			sectionLines = nil
			continue
		}

		if strings.HasPrefix(trimmed, "### ") {
			flushSection()
			currentH3 = strings.TrimPrefix(trimmed, "### ")
			sectionLines = nil
			sectionStart = i + 1
			continue
		}

		if currentH3 != "" {
			sectionLines = append(sectionLines, line)
		}
	}
	flushSection()

	return packs
}

// extractTags tries to pull technology tags from text.
func extractTags(text string) []string {
	// Common tech terms to look for
	techPatterns := []struct {
		re  *regexp.Regexp
		tag string
	}{
		{regexp.MustCompile(`(?i)\brust\b`), "rust"},
		{regexp.MustCompile(`(?i)\bgo\b`), "go"},
		{regexp.MustCompile(`(?i)\bpython\b`), "python"},
		{regexp.MustCompile(`(?i)\btypescript\b`), "typescript"},
		{regexp.MustCompile(`(?i)\bnext\.?js\b`), "nextjs"},
		{regexp.MustCompile(`(?i)\breact\b`), "react"},
		{regexp.MustCompile(`(?i)\bflutter\b`), "flutter"},
		{regexp.MustCompile(`(?i)\bk8s\b|kubernetes`), "k8s"},
		{regexp.MustCompile(`(?i)\bdocker\b`), "docker"},
		{regexp.MustCompile(`(?i)\bpingora\b`), "pingora"},
		{regexp.MustCompile(`(?i)\bwasm\b`), "wasm"},
		{regexp.MustCompile(`(?i)\bsse\b`), "sse"},
		{regexp.MustCompile(`(?i)\bgrpc\b`), "grpc"},
		{regexp.MustCompile(`(?i)\bpostgres\b|postgresql`), "postgres"},
		{regexp.MustCompile(`(?i)\bredis\b`), "redis"},
		{regexp.MustCompile(`(?i)\bsqlite\b`), "sqlite"},
		{regexp.MustCompile(`(?i)\bprisma\b`), "prisma"},
		{regexp.MustCompile(`(?i)\btokio\b`), "tokio"},
		{regexp.MustCompile(`(?i)\baxum\b`), "axum"},
		{regexp.MustCompile(`(?i)\bnginx\b`), "nginx"},
		{regexp.MustCompile(`(?i)\bgithub.actions\b|ci/?cd`), "ci"},
	}

	seen := make(map[string]bool)
	var tags []string
	for _, p := range techPatterns {
		if p.re.MatchString(text) && !seen[p.tag] {
			tags = append(tags, p.tag)
			seen[p.tag] = true
		}
	}
	return tags
}
