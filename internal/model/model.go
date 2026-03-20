package model

import (
	"encoding/json"
	"time"
)

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
	RunStatusPending     RunStatus = "pending"
	RunStatusRunning     RunStatus = "running"
	RunStatusCompleted   RunStatus = "completed"
	RunStatusFailed      RunStatus = "failed"
	RunStatusCancelled   RunStatus = "cancelled"
	RunStatusInterrupted RunStatus = "interrupted"
)

// Run represents a single agent workflow execution.
type Run struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id,omitempty"`
	TeamID    string     `json:"team_id,omitempty"`
	Name      string     `json:"name"`
	Mode      RunMode    `json:"mode"`
	Status    RunStatus  `json:"status"`
	Runtime   string     `json:"runtime,omitempty"`
	Executor  string     `json:"executor,omitempty"` // docker, local, tunnel
	AgentFile string     `json:"agent_file"`
	Config    RunConfig  `json:"config"`
	Result    *Result    `json:"result,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
	LastActivityAt *time.Time `json:"last_activity_at,omitempty"`
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

// IMBinding links a platform IM user to a registered user.
type IMBinding struct {
	ID               string    `json:"id"`
	UserID           string    `json:"user_id"`
	Platform         string    `json:"platform"`
	PlatformUserID   string    `json:"platform_user_id"`
	PlatformUsername string    `json:"platform_username"`
	CreatedAt        time.Time `json:"created_at"`
}

// BindingCode is a short-lived code for verifying IM bindings.
type BindingCode struct {
	Code      string    `json:"code"`
	UserID    string    `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Workflow represents a multi-step agent workflow definition.
type Workflow struct {
	ID          string         `json:"id"`
	UserID      string         `json:"user_id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Steps       []WorkflowStep `json:"steps"`
	Status      string         `json:"status"` // draft, active, paused
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

// WorkflowStep represents a single step in a workflow.
type WorkflowStep struct {
	ID        string            `json:"id"`
	AgentID   string            `json:"agent_id"`
	Runtime   string            `json:"runtime"`
	Input     string            `json:"input"`
	DependsOn []string          `json:"depends_on"`
	Config    map[string]string `json:"config,omitempty"`
}

// WorkflowRun represents a single execution of a workflow.
type WorkflowRun struct {
	ID         string            `json:"id"`
	WorkflowID string           `json:"workflow_id"`
	Status     string            `json:"status"` // pending, running, completed, failed
	Steps      []WorkflowStepRun `json:"steps"`
	StartedAt  *time.Time        `json:"started_at,omitempty"`
	EndedAt    *time.Time        `json:"ended_at,omitempty"`
}

// WorkflowStepRun represents the execution state of a single workflow step.
type WorkflowStepRun struct {
	StepID    string     `json:"step_id"`
	RunID     string     `json:"run_id"`
	Status    string     `json:"status"`
	Output    string     `json:"output,omitempty"`
	StartedAt *time.Time `json:"started_at,omitempty"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
}

// Schedule represents a cron-based recurring agent execution.
type Schedule struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	Name      string     `json:"name"`
	AgentID   string     `json:"agent_id"`
	Runtime   string     `json:"runtime"`
	CronExpr  string     `json:"cron_expr"`
	Timezone  string     `json:"timezone"`
	Input     string     `json:"input"`
	Enabled   bool       `json:"enabled"`
	LastRunAt *time.Time `json:"last_run_at,omitempty"`
	NextRunAt *time.Time `json:"next_run_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// Team represents a collaborative workspace.
type Team struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	OwnerID   string    `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
}

// TeamMember represents a user's membership in a team.
type TeamMember struct {
	TeamID   string    `json:"team_id"`
	UserID   string    `json:"user_id"`
	Role     string    `json:"role"` // owner, admin, member
	JoinedAt time.Time `json:"joined_at"`
}

// PluginInfo describes a loaded plugin for API responses.
type PluginInfo struct {
	Name    string          `json:"name"`
	Type    string          `json:"type"`
	Version string          `json:"version"`
	Config  json.RawMessage `json:"config,omitempty"`
}
