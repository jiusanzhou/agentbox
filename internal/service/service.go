package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"

	"go.zoe.im/agentbox/internal/config"
	"go.zoe.im/agentbox/internal/engine"
	"go.zoe.im/agentbox/internal/executor"
	"go.zoe.im/agentbox/internal/model"
	"go.zoe.im/agentbox/internal/storage"
	"go.zoe.im/agentbox/internal/store"
	"encoding/json"
	"net/http"
	"go.zoe.im/x/talk"

	// register default implementations
	_ "go.zoe.im/agentbox/internal/executor/docker"
	_ "go.zoe.im/agentbox/internal/storage/local"
	_ "go.zoe.im/agentbox/internal/store/memory"
	_ "go.zoe.im/agentbox/internal/store/sqlite"

	// register talk http transport
	_ "go.zoe.im/x/talk/transport/http/std"
)

// Service is the main application service.
type Service struct {
	cfg     *config.Config
	engine  *engine.Engine
	storage storage.Storage
	server  *talk.Server
	logger  *slog.Logger
}

func New(cfg *config.Config) (*Service, error) {
	logger := slog.Default()

	// Create store
	s, err := store.New(cfg.Store)
	if err != nil {
		return nil, fmt.Errorf("init store: %w", err)
	}

	// Create storage
	st, err := storage.New(cfg.Storage)
	if err != nil {
		return nil, fmt.Errorf("init storage: %w", err)
	}

	// Create executor
	exec, err := executor.New(cfg.Executor)
	if err != nil {
		return nil, fmt.Errorf("init executor: %w", err)
	}

	// Create engine
	eng := engine.New(s, exec, logger)

	// Create talk server
	server, err := talk.NewServerFromConfig(cfg.Server,
		talk.WithPathPrefix("/api/v1"),
	)
	if err != nil {
		return nil, fmt.Errorf("init server: %w", err)
	}

	svc := &Service{
		cfg:     cfg,
		engine:  eng,
		storage: st,
		server:  server,
		logger:  logger,
	}

	// Register endpoints via talk reflection
	if err := server.Register(svc); err != nil {
		return nil, fmt.Errorf("register endpoints: %w", err)
	}

	return svc, nil
}

// Start runs the server.
func (s *Service) Start(ctx context.Context) error {
	s.logger.Info("starting agentbox", "addr", s.cfg.Addr)
	return s.server.Serve(ctx)
}

// Shutdown gracefully stops the server.
func (s *Service) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// --- talk endpoints (auto-extracted by reflection) ---

type CreateRunRequest struct {
	Name      string          `json:"name"`
	AgentFile string          `json:"agent_file"`
	Config    model.RunConfig `json:"config"`
}

// CreateRun handles POST /run
func (s *Service) CreateRun(ctx context.Context, req *CreateRunRequest) (*model.Run, error) {
	if req.AgentFile == "" {
		return nil, talk.NewError(talk.InvalidArgument, "agent_file is required")
	}

	run := &model.Run{
		ID:        shortID(),
		Name:      req.Name,
		AgentFile: req.AgentFile,
		Config:    req.Config,
	}
	if run.Config.Timeout == 0 {
		run.Config.Timeout = 3600
	}

	if err := s.engine.Submit(ctx, run); err != nil {
		return nil, talk.NewError(talk.Internal, err.Error())
	}
	return run, nil
}

// GetRun handles GET /run/{id}
func (s *Service) GetRun(ctx context.Context, id string) (*model.Run, error) {
	run, err := s.engine.Get(ctx, id)
	if err != nil {
		return nil, talk.NewError(talk.NotFound, err.Error())
	}
	return run, nil
}

// ListRuns handles GET /runs
func (s *Service) ListRuns(ctx context.Context) ([]*model.Run, error) {
	return s.engine.List(ctx, 50, 0)
}

type CancelRunResponse struct {
	Status string `json:"status"`
}

// DeleteRun handles DELETE /run/{id} (cancel)
func (s *Service) DeleteRun(ctx context.Context, id string) error {
	return s.engine.Cancel(id)
}

// --- session endpoints ---

type CreateSessionRequest struct {
	Name      string          `json:"name"`
	AgentFile string          `json:"agent_file"`
	Config    model.RunConfig `json:"config"`
}

// CreateSession handles POST /session
func (s *Service) CreateSession(ctx context.Context, req *CreateSessionRequest) (*model.Run, error) {
	if req.AgentFile == "" {
		return nil, talk.NewError(talk.InvalidArgument, "agent_file is required")
	}

	run := &model.Run{
		ID:        shortID(),
		Name:      req.Name,
		Mode:      model.RunModeSession,
		AgentFile: req.AgentFile,
		Config:    req.Config,
	}

	if err := s.engine.StartSession(ctx, run); err != nil {
		return nil, talk.NewError(talk.Internal, err.Error())
	}
	return run, nil
}

type CreateSessionMessageRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

type CreateSessionMessageResponse struct {
	Response string `json:"response"`
}

// CreateSessionMessage handles POST /session_message
func (s *Service) CreateSessionMessage(ctx context.Context, req *CreateSessionMessageRequest) (*CreateSessionMessageResponse, error) {
	if req.SessionID == "" {
		return nil, talk.NewError(talk.InvalidArgument, "session_id is required")
	}
	if req.Message == "" {
		return nil, talk.NewError(talk.InvalidArgument, "message is required")
	}

	resp, err := s.engine.SendMessage(ctx, req.SessionID, req.Message)
	if err != nil {
		return nil, talk.NewError(talk.Internal, err.Error())
	}
	return &CreateSessionMessageResponse{Response: resp}, nil
}

// DeleteSession handles DELETE /session/{id}
func (s *Service) DeleteSession(ctx context.Context, id string) error {
	return s.engine.StopSession(ctx, id)
}

// @talk path=/healthz method=GET
func (s *Service) GetHealth(ctx context.Context) (map[string]string, error) {
	return map[string]string{"status": "ok"}, nil
}

// TalkAnnotations controls endpoint extraction.
func (s *Service) TalkAnnotations() map[string]string {
	return map[string]string{
		"Start":    "@talk skip",
		"Shutdown": "@talk skip",
	}
}

func shortID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}



// StreamSessionMessage handles SSE streaming for session messages.
// Registered as raw HTTP handler, not via talk reflection.
func (s *Service) StreamSessionMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SessionID string `+"`"+`json:"session_id"`+"`"+`
		Message   string `+"`"+`json:"message"`+"`"+`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	onToken := func(token string) {
		data, _ := json.Marshal(map[string]string{"token": token})
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}

	result, err := s.engine.SendMessageStream(r.Context(), req.SessionID, req.Message, onToken)
	if err != nil {
		data, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		return
	}

	// Send done event
	data, _ := json.Marshal(map[string]string{"done": "true", "result": result})
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}
