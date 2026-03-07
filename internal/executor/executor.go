package executor

import (
	"context"

	"go.zoe.im/x"
	"go.zoe.im/x/factory"
)

var (
	executorFactory = factory.NewFactory[Executor, any]()

	Create   = executorFactory.Create
	Register = executorFactory.Register
)

type Request struct {
	ID        string
	AgentFile string
	Image     string
	Runtime   string
	Env       map[string]string
	Timeout   int
	Volumes   []VolumeMount
}

type VolumeMount struct {
	Name      string
	Source     string
	MountPath string
}

type Response struct {
	ExitCode  int
	Output    string
	Artifacts []string
	Logs      string
}

// TokenCallback receives streaming tokens.
type TokenCallback func(token string)

type Executor interface {
	Execute(ctx context.Context, req *Request) (*Response, error)
	Logs(ctx context.Context, id string) (string, error)
	Stop(ctx context.Context, id string) error

	// Session methods
	StartSession(ctx context.Context, req *Request) (string, error)
	SendMessage(ctx context.Context, id string, message string) (string, error)
	SendMessageStream(ctx context.Context, id string, message string, onToken TokenCallback) (string, error)
	StopSession(ctx context.Context, id string) error

	// RecoverSessions returns IDs of running session containers/pods.
	RecoverSessions(ctx context.Context) ([]string, error)

	// UploadFile copies a file into a running session container at /workspace/uploads/.
	UploadFile(ctx context.Context, runID string, filename string, data []byte) error

	// StreamLogs streams container logs line by line.
	StreamLogs(ctx context.Context, runID string) (<-chan string, error)
}

func New(cfg x.TypedLazyConfig) (Executor, error) {
	return Create(cfg)
}
