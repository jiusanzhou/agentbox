package mcpserver

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
)

type HTTPServer struct {
	server *Server
	addr   string
	logger *slog.Logger

	mu       sync.Mutex
	sessions map[string]chan []byte
}

func NewHTTPServer(server *Server, addr string, logger *slog.Logger) *HTTPServer {
	if addr == "" {
		addr = ":9800"
	}
	return &HTTPServer{
		server:   server,
		addr:     addr,
		logger:   logger,
		sessions: make(map[string]chan []byte),
	}
}

func (h *HTTPServer) Start(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/sse", h.handleSSE)
	mux.HandleFunc("/message", h.handleMessage)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"name":    "abox-bridge-mcp",
			"version": "1.0.0",
			"sse":     "/sse",
			"message": "/message",
		})
	})

	srv := &http.Server{Addr: h.addr, Handler: mux}
	h.logger.Info("mcp http server starting", "addr", h.addr)

	go func() {
		<-ctx.Done()
		srv.Shutdown(context.Background())
	}()

	return srv.ListenAndServe()
}

// handleSSE establishes an SSE connection for a session.
func (h *HTTPServer) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", 500)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	sessionID := fmt.Sprintf("s%d", r.Context().Value(http.LocalAddrContextKey))
	// Simple: use remote addr as session
	sessionID = r.RemoteAddr

	ch := make(chan []byte, 100)
	h.mu.Lock()
	h.sessions[sessionID] = ch
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.sessions, sessionID)
		h.mu.Unlock()
		close(ch)
	}()

	// Send endpoint event
	fmt.Fprintf(w, "event: endpoint\ndata: /message?sessionId=%s\n\n", sessionID)
	flusher.Flush()

	// Stream responses
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: message\ndata: %s\n\n", msg)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// handleMessage receives JSON-RPC requests and sends responses via SSE.
func (h *HTTPServer) handleMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", 405)
		return
	}

	sessionID := r.URL.Query().Get("sessionId")

	var req jsonrpcRequest
	scanner := bufio.NewScanner(r.Body)
	scanner.Buffer(make([]byte, 10*1024*1024), 10*1024*1024)
	if scanner.Scan() {
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			// Try reading full body
			json.NewDecoder(r.Body).Decode(&req)
		}
	}
	if req.Method == "" {
		// Fallback: try direct decode
		json.NewDecoder(r.Body).Decode(&req)
	}

	resp := h.server.handle(req)

	if sessionID != "" {
		h.mu.Lock()
		ch, ok := h.sessions[sessionID]
		h.mu.Unlock()

		if ok && resp != nil {
			data, _ := json.Marshal(resp)
			select {
			case ch <- data:
			default:
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if resp != nil {
		json.NewEncoder(w).Encode(resp)
	} else {
		w.WriteHeader(202)
	}
}
