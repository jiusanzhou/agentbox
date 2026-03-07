package integration

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.zoe.im/agentbox/internal/channel"
	"go.zoe.im/agentbox/internal/engine"
	"go.zoe.im/agentbox/internal/model"
	"go.zoe.im/agentbox/internal/store"
	"go.zoe.im/x"
)

type activeChannel struct {
	integration *model.Integration
	channel     channel.Channel
	cancel      context.CancelFunc
}

// Manager manages the lifecycle of per-user integration channel instances.
type Manager struct {
	store    store.Store
	engine   *engine.Engine
	channels map[string]*activeChannel // integration_id -> running channel
	mu       sync.RWMutex
	logger   *slog.Logger
	mux      *http.ServeMux
}

// NewManager creates a new integration Manager.
func NewManager(s store.Store, eng *engine.Engine, mux *http.ServeMux, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		store:    s,
		engine:   eng,
		channels: make(map[string]*activeChannel),
		logger:   logger,
		mux:      mux,
	}
}

// Start loads all enabled integrations and starts them.
func (m *Manager) Start(ctx context.Context) error {
	integrations, err := m.store.ListAllEnabledIntegrations(ctx)
	if err != nil {
		return fmt.Errorf("list integrations: %w", err)
	}
	for _, intg := range integrations {
		if err := m.startIntegration(ctx, intg); err != nil {
			m.logger.Error("failed to start integration", "id", intg.ID, "type", intg.Type, "err", err)
		}
	}
	return nil
}

// AddIntegration creates and optionally starts a new integration.
func (m *Manager) AddIntegration(ctx context.Context, intg *model.Integration) error {
	intg.ID = shortID()
	intg.Status = "disconnected"
	intg.CreatedAt = time.Now()
	intg.UpdatedAt = time.Now()

	if err := m.store.CreateIntegration(ctx, intg); err != nil {
		return err
	}

	if intg.Enabled {
		return m.startIntegration(ctx, intg)
	}
	return nil
}

// UpdateIntegration updates config and restarts if needed.
func (m *Manager) UpdateIntegration(ctx context.Context, intg *model.Integration) error {
	intg.UpdatedAt = time.Now()

	// Stop existing if running
	m.mu.RLock()
	_, running := m.channels[intg.ID]
	m.mu.RUnlock()
	if running {
		m.stopChannel(intg.ID)
	}

	if err := m.store.UpdateIntegration(ctx, intg); err != nil {
		return err
	}

	if intg.Enabled {
		return m.startIntegration(ctx, intg)
	}
	return nil
}

// RemoveIntegration stops and deletes an integration.
func (m *Manager) RemoveIntegration(ctx context.Context, id string) error {
	m.stopChannel(id)
	return m.store.DeleteIntegration(ctx, id)
}

// TestIntegration creates a temporary channel to verify credentials.
func (m *Manager) TestIntegration(ctx context.Context, intg *model.Integration) error {
	chCfg := x.TypedLazyConfig{Type: intg.Type, Config: intg.Config}
	ch, err := channel.New(chCfg, m.mux)
	if err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}
	// For channels that support a ping/connect test, start and immediately stop.
	testCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := ch.Start(testCtx, func(_ context.Context, _ *channel.Message) error { return nil }); err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	_ = ch.Stop(testCtx)
	return nil
}

// HandleWebhook handles incoming webhook requests for integrations.
func (m *Manager) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	m.mu.RLock()
	ac, ok := m.channels[id]
	m.mu.RUnlock()

	if !ok {
		http.Error(w, `{"error":"integration not found or not running"}`, http.StatusNotFound)
		return
	}

	// Verify HMAC if configured
	var cfg struct {
		Secret string `json:"secret"`
	}
	json.Unmarshal(ac.integration.Config, &cfg)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, `{"error":"failed to read body"}`, http.StatusBadRequest)
		return
	}

	if cfg.Secret != "" {
		sig := r.Header.Get("X-Signature-256")
		mac := hmac.New(sha256.New, []byte(cfg.Secret))
		mac.Write(body)
		expected := "sha256=" + hex.EncodeToString(mac.Sum(nil))
		if !hmac.Equal([]byte(sig), []byte(expected)) {
			http.Error(w, `{"error":"invalid signature"}`, http.StatusUnauthorized)
			return
		}
	}

	var msg struct {
		Text   string `json:"text"`
		ChatID string `json:"chat_id"`
	}
	if err := json.Unmarshal(body, &msg); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	if msg.ChatID == "" {
		msg.ChatID = "webhook"
	}

	// Route through the integration's handler
	chMsg := &channel.Message{
		ID:     shortID(),
		ChatID: msg.ChatID,
		Text:   msg.Text,
	}

	// Get or create session
	sessionID := ac.integration.SessionID
	if sessionID == "" {
		sessionID = ac.integration.ID // use integration as default session key
	}

	resp, err := m.engine.SendMessage(r.Context(), sessionID, msg.Text)
	if err != nil {
		// Try creating a session first
		run := &model.Run{
			ID:        shortID(),
			Name:      fmt.Sprintf("intg-webhook-%s", chMsg.ChatID),
			Mode:      model.RunModeSession,
			AgentFile: "You are a helpful AI assistant.",
			UserID:    ac.integration.UserID,
		}
		if sErr := m.engine.StartSession(r.Context(), run); sErr != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, sErr.Error()), http.StatusInternalServerError)
			return
		}
		resp, err = m.engine.SendMessage(r.Context(), run.ID, msg.Text)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"response": resp})
}

