package model

import "time"

// RunMode represents the execution mode.
type RunMode string

const (
	RunModeRun     RunMode = "run"     // one-shot execution (default)
	RunModeSession RunMode = "session" // interactive persistent container
	RunModeDaemon  RunMode = "daemon"  // TODO: cron-based scheduled execution
)

// RunStatus represents the lifecycle state of an agent run.
type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"
	RunStatusRunning   RunStatus = "running"
	RunStatusCompleted RunStatus = "completed"
	RunStatusFailed    RunStatus = "failed"
	RunStatusCancelled RunStatus = "cancelled"
)

// Run represents a single agent workflow execution.
type Run struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id,omitempty"`
	Name      string     `json:"name"`
	Mode      RunMode    `json:"mode"`
	Status    RunStatus  `json:"status"`
	AgentFile string     `json:"agent_file"`
	Config    RunConfig  `json:"config"`
	Result    *Result    `json:"result,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

// Message represents a chat message in a session.
type Message struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// RunConfig holds execution configuration.
type RunConfig struct {
	Image    string            `json:"image,omitempty" yaml:"image"`
	Timeout  int               `json:"timeout,omitempty" yaml:"timeout"`
	Env      map[string]string `json:"env,omitempty" yaml:"env"`
	Volumes  []Volume          `json:"volumes,omitempty" yaml:"volumes"`
	Schedule string            `json:"schedule,omitempty" yaml:"schedule"`
}

// Volume defines a data mount.
type Volume struct {
	Name      string `json:"name" yaml:"name"`
	Source    string `json:"source" yaml:"source"`
	MountPath string `json:"mount_path" yaml:"mount_path"`
}

// Result holds the output of a completed run.
type Result struct {
	ExitCode  int      `json:"exit_code"`
	Output    string   `json:"output,omitempty"`
	Artifacts []string `json:"artifacts,omitempty"`
	Error     string   `json:"error,omitempty"`
}

// Agent represents a parsed AGENTS.md definition.
type Agent struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Workflow    []string `json:"workflow"`
	Guidelines  []string `json:"guidelines,omitempty"`
	Skills      []string `json:"skills,omitempty"`
}
