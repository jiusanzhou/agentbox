package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"go.zoe.im/agentbox/internal/channel"
	"go.zoe.im/x"
)

func init() {
	channel.Register("webhook", func(cfg x.TypedLazyConfig, opts ...any) (channel.Channel, error) {
		var c Config
		if len(cfg.Config) > 0 {
			if err := cfg.Unmarshal(&c); err != nil {
				return nil, err
			}
		}
		return New(c)
	})
}

// Config for the generic Webhook channel.
type Config struct {
	Path        string `json:"path" yaml:"path"`                 // e.g. "/api/v1/webhook"
	Secret      string `json:"secret" yaml:"secret"`             // HMAC-SHA256 verification
	ResponseURL string `json:"response_url" yaml:"response_url"` // optional async callback
}

type incomingMessage struct {
	ChatID   string            `json:"chat_id"`
	UserID   string            `json:"user_id"`
	Username string            `json:"username"`
	Text     string            `json:"text"`
	Metadata map[string]string `json:"metadata"`
}

// pendingResponse holds a response waiting for sync delivery.
type pendingResponse struct {
	ch chan string
}

// Webhook implements channel.Channel as a generic HTTP endpoint.
type Webhook struct {
	cfg     Config
	handler channel.Handler
	logger  *slog.Logger
	client  *http.Client

	mu       sync.Mutex
	pending  map[string]*pendingResponse // chatID+msgID -> response channel
}

// New creates a Webhook channel.
func New(cfg Config) (*Webhook, error) {
	if cfg.Path == "" {
		cfg.Path = "/api/v1/webhook"
	}

	return &Webhook{
		cfg:     cfg,
		logger:  slog.Default(),
		client:  &http.Client{Timeout: 30 * time.Second},
		pending: make(map[string]*pendingResponse),
	}, nil
}

func (w *Webhook) Name() string { return "webhook" }

// Path returns the HTTP path for registration.
func (w *Webhook) Path() string { return w.cfg.Path }

// Start is a no-op; the HTTP handler is registered externally.
func (w *Webhook) Start(ctx context.Context, handler channel.Handler) error {
	w.handler = handler
	w.logger.Info("webhook channel started", "path", w.cfg.Path)
	return nil
}

// HandleIncoming handles incoming webhook POST requests.
func (w *Webhook) HandleIncoming(rw http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(rw, `{"error":"read body failed"}`, http.StatusBadRequest)
		return
	}

	// Verify HMAC signature if secret is configured.
	if w.cfg.Secret != "" {
		sig := r.Header.Get("X-Signature")
		if !w.verifyHMAC(body, sig) {
			http.Error(rw, `{"error":"invalid signature"}`, http.StatusForbidden)
			return
		}
	}

	var incoming incomingMessage
	if err := json.Unmarshal(body, &incoming); err != nil {
		http.Error(rw, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}

	if incoming.Text == "" {
		http.Error(rw, `{"error":"text is required"}`, http.StatusBadRequest)
		return
	}
	if incoming.ChatID == "" {
		incoming.ChatID = "webhook-default"
	}

	msg := &channel.Message{
		ChatID:   incoming.ChatID,
		UserID:   incoming.UserID,
		Username: incoming.Username,
		Text:     incoming.Text,
		Extra:    map[string]string{"channel": "webhook"},
	}
	for k, v := range incoming.Metadata {
		msg.Extra[k] = v
	}

	// If no response URL, handle synchronously.
	if w.cfg.ResponseURL == "" {
		pr := &pendingResponse{ch: make(chan string, 1)}
		key := incoming.ChatID
		w.mu.Lock()
		w.pending[key] = pr
		w.mu.Unlock()

		go func() {
			if err := w.handler(r.Context(), msg); err != nil {
				w.logger.Error("handle message failed", "chat_id", msg.ChatID, "err", err)
				pr.ch <- "Error: " + err.Error()
			}
		}()

		// Wait for response with timeout.
		select {
		case resp := <-pr.ch:
			w.mu.Lock()
			delete(w.pending, key)
			w.mu.Unlock()

			rw.Header().Set("Content-Type", "application/json")
			json.NewEncoder(rw).Encode(map[string]string{
				"text":    resp,
				"chat_id": incoming.ChatID,
			})
		case <-time.After(2 * time.Minute):
			w.mu.Lock()
			delete(w.pending, key)
			w.mu.Unlock()
			http.Error(rw, `{"error":"timeout"}`, http.StatusGatewayTimeout)
		}
		return
	}

	// Async: acknowledge immediately, response goes to callback URL.
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusAccepted)
	json.NewEncoder(rw).Encode(map[string]string{"status": "accepted"})

	go func() {
		if err := w.handler(r.Context(), msg); err != nil {
			w.logger.Error("handle message failed", "chat_id", msg.ChatID, "err", err)
		}
	}()
}

// Send delivers a response. For sync mode, it pushes to the pending channel.
// For async mode, it POSTs to the response URL.
func (w *Webhook) Send(ctx context.Context, chatID string, text string, opts *channel.SendOptions) error {
	// Try sync delivery first.
	w.mu.Lock()
	pr, ok := w.pending[chatID]
	w.mu.Unlock()
	if ok {
		select {
		case pr.ch <- text:
		default:
		}
		return nil
	}

	// Async callback.
	if w.cfg.ResponseURL != "" {
		payload, _ := json.Marshal(map[string]string{
			"chat_id": chatID,
			"text":    text,
		})
		resp, err := w.client.Post(w.cfg.ResponseURL, "application/json", bytes.NewReader(payload))
		if err != nil {
			return fmt.Errorf("webhook: callback failed: %w", err)
		}
		resp.Body.Close()
		return nil
	}

	return nil
}

// Stop is a no-op for the webhook channel.
func (w *Webhook) Stop(ctx context.Context) error {
	w.logger.Info("webhook channel stopped")
	return nil
}

func (w *Webhook) verifyHMAC(body []byte, signature string) bool {
	mac := hmac.New(sha256.New, []byte(w.cfg.Secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
