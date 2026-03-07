package shell

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const maxOutput = 1 << 20 // 1MB

type Config struct {
	AllowedDirs []string `json:"allowed_dirs"`
	Timeout     int      `json:"timeout"`
	DenyList    []string `json:"deny_list"`
}

type Shell struct {
	cfg Config
}

func New(cfg Config) *Shell {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30
	}
	if len(cfg.DenyList) == 0 {
		cfg.DenyList = []string{"rm -rf /", "sudo", "shutdown", "reboot", "mkfs", "dd if="}
	}
	return &Shell{cfg: cfg}
}

func (s *Shell) Handler() http.Handler { return s }

func (s *Shell) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	switch {
	case r.Method == http.MethodPost && path == "exec":
		s.handleExec(w, r)
	case r.Method == http.MethodGet && path == "info":
		s.handleInfo(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

func (s *Shell) handleExec(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Command string `json:"command"`
		Cwd     string `json:"cwd"`
		Timeout int    `json:"timeout"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Command == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "command is required"})
		return
	}

	// Check deny list
	cmdLower := strings.ToLower(strings.TrimSpace(req.Command))
	for _, denied := range s.cfg.DenyList {
		if strings.HasPrefix(cmdLower, strings.ToLower(denied)) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "command is denied: " + denied})
			return
		}
	}

	// Validate cwd against allowed dirs
	if req.Cwd != "" {
		if !s.isAllowedDir(req.Cwd) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "directory not allowed"})
			return
		}
	}

	timeout := time.Duration(s.cfg.Timeout) * time.Second
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Second
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	shell := "/bin/sh"
	shellFlag := "-c"
	if runtime.GOOS == "windows" {
		shell = "cmd"
		shellFlag = "/c"
	}

	cmd := exec.CommandContext(ctx, shell, shellFlag, req.Command)
	if req.Cwd != "" {
		cmd.Dir = req.Cwd
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}

	outStr := truncate(stdout.String(), maxOutput)
	errStr := truncate(stderr.String(), maxOutput)

	writeJSON(w, http.StatusOK, map[string]any{
		"stdout":    outStr,
		"stderr":    errStr,
		"exit_code": exitCode,
	})
}

func (s *Shell) handleInfo(w http.ResponseWriter, r *http.Request) {
	shellPath := os.Getenv("SHELL")
	if shellPath == "" {
		shellPath = "/bin/sh"
	}
	home, _ := os.UserHomeDir()
	username := ""
	if u, err := user.Current(); err == nil {
		username = u.Username
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"shell": shellPath,
		"user":  username,
		"home":  home,
		"os":    runtime.GOOS,
		"arch":  runtime.GOARCH,
	})
}

func (s *Shell) isAllowedDir(dir string) bool {
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

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n... [truncated, exceeded 1MB]"
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
