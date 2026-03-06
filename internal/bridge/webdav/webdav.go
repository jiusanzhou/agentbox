package webdav

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"golang.org/x/net/webdav"
)

type Config struct {
	Addr  string   `json:"addr" yaml:"addr"`
	Roots []string `json:"roots" yaml:"roots"`
}

type Server struct {
	cfg    Config
	logger *slog.Logger
	server *http.Server
}

func New(cfg Config, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.Addr == "" {
		cfg.Addr = ":9801"
	}
	return &Server{cfg: cfg, logger: logger}
}

// multiRootFS wraps multiple roots under virtual paths /root0, /root1, etc.
type multiRootFS struct {
	roots []string
}

func (fs *multiRootFS) handler() http.Handler {
	mux := http.NewServeMux()

	for i, root := range fs.roots {
		prefix := fmt.Sprintf("/r%d/", i)
		handler := &webdav.Handler{
			Prefix:     strings.TrimSuffix(prefix, "/"),
			FileSystem: webdav.Dir(root),
			LockSystem: webdav.NewMemLS(),
			Logger: func(r *http.Request, err error) {
				if err != nil {
					slog.Error("webdav", "method", r.Method, "path", r.URL.Path, "err", err)
				}
			},
		}
		mux.Handle(prefix, handler)
	}

	// Index page
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		fmt.Fprintln(w, "ABox Bridge - WebDAV")
		fmt.Fprintln(w, "")
		for i, root := range fs.roots {
			fmt.Fprintf(w, "  /r%d/  ->  %s\n", i, root)
		}
	})

	return mux
}

func (s *Server) Start(ctx context.Context) error {
	fs := &multiRootFS{roots: s.cfg.Roots}
	s.server = &http.Server{
		Addr:    s.cfg.Addr,
		Handler: fs.handler(),
	}
	s.logger.Info("webdav server starting", "addr", s.cfg.Addr, "roots", s.cfg.Roots)
	return s.server.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}
