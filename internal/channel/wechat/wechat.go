package wechat

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.zoe.im/agentbox/internal/channel"
	"go.zoe.im/x"
)

func init() {
	channel.Register("wechat", func(cfg x.TypedLazyConfig, opts ...any) (channel.Channel, error) {
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

// Config for the WeChat Official Account (公众号) channel.
type Config struct {
	AppID          string `json:"app_id" yaml:"app_id"`
	AppSecret      string `json:"app_secret" yaml:"app_secret"`
	Token          string `json:"token" yaml:"token"`
	EncodingAESKey string `json:"encoding_aes_key" yaml:"encoding_aes_key"`
	BaseURL        string `json:"base_url" yaml:"base_url"`
	ListenAddr     string `json:"listen_addr" yaml:"listen_addr"`
	Path           string `json:"path" yaml:"path"`
}

// incomingMsg represents an incoming XML message from WeChat.
type incomingMsg struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
	MsgId        int64    `xml:"MsgId"`
	PicUrl       string   `xml:"PicUrl"`
	MediaId      string   `xml:"MediaId"`
	Format       string   `xml:"Format"`
	Recognition  string   `xml:"Recognition"`
	Event        string   `xml:"Event"`
	EventKey     string   `xml:"EventKey"`
}

// replyMsg represents a text reply XML message to WeChat.
type replyMsg struct {
	XMLName      xml.Name `xml:"xml"`
	ToUserName   string   `xml:"ToUserName"`
	FromUserName string   `xml:"FromUserName"`
	CreateTime   int64    `xml:"CreateTime"`
	MsgType      string   `xml:"MsgType"`
	Content      string   `xml:"Content"`
}

// customerServiceMsg is the JSON body for the customer service message API.
type customerServiceMsg struct {
	ToUser  string              `json:"touser"`
	MsgType string              `json:"msgtype"`
	Text    *customerServiceTxt `json:"text,omitempty"`
}

type customerServiceTxt struct {
	Content string `json:"content"`
}

// templateMsg is the JSON body for the template message API.
type templateMsg struct {
	ToUser     string         `json:"touser"`
	TemplateID string         `json:"template_id"`
	URL        string         `json:"url,omitempty"`
	Data       map[string]any `json:"data,omitempty"`
}

// WeChat implements channel.Channel for WeChat Official Accounts (公众号).
type WeChat struct {
	cfg       Config
	handler   channel.Handler
	cbHandler channel.CallbackHandler
	logger    *slog.Logger
	mux       *http.ServeMux
	server    *http.Server
	client    *http.Client

	mu          sync.RWMutex
	accessToken string
	tokenExpiry time.Time

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates a WeChat Official Account channel.
func New(cfg Config, mux *http.ServeMux) (*WeChat, error) {
	if cfg.AppID == "" || cfg.AppSecret == "" {
		return nil, fmt.Errorf("wechat: app_id and app_secret are required")
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("wechat: token is required")
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.weixin.qq.com"
	}
	if cfg.Path == "" {
		cfg.Path = "/api/v1/wechat/callback"
	}

	return &WeChat{
		cfg:    cfg,
		logger: slog.Default(),
		mux:    mux,
		client: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func (w *WeChat) Name() string { return "wechat" }

// Start registers HTTP handlers and optionally starts an internal HTTP server.
func (w *WeChat) Start(ctx context.Context, handler channel.Handler) error {
	w.handler = handler

	ctx, w.cancel = context.WithCancel(ctx)

	// Register on provided mux or create an internal server.
	if w.mux != nil {
		w.mux.HandleFunc("GET "+w.cfg.Path, w.handleVerify)
		w.mux.HandleFunc("POST "+w.cfg.Path, w.handleMessage)
		w.logger.Info("wechat channel started", "path", w.cfg.Path)
	} else if w.cfg.ListenAddr != "" {
		mux := http.NewServeMux()
		mux.HandleFunc("GET "+w.cfg.Path, w.handleVerify)
		mux.HandleFunc("POST "+w.cfg.Path, w.handleMessage)

		w.server = &http.Server{
			Addr:              w.cfg.ListenAddr,
			Handler:           mux,
			ReadHeaderTimeout: 10 * time.Second,
		}

		w.wg.Add(1)
		go func() {
			defer w.wg.Done()
			w.logger.Info("wechat channel started", "addr", w.cfg.ListenAddr, "path", w.cfg.Path)
			if err := w.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				w.logger.Error("wechat http server error", "err", err)
			}
		}()
	} else {
		return fmt.Errorf("wechat: either a *http.ServeMux must be provided via opts or listen_addr must be set")
	}

	// Start background token refresh.
	w.wg.Add(1)
	go w.tokenRefreshLoop(ctx)

	return nil
}

// tokenRefreshLoop proactively refreshes the access token before expiry.
func (w *WeChat) tokenRefreshLoop(ctx context.Context) {
	defer w.wg.Done()

	// Fetch initial token immediately.
	if _, err := w.getAccessToken(ctx); err != nil {
		w.logger.Error("wechat: initial token fetch failed", "err", err)
	}

	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.mu.RLock()
			needsRefresh := time.Now().After(w.tokenExpiry.Add(-10 * time.Minute))
			w.mu.RUnlock()

			if needsRefresh {
				if _, err := w.refreshAccessToken(ctx); err != nil {
					w.logger.Error("wechat: proactive token refresh failed", "err", err)
				}
			}
		}
	}
}

// handleVerify handles WeChat server URL verification (GET request).
// WeChat sends: signature, timestamp, nonce, echostr
// We must verify the signature and return echostr as plain text.
func (w *WeChat) handleVerify(rw http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	signature := q.Get("signature")
	timestamp := q.Get("timestamp")
	nonce := q.Get("nonce")
	echostr := q.Get("echostr")

	if !w.verifySignature(signature, timestamp, nonce) {
		w.logger.Warn("wechat: verify signature failed", "signature", signature)
		http.Error(rw, "invalid signature", http.StatusForbidden)
		return
	}

	rw.Header().Set("Content-Type", "text/plain; charset=utf-8")
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte(echostr))
}

// handleMessage handles incoming messages from WeChat (POST request).
func (w *WeChat) handleMessage(rw http.ResponseWriter, r *http.Request) {
	// Verify signature.
	q := r.URL.Query()
	signature := q.Get("signature")
	timestamp := q.Get("timestamp")
	nonce := q.Get("nonce")

	if !w.verifySignature(signature, timestamp, nonce) {
		w.logger.Warn("wechat: message signature verification failed")
		http.Error(rw, "invalid signature", http.StatusForbidden)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.logger.Error("wechat: read body failed", "err", err)
		http.Error(rw, "read body failed", http.StatusBadRequest)
		return
	}

	var msg incomingMsg
	if err := xml.Unmarshal(body, &msg); err != nil {
		w.logger.Error("wechat: parse xml failed", "err", err)
		http.Error(rw, "invalid xml", http.StatusBadRequest)
		return
	}

	switch msg.MsgType {
	case "text":
		w.handleTextMessage(r.Context(), rw, &msg)
	case "image":
		w.handleImageMessage(r.Context(), rw, &msg)
	case "voice":
		w.handleVoiceMessage(r.Context(), rw, &msg)
	case "event":
		w.handleEvent(r.Context(), rw, &msg)
	default:
		w.logger.Debug("wechat: unsupported message type", "type", msg.MsgType)
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte("success"))
	}
}

