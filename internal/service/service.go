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
	"go.zoe.im/agentbox/internal/integration"
	"go.zoe.im/agentbox/internal/model"
	"go.zoe.im/agentbox/internal/ratelimit"
	"go.zoe.im/agentbox/internal/runtime"
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
	_ "go.zoe.im/agentbox/internal/channel/discord"
	_ "go.zoe.im/agentbox/internal/channel/slack"
	_ "go.zoe.im/agentbox/internal/channel/telegram"
	_ "go.zoe.im/agentbox/internal/channel/wecom"
	_ "go.zoe.im/agentbox/internal/channel/webhook"

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
	cfg          *config.Config
	configPath   string
	engine       *engine.Engine
	storage      storage.Storage
	server       *talk.Server
	router       *channel.Router
	integrations *integration.Manager
	auth         *auth.Auth
	hub          *tunnel.Hub
	mux          *http.ServeMux
	limiter      *ratelimit.Limiter
	store        store.Store
	logger       *slog.Logger
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
		store:   s,
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
			ch, err := channel.New(chCfg, mux)
			if err != nil {
				return nil, fmt.Errorf("init channel %s: %w", chCfg.Type, err)
			}
			// Register webhook HTTP handler on shared mux.
			if wh, ok := ch.(interface {
				Path() string
				HandleIncoming(http.ResponseWriter, *http.Request)
			}); ok {
				mux.HandleFunc("POST "+wh.Path(), wh.HandleIncoming)
			}
			router.Add(ch)
		}
		svc.router = router
	}

	// Register admin config API endpoints (raw HTTP, not via talk)
	mux.HandleFunc("GET /api/v1/admin/config", svc.GetConfig)
	mux.HandleFunc("PUT /api/v1/admin/config", svc.UpdateConfig)
	mux.HandleFunc("GET /api/v1/admin/config/channels", svc.GetChannels)
	mux.HandleFunc("POST /api/v1/admin/config/channels", svc.AddChannel)
	mux.HandleFunc("DELETE /api/v1/admin/config/channels/{index}", svc.RemoveChannel)
	mux.HandleFunc("GET /api/v1/admin/runtimes", svc.ListRuntimes)

	// Create integration manager for per-user IM channel bindings
	svc.integrations = integration.NewManager(s, eng, mux, logger)

	// Register integration API endpoints
	mux.HandleFunc("GET /api/v1/integrations", svc.ListIntegrations)
	mux.HandleFunc("POST /api/v1/integrations", svc.CreateIntegration)
	mux.HandleFunc("GET /api/v1/integrations/{id}", svc.GetIntegration)
	mux.HandleFunc("PUT /api/v1/integrations/{id}", svc.UpdateIntegration)
	mux.HandleFunc("DELETE /api/v1/integrations/{id}", svc.DeleteIntegration)
	mux.HandleFunc("POST /api/v1/integrations/{id}/test", svc.TestIntegration)

	// Webhook endpoint for integrations (no auth required — verified by HMAC)
	mux.HandleFunc("POST /api/v1/hook/{id}", svc.integrations.HandleWebhook)

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

	// Start per-user integration channels
	if err := s.integrations.Start(ctx); err != nil {
		s.logger.Warn("integration manager start failed", "err", err)
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
	s.limiter = ratelimit.New(s.cfg.RateLimit)
	handler = s.limiter.Middleware(func(r *http.Request) string {
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
				s.limiter.Cleanup(30 * time.Minute)
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
	if s.integrations != nil {
		_ = s.integrations.Stop(ctx)
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
		// Use runtime's EnvKeys for dynamic API key mapping
		rt := runtime.Get(req.Runtime)
		if rt == nil {
			rt = runtime.Default()
		}
		for _, key := range rt.EnvKeys() {
			run.Config.Env[key] = apiKey
		}
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
		"Start":             "@talk skip",
		"Shutdown":          "@talk skip",
		"GetConfig":         "@talk skip",
		"UpdateConfig":      "@talk skip",
		"GetChannels":       "@talk skip",
		"AddChannel":        "@talk skip",
		"RemoveChannel":     "@talk skip",
		"ListRuntimes":      "@talk skip",
		"ReloadChannels":    "@talk skip",
		"SetConfigPath":     "@talk skip",
		"ListIntegrations":  "@talk skip",
		"CreateIntegration": "@talk skip",
		"GetIntegration":    "@talk skip",
		"UpdateIntegration": "@talk skip",
		"DeleteIntegration": "@talk skip",
		"TestIntegration":   "@talk skip",
	}
}

func shortID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// --- Integration endpoints (raw HTTP, not via talk) ---

// ListIntegrations handles GET /api/v1/integrations
func (s *Service) ListIntegrations(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	list, err := s.store.ListIntegrations(r.Context(), user.ID)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []*model.Integration{}
	}

	// Mask secrets in config
	for _, intg := range list {
		intg.Config = maskConfig(intg.Type, intg.Config)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}

// CreateIntegration handles POST /api/v1/integrations
func (s *Service) CreateIntegration(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	var req struct {
		Type      string          `json:"type"`
		Name      string          `json:"name"`
		Config    json.RawMessage `json:"config"`
		SessionID string          `json:"session_id"`
		Enabled   bool            `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if req.Type == "" {
		http.Error(w, `{"error":"type is required"}`, http.StatusBadRequest)
		return
	}

	intg := &model.Integration{
		UserID:    user.ID,
		Type:      req.Type,
		Name:      req.Name,
		Config:    req.Config,
		SessionID: req.SessionID,
		Enabled:   req.Enabled,
	}

	if err := s.integrations.AddIntegration(r.Context(), intg); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(intg)
}

// GetIntegration handles GET /api/v1/integrations/{id}
func (s *Service) GetIntegration(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	intg, err := s.store.GetIntegration(r.Context(), r.PathValue("id"))
	if err != nil {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}
	if intg.UserID != user.ID {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	intg.Config = maskConfig(intg.Type, intg.Config)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(intg)
}

// UpdateIntegration handles PUT /api/v1/integrations/{id}
func (s *Service) UpdateIntegration(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	intg, err := s.store.GetIntegration(r.Context(), r.PathValue("id"))
	if err != nil || intg.UserID != user.ID {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	var req struct {
		Name      *string          `json:"name"`
		Config    *json.RawMessage `json:"config"`
		SessionID *string          `json:"session_id"`
		Enabled   *bool            `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if req.Name != nil {
		intg.Name = *req.Name
	}
	if req.Config != nil {
		intg.Config = *req.Config
	}
	if req.SessionID != nil {
		intg.SessionID = *req.SessionID
	}
	if req.Enabled != nil {
		intg.Enabled = *req.Enabled
	}

	if err := s.integrations.UpdateIntegration(r.Context(), intg); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(intg)
}

// DeleteIntegration handles DELETE /api/v1/integrations/{id}
func (s *Service) DeleteIntegration(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	intg, err := s.store.GetIntegration(r.Context(), r.PathValue("id"))
	if err != nil || intg.UserID != user.ID {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	if err := s.integrations.RemoveIntegration(r.Context(), intg.ID); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// TestIntegration handles POST /api/v1/integrations/{id}/test
func (s *Service) TestIntegration(w http.ResponseWriter, r *http.Request) {
	user := auth.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	intg, err := s.store.GetIntegration(r.Context(), r.PathValue("id"))
	if err != nil || intg.UserID != user.ID {
		http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
		return
	}

	if err := s.integrations.TestIntegration(r.Context(), intg); err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "error", "error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// maskConfig masks sensitive fields in integration config for API responses.
func maskConfig(typ string, raw json.RawMessage) json.RawMessage {
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return raw
	}
	secretKeys := []string{"bot_token", "token", "secret", "app_token", "encoding_aes_key"}
	for _, key := range secretKeys {
		if v, ok := m[key].(string); ok && len(v) > 8 {
			m[key] = v[:4] + "****" + v[len(v)-4:]
		}
	}
	masked, _ := json.Marshal(m)
	return masked
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
