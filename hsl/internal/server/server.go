package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
)

// Config holds server runtime configuration from flags.
type Config struct {
	Addr        string // HTTP listen address, e.g. ":8080"
	DBPath      string // SQLite path (unused until Task 13)
	Endpoint    string // public WireGuard endpoint host:port returned to clients
	OverlayCIDR string // e.g. "10.100.0.0/24"
}

// Server is the hsl control plane.
type Server struct {
	cfg   Config
	store Store
	log   *slog.Logger
	hubIP string // first host of overlay, e.g. 10.100.0.1
}

func New(cfg Config, store Store, logger *slog.Logger) (*Server, error) {
	if cfg.OverlayCIDR == "" {
		cfg.OverlayCIDR = "10.100.0.0/24"
	}
	ip, _, err := net.ParseCIDR(cfg.OverlayCIDR)
	if err != nil {
		return nil, fmt.Errorf("invalid overlay cidr: %w", err)
	}
	hub := ip.Mask(net.CIDRMask(24, 32)).To4()
	hub[3] = 1
	return &Server{cfg: cfg, store: store, log: logger, hubIP: hub.String()}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.handleHealthz)
	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// Run starts the HTTP server and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{Addr: s.cfg.Addr, Handler: s.Handler()}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	s.log.Info("hsl server listening", "addr", s.cfg.Addr)
	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 0)
		defer cancel()
		return srv.Shutdown(shutCtx)
	}
}
