package docker

import (
	"archive/tar"
	"bufio"
	"bytes"
	"net"
	"os"
	"path/filepath"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"sync"
	"time"

	"go.zoe.im/agentbox/internal/executor"
	"go.zoe.im/agentbox/internal/runtime"
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

	mu             sync.Mutex
	containers     map[string]string          // runID -> containerID
	sessionMsgCnt  map[string]int             // runID -> message count
	sessionBridge  map[string]bool            // runID -> bridge available
	sessionRuntime map[string]runtime.Runtime // runID -> runtime
}

func New(cfg Config) (executor.Executor, error) {
	// Verify docker is available
	if _, err := exec.LookPath("docker"); err != nil {
		return nil, fmt.Errorf("docker not found in PATH: %w", err)
	}
	return &dockerExecutor{
		cfg:            cfg,
		logger:         slog.Default(),
		containers:     make(map[string]string),
		sessionMsgCnt:  make(map[string]int),
		sessionBridge:  make(map[string]bool),
		sessionRuntime: make(map[string]runtime.Runtime),
	}, nil
}

func (e *dockerExecutor) Execute(ctx context.Context, req *executor.Request) (*executor.Response, error) {
	image := req.Image
	if image == "" {
		image = e.cfg.Image
	}

	containerName := fmt.Sprintf("abox-%s", req.ID)

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

	// Auto-mount Claude config if exists
	homeDir, _ := os.UserHomeDir()
	claudeDir := filepath.Join(homeDir, ".claude")
	if _, err := os.Stat(claudeDir); err == nil {
		args = append(args, "-v", fmt.Sprintf("%s:/home/agent/.claude", claudeDir))
	}
	claudeJSON := filepath.Join(homeDir, ".claude.json")
	if _, err := os.Stat(claudeJSON); err == nil {
		args = append(args, "-v", fmt.Sprintf("%s:/home/agent/.claude.json:ro", claudeJSON))
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
		containerName = fmt.Sprintf("abox-%s", id)
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
		containerName = fmt.Sprintf("abox-%s", id)
	}

	cmd := exec.CommandContext(ctx, "docker", "stop", "-t", "10", containerName)
	if e.cfg.Host != "" {
		cmd.Env = append(os.Environ(), "DOCKER_HOST="+e.cfg.Host)
	}

	return cmd.Run()
}

// StartSession starts a detached container for interactive session mode.
func (e *dockerExecutor) StartSession(ctx context.Context, req *executor.Request) (string, error) {
	image := req.Image
	if image == "" {
		image = e.cfg.Image
	}

	containerName := fmt.Sprintf("abox-%s", req.ID)

	args := []string{
		"run", "-d",
		"--name", containerName,
	}

	// Environment variables
	args = append(args, "-e", fmt.Sprintf("AGENTBOX_RUN_ID=%s", req.ID))
	args = append(args, "-e", fmt.Sprintf("AGENTBOX_AGENT_FILE=%s", req.AgentFile))
	args = append(args, "-e", "AGENTBOX_MODE=session")
	for k, v := range req.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}

	// Volumes
	for _, vol := range req.Volumes {
		args = append(args, "-v", fmt.Sprintf("%s:%s:ro", vol.Source, vol.MountPath))
	}

	// Auto-mount Claude config if exists
	homeDir, _ := os.UserHomeDir()
	claudeDir := filepath.Join(homeDir, ".claude")
	if _, err := os.Stat(claudeDir); err == nil {
		args = append(args, "-v", fmt.Sprintf("%s:/home/agent/.claude", claudeDir))
	}
	claudeJSON := filepath.Join(homeDir, ".claude.json")
	if _, err := os.Stat(claudeJSON); err == nil {
		args = append(args, "-v", fmt.Sprintf("%s:/home/agent/.claude.json:ro", claudeJSON))
	}

	// Auto-detect abox-bridge and inject MCP config
	bridgeAddr := os.Getenv("ABOX_BRIDGE_ADDR")
	if bridgeAddr == "" {
		bridgeAddr = "host.docker.internal:9800"
	}
	// Check if bridge is reachable
	bridgeAvailable := false
	if conn, err := net.DialTimeout("tcp", strings.Replace(bridgeAddr, "host.docker.internal", "localhost", 1), time.Second); err == nil {
		conn.Close()
		bridgeAvailable = true
	}

	if bridgeAvailable {
		// Add host.docker.internal resolution
		args = append(args, "--add-host", "host.docker.internal:host-gateway")

		// Create MCP config temp file
		mcpConfig := fmt.Sprintf(`{"mcpServers":{"abox-bridge":{"url":"http://%s/sse"}}}`, bridgeAddr)
		tmpFile, err := os.CreateTemp("", "abox-mcp-*.json")
		if err == nil {
			tmpFile.WriteString(mcpConfig)
			tmpFile.Close()
			args = append(args, "-v", fmt.Sprintf("%s:/home/agent/.claude/mcp.json:ro", tmpFile.Name()))
			e.logger.Info("bridge detected, injecting MCP config", "addr", bridgeAddr)
		}

		// Also expose WebDAV addr
		webdavAddr := os.Getenv("ABOX_WEBDAV_ADDR")
		if webdavAddr == "" {
			webdavAddr = "host.docker.internal:9801"
		}
		args = append(args, "-e", fmt.Sprintf("ABOX_WEBDAV_URL=http://%s", webdavAddr))
		args = append(args, "-e", fmt.Sprintf("ABOX_BRIDGE_URL=http://%s", bridgeAddr))
	}

	args = append(args, image)

	e.logger.Info("starting session container", "name", containerName, "image", image, "bridge", bridgeAvailable)

	cmd := exec.CommandContext(ctx, "docker", args...)
	if e.cfg.Host != "" {
		cmd.Env = append(os.Environ(), "DOCKER_HOST="+e.cfg.Host)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("docker run: %s: %w", strings.TrimSpace(string(out)), err)
	}

	containerID := strings.TrimSpace(string(out))

	e.mu.Lock()
	e.containers[req.ID] = containerName
	e.sessionBridge[req.ID] = bridgeAvailable
	e.sessionRuntime[req.ID] = e.getRuntime(req.Runtime)
	e.mu.Unlock()

	e.logger.Info("session container started", "name", containerName, "container_id", containerID[:12])

	// If bridge available, write MCP config inside container
	if bridgeAvailable {
		mcpJSON := fmt.Sprintf(`{"mcpServers":{"abox-bridge":{"url":"http://%s/sse"}}}`, bridgeAddr)
		writeCmd := exec.CommandContext(ctx, "docker", "exec", containerName,
			"bash", "-c", fmt.Sprintf("echo '%s' > /tmp/mcp.json && chown agent:agent /tmp/mcp.json", mcpJSON))
		if e.cfg.Host != "" {
			writeCmd.Env = append(os.Environ(), "DOCKER_HOST="+e.cfg.Host)
		}
		if err := writeCmd.Run(); err != nil {
			e.logger.Warn("failed to write MCP config", "err", err)
		}
	}

	return containerID, nil
}

