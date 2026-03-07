package search

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	defaultMax     = 100
	defaultTimeout = 10 * time.Second
	maxOutput      = 1 << 20 // 1MB
)

type Config struct {
	AllowedDirs []string `json:"allowed_dirs"`
}

type Search struct {
	cfg   Config
	hasRg bool
	hasFd bool
}

func New(cfg Config) *Search {
	_, rgErr := exec.LookPath("rg")
	_, fdErr := exec.LookPath("fd")
	return &Search{
		cfg:   cfg,
		hasRg: rgErr == nil,
		hasFd: fdErr == nil,
	}
}

func (s *Search) Handler() http.Handler { return s }

func (s *Search) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	switch {
	case r.Method == http.MethodPost && path == "files":
		s.handleFiles(w, r)
	case r.Method == http.MethodPost && path == "grep":
		s.handleGrep(w, r)
	case r.Method == http.MethodPost && path == "content":
		s.handleContent(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (s *Search) handleFiles(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pattern string `json:"pattern"`
		Dir     string `json:"dir"`
		Max     int    `json:"max"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Dir == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "dir is required"})
		return
	}
	if !s.isAllowedDir(req.Dir) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "directory not allowed"})
		return
	}
	if req.Max <= 0 {
		req.Max = defaultMax
	}

	ctx, cancel := context.WithTimeout(r.Context(), defaultTimeout)
	defer cancel()

	var files []string
	if s.hasFd && req.Pattern != "" {
		args := []string{"--type", "f", "--glob", req.Pattern, ".", req.Dir}
		out, err := exec.CommandContext(ctx, "fd", args...).Output()
		if err == nil {
			files = splitLines(string(out), req.Max)
		}
	}
	if files == nil {
		files = walkFiles(ctx, req.Dir, req.Pattern, req.Max)
	}

	writeJSON(w, http.StatusOK, map[string]any{"files": files})
}

func (s *Search) handleGrep(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Pattern string `json:"pattern"`
		Dir     string `json:"dir"`
		Max     int    `json:"max"`
		Type    string `json:"type"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Pattern == "" || req.Dir == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "pattern and dir are required"})
		return
	}
	if !s.isAllowedDir(req.Dir) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "directory not allowed"})
		return
	}
	if req.Max <= 0 {
		req.Max = defaultMax
	}

	ctx, cancel := context.WithTimeout(r.Context(), defaultTimeout)
	defer cancel()

	var matches []grepMatch

	if s.hasRg {
		args := []string{"-n", "--max-count", strconv.Itoa(req.Max), "--no-heading"}
		if req.Type != "" {
			args = append(args, "--type", req.Type)
		}
		args = append(args, req.Pattern, req.Dir)
		out, _ := exec.CommandContext(ctx, "rg", args...).Output()
		matches = parseGrepOutput(string(out), req.Max)
	} else {
		args := []string{"-rn", "--max-count=" + strconv.Itoa(req.Max)}
		if req.Type != "" {
			args = append(args, "--include=*."+req.Type)
		}
		args = append(args, req.Pattern, req.Dir)
		out, _ := exec.CommandContext(ctx, "grep", args...).Output()
		matches = parseGrepOutput(string(out), req.Max)
	}

	writeJSON(w, http.StatusOK, map[string]any{"matches": matches})
}

func (s *Search) handleContent(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Query string `json:"query"`
		Dir   string `json:"dir"`
		Max   int    `json:"max"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Query == "" || req.Dir == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query and dir are required"})
		return
	}
	if !s.isAllowedDir(req.Dir) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "directory not allowed"})
		return
	}
	if req.Max <= 0 {
		req.Max = 10
	}

	ctx, cancel := context.WithTimeout(r.Context(), defaultTimeout)
	defer cancel()

	type result struct {
		File    string `json:"file"`
		Line    int    `json:"line"`
		Snippet string `json:"snippet"`
	}

	var results []result

	if s.hasRg {
		args := []string{"-n", "-C", "1", "--max-count", strconv.Itoa(req.Max), req.Query, req.Dir}
		out, _ := exec.CommandContext(ctx, "rg", args...).Output()
		for _, m := range parseGrepOutput(string(out), req.Max) {
			results = append(results, result{File: m.File, Line: m.Line, Snippet: m.Text})
		}
	} else {
		args := []string{"-rn", "-C", "1", "--max-count=" + strconv.Itoa(req.Max), req.Query, req.Dir}
		out, _ := exec.CommandContext(ctx, "grep", args...).Output()
		for _, m := range parseGrepOutput(string(out), req.Max) {
			results = append(results, result{File: m.File, Line: m.Line, Snippet: m.Text})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}

func (s *Search) isAllowedDir(dir string) bool {
	if len(s.cfg.AllowedDirs) == 0 {
		return true
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return false
	}
	for _, allowed := range s.cfg.AllowedDirs {
		allowedAbs, err := filepath.Abs(allowed)
		if err != nil {
			continue
		}
		if abs == allowedAbs || strings.HasPrefix(abs, allowedAbs+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func walkFiles(ctx context.Context, dir, pattern string, max int) []string {
	var files []string
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || ctx.Err() != nil || len(files) >= max {
			return filepath.SkipDir
		}
		if d.IsDir() {
			// Skip hidden dirs
			if base := filepath.Base(path); len(base) > 0 && base[0] == '.' && path != dir {
				return filepath.SkipDir
			}
			return nil
		}
		if pattern == "" {
			files = append(files, path)
		} else if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			files = append(files, path)
		}
		return nil
	})
	return files
}

type grepMatch struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

func parseGrepOutput(output string, max int) []grepMatch {
	var matches []grepMatch
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() && len(matches) < max {
		line := scanner.Text()
		// Format: file:line:text
		parts := strings.SplitN(line, ":", 3)
		if len(parts) < 3 {
			continue
		}
		lineNum, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		matches = append(matches, grepMatch{
			File: parts[0],
			Line: lineNum,
			Text: parts[2],
		})
	}
	return matches
}

func splitLines(s string, max int) []string {
	var lines []string
	scanner := bufio.NewScanner(strings.NewReader(s))
	for scanner.Scan() && len(lines) < max {
		if t := strings.TrimSpace(scanner.Text()); t != "" {
			lines = append(lines, t)
		}
	}
	return lines
}

// --- Helpers ---

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return false
	}
	if len(body) == 0 {
		return true
	}
	if err := json.Unmarshal(body, v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}