// handleTextMessage processes text messages.
func (w *WeChat) handleTextMessage(ctx context.Context, rw http.ResponseWriter, msg *incomingMsg) {
	text := strings.TrimSpace(msg.Content)
	if text == "" {
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte("success"))
		return
	}

	channelMsg := w.toChannelMessage(msg, text)

	if err := w.handler(ctx, channelMsg); err != nil {
		w.logger.Error("wechat: handle text message failed", "user", msg.FromUserName, "err", err)
	}

	// Return "success" to WeChat to prevent retries. Async reply via customer service API.
	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("success"))
}

// handleImageMessage processes image messages.
func (w *WeChat) handleImageMessage(ctx context.Context, rw http.ResponseWriter, msg *incomingMsg) {
	text := "[image]"
	if msg.PicUrl != "" {
		text = fmt.Sprintf("[image:%s]", msg.PicUrl)
	}

	channelMsg := w.toChannelMessage(msg, text)
	channelMsg.Extra["media_id"] = msg.MediaId
	channelMsg.Extra["pic_url"] = msg.PicUrl

	if err := w.handler(ctx, channelMsg); err != nil {
		w.logger.Error("wechat: handle image message failed", "user", msg.FromUserName, "err", err)
	}

	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("success"))
}

// handleVoiceMessage processes voice messages.
func (w *WeChat) handleVoiceMessage(ctx context.Context, rw http.ResponseWriter, msg *incomingMsg) {
	// If speech recognition is enabled, use the recognition result.
	text := msg.Recognition
	if text == "" {
		text = fmt.Sprintf("[voice:%s]", msg.Format)
	}

	channelMsg := w.toChannelMessage(msg, text)
	channelMsg.Extra["media_id"] = msg.MediaId
	channelMsg.Extra["format"] = msg.Format

	if err := w.handler(ctx, channelMsg); err != nil {
		w.logger.Error("wechat: handle voice message failed", "user", msg.FromUserName, "err", err)
	}

	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("success"))
}

