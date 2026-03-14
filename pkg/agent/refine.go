package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// L2RefineConfig holds configuration for AI-assisted experience refinement.
type L2RefineConfig struct {
	// LLM endpoint (e.g. Prism, OpenAI-compatible)
	BaseURL string `json:"base_url" yaml:"base_url"`
	APIKey  string `json:"api_key" yaml:"api_key"`
	Model   string `json:"model" yaml:"model"`
	Timeout time.Duration `json:"timeout" yaml:"timeout"`
}

const l2SystemPrompt = `You are an experience sanitization assistant. Your job is to take raw technical experience entries (already L1-sanitized with regex) and refine them:

1. ABSTRACT: Replace any remaining specific references (project names, company names, team names, person names) with generic equivalents
   - "Prism project" → "an AI model router project"  
   - "万擎 LLM Gateway" → "a cloud LLM gateway"
   - "Zoe's MacBook" → "a development machine"

2. GENERALIZE: Convert project-specific knowledge into reusable technical patterns
   - "Fixed SSE in our Pingora proxy" → "Resolved SSE streaming issues in a Pingora-based reverse proxy"

3. PRESERVE: Keep all technical details, error messages, code patterns, and debugging steps intact — these are the valuable parts

4. STRUCTURE: Output clean markdown with sections: Summary, Problem, Investigation, Solution, Lessons Learned

5. TAGS: Suggest relevant technology tags

Respond in the SAME LANGUAGE as the input.

Output JSON:
{
  "summary": "one-line summary (max 200 chars)",
  "detail": "full refined experience in markdown",
  "domain": "technology domain",
  "difficulty": "basic|intermediate|advanced|expert",
  "tags": ["tag1", "tag2"]
}`

// RefineL2 uses an LLM to abstract and generalize an experience pack.
// Input should already be L1-sanitized.
func RefineL2(pack *ExperiencePack, cfg L2RefineConfig) error {
	if cfg.BaseURL == "" {
		return fmt.Errorf("L2 refine: base_url is required")
	}
	if cfg.Model == "" {
		cfg.Model = "claude-sonnet-4-20250514"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60 * time.Second
	}

	// Build prompt
	userMsg := fmt.Sprintf("Refine this experience pack:\n\nID: %s\nDomain: %s\nSummary: %s\n\nDetail:\n%s\n\nTags: %v",
		pack.ID, pack.Domain, pack.Summary, pack.Detail, pack.Tags)

	// Call LLM
	reqBody := map[string]any{
		"model": cfg.Model,
		"messages": []map[string]string{
			{"role": "system", "content": l2SystemPrompt},
			{"role": "user", "content": userMsg},
		},
		"temperature":  0.3,
		"max_tokens":   2000,
		"response_format": map[string]string{"type": "json_object"},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("L2 refine: marshal: %w", err)
	}

	url := cfg.BaseURL + "/v1/chat/completions"
	req, err := http.NewRequest("POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("L2 refine: new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if cfg.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	}

	client := &http.Client{Timeout: cfg.Timeout}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("L2 refine: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("L2 refine: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("L2 refine: decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return fmt.Errorf("L2 refine: no choices in response")
	}

	// Parse LLM output
	var refined struct {
		Summary    string   `json:"summary"`
		Detail     string   `json:"detail"`
		Domain     string   `json:"domain"`
		Difficulty string   `json:"difficulty"`
		Tags       []string `json:"tags"`
	}
	if err := json.Unmarshal([]byte(result.Choices[0].Message.Content), &refined); err != nil {
		return fmt.Errorf("L2 refine: parse LLM output: %w", err)
	}

	// Apply refined content
	if refined.Summary != "" {
		pack.Summary = refined.Summary
	}
	if refined.Detail != "" {
		pack.Detail = refined.Detail
	}
	if refined.Domain != "" {
		pack.Domain = refined.Domain
	}
	if refined.Difficulty != "" {
		pack.Difficulty = refined.Difficulty
	}
	if len(refined.Tags) > 0 {
		pack.Tags = refined.Tags
	}

	// Update sanitization info
	pack.Sanitization = &SanitizationInfo{
		Level:       "L2",
		SanitizedAt: time.Now().UTC().Format(time.RFC3339),
	}

	return nil
}

// RefineL2Batch refines multiple packs, returning errors per pack (non-fatal).
func RefineL2Batch(packs []ExperiencePack, cfg L2RefineConfig) map[string]error {
	errs := make(map[string]error)
	for i := range packs {
		if err := RefineL2(&packs[i], cfg); err != nil {
			errs[packs[i].ID] = err
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}
