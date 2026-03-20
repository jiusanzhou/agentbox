package channel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"go.zoe.im/agentbox/internal/engine"
	"go.zoe.im/agentbox/internal/model"
	"go.zoe.im/agentbox/internal/store"
)

const (
	streamEditInterval = 500 * time.Millisecond
	telegramCharLimit  = 4096
)

// Router routes IM messages to engine sessions.
type Router struct {
	engine      *engine.Engine
	store       store.Store
	channels    []Channel
	sessions    map[string]string // chatID -> sessionID
	mu          sync.RWMutex
	logger      *slog.Logger
	Permissions *PermissionGateway
}

// NewRouter creates a Router.
func NewRouter(eng *engine.Engine, st store.Store, logger *slog.Logger) *Router {
	if logger == nil {
		logger = slog.Default()
	}
	return &Router{
		engine:      eng,
		store:       st,
		channels:    nil,
		sessions:    make(map[string]string),
		logger:      logger,
		Permissions: NewPermissionGateway(),
	}
}

// Add registers a channel with the router.
func (r *Router) Add(ch Channel) {
	r.channels = append(r.channels, ch)
	// Register callback handler for permission buttons.
	ch.OnCallback(r.handleCallback)
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
	r.Permissions.DenyAll()
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
	case strings.HasPrefix(text, "/bind "):
		code := strings.TrimPrefix(text, "/bind ")
		return r.handleBind(ctx, ch, msg, strings.TrimSpace(code))
	case strings.HasPrefix(text, "/switch "):
		agentID := strings.TrimSpace(strings.TrimPrefix(text, "/switch "))
		return r.handleSwitch(ctx, ch, msg, agentID)
	case text == "/sessions":
		return r.handleSessions(ctx, ch, msg)
	}

	// Regular message — route to session.
	return r.handleMessage(ctx, ch, msg, text)
}

func (r *Router) handleMessage(ctx context.Context, ch Channel, msg *Message, text string) error {
	platform := r.platformFromMsg(msg)
	sessionID, err := r.getOrCreateSession(ctx, msg.ChatID, platform)
	if err != nil {
		r.logger.Error("failed to get/create session", "chat_id", msg.ChatID, "err", err)
		return r.send(ctx, ch, msg.ChatID, "Failed to create session: "+err.Error(), msg.ID)
	}

	// Update LastMessageAt on the IMSession.
	r.updateLastMessage(ctx, platform, msg.ChatID)

	// Stream tokens with debounced message editing.
	return r.streamResponse(ctx, ch, msg, sessionID, text)
}

