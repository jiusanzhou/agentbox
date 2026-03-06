package channel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"go.zoe.im/agentbox/internal/engine"
	"go.zoe.im/agentbox/internal/model"
)

// Router routes IM messages to engine sessions.
type Router struct {
	engine   *engine.Engine
	channels []Channel
	sessions map[string]string // chatID -> sessionID
	mu       sync.RWMutex
	logger   *slog.Logger
}

// NewRouter creates a Router.
func NewRouter(eng *engine.Engine, logger *slog.Logger) *Router {
	if logger == nil {
		logger = slog.Default()
	}
	return &Router{
		engine:   eng,
		channels: nil,
		sessions: make(map[string]string),
		logger:   logger,
	}
}

// Add registers a channel with the router.
func (r *Router) Add(ch Channel) {
	r.channels = append(r.channels, ch)
}

// Start starts all channels and begins routing messages.
func (r *Router) Start(ctx context.Context) error {
	for _, ch := range r.channels {
		r.logger.Info("starting channel", "name", ch.Name())
		if err := ch.Start(ctx, r.handle); err != nil {
			return fmt.Errorf("start channel %s: %w", ch.Name(), err)
		}
	}
	return nil
}

// Stop gracefully stops all channels.
func (r *Router) Stop(ctx context.Context) error {
	var errs []error
	for _, ch := range r.channels {
		if err := ch.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("stop channel %s: %w", ch.Name(), err))
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

func (r *Router) handle(ctx context.Context, msg *Message) error {
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return nil
	}

	// Find the channel that received this message (use first match by chatID prefix or just first).
	var ch Channel
	if len(r.channels) > 0 {
		ch = r.channels[0]
	}
	// Try to find the specific channel from Extra metadata.
	if msg.Extra != nil {
		if chName, ok := msg.Extra["channel"]; ok {
			for _, c := range r.channels {
				if c.Name() == chName {
					ch = c
					break
				}
			}
		}
	}

	// Handle special commands.
	switch {
	case text == "/new" || text == "/reset":
		return r.handleReset(ctx, ch, msg)
	case text == "/stop":
		return r.handleStop(ctx, ch, msg)
	case text == "/status":
		return r.handleStatus(ctx, ch, msg)
	case strings.HasPrefix(text, "/agent "):
		prompt := strings.TrimPrefix(text, "/agent ")
		return r.handleAgent(ctx, ch, msg, prompt)
	}

	// Regular message — route to session.
	return r.handleMessage(ctx, ch, msg, text)
}

func (r *Router) handleMessage(ctx context.Context, ch Channel, msg *Message, text string) error {
	sessionID, err := r.getOrCreateSession(ctx, msg.ChatID)
	if err != nil {
		r.logger.Error("failed to get/create session", "chat_id", msg.ChatID, "err", err)
		return r.send(ctx, ch, msg.ChatID, "Failed to create session: "+err.Error(), msg.ID)
	}

	resp, err := r.engine.SendMessage(ctx, sessionID, text)
	if err != nil {
		r.logger.Error("send message failed", "session_id", sessionID, "err", err)
		// If session is broken, clear it so next message creates a new one.
		r.mu.Lock()
		delete(r.sessions, msg.ChatID)
		r.mu.Unlock()
		return r.send(ctx, ch, msg.ChatID, "Session error: "+err.Error()+"\nUse /new to start a fresh session.", msg.ID)
	}

	return r.send(ctx, ch, msg.ChatID, resp, msg.ID)
}

func (r *Router) handleReset(ctx context.Context, ch Channel, msg *Message) error {
	r.mu.Lock()
	oldID, exists := r.sessions[msg.ChatID]
	delete(r.sessions, msg.ChatID)
	r.mu.Unlock()

	if exists {
		_ = r.engine.StopSession(ctx, oldID)
	}

	sessionID, err := r.createSession(ctx, msg.ChatID, "")
	if err != nil {
		return r.send(ctx, ch, msg.ChatID, "Failed to create session: "+err.Error(), msg.ID)
	}

	return r.send(ctx, ch, msg.ChatID, "New session started: "+sessionID, msg.ID)
}

func (r *Router) handleStop(ctx context.Context, ch Channel, msg *Message) error {
	r.mu.Lock()
	sessionID, exists := r.sessions[msg.ChatID]
	delete(r.sessions, msg.ChatID)
	r.mu.Unlock()

	if !exists {
		return r.send(ctx, ch, msg.ChatID, "No active session.", msg.ID)
	}

	_ = r.engine.StopSession(ctx, sessionID)
	return r.send(ctx, ch, msg.ChatID, "Session stopped.", msg.ID)
}

func (r *Router) handleStatus(ctx context.Context, ch Channel, msg *Message) error {
	r.mu.RLock()
	sessionID, exists := r.sessions[msg.ChatID]
	r.mu.RUnlock()

	if !exists {
		return r.send(ctx, ch, msg.ChatID, "No active session. Send a message to start one.", msg.ID)
	}

	run, err := r.engine.Get(ctx, sessionID)
	if err != nil {
		return r.send(ctx, ch, msg.ChatID, "Session "+sessionID+" (status unknown)", msg.ID)
	}

	info := fmt.Sprintf("Session: %s\nStatus: %s\nAgent: %s\nStarted: %s",
		run.ID, run.Status, run.AgentFile, run.CreatedAt.Format("2006-01-02 15:04:05"))
	return r.send(ctx, ch, msg.ChatID, info, msg.ID)
}

func (r *Router) handleAgent(ctx context.Context, ch Channel, msg *Message, prompt string) error {
	// Stop existing session if any.
	r.mu.Lock()
	oldID, exists := r.sessions[msg.ChatID]
	delete(r.sessions, msg.ChatID)
	r.mu.Unlock()

	if exists {
		_ = r.engine.StopSession(ctx, oldID)
	}

	sessionID, err := r.createSession(ctx, msg.ChatID, prompt)
	if err != nil {
		return r.send(ctx, ch, msg.ChatID, "Failed to create session: "+err.Error(), msg.ID)
	}

	return r.send(ctx, ch, msg.ChatID, "Session started with custom agent: "+sessionID, msg.ID)
}

func (r *Router) getOrCreateSession(ctx context.Context, chatID string) (string, error) {
	r.mu.RLock()
	sessionID, exists := r.sessions[chatID]
	r.mu.RUnlock()

	if exists {
		return sessionID, nil
	}

	return r.createSession(ctx, chatID, "")
}

func (r *Router) createSession(ctx context.Context, chatID string, agentFile string) (string, error) {
	if agentFile == "" {
		agentFile = "default"
	}

	id := shortID()
	run := &model.Run{
		ID:        id,
		Name:      "im-" + chatID,
		Mode:      model.RunModeSession,
		AgentFile: agentFile,
	}

	if err := r.engine.StartSession(ctx, run); err != nil {
		return "", err
	}

	r.mu.Lock()
	r.sessions[chatID] = id
	r.mu.Unlock()

	r.logger.Info("session created", "chat_id", chatID, "session_id", id)
	return id, nil
}

func (r *Router) send(ctx context.Context, ch Channel, chatID string, text string, replyTo string) error {
	if ch == nil {
		return fmt.Errorf("no channel available")
	}
	return ch.Send(ctx, chatID, text, &SendOptions{
		ReplyToID: replyTo,
		ParseMode: "markdown",
	})
}

func shortID() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
}
