package executor

import (
	"context"

	"go.zoe.im/x"
	"go.zoe.im/x/factory"
)

var (
	executorFactory = factory.NewFactory[Executor, any]()

	// Create creates an Executor from config.
	Create = executorFactory.Create

	// Register registers an Executor implementation.
	Register = executorFactory.Register
)

// Request defines what to execute in the sandbox.
type Request struct {
	ID        string
	AgentFile string
	Image     string
	Env       map[string]string
	Timeout   int
	Volumes   []VolumeMount
}

// VolumeMount defines a volume to mount in the sandbox.
type VolumeMount struct {
	Name      string
	Source    string
	MountPath string
}

// Response holds sandbox execution results.
type Response struct {
	ExitCode  int
	Output    string
	Artifacts []string
	Logs      string
}

// Executor creates and manages sandbox environments.
type Executor interface {
	Execute(ctx context.Context, req *Request) (*Response, error)
	Logs(ctx context.Context, id string) (string, error)
	Stop(ctx context.Context, id string) error

	// Session methods for interactive persistent containers.
	StartSession(ctx context.Context, req *Request) (string, error)
	SendMessage(ctx context.Context, id string, message string) (string, error)
	StopSession(ctx context.Context, id string) error
}

// New creates a new Executor from a TypedLazyConfig.
func New(cfg x.TypedLazyConfig) (Executor, error) {
	return Create(cfg)
}
