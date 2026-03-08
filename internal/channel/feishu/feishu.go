package feishu

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.zoe.im/agentbox/internal/channel"
	"go.zoe.im/x"
)

func init() {
	channel.Register("feishu", func(cfg x.TypedLazyConfig, opts ...any) (channel.Channel, error) {
		var c Config
		if len(cfg.Config) > 0 {
			if err := cfg.Unmarshal(&c); err != nil {
				return nil, err
			}
		}
		// Extract mux from opts.
		var mux *http.ServeMux
		for _, opt := range opts {
			if m, ok := opt.(*http.ServeMux); ok {
				mux = m
			}
		}
		return New(c, mux)
	})
}

// Config for the Feishu/Lark channel.
type Config struct {
	AppID             string `json:"app_id" yaml:"app_id"`
	AppSecret         string `json:"app_secret" yaml:"app_secret"`
	VerificationToken string `json:"verification_token" yaml:"verification_token"`
	EncryptKey        string `json:"encrypt_key" yaml:"encrypt_key"`
	CallbackPath      string `json:"callback_path" yaml:"callback_path"`
}

// Feishu implements channel.Channel for 飞书/Lark.
type Feishu struct {
	cfg       Config
	handler   channel.Handler
	cbHandler channel.CallbackHandler
	logger    *slog.Logger
	mux       *http.ServeMux
	client    *http.Client

	mu          sync.RWMutex
	accessToken string
	tokenExpiry time.Time

	// Track processed event IDs to deduplicate.
	eventIDs sync.Map
}

