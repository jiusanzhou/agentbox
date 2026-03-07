package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"go.zoe.im/agentbox/internal/auth"
	"go.zoe.im/agentbox/internal/channel"
	"go.zoe.im/agentbox/internal/config"
	"go.zoe.im/agentbox/internal/engine"
	"go.zoe.im/agentbox/internal/executor"
	"go.zoe.im/agentbox/internal/model"
	"go.zoe.im/agentbox/internal/ratelimit"
	"go.zoe.im/agentbox/internal/storage"
	"go.zoe.im/agentbox/internal/store"
	"go.zoe.im/agentbox/internal/tunnel"
	"go.zoe.im/x/talk"
	stdhttp "go.zoe.im/x/talk/transport/http/std"

	// register default implementations
	_ "go.zoe.im/agentbox/internal/executor/docker"
	_ "go.zoe.im/agentbox/internal/storage/local"
	_ "go.zoe.im/agentbox/internal/store/memory"
	_ "go.zoe.im/agentbox/internal/store/sqlite"

	// register channel implementations
	_ "go.zoe.im/agentbox/internal/channel/telegram"

	// register runtime implementations
	_ "go.zoe.im/agentbox/internal/runtime"
)

// Context keys for API provider headers.
type contextKey string

const (
	apiKeyContextKey  contextKey = "x_api_key"
	baseURLContextKey contextKey = "x_base_url"
	modelContextKey   contextKey = "x_model"
)

// APIKeyFromContext returns the Anthropic API key from context (set by middleware).
func APIKeyFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(apiKeyContextKey).(string); ok {
		return v
	}
	return ""
}

// BaseURLFromContext returns the base URL override from context.
func BaseURLFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(baseURLContextKey).(string); ok {
		return v
	}
	return ""
}

// ModelFromContext returns the model override from context.
func ModelFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(modelContextKey).(string); ok {
		return v
	}
	return ""
}

// Service is the main application service.
type Service struct {
	cfg     *config.Config
	engine  *engine.Engine
	storage storage.Storage
	server  *talk.Server
	router  *channel.Router
	auth    *auth.Auth
	hub     *tunnel.Hub
	mux     *http.ServeMux
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

	// Create auth (if enabled)
	var authInst *auth.Auth
	if cfg.Auth.Enabled {
		authInst = auth.New(s, cfg.Auth.JWTSecret)
	}

	// Create shared HTTP mux for both talk endpoints and tunnel WebSocket
	mux := http.NewServeMux()

	// Create talk transport with shared mux
	transport, err := stdhttp.NewServer(cfg.Server, stdhttp.WithServeMux(mux))
	if err != nil {
		return nil, fmt.Errorf("init transport: %w", err)
	}

	// Create talk server
	server := talk.NewServer(transport, talk.WithPathPrefix("/api/v1"))

	// Create tunnel hub with token validator
	hub := tunnel.NewHub(logger, func(token string) (string, error) {
		if authInst == nil {
			return "anonymous", nil
		}
		// Try JWT first, then API key
		if strings.HasPrefix(token, "ak_") {
			user, err := authInst.ValidateAPIKey(context.Background(), token)
			if err != nil {
				return "", err
			}
			return user.ID, nil
		}
		user, err := authInst.ValidateToken(context.Background(), token)
		if err != nil {
			return "", err
		}
		return user.ID, nil
	})

	// Register tunnel WebSocket endpoint on shared mux
	mux.HandleFunc("GET /api/v1/tunnel", hub.HandleConnect)

	svc := &Service{
		cfg:     cfg,
		engine:  eng,
		storage: st,
		server:  server,
		auth:    authInst,
		hub:     hub,
		mux:     mux,
		logger:  logger,
	}

	// Register SSE streaming endpoint (raw HTTP, not via talk)
	mux.HandleFunc("POST /api/v1/stream", svc.StreamSessionMessage)

	// Register file upload endpoint (multipart, not via talk)
	mux.HandleFunc("POST /api/v1/upload", svc.HandleUpload)

	// Register log streaming endpoint (SSE, not via talk)
	mux.HandleFunc("GET /api/v1/logs/{id}", svc.HandleStreamLogs)

	// Initialize channel router if channels are configured.
	if len(cfg.Channels) > 0 {
		router := channel.NewRouter(eng, logger)
		for _, chCfg := range cfg.Channels {
			ch, err := channel.New(chCfg)
			if err != nil {
				return nil, fmt.Errorf("init channel %s: %w", chCfg.Type, err)
			}
			router.Add(ch)
		}
		svc.router = router
	}

	// Register endpoints via talk reflection
	if err := server.Register(svc); err != nil {
		return nil, fmt.Errorf("register endpoints: %w", err)
	}

	return svc, nil
}

