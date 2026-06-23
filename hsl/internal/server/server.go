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
	"github.com/nromero/hsl/internal/wgmgr"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Config holds server runtime configuration from flags.
type Config struct {
	Addr             string   // HTTP listen address, e.g. ":8080"
	DBPath           string   // SQLite path (unused until Task 13)
	Endpoint         string   // public WireGuard endpoint host:port returned to clients
	OverlayCIDR      string   // e.g. "10.100.0.0/24"
	KeyPath          string   // path to hub WireGuard private key; default /var/lib/hsl/identity.key
	AdvertisedRoutes []string // CIDRs to advertise to clients (LAN reachability)
}

const (
	wgInterface = "wg0"
	wgMTU       = 1420
	wgPort      = 51820
)

// Server is the hsl control plane.
type Server struct {
	cfg     Config
	store   Store
	log     *slog.Logger
	hubIP   string      // first host of overlay, e.g. 10.100.0.1
	pubKey  string      // WireGuard public key; set in Task 10
	privKey wgtypes.Key // hub WireGuard private key
	skipWG  bool        // test seam: skip all kernel/netlink work
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
	mux.HandleFunc("GET /peers", s.handlePeers)
	mux.HandleFunc("POST /heartbeat", s.handleHeartbeat)
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
		NodeID:           n.ID,
		OverlayIP:        n.OverlayIP,
		ServerKey:        s.serverPublicKey(),
		ServerEndpoint:   s.cfg.Endpoint,
		OverlayNet:       s.cfg.OverlayCIDR,
		AdvertisedRoutes: s.cfg.AdvertisedRoutes,
	}
}

func (s *Server) authNodeID(r *http.Request) (string, bool) {
	id := r.Header.Get("X-Node-ID")
	if id == "" {
		return "", false
	}
	nodes, err := s.store.List()
	if err != nil {
		return "", false
	}
	for _, n := range nodes {
		if n.ID == id {
			return id, true
		}
	}
	return "", false
}

func (s *Server) handlePeers(w http.ResponseWriter, r *http.Request) {
	id, ok := s.authNodeID(r)
	if !ok {
		httpError(w, http.StatusUnauthorized, "missing or unknown X-Node-ID")
		return
	}
	_ = s.store.Touch(id, time.Now())
	nodes, err := s.store.List()
	if err != nil {
		httpError(w, http.StatusInternalServerError, "store error")
		return
	}
	resp := proto.PeersResponse{}
	// Advertise the hub itself first.
	resp.Peers = append(resp.Peers, proto.Peer{
		ID: "hub", PublicKey: s.serverPublicKey(), OverlayIP: s.hubIP, Hostname: "hub",
		AdvertisedRoutes: s.cfg.AdvertisedRoutes,
	})
	for _, n := range nodes {
		resp.Peers = append(resp.Peers, proto.Peer{
			ID: n.ID, PublicKey: n.PublicKey, OverlayIP: n.OverlayIP,
			Hostname: n.Hostname, LastSeen: n.LastSeen.UTC().Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
	id, ok := s.authNodeID(r)
	if !ok {
		httpError(w, http.StatusUnauthorized, "missing or unknown X-Node-ID")
		return
	}
	_ = s.store.Touch(id, time.Now())
	writeJSON(w, http.StatusOK, proto.HeartbeatResponse{OK: true})
}

func (s *Server) loadKey() error {
	key, err := wgmgr.LoadOrCreateKey(s.cfg.KeyPath)
	if err != nil {
		return fmt.Errorf("load hub key: %w", err)
	}
	s.privKey = key
	s.pubKey = key.PublicKey().String()
	return nil
}

func (s *Server) reconcileWG() error {
	if err := wgmgr.EnsureInterface(wgInterface, s.hubIP+"/24", wgMTU); err != nil {
		return err
	}
	nodes, err := s.store.List()
	if err != nil {
		return err
	}
	peers := make([]wgmgr.PeerConfig, 0, len(nodes))
	for _, n := range nodes {
		peers = append(peers, wgmgr.PeerConfig{
			PublicKey:  n.PublicKey,
			AllowedIPs: []string{n.OverlayIP + "/32"}, // hub side: per-client /32
		})
	}
	return wgmgr.ConfigureDevice(wgInterface, s.privKey, wgPort, peers)
}

// afterRegister reconciles WireGuard peers after a new node registers.
func (s *Server) afterRegister(n Node) {
	if s.skipWG {
		return
	}
	if err := s.reconcileWG(); err != nil {
		s.log.Error("reconcile wireguard after register failed", "node", n.ID, "err", err)
	}
}

// Run starts the HTTP server and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	if !s.skipWG {
		if err := s.loadKey(); err != nil {
			return err
		}
		if err := s.reconcileWG(); err != nil {
			return fmt.Errorf("initial wireguard reconcile: %w", err)
		}
		if err := s.setupSNAT(); err != nil {
			return fmt.Errorf("setup snat: %w", err)
		}
	}
	srv := &http.Server{Addr: s.cfg.Addr, Handler: s.Handler()}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.ListenAndServe() }()
	s.log.Info("hsl server listening", "addr", s.cfg.Addr)
	select {
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	case <-ctx.Done():
		shutCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		s.log.Info("shutting down")
		if !s.skipWG {
			if err := s.teardownSNAT(); err != nil {
				s.log.Warn("teardown snat failed", "err", err)
			}
		}
		return srv.Shutdown(shutCtx)
	}
}

func (s *Server) setupSNAT() error {
	return SetupSNAT(s.cfg.OverlayCIDR, s.cfg.AdvertisedRoutes)
}

func (s *Server) teardownSNAT() error {
	return TeardownSNAT(s.cfg.OverlayCIDR, s.cfg.AdvertisedRoutes)
}
