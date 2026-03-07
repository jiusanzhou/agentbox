package wecom

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"go.zoe.im/agentbox/internal/channel"
	"go.zoe.im/x"
)

func init() {
	channel.Register("wecom", func(cfg x.TypedLazyConfig, opts ...any) (channel.Channel, error) {
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

// Config for the WeCom (企业微信) channel.
type Config struct {
	CorpID         string `json:"corp_id" yaml:"corp_id"`
	AgentID        int    `json:"agent_id" yaml:"agent_id"`
	Secret         string `json:"secret" yaml:"secret"`
	Token          string `json:"token" yaml:"token"`
	EncodingAESKey string `json:"encoding_aes_key" yaml:"encoding_aes_key"`
	CallbackPath   string `json:"callback_path" yaml:"callback_path"`
}

// WeCom implements channel.Channel for 企业微信.
type WeCom struct {
	cfg     Config
	handler channel.Handler
	logger  *slog.Logger
	mux     *http.ServeMux
	aesKey  []byte
	client  *http.Client

	mu          sync.RWMutex
	accessToken string
	tokenExpiry time.Time
}

// New creates a WeCom channel.
func New(cfg Config, mux *http.ServeMux) (*WeCom, error) {
	if cfg.CorpID == "" || cfg.Secret == "" {
		return nil, fmt.Errorf("wecom: corp_id and secret are required")
	}
	if cfg.Token == "" || cfg.EncodingAESKey == "" {
		return nil, fmt.Errorf("wecom: token and encoding_aes_key are required")
	}
	if cfg.CallbackPath == "" {
		cfg.CallbackPath = "/api/v1/wecom/callback"
	}

	aesKey, err := base64.StdEncoding.DecodeString(cfg.EncodingAESKey + "=")
	if err != nil {
		return nil, fmt.Errorf("wecom: invalid encoding_aes_key: %w", err)
	}

	return &WeCom{
		cfg:    cfg,
		logger: slog.Default(),
		mux:    mux,
		aesKey: aesKey,
		client: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (w *WeCom) Name() string { return "wecom" }

// Path returns the callback path for HTTP registration.
func (w *WeCom) Path() string { return w.cfg.CallbackPath }

// Start registers the callback handler and begins processing.
func (w *WeCom) Start(ctx context.Context, handler channel.Handler) error {
	w.handler = handler

	if w.mux != nil {
		w.mux.HandleFunc("GET "+w.cfg.CallbackPath, w.handleVerify)
		w.mux.HandleFunc("POST "+w.cfg.CallbackPath, w.handleCallback)
	}

	w.logger.Info("wecom channel started", "path", w.cfg.CallbackPath)
	return nil
}

// handleVerify handles the URL verification request from WeCom.
func (w *WeCom) handleVerify(rw http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	msgSignature := q.Get("msg_signature")
	timestamp := q.Get("timestamp")
	nonce := q.Get("nonce")
	echostr := q.Get("echostr")

	if !w.verifySignature(msgSignature, timestamp, nonce, echostr) {
		http.Error(rw, "invalid signature", http.StatusForbidden)
		return
	}

	decrypted, err := w.decrypt(echostr)
	if err != nil {
		http.Error(rw, "decrypt failed", http.StatusBadRequest)
		return
	}
	rw.Write([]byte(decrypted))
}

// handleCallback processes incoming messages.
func (w *WeCom) handleCallback(rw http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	msgSignature := q.Get("msg_signature")
	timestamp := q.Get("timestamp")
	nonce := q.Get("nonce")

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(rw, "read body failed", http.StatusBadRequest)
		return
	}

	var envelope struct {
		XMLName    xml.Name `xml:"xml"`
		Encrypt    string   `xml:"Encrypt"`
		AgentID    string   `xml:"AgentID"`
	}
	if err := xml.Unmarshal(body, &envelope); err != nil {
		http.Error(rw, "invalid xml", http.StatusBadRequest)
		return
	}

	if !w.verifySignature(msgSignature, timestamp, nonce, envelope.Encrypt) {
		http.Error(rw, "invalid signature", http.StatusForbidden)
		return
	}

	plaintext, err := w.decrypt(envelope.Encrypt)
	if err != nil {
		http.Error(rw, "decrypt failed", http.StatusBadRequest)
		return
	}

	var msg struct {
		XMLName      xml.Name `xml:"xml"`
		MsgType      string   `xml:"MsgType"`
		Content      string   `xml:"Content"`
		FromUserName string   `xml:"FromUserName"`
		MsgId        string   `xml:"MsgId"`
		AgentID      string   `xml:"AgentID"`
	}
	if err := xml.Unmarshal([]byte(plaintext), &msg); err != nil {
		http.Error(rw, "parse message failed", http.StatusBadRequest)
		return
	}

	// Only handle text messages.
	if msg.MsgType != "text" {
		rw.WriteHeader(http.StatusOK)
		return
	}

	text := strings.TrimSpace(msg.Content)
	if text == "" {
		rw.WriteHeader(http.StatusOK)
		return
	}

	channelMsg := &channel.Message{
		ID:       msg.MsgId,
		ChatID:   msg.FromUserName,
		UserID:   msg.FromUserName,
		Username: msg.FromUserName,
		Text:     text,
		Extra:    map[string]string{"channel": "wecom"},
	}

	if err := w.handler(r.Context(), channelMsg); err != nil {
		w.logger.Error("handle message failed", "user", msg.FromUserName, "err", err)
	}

	rw.WriteHeader(http.StatusOK)
}

// Send sends a text message to a WeCom user.
func (w *WeCom) Send(ctx context.Context, chatID string, text string, opts *channel.SendOptions) error {
	token, err := w.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("wecom: get token: %w", err)
	}

	payload := map[string]any{
		"touser":  chatID,
		"msgtype": "text",
		"agentid": w.cfg.AgentID,
		"text":    map[string]string{"content": text},
	}

	data, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/message/send?access_token=%s", token)

	resp, err := w.client.Post(url, "application/json", strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("wecom: send message: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("wecom: decode response: %w", err)
	}
	if result.ErrCode != 0 {
		return fmt.Errorf("wecom: send failed: %d %s", result.ErrCode, result.ErrMsg)
	}
	return nil
}

// Stop is a no-op since WeCom uses HTTP callbacks.
func (w *WeCom) Stop(ctx context.Context) error {
	w.logger.Info("wecom channel stopped")
	return nil
}

// getAccessToken returns a cached or fresh access token.
func (w *WeCom) getAccessToken(ctx context.Context) (string, error) {
	w.mu.RLock()
	if w.accessToken != "" && time.Now().Before(w.tokenExpiry) {
		token := w.accessToken
		w.mu.RUnlock()
		return token, nil
	}
	w.mu.RUnlock()

	w.mu.Lock()
	defer w.mu.Unlock()

	// Double-check after acquiring write lock.
	if w.accessToken != "" && time.Now().Before(w.tokenExpiry) {
		return w.accessToken, nil
	}

	url := fmt.Sprintf("https://qyapi.weixin.qq.com/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		w.cfg.CorpID, w.cfg.Secret)

	resp, err := w.client.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("wecom: get token: %d %s", result.ErrCode, result.ErrMsg)
	}

	w.accessToken = result.AccessToken
	// Refresh 5 minutes before expiry.
	w.tokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second)
	return w.accessToken, nil
}

// verifySignature checks the WeCom callback signature.
func (w *WeCom) verifySignature(msgSignature, timestamp, nonce, encrypt string) bool {
	strs := []string{w.cfg.Token, timestamp, nonce, encrypt}
	sort.Strings(strs)
	h := sha1.New()
	h.Write([]byte(strings.Join(strs, "")))
	return fmt.Sprintf("%x", h.Sum(nil)) == msgSignature
}

// decrypt decodes and decrypts a WeCom encrypted message.
func (w *WeCom) decrypt(encrypted string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(w.aesKey)
	if err != nil {
		return "", err
	}

	if len(ciphertext) < aes.BlockSize || len(ciphertext)%aes.BlockSize != 0 {
		return "", fmt.Errorf("wecom: invalid ciphertext size")
	}

	iv := w.aesKey[:aes.BlockSize]
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)

	// Remove PKCS#7 padding.
	pad := int(ciphertext[len(ciphertext)-1])
	if pad < 1 || pad > aes.BlockSize || pad > len(ciphertext) {
		return "", fmt.Errorf("wecom: invalid padding")
	}
	ciphertext = ciphertext[:len(ciphertext)-pad]

	// Format: random(16) + msgLen(4) + msg + corpID
	if len(ciphertext) < 20 {
		return "", fmt.Errorf("wecom: message too short")
	}
	msgLen := binary.BigEndian.Uint32(ciphertext[16:20])
	if 20+msgLen > uint32(len(ciphertext)) {
		return "", fmt.Errorf("wecom: invalid message length")
	}
	return string(ciphertext[20 : 20+msgLen]), nil
}
