package service

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sync"
	"time"

	"go.zoe.im/agentbox/internal/runtime"
)

// installJob tracks a runtime install process.
type installJob struct {
	Name    string   `json:"name"`
	Command string   `json:"command"`
	Status  string   `json:"status"` // pending, running, completed, failed
	Output  []string `json:"output"`
	Error   string   `json:"error,omitempty"`
	mu      sync.Mutex
}

// installManager manages background runtime installs.
type installManager struct {
	jobs map[string]*installJob
	mu   sync.RWMutex
}

func newInstallManager() *installManager {
	return &installManager{
		jobs: make(map[string]*installJob),
	}
}

func (m *installManager) get(name string) *installJob {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.jobs[name]
}

func (m *installManager) start(name, command string) (*installJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if existing, ok := m.jobs[name]; ok {
		existing.mu.Lock()
		status := existing.Status
		existing.mu.Unlock()
		if status == "running" {
			return nil, fmt.Errorf("install already in progress for %s", name)
		}
	}

	job := &installJob{
		Name:    name,
		Command: command,
		Status:  "running",
	}
	m.jobs[name] = job

	go m.run(job)

	return job, nil
}

func (m *installManager) run(job *installJob) {
	// 5 minute timeout for install processes
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sh", "-c", job.Command)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		job.mu.Lock()
		job.Status = "failed"
		job.Error = err.Error()
		job.mu.Unlock()
		return
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout

	if err := cmd.Start(); err != nil {
		job.mu.Lock()
		job.Status = "failed"
		job.Error = err.Error()
		job.mu.Unlock()
		return
	}

	// Limit output to 1MB
	const maxOutput = 1 << 20 // 1MB
	var totalOutput int
	scanner := bufio.NewScanner(io.LimitReader(stdout, maxOutput))
	for scanner.Scan() {
		line := scanner.Text()
		totalOutput += len(line)
		job.mu.Lock()
		job.Output = append(job.Output, line)
		job.mu.Unlock()
		if totalOutput >= maxOutput {
			job.mu.Lock()
			job.Output = append(job.Output, "[output truncated at 1MB]")
			job.mu.Unlock()
			break
		}
	}

	if err := cmd.Wait(); err != nil {
		job.mu.Lock()
		job.Status = "failed"
		if ctx.Err() == context.DeadlineExceeded {
			job.Error = "install timed out after 5 minutes"
		} else {
			job.Error = err.Error()
		}
		job.mu.Unlock()
		return
	}

	job.mu.Lock()
	job.Status = "completed"
	job.mu.Unlock()
}

// --- HTTP handlers ---

// GetRuntimesStatus returns availability status for all runtimes.
func (s *Service) GetRuntimesStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(runtime.CheckAll())
}

// InstallRuntime starts a background install for a runtime.
// Only allows commands from the runtime's InstallCommand() — no user-supplied commands.
func (s *Service) InstallRuntime(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	rt := runtime.Get(req.Name)
	if rt == nil {
		http.Error(w, `{"error":"unknown runtime"}`, http.StatusNotFound)
		return
	}

	// Only allow the runtime's own install command — no user-supplied commands
	baseCmd := rt.InstallCommand()
	if baseCmd == "" {
		http.Error(w, `{"error":"runtime does not support automatic install"}`, http.StatusBadRequest)
		return
	}

	cmd := runtime.WrapInstallCommand(baseCmd)
	if cmd == "" {
		http.Error(w, `{"error":"runtime does not support automatic install"}`, http.StatusBadRequest)
		return
	}

	job, err := s.installs.start(req.Name, cmd)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":%q}`, err.Error()), http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{
		"name":   job.Name,
		"status": job.Status,
	})
}

// StreamInstallOutput streams install output via SSE.
func (s *Service) StreamInstallOutput(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	job := s.installs.get(name)
	if job == nil {
		http.Error(w, `{"error":"no install job found"}`, http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
		return
	}

	sent := 0
	for {
		job.mu.Lock()
		lines := make([]string, len(job.Output))
		copy(lines, job.Output)
		status := job.Status
		errMsg := job.Error
		job.mu.Unlock()

		// Send new lines
		for i := sent; i < len(lines); i++ {
			fmt.Fprintf(w, "data: %s\n\n", lines[i])
			flusher.Flush()
		}
		sent = len(lines)

		if status == "completed" || status == "failed" {
			// Send final status event
			data := fmt.Sprintf(`{"status":%q,"error":%q}`, status, errMsg)
			fmt.Fprintf(w, "event: done\ndata: %s\n\n", data)
			flusher.Flush()
			return
		}

		select {
		case <-r.Context().Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
}
