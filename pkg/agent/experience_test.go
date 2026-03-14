package agent

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeL1Text(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string // substring that should NOT appear
	}{
		{"email", "contact admin@company.com for help", "admin@company.com"},
		{"ip", "connect to 192.168.1.100", "192.168.1.100"},
		{"user path", "file at /Users/zoe/Projects/foo", "zoe"},
		{"github url", "see github.com/jiusanzhou/prism", "jiusanzhou"},
		{"api key", "api_key=sk-1234567890abcdef1234567890abcdef", "sk-1234567890"},
		{"mention", "ask @noboddyim about it", "@noboddyim"},
		{"ssh", "ssh admin@prod-server.internal.corp.com", "admin@prod"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, applied := SanitizeL1Text(tt.input)
			if strings.Contains(result, tt.expect) {
				t.Errorf("SanitizeL1 did not redact %q in %q → %q", tt.expect, tt.input, result)
			}
			if len(applied) == 0 {
				t.Error("expected at least one redaction applied")
			}
		})
	}
}

func TestSanitizeL1PreservesNormal(t *testing.T) {
	normal := "Pingora SSE streaming downstream write blocked due to response_body_filter returning Duration"
	result, applied := SanitizeL1Text(normal)
	if result != normal {
		t.Errorf("normal text was modified: %q → %q", normal, result)
	}
	if len(applied) > 0 {
		t.Errorf("unexpected redactions on normal text: %v", applied)
	}
}

func TestGenerateExperienceID(t *testing.T) {
	id := GenerateExperienceID("Rust", "Pingora SSE fix")
	if !strings.HasPrefix(id, "exp-") {
		t.Errorf("id should start with exp-, got %q", id)
	}
	if !expIDPattern.MatchString(id) {
		t.Errorf("id doesn't match pattern: %q", id)
	}

	// Deterministic
	id2 := GenerateExperienceID("Rust", "Pingora SSE fix")
	if id != id2 {
		t.Errorf("not deterministic: %q != %q", id, id2)
	}

	// Different inputs → different IDs
	id3 := GenerateExperienceID("Go", "Different problem")
	if id == id3 {
		t.Error("different inputs should produce different IDs")
	}
}

func TestParseExperienceDir(t *testing.T) {
	dir := filepath.Join("schema", "examples")
	// Use the registry example
	regDir := filepath.Join("..", "..", "registry", "agents", "rust-proxy-expert", "experience")
	idx, err := ParseExperienceDir(regDir)
	if err != nil {
		// Try the local test dir
		idx, err = ParseExperienceDir(dir)
		if err != nil {
			t.Skip("no experience directory available")
		}
	}

	if len(idx.Packs) == 0 {
		t.Error("expected at least one experience pack")
	}

	for _, p := range idx.Packs {
		if err := p.Validate(); err != nil {
			t.Errorf("pack %s validation failed: %v", p.ID, err)
		}
	}
}

func TestMemoryToExperiences(t *testing.T) {
	memory := `# MEMORY.md

## 初始化
- 创建时间：2026-02-19

## 团队
- main (小Z)

## Prism 项目

### SSE Streaming 不工作
- 现象: upstream SSE chunks 正常接收但 downstream 0 bytes
- 排查: response_filter 正常, response_body_filter 返回 Duration 导致 sleep
- 解决: SSE 流不设 idle timeout
- commit: 4b8bcf6

### Body Forwarding 踩坑
- 问题：request_filter 读 body 后 upstream 收不到
- 根因：删 Content-Length 不加 Transfer-Encoding: chunked
- 解决：enable_retry_buffering + chunked
- 文档：docs/PROXY_BODY_FORWARDING.md

## TODO: 未完成
- [ ] 迁移到 Rust
`

	packs := MemoryToExperiences(memory)

	// Should skip 初始化, 团队, TODO sections
	// Should extract SSE and Body Forwarding
	if len(packs) < 2 {
		t.Errorf("expected at least 2 packs, got %d", len(packs))
		for _, p := range packs {
			t.Logf("  pack: %s — %s", p.ID, p.Summary)
		}
		return
	}

	// Check sanitization was applied
	for _, p := range packs {
		if p.Sanitization == nil {
			t.Errorf("pack %s missing sanitization info", p.ID)
		}
		if p.Source == nil || p.Source.Type != "memory" {
			t.Errorf("pack %s missing source info", p.ID)
		}
	}
}

func TestExperiencePackValidation(t *testing.T) {
	tests := []struct {
		name string
		pack ExperiencePack
		err  bool
	}{
		{"empty id", ExperiencePack{}, true},
		{"bad id", ExperiencePack{ID: "bad"}, true},
		{"no domain", ExperiencePack{ID: "exp-test-001"}, true},
		{"no summary", ExperiencePack{ID: "exp-test-001", Domain: "Rust"}, true},
		{"no detail", ExperiencePack{ID: "exp-test-001", Domain: "Rust", Summary: "test"}, true},
		{"valid", ExperiencePack{ID: "exp-test-001", Domain: "Rust", Summary: "test", Detail: "details"}, false},
		{"bad difficulty", ExperiencePack{ID: "exp-test-001", Domain: "Rust", Summary: "test", Detail: "d", Difficulty: "impossible"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.pack.Validate()
			if (err != nil) != tt.err {
				t.Errorf("Validate() error = %v, want err = %v", err, tt.err)
			}
		})
	}
}

func TestSanitizeExperiencePack(t *testing.T) {
	pack := &ExperiencePack{
		ID:      "exp-test-001",
		Domain:  "Rust",
		Summary: "Fixed issue in github.com/jiusanzhou/prism",
		Detail:  "Connected to 192.168.1.100 and found /Users/zoe/Projects had the fix",
	}

	applied := SanitizeExperiencePack(pack)

	if strings.Contains(pack.Summary, "jiusanzhou") {
		t.Error("summary still contains github username")
	}
	if strings.Contains(pack.Detail, "192.168.1.100") {
		t.Error("detail still contains IP")
	}
	if strings.Contains(pack.Detail, "zoe") {
		t.Error("detail still contains username")
	}
	if pack.Sanitization == nil || pack.Sanitization.Level != "L1" {
		t.Error("sanitization info not set")
	}
	if len(applied) == 0 {
		t.Error("expected redactions")
	}
}