// Stop shuts down all running integrations.
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.Lock()
	ids := make([]string, 0, len(m.channels))
	for id := range m.channels {
		ids = append(ids, id)
	}
	m.mu.Unlock()

	for _, id := range ids {
		m.stopChannel(id)
	}
	return nil
}

func (m *Manager) startIntegration(ctx context.Context, intg *model.Integration) error {
	chCfg := x.TypedLazyConfig{Type: intg.Type, Config: intg.Config}
	ch, err := channel.New(chCfg, m.mux)
	if err != nil {
		intg.Status = "error"
		intg.Error = err.Error()
		m.store.UpdateIntegration(ctx, intg)
		return err
	}

	handler := m.makeHandler(intg)

	childCtx, cancel := context.WithCancel(ctx)
	if err := ch.Start(childCtx, handler); err != nil {
		cancel()
		intg.Status = "error"
		intg.Error = err.Error()
		m.store.UpdateIntegration(ctx, intg)
		return err
	}

	m.mu.Lock()
	m.channels[intg.ID] = &activeChannel{
		integration: intg,
		channel:     ch,
		cancel:      cancel,
	}
	m.mu.Unlock()

	intg.Status = "connected"
	intg.Error = ""
	m.store.UpdateIntegration(ctx, intg)

	m.logger.Info("integration started", "id", intg.ID, "type", intg.Type, "user", intg.UserID)
	return nil
}

func (m *Manager) makeHandler(intg *model.Integration) channel.Handler {
	sessions := make(map[string]string) // chatID -> sessionID
	var mu sync.RWMutex

	return func(ctx context.Context, msg *channel.Message) error {
		text := strings.TrimSpace(msg.Text)
		if text == "" {
			return nil
		}

		// Handle commands
		switch text {
		case "/new", "/reset":
			mu.Lock()
			delete(sessions, msg.ChatID)
			mu.Unlock()
			m.sendReply(intg.ID, ctx, msg.ChatID, "Session reset. Send a message to start a new one.", msg.ID)
			return nil
		case "/status":
			mu.RLock()
			sid, ok := sessions[msg.ChatID]
			mu.RUnlock()
			if !ok {
				m.sendReply(intg.ID, ctx, msg.ChatID, "No active session.", msg.ID)
			} else {
				m.sendReply(intg.ID, ctx, msg.ChatID, "Active session: "+sid, msg.ID)
			}
			return nil
		}

		// Get or create session for this chat
		mu.RLock()
		sessionID, exists := sessions[msg.ChatID]
		mu.RUnlock()

		if !exists {
			if intg.SessionID != "" {
				sessionID = intg.SessionID
			} else {
				run := &model.Run{
					ID:        shortID(),
					Name:      fmt.Sprintf("intg-%s-%s", intg.Type, msg.ChatID),
					Mode:      model.RunModeSession,
					AgentFile: "You are a helpful AI assistant.",
					UserID:    intg.UserID,
				}
				if err := m.engine.StartSession(ctx, run); err != nil {
					m.sendReply(intg.ID, ctx, msg.ChatID, "Failed to create session: "+err.Error(), msg.ID)
					return err
				}
				sessionID = run.ID
			}
			mu.Lock()
			sessions[msg.ChatID] = sessionID
			mu.Unlock()
		}

		resp, err := m.engine.SendMessage(ctx, sessionID, text)
		if err != nil {
			m.sendReply(intg.ID, ctx, msg.ChatID, "Error: "+err.Error(), msg.ID)
			// Clear broken session
			mu.Lock()
			delete(sessions, msg.ChatID)
			mu.Unlock()
			return err
		}

		m.sendReply(intg.ID, ctx, msg.ChatID, resp, msg.ID)
		return nil
	}
}

func (m *Manager) sendReply(integrationID string, ctx context.Context, chatID, text, replyTo string) {
	m.mu.RLock()
	ac, ok := m.channels[integrationID]
	m.mu.RUnlock()
	if ok {
		ac.channel.Send(ctx, chatID, text, &channel.SendOptions{ReplyToID: replyTo, ParseMode: "markdown"})
	}
}

func (m *Manager) stopChannel(id string) {
	m.mu.Lock()
	ac, ok := m.channels[id]
	if ok {
		delete(m.channels, id)
	}
	m.mu.Unlock()

	if ok {
		ac.cancel()
		ac.channel.Stop(context.Background())
		m.logger.Info("integration stopped", "id", id)
	}
}

func shortID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