// New creates a Feishu channel.
func New(cfg Config, mux *http.ServeMux) (*Feishu, error) {
	if cfg.AppID == "" || cfg.AppSecret == "" {
		return nil, fmt.Errorf("feishu: app_id and app_secret are required")
	}
	if cfg.CallbackPath == "" {
		cfg.CallbackPath = "/api/v1/feishu/callback"
	}

	return &Feishu{
		cfg:    cfg,
		logger: slog.Default(),
		mux:    mux,
		client: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (f *Feishu) Name() string { return "feishu" }

// Path returns the callback path for HTTP registration.
func (f *Feishu) Path() string { return f.cfg.CallbackPath }

// Start registers HTTP handlers and begins processing.
func (f *Feishu) Start(ctx context.Context, handler channel.Handler) error {
	f.handler = handler
	f.logger.Info("feishu channel started", "path", f.cfg.CallbackPath)
	return nil
}

// handleCallback processes incoming Feishu events (messages, URL verification, card actions).
func (f *Feishu) handleCallback(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	// Decrypt if encrypted.
	var raw json.RawMessage
	if f.cfg.EncryptKey != "" {
		var encrypted struct {
			Encrypt string `json:"encrypt"`
		}
		if err := json.Unmarshal(body, &encrypted); err == nil && encrypted.Encrypt != "" {
			decrypted, err := f.decrypt(encrypted.Encrypt)
			if err != nil {
				f.logger.Error("feishu decrypt failed", "err", err)
				http.Error(w, "decrypt failed", http.StatusBadRequest)
				return
			}
			raw = json.RawMessage(decrypted)
		} else {
			raw = body
		}
	} else {
		raw = body
	}

	// Parse the outer envelope to determine event type.
	var envelope struct {
		Challenge string `json:"challenge"` // URL verification
		Token     string `json:"token"`
		Type      string `json:"type"`   // "url_verification" or "event_callback"
		Schema    string `json:"schema"` // "2.0" for v2 events
		Header    *struct {
			EventID   string `json:"event_id"`
			EventType string `json:"event_type"`
			Token     string `json:"token"`
		} `json:"header"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	// URL verification challenge.
	if envelope.Type == "url_verification" || envelope.Challenge != "" {
		// Verify token if configured.
		if f.cfg.VerificationToken != "" && envelope.Token != f.cfg.VerificationToken {
			http.Error(w, "invalid token", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"challenge": envelope.Challenge})
		return
	}

	// v2 event format (schema "2.0").
	if envelope.Schema == "2.0" && envelope.Header != nil {
		// Verify token.
		if f.cfg.VerificationToken != "" && envelope.Header.Token != f.cfg.VerificationToken {
			http.Error(w, "invalid token", http.StatusForbidden)
			return
		}

		// Deduplicate events.
		if _, loaded := f.eventIDs.LoadOrStore(envelope.Header.EventID, true); loaded {
			w.WriteHeader(http.StatusOK)
			return
		}
		// Clean up old event IDs after 5 minutes.
		go func() {
			time.Sleep(5 * time.Minute)
			f.eventIDs.Delete(envelope.Header.EventID)
		}()

		switch envelope.Header.EventType {
		case "im.message.receive_v1":
			f.handleMessageEvent(r.Context(), raw)
		case "card.action.trigger":
			f.handleCardAction(r.Context(), raw)
		}
	}

	w.WriteHeader(http.StatusOK)
}

// handleMessageEvent handles im.message.receive_v1 events.
func (f *Feishu) handleMessageEvent(ctx context.Context, raw json.RawMessage) {
	var event struct {
		Event struct {
			Sender struct {
				SenderID struct {
					OpenID string `json:"open_id"`
				} `json:"sender_id"`
				SenderType string `json:"sender_type"`
			} `json:"sender"`
			Message struct {
				MessageID   string `json:"message_id"`
				ChatID      string `json:"chat_id"`
				MessageType string `json:"message_type"`
				Content     string `json:"content"` // JSON string
			} `json:"message"`
		} `json:"event"`
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		f.logger.Error("parse message event failed", "err", err)
		return
	}

	// Only handle text messages.
	if event.Event.Message.MessageType != "text" {
		return
	}

	// Parse content JSON to get the actual text.
	var content struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(event.Event.Message.Content), &content); err != nil {
		f.logger.Error("parse message content failed", "err", err)
		return
	}

	// Strip @bot mentions (Feishu uses @_user_xxx pattern).
	text := strings.TrimSpace(content.Text)
	// Remove @_user_N mentions.
	for {
		idx := strings.Index(text, "@_user_")
		if idx < 0 {
			break
		}
		end := idx + 7
		for end < len(text) && text[end] != ' ' && text[end] != '\n' {
			end++
		}
		text = text[:idx] + text[end:]
	}
	text = strings.TrimSpace(text)

	if text == "" {
		return
	}

	msg := &channel.Message{
		ID:       event.Event.Message.MessageID,
		ChatID:   event.Event.Message.ChatID,
		UserID:   event.Event.Sender.SenderID.OpenID,
		Username: event.Event.Sender.SenderID.OpenID,
		Text:     text,
		Extra:    map[string]string{"channel": "feishu"},
	}

	if err := f.handler(ctx, msg); err != nil {
		f.logger.Error("handle message failed", "chat_id", msg.ChatID, "err", err)
	}
}

// handleCardAction handles card.action.trigger events (button clicks).
func (f *Feishu) handleCardAction(ctx context.Context, raw json.RawMessage) {
	if f.cbHandler == nil {
		return
	}

	var event struct {
		Event struct {
			Operator struct {
				OpenID string `json:"open_id"`
			} `json:"operator"`
			Action struct {
				Value map[string]string `json:"value"`
				Tag   string            `json:"tag"`
			} `json:"action"`
			Context struct {
				OpenChatID    string `json:"open_chat_id"`
				OpenMessageID string `json:"open_message_id"`
			} `json:"context"`
		} `json:"event"`
	}
	if err := json.Unmarshal(raw, &event); err != nil {
		f.logger.Error("parse card action failed", "err", err)
		return
	}

	actionID := event.Event.Action.Value["action"]

	cb := &channel.Callback{
		ID:        actionID,
		ChatID:    event.Event.Context.OpenChatID,
		UserID:    event.Event.Operator.OpenID,
		MessageID: event.Event.Context.OpenMessageID,
		Extra:     map[string]string{"channel": "feishu"},
	}

	if err := f.cbHandler(ctx, cb); err != nil {
		f.logger.Error("handle card action failed", "action", actionID, "err", err)
	}
}

// Send sends a text message to a Feishu chat.
func (f *Feishu) Send(ctx context.Context, chatID string, text string, opts *channel.SendOptions) error {
	token, err := f.getTenantAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("feishu: get token: %w", err)
	}

	content, _ := json.Marshal(map[string]string{"text": text})
	payload := map[string]string{
		"receive_id": chatID,
		"msg_type":   "text",
		"content":    string(content),
	}
	data, _ := json.Marshal(payload)

	url := "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id"
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("feishu: send message: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("feishu: decode response: %w", err)
	}
	if result.Code != 0 {
		return fmt.Errorf("feishu: send failed: %d %s", result.Code, result.Msg)
	}
	return nil
}

// EditMessage edits an existing Feishu message via PATCH API.
func (f *Feishu) EditMessage(ctx context.Context, chatID string, messageID string, text string, opts *channel.SendOptions) error {
	token, err := f.getTenantAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("feishu: get token: %w", err)
	}

	content, _ := json.Marshal(map[string]string{"text": text})
	payload := map[string]string{
		"content": string(content),
	}
	data, _ := json.Marshal(payload)

	url := fmt.Sprintf("https://open.feishu.cn/open-apis/im/v1/messages/%s", messageID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := f.client.Do(req)
	if err != nil {
		return fmt.Errorf("feishu: edit message: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("feishu: decode response: %w", err)
	}
	if result.Code != 0 {
		return fmt.Errorf("feishu: edit failed: %d %s", result.Code, result.Msg)
	}
	return nil
}

// SendWithButtons sends an interactive card message with buttons.
func (f *Feishu) SendWithButtons(ctx context.Context, chatID string, text string, buttons []channel.Button, opts *channel.SendOptions) (string, error) {
	token, err := f.getTenantAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("feishu: get token: %w", err)
	}

	// Build interactive card with buttons.
	var actions []map[string]any
	for _, b := range buttons {
		actions = append(actions, map[string]any{
			"tag": "button",
			"text": map[string]string{
				"tag":     "plain_text",
				"content": b.Text,
			},
			"type":  "primary",
			"value": map[string]string{"action": b.ID},
		})
	}

	card := map[string]any{
		"config": map[string]any{
			"wide_screen_mode": true,
		},
		"elements": []map[string]any{
			{
				"tag": "div",
				"text": map[string]string{
					"tag":     "lark_md",
					"content": text,
				},
			},
			{
				"tag":     "action",
				"actions": actions,
			},
		},
	}

	cardJSON, _ := json.Marshal(card)
	payload := map[string]string{
		"receive_id": chatID,
		"msg_type":   "interactive",
		"content":    string(cardJSON),
	}
	data, _ := json.Marshal(payload)

	url := "https://open.feishu.cn/open-apis/im/v1/messages?receive_id_type=chat_id"
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := f.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("feishu: send card: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			MessageID string `json:"message_id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("feishu: decode response: %w", err)
	}
	if result.Code != 0 {
		return "", fmt.Errorf("feishu: send card failed: %d %s", result.Code, result.Msg)
	}
	return result.Data.MessageID, nil
}

// OnCallback registers a handler for card action callbacks.
func (f *Feishu) OnCallback(handler channel.CallbackHandler) {
	f.cbHandler = handler
}

// Stop is a no-op since Feishu uses HTTP callbacks.
func (f *Feishu) Stop(ctx context.Context) error {
	f.logger.Info("feishu channel stopped")
	return nil
}

// HandleIncoming handles incoming HTTP requests (for service.go registration).
func (f *Feishu) HandleIncoming(w http.ResponseWriter, r *http.Request) {
	f.handleCallback(w, r)
}

// getTenantAccessToken returns a cached or fresh tenant access token.
func (f *Feishu) getTenantAccessToken(ctx context.Context) (string, error) {
	f.mu.RLock()
	if f.accessToken != "" && time.Now().Before(f.tokenExpiry) {
		token := f.accessToken
		f.mu.RUnlock()
		return token, nil
	}
	f.mu.RUnlock()

	f.mu.Lock()
	defer f.mu.Unlock()

	// Double-check after acquiring write lock.
	if f.accessToken != "" && time.Now().Before(f.tokenExpiry) {
		return f.accessToken, nil
	}

	payload := map[string]string{
		"app_id":     f.cfg.AppID,
		"app_secret": f.cfg.AppSecret,
	}
	data, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://open.feishu.cn/open-apis/auth/v3/tenant_access_token/internal",
		bytes.NewReader(data))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	resp, err := f.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Code              int    `json:"code"`
		Msg               string `json:"msg"`
		TenantAccessToken string `json:"tenant_access_token"`
		Expire            int    `json:"expire"` // seconds
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Code != 0 {
		return "", fmt.Errorf("feishu: get token: %d %s", result.Code, result.Msg)
	}

	f.accessToken = result.TenantAccessToken
	// Refresh 5 minutes before expiry.
	f.tokenExpiry = time.Now().Add(time.Duration(result.Expire-300) * time.Second)
	return f.accessToken, nil
}

// decrypt decrypts a Feishu encrypted event body.
// Feishu uses AES-256-CBC with SHA-256 hash of encrypt_key as the key.
func (f *Feishu) decrypt(encrypted string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", fmt.Errorf("feishu: base64 decode: %w", err)
	}

	key := sha256.Sum256([]byte(f.cfg.EncryptKey))

	if len(ciphertext) < aes.BlockSize {
		return "", fmt.Errorf("feishu: ciphertext too short")
	}

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return "", fmt.Errorf("feishu: new cipher: %w", err)
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	if len(ciphertext)%aes.BlockSize != 0 {
		return "", fmt.Errorf("feishu: invalid ciphertext length")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)

	// Remove PKCS#7 padding.
	if len(ciphertext) == 0 {
		return "", fmt.Errorf("feishu: empty plaintext")
	}
	pad := int(ciphertext[len(ciphertext)-1])
	if pad < 1 || pad > aes.BlockSize || pad > len(ciphertext) {
		return "", fmt.Errorf("feishu: invalid padding")
	}
	return string(ciphertext[:len(ciphertext)-pad]), nil
}
