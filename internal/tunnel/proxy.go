package tunnel

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"log/slog"
	"net/http"
	"strings"
)

// Proxy listens for HTTP requests from sandbox containers and forwards
// them through WebSocket tunnels to connected clients.
type Proxy struct {
	hub    *Hub
	addr   string
	logger *slog.Logger
}

func NewProxy(hub *Hub, addr string, logger *slog.Logger) *Proxy {
	if addr == "" {
		addr = ":9900"
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Proxy{hub: hub, addr: addr, logger: logger}
}

// Start starts the proxy HTTP server.
func (p *Proxy) Start(ctx context.Context) error {
	srv := &http.Server{Addr: p.addr, Handler: p}
	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()
	p.logger.Info("tunnel proxy starting", "addr", p.addr)
	return srv.ListenAndServe()
}

// ServeHTTP handles requests in the format /{userID}/{service}/...
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Parse: /{userID}/rest/of/path
	path := strings.TrimPrefix(r.URL.Path, "/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 1 || parts[0] == "" {
		http.Error(w, "missing user ID in path", http.StatusBadRequest)
		return
	}

	userID := parts[0]
	forwardPath := "/"
	if len(parts) > 1 {
		forwardPath = "/" + parts[1]
	}

	// Read body
	var body []byte
	if r.Body != nil {
		body, _ = io.ReadAll(r.Body)
	}

	// Flatten headers (first value only)
	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = v[0]
		}
	}

	reqID := randomID()
	tunnelReq := &TunnelRequest{
		ID:      reqID,
		Method:  r.Method,
		Path:    forwardPath,
		Headers: headers,
		Body:    body,
	}

	resp, err := p.hub.Forward(userID, tunnelReq)
	if err != nil {
		p.logger.Error("tunnel forward failed", "user", userID, "path", forwardPath, "err", err)
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	// Write response
	for k, v := range resp.Headers {
		w.Header().Set(k, v)
	}
	w.WriteHeader(resp.StatusCode)
	if len(resp.Body) > 0 {
		w.Write(resp.Body)
	}
}

func randomID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