// handleEvent processes event messages (subscribe, unsubscribe, CLICK, VIEW).
func (w *WeChat) handleEvent(ctx context.Context, rw http.ResponseWriter, msg *incomingMsg) {
	event := strings.ToLower(msg.Event)

	switch event {
	case "subscribe":
		w.logger.Info("wechat: user subscribed", "user", msg.FromUserName)
		channelMsg := w.toChannelMessage(msg, "/subscribe")
		channelMsg.Extra["event"] = "subscribe"
		if err := w.handler(ctx, channelMsg); err != nil {
			w.logger.Error("wechat: handle subscribe event failed", "err", err)
		}

	case "unsubscribe":
		w.logger.Info("wechat: user unsubscribed", "user", msg.FromUserName)
		// No need to reply since the user has unfollowed.

	case "click":
		w.logger.Debug("wechat: menu click event", "key", msg.EventKey, "user", msg.FromUserName)
		if w.cbHandler != nil {
			cb := &channel.Callback{
				ID:     msg.EventKey,
				ChatID: msg.FromUserName,
				UserID: msg.FromUserName,
				Extra:  map[string]string{"channel": "wechat", "event": "click"},
			}
			if err := w.cbHandler(ctx, cb); err != nil {
				w.logger.Error("wechat: handle click callback failed", "key", msg.EventKey, "err", err)
			}
		}

	case "view":
		w.logger.Debug("wechat: menu view event", "url", msg.EventKey, "user", msg.FromUserName)

	default:
		w.logger.Debug("wechat: unhandled event", "event", event, "user", msg.FromUserName)
	}

	rw.WriteHeader(http.StatusOK)
	rw.Write([]byte("success"))
}

// toChannelMessage converts an incoming WeChat message to a channel.Message.
func (w *WeChat) toChannelMessage(msg *incomingMsg, text string) *channel.Message {
	return &channel.Message{
		ID:       strconv.FormatInt(msg.MsgId, 10),
		ChatID:   msg.FromUserName,
		UserID:   msg.FromUserName,
		Username: msg.FromUserName,
		Text:     text,
		Extra: map[string]string{
			"channel":    "wechat",
			"to_user":    msg.ToUserName,
			"msg_type":   msg.MsgType,
			"create_time": strconv.FormatInt(msg.CreateTime, 10),
		},
	}
}

// Send sends a text message to a WeChat user via the customer service message API.
func (w *WeChat) Send(ctx context.Context, chatID string, text string, opts *channel.SendOptions) error {
	token, err := w.getAccessToken(ctx)
	if err != nil {
		return fmt.Errorf("wechat: get token: %w", err)
	}

	payload := customerServiceMsg{
		ToUser:  chatID,
		MsgType: "text",
		Text:    &customerServiceTxt{Content: text},
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("wechat: marshal message: %w", err)
	}

	url := fmt.Sprintf("%s/cgi-bin/message/custom/send?access_token=%s", w.cfg.BaseURL, token)

	resp, err := w.client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("wechat: send message: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("wechat: decode response: %w", err)
	}
	if result.ErrCode != 0 {
		return fmt.Errorf("wechat: send failed: %d %s", result.ErrCode, result.ErrMsg)
	}

	return nil
}