// Start runs the server and channel router.
func (s *Service) Start(ctx context.Context) error {
	// Recover existing sessions from running containers/pods
	if err := s.engine.RecoverSessions(ctx); err != nil {
		s.logger.Warn("session recovery failed", "err", err)
	}

	if s.router != nil {
		if err := s.router.Start(ctx); err != nil {
			return fmt.Errorf("start channel router: %w", err)
		}
	}

	// Start tunnel proxy for sandbox containers
	if s.cfg.TunnelProxyAddr != "" {
		proxy := tunnel.NewProxy(s.hub, s.cfg.TunnelProxyAddr, s.logger)
		go func() {
			if err := proxy.Start(ctx); err != nil && err != http.ErrServerClosed {
				s.logger.Error("tunnel proxy error", "err", err)
			}
		}()
	}

	// Register talk endpoints on the shared mux
	// Note: with external mux, Serve() blocks on <-ctx.Done(), so run in goroutine
	go s.server.Serve(ctx)

	// Start HTTP server with the shared mux
	s.logger.Info("starting agentbox", "addr", s.cfg.Addr)
	var handler http.Handler = s.mux
	if s.auth != nil {
		handler = s.auth.Middleware(handler)
	}

	// Rate limiting middleware
	limiter := ratelimit.New(s.cfg.RateLimit)
	handler = limiter.Middleware(func(r *http.Request) string {
		if user := auth.UserFromContext(r.Context()); user != nil {
			return user.ID
		}
		return ""
	})(handler)
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				limiter.Cleanup(30 * time.Minute)
			}
		}
	}()

	// Session TTL cleanup
	ttl := time.Hour
	if s.cfg.SessionTTL != "" {
		if d, err := time.ParseDuration(s.cfg.SessionTTL); err == nil {
			ttl = d
		}
	}
	cleanupInterval := 5 * time.Minute
	if s.cfg.CleanupInterval != "" {
		if d, err := time.ParseDuration(s.cfg.CleanupInterval); err == nil {
			cleanupInterval = d
		}
	}
	go s.engine.StartCleanup(ctx, ttl, cleanupInterval)
	s.logger.Info("session cleanup started", "ttl", ttl, "interval", cleanupInterval)

	// Middleware to extract API provider headers into context
	handler = apiHeaderMiddleware(handler)
	httpSrv := &http.Server{Addr: s.cfg.Addr, Handler: handler}
	go func() {
		<-ctx.Done()
		httpSrv.Shutdown(context.Background())
	}()
	return httpSrv.ListenAndServe()
}