// streamResponse sends a message via engine streaming and edits the IM message
// as tokens arrive, using a debounced ticker (~500ms).
func (r *Router) streamResponse(ctx context.Context, ch Channel, msg *Message, sessionID, text string) error {
	var (
		mu        sync.Mutex
		buf       strings.Builder
		sentMsgID string
		lastSent  string
		done      = make(chan struct{})
		errCh     = make(chan error, 1)
	)

	// Send initial placeholder.
	if err := ch.Send(ctx, msg.ChatID, "...", &SendOptions{ReplyToID: msg.ID}); err != nil {
		// If we can't even send, fall back to non-streaming.
		return r.sendNonStreaming(ctx, ch, msg, sessionID, text)
	}

	onToken := func(token string) {
		mu.Lock()
		buf.WriteString(token)
		mu.Unlock()
	}

	// Start streaming in background.
	go func() {
		resp, err := r.engine.SendMessageStream(ctx, sessionID, text, onToken)
		if err != nil {
			errCh <- err
			close(done)
			return
		}
		// If no tokens were streamed, use the full response.
		mu.Lock()
		if buf.Len() == 0 && resp != "" {
			buf.WriteString(resp)
		}
		mu.Unlock()
		close(done)
	}()

	// Debounced editing loop.
	ticker := time.NewTicker(streamEditInterval)
	defer ticker.Stop()

	flushEdit := func() {
		mu.Lock()
		current := buf.String()
		mu.Unlock()

		if current == "" || current == lastSent {
			return
		}

		// Handle Telegram's 4096 char limit: if text exceeds it, send as new message.
		chName := ""
		if msg.Extra != nil {
			chName = msg.Extra["channel"]
		}

		if chName == "telegram" && len(current) > telegramCharLimit {
			// Split: send what we haven't sent yet as a new message.
			unsent := current[len(lastSent):]
			for len(unsent) > 0 {
				chunk := unsent
				if len(chunk) > telegramCharLimit {
					chunk = chunk[:telegramCharLimit]
				}
				unsent = unsent[len(chunk):]
				if err := ch.Send(ctx, msg.ChatID, chunk, &SendOptions{ParseMode: "markdown"}); err != nil {
					r.logger.Error("send chunk failed", "err", err)
				}
			}
			lastSent = current
			return
		}

		if sentMsgID == "" {
			// Haven't gotten the message ID from the initial send. Try editing placeholder.
			// Use send instead for first real content.
			if err := ch.Send(ctx, msg.ChatID, current, &SendOptions{
				ReplyToID: msg.ID,
				ParseMode: "markdown",
			}); err != nil {
				r.logger.Error("send stream update failed", "err", err)
			}
		} else {
			_ = ch.EditMessage(ctx, msg.ChatID, sentMsgID, current, &SendOptions{ParseMode: "markdown"})
		}
		lastSent = current
	}

	for {
		select {
		case <-ticker.C:
			flushEdit()
		case err := <-errCh:
			// Streaming error — report it.
			r.logger.Error("stream failed", "session_id", sessionID, "err", err)
			r.mu.Lock()
			delete(r.sessions, msg.ChatID)
			r.mu.Unlock()
			return r.send(ctx, ch, msg.ChatID, "Session error: "+err.Error()+"\nUse /new to start a fresh session.", msg.ID)
		case <-done:
			// Final flush.
			mu.Lock()
			final := buf.String()
			mu.Unlock()

			if final == "" {
				final = "(empty response)"
			}

			if final != lastSent {
				if sentMsgID != "" {
					_ = ch.EditMessage(ctx, msg.ChatID, sentMsgID, final, &SendOptions{ParseMode: "markdown"})
				} else {
					_ = ch.Send(ctx, msg.ChatID, final, &SendOptions{
						ReplyToID: msg.ID,
						ParseMode: "markdown",
					})
				}
			}
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// sendNonStreaming falls back to the original non-streaming behavior.
func (r *Router) sendNonStreaming(ctx context.Context, ch Channel, msg *Message, sessionID, text string) error {
	resp, err := r.engine.SendMessage(ctx, sessionID, text)
	if err != nil {
		r.logger.Error("send message failed", "session_id", sessionID, "err", err)
		r.mu.Lock()
		delete(r.sessions, msg.ChatID)
		r.mu.Unlock()
		return r.send(ctx, ch, msg.ChatID, "Session error: "+err.Error()+"\nUse /new to start a fresh session.", msg.ID)
	}
	return r.send(ctx, ch, msg.ChatID, resp, msg.ID)
}

// handleCallback processes button clicks. Permission button IDs start with "permission_".
func (r *Router) handleCallback(ctx context.Context, cb *Callback) error {
	if strings.HasPrefix(cb.ID, "permission_allow_") {
		reqID := strings.TrimPrefix(cb.ID, "permission_allow_")
		return r.Permissions.Resolve(reqID, true)
	}
	if strings.HasPrefix(cb.ID, "permission_deny_") {
		reqID := strings.TrimPrefix(cb.ID, "permission_deny_")
		return r.Permissions.Resolve(reqID, false)
	}
	return nil
}

// RequestPermission sends permission buttons to a chat and blocks until the user responds.
func (r *Router) RequestPermission(ctx context.Context, chatID, tool, description string) bool {
	reqID := shortID()

	r.Permissions.Register(reqID, tool, chatID)

	text := fmt.Sprintf("🔐 **Permission Request**\nTool: `%s`\n%s", tool, description)
	buttons := []Button{
		{ID: "permission_allow_" + reqID, Text: "✅ Allow"},
		{ID: "permission_deny_" + reqID, Text: "❌ Deny"},
	}

	// Find the channel for this chat.
	ch := r.findChannel(chatID)
	if ch == nil {
		r.Permissions.Resolve(reqID, false)
		return false
	}

	_, err := ch.SendWithButtons(ctx, chatID, text, buttons, &SendOptions{ParseMode: "markdown"})
	if err != nil {
		r.logger.Error("send permission buttons failed", "err", err)
		r.Permissions.Resolve(reqID, false)
		return false
	}

	return r.Permissions.WaitFor(reqID)
}

func (r *Router) findChannel(chatID string) Channel {
	if len(r.channels) > 0 {
		return r.channels[0]
	}
	return nil
}

func (r *Router) handleReset(ctx context.Context, ch Channel, msg *Message) error {
	platform := r.platformFromMsg(msg)

	// Deactivate old IMSession if it exists.
	oldIMSession, err := r.store.GetIMSessionByChat(ctx, platform, msg.ChatID)
	if err == nil && oldIMSession != nil {
		oldIMSession.Active = false
		_ = r.store.UpdateIMSession(ctx, oldIMSession)
		_ = r.engine.StopSession(ctx, oldIMSession.SessionID)
	}

	// Also clean in-memory cache.
	r.mu.Lock()
	delete(r.sessions, msg.ChatID)
	r.mu.Unlock()

	sessionID, err := r.createSession(ctx, msg.ChatID, platform, "")
	if err != nil {
		return r.send(ctx, ch, msg.ChatID, "Failed to create session: "+err.Error(), msg.ID)
	}

	return r.send(ctx, ch, msg.ChatID, "New session started: "+sessionID, msg.ID)
}

func (r *Router) handleStop(ctx context.Context, ch Channel, msg *Message) error {
	platform := r.platformFromMsg(msg)

	// Deactivate IMSession in store.
	imSession, err := r.store.GetIMSessionByChat(ctx, platform, msg.ChatID)
	if err == nil && imSession != nil {
		imSession.Active = false
		_ = r.store.UpdateIMSession(ctx, imSession)
		_ = r.engine.StopSession(ctx, imSession.SessionID)
	}

	r.mu.Lock()
	sessionID, exists := r.sessions[msg.ChatID]
	delete(r.sessions, msg.ChatID)
	r.mu.Unlock()

	if !exists && imSession == nil {
		return r.send(ctx, ch, msg.ChatID, "No active session.", msg.ID)
	}

	// Stop engine session from in-memory cache if different from IMSession.
	if exists && (imSession == nil || imSession.SessionID != sessionID) {
		_ = r.engine.StopSession(ctx, sessionID)
	}

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
	platform := r.platformFromMsg(msg)

	// Deactivate old IMSession.
	oldIMSession, err := r.store.GetIMSessionByChat(ctx, platform, msg.ChatID)
	if err == nil && oldIMSession != nil {
		oldIMSession.Active = false
		_ = r.store.UpdateIMSession(ctx, oldIMSession)
		_ = r.engine.StopSession(ctx, oldIMSession.SessionID)
	}

	// Also clean in-memory cache.
	r.mu.Lock()
	delete(r.sessions, msg.ChatID)
	r.mu.Unlock()

	sessionID, err := r.createSession(ctx, msg.ChatID, platform, prompt)
	if err != nil {
		return r.send(ctx, ch, msg.ChatID, "Failed to create session: "+err.Error(), msg.ID)
	}

	return r.send(ctx, ch, msg.ChatID, "Session started with custom agent: "+sessionID, msg.ID)
}

func (r *Router) getOrCreateSession(ctx context.Context, chatID, platform string) (string, error) {
	// First check in-memory cache.
	r.mu.RLock()
	sessionID, exists := r.sessions[chatID]
	r.mu.RUnlock()

	if exists {
		return sessionID, nil
	}

	// Check store for persistent IMSession.
	imSession, err := r.store.GetIMSessionByChat(ctx, platform, chatID)
	if err == nil && imSession != nil {
		// Cache it in memory.
		r.mu.Lock()
		r.sessions[chatID] = imSession.SessionID
		r.mu.Unlock()
		return imSession.SessionID, nil
	}

	return r.createSession(ctx, chatID, platform, "")
}

func (r *Router) createSession(ctx context.Context, chatID string, platform string, agentFile string) (string, error) {
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

	// Resolve the user from IM binding (best-effort).
	userID := ""
	bindingID := ""
	if platform != "" {
		binding, err := r.store.GetIMBindingByPlatform(ctx, platform, chatID)
		if err == nil && binding != nil {
			userID = binding.UserID
			bindingID = binding.ID
		}
	}

	// Persist the IMSession.
	now := time.Now()
	imSession := &model.IMSession{
		ID:             shortID() + shortID(),
		BindingID:      bindingID,
		UserID:         userID,
		Platform:       platform,
		PlatformChatID: chatID,
		SessionID:      id,
		AgentID:        agentFile,
		Active:         true,
		LastMessageAt:  &now,
		CreatedAt:      now,
	}
	if err := r.store.CreateIMSession(ctx, imSession); err != nil {
		r.logger.Error("failed to persist im session", "err", err)
	}

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

func (r *Router) handleSwitch(ctx context.Context, ch Channel, msg *Message, agentID string) error {
	if agentID == "" {
		return r.send(ctx, ch, msg.ChatID, "Usage: /switch <agent-id>", msg.ID)
	}

	platform := r.platformFromMsg(msg)

	// Deactivate old IMSession.
	oldIMSession, err := r.store.GetIMSessionByChat(ctx, platform, msg.ChatID)
	if err == nil && oldIMSession != nil {
		oldIMSession.Active = false
		_ = r.store.UpdateIMSession(ctx, oldIMSession)
		_ = r.engine.StopSession(ctx, oldIMSession.SessionID)
	}

	// Clean in-memory cache.
	r.mu.Lock()
	delete(r.sessions, msg.ChatID)
	r.mu.Unlock()

	sessionID, err := r.createSession(ctx, msg.ChatID, platform, agentID)
	if err != nil {
		return r.send(ctx, ch, msg.ChatID, "Failed to switch agent: "+err.Error(), msg.ID)
	}

	return r.send(ctx, ch, msg.ChatID, fmt.Sprintf("Switched to agent `%s`. Session: %s", agentID, sessionID), msg.ID)
}

func (r *Router) handleSessions(ctx context.Context, ch Channel, msg *Message) error {
	platform := r.platformFromMsg(msg)

	// Resolve user from IM binding.
	userID := ""
	binding, err := r.store.GetIMBindingByPlatform(ctx, platform, msg.ChatID)
	if err == nil && binding != nil {
		userID = binding.UserID
	}

	if userID == "" {
		// No binding -- show current session only.
		r.mu.RLock()
		sessionID, exists := r.sessions[msg.ChatID]
		r.mu.RUnlock()
		if !exists {
			return r.send(ctx, ch, msg.ChatID, "No active sessions. Send a message to start one.", msg.ID)
		}
		return r.send(ctx, ch, msg.ChatID, fmt.Sprintf("Active session: %s\n(Link your account with /bind for full session history)", sessionID), msg.ID)
	}

	sessions, err := r.store.ListIMSessionsByUser(ctx, userID)
	if err != nil {
		return r.send(ctx, ch, msg.ChatID, "Failed to list sessions: "+err.Error(), msg.ID)
	}

	if len(sessions) == 0 {
		return r.send(ctx, ch, msg.ChatID, "No sessions found.", msg.ID)
	}

	var sb strings.Builder
	sb.WriteString("Your sessions:\n")
	for _, sess := range sessions {
		status := "inactive"
		if sess.Active {
			status = "active"
		}
		agent := sess.AgentID
		if agent == "" {
			agent = "default"
		}
		line := fmt.Sprintf("- `%s` [%s] agent:%s", sess.SessionID, status, agent)
		if sess.LastMessageAt != nil {
			line += fmt.Sprintf(" (last: %s)", sess.LastMessageAt.Format("01-02 15:04"))
		}
		sb.WriteString(line + "\n")
	}
	return r.send(ctx, ch, msg.ChatID, sb.String(), msg.ID)
}

func (r *Router) platformFromMsg(msg *Message) string {
	if msg.Extra != nil {
		if chName, ok := msg.Extra["channel"]; ok {
			return chName
		}
	}
	return "unknown"
}

func (r *Router) updateLastMessage(ctx context.Context, platform, chatID string) {
	imSession, err := r.store.GetIMSessionByChat(ctx, platform, chatID)
	if err != nil || imSession == nil {
		return
	}
	now := time.Now()
	imSession.LastMessageAt = &now
	_ = r.store.UpdateIMSession(ctx, imSession)
}

func (r *Router) handleBind(ctx context.Context, ch Channel, msg *Message, code string) error {
	if code == "" {
		return r.send(ctx, ch, msg.ChatID, "Usage: /bind <CODE>", msg.ID)
	}

	// Look up the binding code.
	bc, err := r.store.GetBindingCode(ctx, code)
	if err != nil {
		return r.send(ctx, ch, msg.ChatID, "Invalid or expired binding code.", msg.ID)
	}

	// Determine the platform from the channel name.
	platform := "unknown"
	if msg.Extra != nil {
		if chName, ok := msg.Extra["channel"]; ok {
			platform = chName
		}
	}

	// Create the IM binding.
	binding := &model.IMBinding{
		ID:               shortID() + shortID(),
		UserID:           bc.UserID,
		Platform:         platform,
		PlatformUserID:   msg.ChatID,
		PlatformUsername: msg.ChatID,
		CreatedAt:        time.Now(),
	}
	if err := r.store.CreateIMBinding(ctx, binding); err != nil {
		return r.send(ctx, ch, msg.ChatID, "Failed to create binding: "+err.Error(), msg.ID)
	}

	// Delete the used code.
	_ = r.store.DeleteBindingCode(ctx, code)

	return r.send(ctx, ch, msg.ChatID, "Account linked successfully!", msg.ID)
}