// EditMessage is not supported by WeChat Official Accounts.
func (w *WeChat) EditMessage(ctx context.Context, chatID string, messageID string, text string, opts *channel.SendOptions) error {
	return fmt.Errorf("wechat: editing messages is not supported")
}

// SendWithButtons sends a text message with button labels appended.
// WeChat Official Accounts do not support inline buttons, so buttons
// are emulated by appending their labels to the message text.
func (w *WeChat) SendWithButtons(ctx context.Context, chatID string, text string, buttons []channel.Button, opts *channel.SendOptions) (string, error) {
	if len(buttons) > 0 {
		var sb strings.Builder
		sb.WriteString(text)
		sb.WriteString("\n\n")
		for i, btn := range buttons {
			sb.WriteString(fmt.Sprintf("[%d] %s", i+1, btn.Text))
			if i < len(buttons)-1 {
				sb.WriteString("\n")
			}
		}
		text = sb.String()
	}

	if err := w.Send(ctx, chatID, text, opts); err != nil {
		return "", err
	}

	// WeChat customer service API does not return a message ID.
	msgID := fmt.Sprintf("wechat_%s_%d", chatID, time.Now().UnixNano())
	return msgID, nil
}

// OnCallback registers a handler for menu click callbacks.
func (w *WeChat) OnCallback(handler channel.CallbackHandler) {
	w.cbHandler = handler
}

// Stop gracefully shuts down the channel.
func (w *WeChat) Stop(ctx context.Context) error {
	if w.cancel != nil {
		w.cancel()
	}

	if w.server != nil {
		if err := w.server.Shutdown(ctx); err != nil {
			w.logger.Error("wechat: server shutdown error", "err", err)
			return fmt.Errorf("wechat: shutdown: %w", err)
		}
	}

	w.wg.Wait()
	w.logger.Info("wechat channel stopped")
	return nil
}

// SendTemplateMessage sends a template message to a WeChat user.
func (w *WeChat) SendTemplateMessage(ctx context.Context, chatID string, templateID string, url string, data map[string]any) (int64, error) {
	token, err := w.getAccessToken(ctx)
	if err != nil {
		return 0, fmt.Errorf("wechat: get token: %w", err)
	}

	payload := templateMsg{
		ToUser:     chatID,
		TemplateID: templateID,
		URL:        url,
		Data:       data,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("wechat: marshal template message: %w", err)
	}

	apiURL := fmt.Sprintf("%s/cgi-bin/message/template/send?access_token=%s", w.cfg.BaseURL, token)

	resp, err := w.client.Post(apiURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("wechat: send template message: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		MsgID   int64  `json:"msgid"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("wechat: decode response: %w", err)
	}
	if result.ErrCode != 0 {
		return 0, fmt.Errorf("wechat: send template failed: %d %s", result.ErrCode, result.ErrMsg)
	}

	return result.MsgID, nil
}

// UploadMedia uploads a temporary media file to WeChat.
// mediaType can be "image", "voice", "video", or "thumb".
func (w *WeChat) UploadMedia(ctx context.Context, mediaType string, filename string, reader io.Reader) (string, error) {
	token, err := w.getAccessToken(ctx)
	if err != nil {
		return "", fmt.Errorf("wechat: get token: %w", err)
	}

	// Build multipart body.
	var buf bytes.Buffer
	boundary := fmt.Sprintf("----WeChatBoundary%d", time.Now().UnixNano())

	buf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	buf.WriteString(fmt.Sprintf(`Content-Disposition: form-data; name="media"; filename="%s"`+"\r\n", filename))
	buf.WriteString("Content-Type: application/octet-stream\r\n\r\n")

	if _, err := io.Copy(&buf, reader); err != nil {
		return "", fmt.Errorf("wechat: read media: %w", err)
	}

	buf.WriteString(fmt.Sprintf("\r\n--%s--\r\n", boundary))

	apiURL := fmt.Sprintf("%s/cgi-bin/media/upload?access_token=%s&type=%s", w.cfg.BaseURL, token, mediaType)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, &buf)
	if err != nil {
		return "", fmt.Errorf("wechat: create upload request: %w", err)
	}
	req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", boundary))

	resp, err := w.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("wechat: upload media: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode  int    `json:"errcode"`
		ErrMsg   string `json:"errmsg"`
		Type     string `json:"type"`
		MediaID  string `json:"media_id"`
		CreateAt int64  `json:"created_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("wechat: decode upload response: %w", err)
	}
	if result.ErrCode != 0 {
		return "", fmt.Errorf("wechat: upload failed: %d %s", result.ErrCode, result.ErrMsg)
	}

	return result.MediaID, nil
}

// DownloadMedia downloads a temporary media file from WeChat.
func (w *WeChat) DownloadMedia(ctx context.Context, mediaID string) (io.ReadCloser, error) {
	token, err := w.getAccessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("wechat: get token: %w", err)
	}

	apiURL := fmt.Sprintf("%s/cgi-bin/media/get?access_token=%s&media_id=%s", w.cfg.BaseURL, token, mediaID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("wechat: create download request: %w", err)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("wechat: download media: %w", err)
	}

	// Check if the response is an error JSON instead of media content.
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") || strings.Contains(contentType, "text/plain") {
		defer resp.Body.Close()
		var result struct {
			ErrCode int    `json:"errcode"`
			ErrMsg  string `json:"errmsg"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, fmt.Errorf("wechat: decode download error: %w", err)
		}
		return nil, fmt.Errorf("wechat: download failed: %d %s", result.ErrCode, result.ErrMsg)
	}

	return resp.Body, nil
}

