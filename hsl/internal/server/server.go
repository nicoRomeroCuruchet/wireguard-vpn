package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/nromero/hsl/internal/proto"
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
	cfg    Config
	store  Store
	log    *slog.Logger
	hubIP  string // first host of overlay, e.g. 10.100.0.1
	pubKey string // WireGuard public key; set in Task 10
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
	mux.HandleFunc("POST /register", s.handleRegister)
	return mux
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func httpError(w http.ResponseWriter, code int, msg string) {
	http.Error(w, msg, code)
}

// serverPublicKey returns the hub's WireGuard public key. Wired to a real key
// in Task 10; empty until then.
func (s *Server) serverPublicKey() string { return s.pubKey }

func validWGKey(b64 string) bool {
	raw, err := base64.StdEncoding.DecodeString(b64)
	return err == nil && len(raw) == 32
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req proto.RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !validWGKey(req.PublicKey) {
		httpError(w, http.StatusBadRequest, "public_key must be base64 of 32 bytes")
		return
	}

	if existing, ok, err := s.store.GetByPublicKey(req.PublicKey); err != nil {
		httpError(w, http.StatusInternalServerError, "store error")
		return
	} else if ok {
		writeJSON(w, http.StatusOK, s.registerResponse(existing))
		return
	}

	nodes, err := s.store.List()
	if err != nil {
		httpError(w, http.StatusInternalServerError, "store error")
		return
	}
	used := make(map[string]bool, len(nodes)+1)
	used[s.hubIP] = true
	for _, n := range nodes {
		used[n.OverlayIP] = true
	}
	ip, err := nextFreeIP(s.cfg.OverlayCIDR, used)
	if err != nil {
		httpError(w, http.StatusServiceUnavailable, "overlay pool exhausted")
		return
	}
	id, err := newUUID()
	if err != nil {
		httpError(w, http.StatusInternalServerError, "id error")
		return
	}
	node := Node{ID: id, PublicKey: req.PublicKey, OverlayIP: ip, Hostname: req.Hostname, LastSeen: time.Now()}
	if err := s.store.Create(node); err != nil {
		httpError(w, http.StatusInternalServerError, "store error")
		return
	}
	s.afterRegister(node) // no-op until Task 10 (kernel peer programming)
	writeJSON(w, http.StatusOK, s.registerResponse(node))
}

func (s *Server) registerResponse(n Node) proto.RegisterResponse {
	return proto.RegisterResponse{
		NodeID:         n.ID,
		OverlayIP:      n.OverlayIP,
		ServerKey:      s.serverPublicKey(),
		ServerEndpoint: s.cfg.Endpoint,
		OverlayNet:     s.cfg.OverlayCIDR,
	}
}

// afterRegister is a hook for kernel reconciliation, wired in Task 10.
func (s *Server) afterRegister(n Node) {}

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
