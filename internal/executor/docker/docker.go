package docker

import (
	"bytes"
	"os"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"

	"go.zoe.im/agentbox/internal/executor"
	"go.zoe.im/x"
)

// Config for docker executor.
type Config struct {
	Host  string `json:"host,omitempty" yaml:"host"`
	Image string `json:"image" yaml:"image"`
}

func init() {
	executor.Register("docker", func(cfg x.TypedLazyConfig, opts ...any) (executor.Executor, error) {
		var c Config
		if len(cfg.Config) > 0 {
			if err := cfg.Unmarshal(&c); err != nil {
				return nil, err
			}
		}
		if c.Image == "" {
			c.Image = "agentbox-sandbox:latest"
		}
		return New(c)
	})
}

type dockerExecutor struct {
	cfg    Config
	logger *slog.Logger

	mu         sync.Mutex
	containers map[string]string // runID -> containerID
}

func New(cfg Config) (executor.Executor, error) {
	// Verify docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, fmt.Errorf("docker not found in PATH: %w", err)
	}
	return &dockerExecutor{
		cfg:        cfg,
		logger:     slog.Default(),
		containers: make(map[string]string),
	}, nil
}

func (e *dockerExecutor) Execute(ctx context.Context, req *executor.Request) (*executor.Response, error) {
	image := req.Image
	if image == "" {
		image = e.cfg.Image
	}

	containerName := fmt.Sprintf("agentbox-%s", req.ID)

	// Build docker run args
	args := []string{
		"run",
		"--rm",
		"--name", containerName,
	}

	// Environment variables
	args = append(args, "-e", fmt.Sprintf("AGENTBOX_RUN_ID=%s", req.ID))
	args = append(args, "-e", fmt.Sprintf("AGENTBOX_AGENT_FILE=%s", req.AgentFile))
	for k, v := range req.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// Volumes
	for _, vol := range req.Volumes {
		args = append(args, "-v", fmt.Sprintf("%s:%s:ro", vol.Source, vol.MountPath))
	}

	args = append(args, image)

	e.logger.Info("starting docker container", "name", containerName, "image", image)

	// Set timeout
	timeout := time.Duration(req.Timeout) * time.Second
	if timeout == 0 {
		timeout = time.Hour
	}
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "docker", args...)

	// Set DOCKER_HOST if configured
	if e.cfg.Host != "" {
		cmd.Env = append(os.Environ(), "DOCKER_HOST="+e.cfg.Host)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Track container
	e.mu.Lock()
	e.containers[req.ID] = containerName
	e.mu.Unlock()

	defer func() {
		e.mu.Lock()
		delete(e.containers, req.ID)
		e.mu.Unlock()
	}()

	err := cmd.Run()

	output := stdout.String()
	if stderr.Len() > 0 {
		output += "\n--- stderr ---\n" + stderr.String()
	}

	if err != nil {
		exitCode := 1
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}

		if execCtx.Err() == context.DeadlineExceeded {
			return &executor.Response{
				ExitCode: exitCode,
				Output:   output,
				Logs:     output,
			}, fmt.Errorf("container %s timed out after %v", containerName, timeout)
		}

		return &executor.Response{
			ExitCode: exitCode,
			Output:   output,
			Logs:     output,
		}, nil
	}

	e.logger.Info("container completed", "name", containerName)

	return &executor.Response{
		ExitCode: 0,
		Output:   output,
		Logs:     output,
	}, nil
}

func (e *dockerExecutor) Logs(ctx context.Context, id string) (string, error) {
	e.mu.Lock()
	containerName, ok := e.containers[id]
	e.mu.Unlock()

	if !ok {
		containerName = fmt.Sprintf("agentbox-%s", id)
	}

	cmd := exec.CommandContext(ctx, "docker", "logs", containerName)
	if e.cfg.Host != "" {
		cmd.Env = append(os.Environ(), "DOCKER_HOST="+e.cfg.Host)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker logs: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (e *dockerExecutor) Stop(ctx context.Context, id string) error {
	e.mu.Lock()
	containerName, ok := e.containers[id]
	e.mu.Unlock()

	if !ok {
		containerName = fmt.Sprintf("agentbox-%s", id)
	}

	cmd := exec.CommandContext(ctx, "docker", "stop", "-t", "10", containerName)
	if e.cfg.Host != "" {
		cmd.Env = append(os.Environ(), "DOCKER_HOST="+e.cfg.Host)
	}

	return cmd.Run()
}