// apiHeaderMiddleware extracts x-api-key, x-base-url, x-model headers into context.
func apiHeaderMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if v := r.Header.Get("x-api-key"); v != "" {
			ctx = context.WithValue(ctx, apiKeyContextKey, v)
		}
		if v := r.Header.Get("x-base-url"); v != "" {
			ctx = context.WithValue(ctx, baseURLContextKey, v)
		}
		if v := r.Header.Get("x-model"); v != "" {
			ctx = context.WithValue(ctx, modelContextKey, v)
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Shutdown gracefully stops the server and channels.
func (s *Service) Shutdown(ctx context.Context) error {
	if s.router != nil {
		_ = s.router.Stop(ctx)
	}
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
	Runtime   string          `json:"runtime"`
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
		Runtime:   req.Runtime,
		AgentFile: req.AgentFile,
		Config:    req.Config,
	}

	// Inject API provider settings from request headers (via context middleware)
	if apiKey := APIKeyFromContext(ctx); apiKey != "" {
		if run.Config.Env == nil {
			run.Config.Env = make(map[string]string)
		}
		run.Config.Env["ANTHROPIC_API_KEY"] = apiKey
	}
	if baseURL := BaseURLFromContext(ctx); baseURL != "" {
		if run.Config.Env == nil {
			run.Config.Env = make(map[string]string)
		}
		run.Config.Env["ANTHROPIC_BASE_URL"] = baseURL
	}
	if model := ModelFromContext(ctx); model != "" {
		if run.Config.Env == nil {
			run.Config.Env = make(map[string]string)
		}
		run.Config.Env["ANTHROPIC_MODEL"] = model
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
		SessionID string `json:"session_id"`
		Message   string `json:"message"`
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

	var tokensSent bool
	wrappedOnToken := func(token string) {
		tokensSent = true
		onToken(token)
	}

	result, err := s.engine.SendMessageStream(r.Context(), req.SessionID, req.Message, wrappedOnToken)
	if err != nil {
		data, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		return
	}

	// If no streaming tokens were sent, simulate typing effect
	if !tokensSent && result != "" {
		runes := []rune(result)
		chunkSize := 4 // characters per SSE event
		for i := 0; i < len(runes); i += chunkSize {
			end := i + chunkSize
			if end > len(runes) {
				end = len(runes)
			}
			token := string(runes[i:end])
			data, _ := json.Marshal(map[string]string{"token": token})
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}

	// Send done event
	data, _ := json.Marshal(map[string]string{"done": "true", "result": result})
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()
}

// HandleUpload handles file uploads to a running session container.
func (s *Service) HandleUpload(w http.ResponseWriter, r *http.Request) {
	// Auth check
	if s.auth != nil {
		user := auth.UserFromContext(r.Context())
		if user == nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
	}

	sessionID := r.FormValue("session_id")
	if sessionID == "" {
		http.Error(w, `{"error":"session_id required"}`, http.StatusBadRequest)
		return
	}

	// Max 50MB
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		http.Error(w, `{"error":"invalid multipart form"}`, http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, `{"error":"file required"}`, http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		http.Error(w, `{"error":"failed to read file"}`, http.StatusInternalServerError)
		return
	}

	if err := s.engine.UploadFile(r.Context(), sessionID, header.Filename, data); err != nil {
		s.logger.Error("upload file failed", "session", sessionID, "err", err)
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"path": "/workspace/uploads/" + header.Filename,
		"name": header.Filename,
		"size": fmt.Sprintf("%d", len(data)),
	})
}

// HandleStreamLogs streams container logs via SSE.
func (s *Service) HandleStreamLogs(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, `{"error":"streaming not supported"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ctx := r.Context()
	logCh, err := s.engine.StreamLogs(ctx, id)
	if err != nil {
		data, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
		return
	}

	for line := range logCh {
		data, _ := json.Marshal(map[string]string{"log": line})
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type RegisterResponse struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

// CreateAuthRegister handles POST /auth/register
func (s *Service) CreateAuthRegister(ctx context.Context, req *RegisterRequest) (*RegisterResponse, error) {
	if s.auth == nil {
		return nil, talk.NewError(talk.FailedPrecondition, "auth not enabled")
	}
	if req.Email == "" || req.Password == "" {
		return nil, talk.NewError(talk.InvalidArgument, "email and password required")
	}

	user, err := s.auth.Register(ctx, req.Email, req.Password, req.Name)
	if err != nil {
		return nil, talk.NewError(talk.AlreadyExists, err.Error())
	}

	token, _, err := s.auth.Login(ctx, req.Email, req.Password)
	if err != nil {
		return nil, talk.NewError(talk.Internal, err.Error())
	}

	return &RegisterResponse{Token: token, User: user}, nil
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

// CreateAuthLogin handles POST /auth/login
func (s *Service) CreateAuthLogin(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
	if s.auth == nil {
		return nil, talk.NewError(talk.FailedPrecondition, "auth not enabled")
	}

	token, user, err := s.auth.Login(ctx, req.Email, req.Password)
	if err != nil {
		return nil, talk.NewError(talk.Unauthenticated, "invalid credentials")
	}

	return &LoginResponse{Token: token, User: user}, nil
}

// GetAuthMe handles GET /auth/me
func (s *Service) GetAuthMe(ctx context.Context) (*model.User, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return nil, talk.NewError(talk.Unauthenticated, "not authenticated")
	}
	return user, nil
}

type APIKeyResponse struct {
	APIKey string `json:"api_key"`
}

// CreateAuthApikey handles POST /auth/apikey
func (s *Service) CreateAuthApikey(ctx context.Context) (*APIKeyResponse, error) {
	user := auth.UserFromContext(ctx)
	if user == nil {
		return nil, talk.NewError(talk.Unauthenticated, "not authenticated")
	}

	key, err := s.auth.GenerateAPIKey(ctx, user.ID)
	if err != nil {
		return nil, talk.NewError(talk.Internal, err.Error())
	}

	return &APIKeyResponse{APIKey: key}, nil
}
