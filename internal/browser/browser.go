package browser

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// Config configures the browser provider.
type Config struct {
	ChromePath string `json:"chrome_path,omitempty"` // custom chrome binary path
	RemoteURL  string `json:"remote_url,omitempty"`  // connect to existing Chrome (ws://...)
	Headless   bool   `json:"headless"`              // run headless (default: false for local use)
}

// Browser is a CDP-backed browser provider that implements http.Handler.
type Browser struct {
	allocCtx    context.Context
	allocCancel context.CancelFunc
	ctx         context.Context
	ctxCancel   context.CancelFunc
	logger      *slog.Logger
}

// New creates a new Browser with the given config.
func New(cfg Config, logger *slog.Logger) (*Browser, error) {
	if logger == nil {
		logger = slog.Default()
	}

	b := &Browser{logger: logger}

	var allocCtx context.Context
	var allocCancel context.CancelFunc

	if cfg.RemoteURL != "" {
		// Connect to existing Chrome via DevTools WebSocket URL
		allocCtx, allocCancel = chromedp.NewRemoteAllocator(context.Background(), cfg.RemoteURL)
		logger.Info("connecting to remote Chrome", "url", cfg.RemoteURL)
	} else {
		// Try connecting to localhost:9222 first
		wsURL, err := detectChrome("http://localhost:9222")
		if err == nil && wsURL != "" {
			allocCtx, allocCancel = chromedp.NewRemoteAllocator(context.Background(), wsURL)
			logger.Info("connected to existing Chrome", "url", wsURL)
		} else {
			// Launch a new Chrome instance
			opts := append(chromedp.DefaultExecAllocatorOptions[:],
				chromedp.Flag("headless", cfg.Headless),
				chromedp.Flag("no-first-run", true),
				chromedp.Flag("disable-default-apps", true),
			)
			if cfg.ChromePath != "" {
				opts = append(opts, chromedp.ExecPath(cfg.ChromePath))
			}
			allocCtx, allocCancel = chromedp.NewExecAllocator(context.Background(), opts...)
			logger.Info("launching new Chrome instance", "headless", cfg.Headless)
		}
	}

	b.allocCtx = allocCtx
	b.allocCancel = allocCancel

	// Create the first browser context (tab)
	ctx, cancel := chromedp.NewContext(allocCtx)
	b.ctx = ctx
	b.ctxCancel = cancel

	// Run a no-op to ensure the browser starts
	if err := chromedp.Run(ctx); err != nil {
		allocCancel()
		return nil, fmt.Errorf("failed to start browser: %w", err)
	}

	return b, nil
}

// Handler returns the http.Handler for the tunnel provider.
func (b *Browser) Handler() http.Handler {
	return b
}

// Close shuts down the browser.
func (b *Browser) Close() {
	b.ctxCancel()
	b.allocCancel()
}

// ServeHTTP routes requests to the appropriate handler.
func (b *Browser) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")

	switch {
	case r.Method == http.MethodPost && path == "navigate":
		b.handleNavigate(w, r)
	case r.Method == http.MethodPost && path == "screenshot":
		b.handleScreenshot(w, r)
	case r.Method == http.MethodPost && path == "content":
		b.handleContent(w, r)
	case r.Method == http.MethodPost && path == "click":
		b.handleClick(w, r)
	case r.Method == http.MethodPost && path == "type":
		b.handleType(w, r)
	case r.Method == http.MethodPost && path == "evaluate":
		b.handleEvaluate(w, r)
	case r.Method == http.MethodPost && path == "wait":
		b.handleWait(w, r)
	case r.Method == http.MethodGet && path == "tabs":
		b.handleListTabs(w, r)
	case r.Method == http.MethodPost && path == "tab":
		b.handleSwitchTab(w, r)
	case r.Method == http.MethodPost && path == "tab/new":
		b.handleNewTab(w, r)
	case r.Method == http.MethodPost && path == "tab/close":
		b.handleCloseTab(w, r)
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	}
}

// --- Handlers ---

func (b *Browser) handleNavigate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}
	if err := chromedp.Run(b.ctx, chromedp.Navigate(req.URL)); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (b *Browser) handleScreenshot(w http.ResponseWriter, r *http.Request) {
	var buf []byte
	if err := chromedp.Run(b.ctx, chromedp.FullScreenshot(&buf, 90)); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"image": base64.StdEncoding.EncodeToString(buf),
	})
}

func (b *Browser) handleContent(w http.ResponseWriter, r *http.Request) {
	var html, text, title, pageURL string
	err := chromedp.Run(b.ctx,
		chromedp.OuterHTML("html", &html),
		chromedp.Evaluate(`document.body.innerText`, &text),
		chromedp.Title(&title),
		chromedp.Location(&pageURL),
	)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"html":  html,
		"text":  text,
		"title": title,
		"url":   pageURL,
	})
}