// SendMessage executes the agent CLI in a running session container.
func (e *dockerExecutor) SendMessage(ctx context.Context, id string, message string) (string, error) {
	e.mu.Lock()
	containerName, ok := e.containers[id]
	msgCnt := e.sessionMsgCnt[id]
	e.sessionMsgCnt[id] = msgCnt + 1
	rt := e.sessionRuntime[id]
	e.mu.Unlock()

	if !ok {
		containerName = fmt.Sprintf("abox-%s", id)
	}
	if rt == nil {
		rt = runtime.Default()
	}

	// Build docker exec args with runtime CLI args
	dockerArgs := []string{"exec",
		"-u", "agent",
		"-w", "/workspace",
		"-e", "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1",
		"-e", "PATH=/home/agent/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		containerName,
	}
	cliArgs := rt.BuildExecArgs(message, msgCnt > 0)
	// For non-streaming SendMessage, remove stream-json flags
	filtered := make([]string, 0, len(cliArgs))
	for _, a := range cliArgs {
		if a == "--output-format" || a == "stream-json" || a == "--verbose" {
			continue
		}
		filtered = append(filtered, a)
	}
	dockerArgs = append(dockerArgs, filtered...)

	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	if e.cfg.Host != "" {
		cmd.Env = append(os.Environ(), "DOCKER_HOST="+e.cfg.Host)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	e.logger.Info("sending message to session", "container", containerName, "runtime", rt.Name())

	if err := cmd.Run(); err != nil {
		output := stdout.String()
		if stderr.Len() > 0 {
			output += "\n" + stderr.String()
		}
		return output, fmt.Errorf("docker exec: %w", err)
	}

	return strings.TrimSpace(stdout.String()), nil
}

// StopSession stops and removes a session container.
func (e *dockerExecutor) StopSession(ctx context.Context, id string) error {
	e.mu.Lock()
	containerName, ok := e.containers[id]
	e.mu.Unlock()

	if !ok {
		containerName = fmt.Sprintf("abox-%s", id)
	}

	e.logger.Info("stopping session container", "name", containerName)

	// Stop the container
	stopCmd := exec.CommandContext(ctx, "docker", "stop", "-t", "10", containerName)
	if e.cfg.Host != "" {
		stopCmd.Env = append(os.Environ(), "DOCKER_HOST="+e.cfg.Host)
	}
	_ = stopCmd.Run()

	// Remove the container
	rmCmd := exec.CommandContext(ctx, "docker", "rm", "-f", containerName)
	if e.cfg.Host != "" {
		rmCmd.Env = append(os.Environ(), "DOCKER_HOST="+e.cfg.Host)
	}
	err := rmCmd.Run()

	e.mu.Lock()
	delete(e.containers, id)
	delete(e.sessionMsgCnt, id)
	delete(e.sessionBridge, id)
	delete(e.sessionRuntime, id)
	e.mu.Unlock()

	return err
}

func (e *dockerExecutor) SendMessageStream(ctx context.Context, id string, message string, onToken executor.TokenCallback) (string, error) {
	e.mu.Lock()
	containerName, ok := e.containers[id]
	msgCnt := e.sessionMsgCnt[id]
	e.sessionMsgCnt[id] = msgCnt + 1
	rt := e.sessionRuntime[id]
	e.mu.Unlock()

	if !ok {
		containerName = fmt.Sprintf("abox-%s", id)
	}
	if rt == nil {
		rt = runtime.Default()
	}

	dockerArgs := []string{"exec",
		"-u", "agent",
		"-w", "/workspace",
		"-e", "CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1",
		"-e", "PATH=/home/agent/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		containerName,
	}
	dockerArgs = append(dockerArgs, rt.BuildExecArgs(message, msgCnt > 0)...)

	cmd := exec.CommandContext(ctx, "docker", dockerArgs...)
	if e.cfg.Host != "" {
		cmd.Env = append(os.Environ(), "DOCKER_HOST="+e.cfg.Host)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}

	e.logger.Info("streaming message to session", "container", containerName, "runtime", rt.Name())

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("docker exec start: %w", err)
	}

	var fullResponse strings.Builder
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		token, result, done := rt.ParseStreamLine(line)
		if done && result != "" && fullResponse.Len() == 0 {
			fullResponse.WriteString(result)
			continue
		}
		if token != "" {
			// For claude runtime, token is the full text so far (diff needed)
			if rt.Name() == "claude" {
				existing := fullResponse.String()
				if len(token) > len(existing) {
					delta := token[len(existing):]
					fullResponse.Reset()
					fullResponse.WriteString(token)
					if onToken != nil {
						onToken(delta)
					}
				} else if token != existing {
					fullResponse.Reset()
					fullResponse.WriteString(token)
				}
			} else {
				fullResponse.WriteString(token)
				if onToken != nil {
					onToken(token)
				}
			}
		}
	}

	cmd.Wait()
	return fullResponse.String(), nil
}