// getAccessToken returns a cached or fresh access token.
func (w *WeChat) getAccessToken(ctx context.Context) (string, error) {
	w.mu.RLock()
	if w.accessToken != "" && time.Now().Before(w.tokenExpiry) {
		token := w.accessToken
		w.mu.RUnlock()
		return token, nil
	}
	w.mu.RUnlock()

	return w.refreshAccessToken(ctx)
}

// refreshAccessToken fetches a new access token from WeChat API.
func (w *WeChat) refreshAccessToken(ctx context.Context) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Double-check after acquiring write lock.
	if w.accessToken != "" && time.Now().Before(w.tokenExpiry) {
		return w.accessToken, nil
	}

	apiURL := fmt.Sprintf("%s/cgi-bin/token?grant_type=client_credential&appid=%s&secret=%s",
		w.cfg.BaseURL, w.cfg.AppID, w.cfg.AppSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("wechat: create token request: %w", err)
	}

	resp, err := w.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("wechat: fetch token: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("wechat: decode token response: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("wechat: get token failed: %d %s", result.ErrCode, result.ErrMsg)
	}

	w.accessToken = result.AccessToken
	// Refresh 5 minutes before expiry.
	w.tokenExpiry = time.Now().Add(time.Duration(result.ExpiresIn-300) * time.Second)

	w.logger.Debug("wechat: access token refreshed", "expires_in", result.ExpiresIn)
	return w.accessToken, nil
}

// verifySignature checks the WeChat callback signature.
// Signature = SHA1(sort(token, timestamp, nonce))
func (w *WeChat) verifySignature(signature, timestamp, nonce string) bool {
	if signature == "" || timestamp == "" || nonce == "" {
		return false
	}

	strs := []string{w.cfg.Token, timestamp, nonce}
	sort.Strings(strs)

	h := sha1.New()
	h.Write([]byte(strings.Join(strs, "")))
	computed := fmt.Sprintf("%x", h.Sum(nil))

	return computed == signature
}

// ReplyXML writes a passive XML text reply to the response writer.
// This is used for synchronous replies within the 5-second window.
func (w *WeChat) ReplyXML(rw http.ResponseWriter, toUser, fromUser, content string) {
	reply := replyMsg{
		ToUserName:   toUser,
		FromUserName: fromUser,
		CreateTime:   time.Now().Unix(),
		MsgType:      "text",
		Content:      content,
	}

	data, err := xml.Marshal(&reply)
	if err != nil {
		w.logger.Error("wechat: marshal reply xml failed", "err", err)
		rw.WriteHeader(http.StatusOK)
		rw.Write([]byte("success"))
		return
	}

	rw.Header().Set("Content-Type", "application/xml; charset=utf-8")
	rw.WriteHeader(http.StatusOK)
	rw.Write(data)
}