func (b *Browser) handleClick(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Selector string `json:"selector"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Selector == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "selector is required"})
		return
	}
	if err := chromedp.Run(b.ctx, chromedp.Click(req.Selector, chromedp.ByQuery)); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (b *Browser) handleType(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Selector string `json:"selector"`
		Text     string `json:"text"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Selector == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "selector is required"})
		return
	}
	err := chromedp.Run(b.ctx,
		chromedp.Clear(req.Selector, chromedp.ByQuery),
		chromedp.SendKeys(req.Selector, req.Text, chromedp.ByQuery),
	)
	if err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (b *Browser) handleEvaluate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Expression string `json:"expression"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Expression == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "expression is required"})
		return
	}
	var result any
	if err := chromedp.Run(b.ctx, chromedp.Evaluate(req.Expression, &result)); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"result": result})
}

func (b *Browser) handleWait(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Selector string `json:"selector"`
		Timeout  int    `json:"timeout"` // milliseconds
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Selector == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "selector is required"})
		return
	}
	timeout := 30 * time.Second
	if req.Timeout > 0 {
		timeout = time.Duration(req.Timeout) * time.Millisecond
	}
	ctx, cancel := context.WithTimeout(b.ctx, timeout)
	defer cancel()
	if err := chromedp.Run(ctx, chromedp.WaitVisible(req.Selector, chromedp.ByQuery)); err != nil {
		writeErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (b *Browser) handleListTabs(w http.ResponseWriter, r *http.Request) {
	targets, err := chromedp.Targets(b.ctx)
	if err != nil {
		writeErr(w, err)
		return
	}
	type tab struct {
		ID    string `json:"id"`
		URL   string `json:"url"`
		Title string `json:"title"`
	}
	tabs := make([]tab, 0, len(targets))
	for _, t := range targets {
		if t.Type == "page" {
			tabs = append(tabs, tab{ID: string(t.TargetID), URL: t.URL, Title: t.Title})
		}
	}
	writeJSON(w, http.StatusOK, tabs)
}

func (b *Browser) handleSwitchTab(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	targets, err := chromedp.Targets(b.ctx)
	if err != nil {
		writeErr(w, err)
		return
	}
	for _, t := range targets {
		if string(t.TargetID) == req.ID {
			// Cancel old context and create new one attached to this target
			b.ctxCancel()
			ctx, cancel := chromedp.NewContext(b.allocCtx, chromedp.WithTargetID(t.TargetID))
			b.ctx = ctx
			b.ctxCancel = cancel
			// Activate the target
			if err := chromedp.Run(ctx); err != nil {
				writeErr(w, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "tab not found"})
}

func (b *Browser) handleNewTab(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if !decodeBody(w, r, &req) {
		return
	}

	// Cancel old context, create a new one (new tab)
	b.ctxCancel()
	ctx, cancel := chromedp.NewContext(b.allocCtx)
	b.ctx = ctx
	b.ctxCancel = cancel

	actions := []chromedp.Action{}
	if req.URL != "" {
		actions = append(actions, chromedp.Navigate(req.URL))
	}
	if err := chromedp.Run(ctx, actions...); err != nil {
		writeErr(w, err)
		return
	}

	// Get the target ID of the new tab
	target := chromedp.FromContext(ctx).Target
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "ok",
		"id":     string(target.TargetID),
	})
}

func (b *Browser) handleCloseTab(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID string `json:"id"`
	}
	if !decodeBody(w, r, &req) {
		return
	}
	if req.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id is required"})
		return
	}

	targets, err := chromedp.Targets(b.ctx)
	if err != nil {
		writeErr(w, err)
		return
	}
	for _, t := range targets {
		if string(t.TargetID) == req.ID {
			closeCtx, closeCancel := chromedp.NewContext(b.allocCtx, chromedp.WithTargetID(t.TargetID))
			if err := chromedp.Cancel(closeCtx); err != nil {
				closeCancel()
				writeErr(w, err)
				return
			}
			closeCancel()
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "tab not found"})
}

// --- Helpers ---

func decodeBody(w http.ResponseWriter, r *http.Request, v any) bool {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return false
	}
	if len(body) == 0 {
		return true // allow empty body for endpoints that don't require it
	}
	if err := json.Unmarshal(body, v); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON: " + err.Error()})
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
}

// detectChrome tries to connect to Chrome's debug endpoint and returns the WebSocket URL.
func detectChrome(debugURL string) (string, error) {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(debugURL + "/json/version")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var info struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", err
	}
	return info.WebSocketDebuggerURL, nil
}