// RecoverSessions lists running containers with the "abox-" prefix and re-registers them.
func (e *dockerExecutor) RecoverSessions(ctx context.Context) ([]string, error) {
	cmd := exec.CommandContext(ctx, "docker", "ps", "--filter", "name=abox-", "--format", "{{.Names}}")
	if e.cfg.Host != "" {
		cmd.Env = append(os.Environ(), "DOCKER_HOST="+e.cfg.Host)
	}
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("docker ps: %w", err)
	}

	var ids []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		id := strings.TrimPrefix(line, "abox-")
		e.mu.Lock()
		e.containers[id] = line
		e.mu.Unlock()
		ids = append(ids, id)
	}
	return ids, nil
}

// getRuntime returns the runtime for the given name, falling back to default.
func (e *dockerExecutor) getRuntime(name string) runtime.Runtime {
	if name == "" {
		return runtime.Default()
	}
	if rt := runtime.Get(name); rt != nil {
		return rt
	}
	return runtime.Default()
}

// UploadFile copies a file into a running session container at /workspace/uploads/.
func (e *dockerExecutor) UploadFile(ctx context.Context, id string, filename string, data []byte) error {
	e.mu.Lock()
	containerName, ok := e.containers[id]
	e.mu.Unlock()

	if !ok {
		containerName = fmt.Sprintf("abox-%s", id)
	}

	// Create tar archive with the file under uploads/
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{
		Name: "uploads/" + filename,
		Mode: 0644,
		Size: int64(len(data)),
	}); err != nil {
		return fmt.Errorf("tar header: %w", err)
	}
	if _, err := tw.Write(data); err != nil {
		return fmt.Errorf("tar write: %w", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("tar close: %w", err)
	}

	cmd := exec.CommandContext(ctx, "docker", "cp", "-", containerName+":/workspace/")
	if e.cfg.Host != "" {
		cmd.Env = append(os.Environ(), "DOCKER_HOST="+e.cfg.Host)
	}
	cmd.Stdin = &buf
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker cp: %s: %w", strings.TrimSpace(string(out)), err)
	}

	e.logger.Info("uploaded file to session", "container", containerName, "file", filename, "size", len(data))
	return nil
}

// StreamLogs streams container logs line by line via a channel.
func (e *dockerExecutor) StreamLogs(ctx context.Context, id string) (<-chan string, error) {
	e.mu.Lock()
	containerName, ok := e.containers[id]
	e.mu.Unlock()

	if !ok {
		containerName = fmt.Sprintf("abox-%s", id)
	}

	cmd := exec.CommandContext(ctx, "docker", "logs", "-f", "--tail", "100", containerName)
	if e.cfg.Host != "" {
		cmd.Env = append(os.Environ(), "DOCKER_HOST="+e.cfg.Host)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = cmd.Stdout // merge stderr into stdout

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("docker logs start: %w", err)
	}

	ch := make(chan string, 64)
	go func() {
		defer close(ch)
		defer cmd.Wait()
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 64*1024), 64*1024)
		for scanner.Scan() {
			select {
			case <-ctx.Done():
				return
			case ch <- scanner.Text():
			}
		}
	}()

	return ch, nil
}
