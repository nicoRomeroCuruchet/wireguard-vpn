// Package client implements the hsl node agent: registration and the
// reconciliation run loop.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nromero/hsl/internal/proto"
	"github.com/nromero/hsl/internal/wgmgr"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	wgInterface = "wg0"
	wgMTU       = 1420
	pollEvery   = 10 * time.Second
	keepalive   = 25 * time.Second
)

// configureWG is a seam so tests can avoid kernel access.
var configureWG = realConfigureWG

// ifaceAddrCIDR returns the address to assign to the client's wg0: the node's
// overlay IP carrying the OVERLAY prefix length (e.g. 10.100.0.2/24), not /32.
// The overlay prefix is what installs a connected route for the whole overlay
// via wg0 — a /32 installs only the host's own route, so without it the client
// has no kernel route to reach other overlay IPs and AllowedIPs (cryptokey
// routing) alone cannot carry the traffic.
func ifaceAddrCIDR(overlayIP, overlayNet string) (string, error) {
	_, ipnet, err := net.ParseCIDR(overlayNet)
	if err != nil {
		return "", fmt.Errorf("parse overlay net %q: %w", overlayNet, err)
	}
	ones, _ := ipnet.Mask.Size()
	return fmt.Sprintf("%s/%d", overlayIP, ones), nil
}

func realConfigureWG(st State, priv wgtypes.Key) error {
	addrCIDR, err := ifaceAddrCIDR(st.OverlayIP, st.OverlayNet)
	if err != nil {
		return err
	}
	if err := wgmgr.EnsureInterface(wgInterface, addrCIDR, wgMTU); err != nil {
		return err
	}
	allowedIPs := make([]string, 0, 1+len(st.AdvertisedRoutes))
	allowedIPs = append(allowedIPs, st.OverlayNet)
	allowedIPs = append(allowedIPs, st.AdvertisedRoutes...)
	return wgmgr.ConfigureDevice(wgInterface, priv, 0, []wgmgr.PeerConfig{{
		PublicKey:  st.ServerKey,
		Endpoint:   st.ServerEndpoint,
		AllowedIPs: allowedIPs,
		Keepalive:  keepalive,
	}})
}

func fetchPeers(httpClient *http.Client, serverURL, nodeID string) (proto.PeersResponse, error) {
	var out proto.PeersResponse
	req, err := http.NewRequest(http.MethodGet, serverURL+"/peers", nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("X-Node-ID", nodeID)
	resp, err := httpClient.Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return out, fmt.Errorf("peers: server returned %s", resp.Status)
	}
	return out, json.NewDecoder(resp.Body).Decode(&out)
}

func heartbeat(httpClient *http.Client, serverURL, nodeID string) error {
	req, err := http.NewRequest(http.MethodPost, serverURL+"/heartbeat", nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Node-ID", nodeID)
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("heartbeat: server returned %s", resp.Status)
	}
	return nil
}

// Run configures wg0 from local state and then polls the server every 10s,
// sending heartbeats, until ctx is cancelled. serverURL is the control-plane
// HTTP URL (separate from the WireGuard endpoint).
func Run(ctx context.Context, serverURL, stateDir string, logger *slog.Logger) error {
	if stateDir == "" {
		stateDir = defaultStateDir()
	}
	st, err := loadState(stateDir)
	if err != nil {
		return fmt.Errorf("load state (run `hsl client register` first): %w", err)
	}
	priv, err := wgmgr.LoadOrCreateKey(keyPath(stateDir))
	if err != nil {
		return err
	}
	if err := configureWG(st, priv); err != nil {
		return fmt.Errorf("configure wg0: %w", err)
	}
	logger.Info("wg0 configured", "overlay_ip", st.OverlayIP, "hub", st.ServerEndpoint)

	httpClient := &http.Client{Timeout: 10 * time.Second}
	ticker := time.NewTicker(pollEvery)
	defer ticker.Stop()
	var lastSig string
	for {
		if resp, err := fetchPeers(httpClient, serverURL, st.NodeID); err != nil {
			logger.Warn("fetch peers failed", "err", err)
		} else if sig := peersSignature(resp); sig != lastSig {
			// In strict hub-and-spoke the client's only peer is the hub, so a
			// changed overlay peer set needs no wg0 reconfiguration — we observe
			// and log it for visibility rather than reconcile.
			logger.Info("overlay peer set changed", "peers", len(resp.Peers))
			lastSig = sig
		}
		if err := heartbeat(httpClient, serverURL, st.NodeID); err != nil {
			logger.Warn("heartbeat failed", "err", err)
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
}

// peersSignature returns a stable, order-independent signature of the overlay
// peer set (sorted overlay IPs) so the run loop can detect when it changes.
func peersSignature(resp proto.PeersResponse) string {
	ips := make([]string, 0, len(resp.Peers))
	for _, p := range resp.Peers {
		ips = append(ips, p.OverlayIP)
	}
	sort.Strings(ips)
	return strings.Join(ips, ",")
}

// State is the persisted client identity + assignment (node.json).
type State struct {
	NodeID           string   `json:"node_id"`
	OverlayIP        string   `json:"overlay_ip"`
	ServerKey        string   `json:"server_key"`
	ServerEndpoint   string   `json:"server_endpoint"`
	OverlayNet       string   `json:"overlay_net"`
	AdvertisedRoutes []string `json:"advertised_routes"`
}

func defaultStateDir() string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "hsl")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "hsl")
}

func keyPath(stateDir string) string   { return filepath.Join(stateDir, "identity.key") }
func statePath(stateDir string) string { return filepath.Join(stateDir, "node.json") }

func loadState(stateDir string) (State, error) {
	var st State
	data, err := os.ReadFile(statePath(stateDir))
	if err != nil {
		return st, err
	}
	return st, json.Unmarshal(data, &st)
}

func saveState(stateDir string, st State) error {
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath(stateDir), data, 0o600)
}

// Register loads-or-creates the node identity, registers with the server, and
// persists the returned assignment.
func Register(serverURL, hostname, stateDir string) (State, error) {
	if stateDir == "" {
		stateDir = defaultStateDir()
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return State{}, err
	}
	key, err := wgmgr.LoadOrCreateKey(keyPath(stateDir))
	if err != nil {
		return State{}, err
	}
	if hostname == "" {
		hostname, _ = os.Hostname()
	}
	body, _ := json.Marshal(proto.RegisterRequest{
		PublicKey: key.PublicKey().String(), Hostname: hostname,
	})
	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Post(serverURL+"/register", "application/json", bytes.NewReader(body))
	if err != nil {
		return State{}, fmt.Errorf("register request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return State{}, fmt.Errorf("register: server returned %s", resp.Status)
	}
	var rr proto.RegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return State{}, fmt.Errorf("decode register response: %w", err)
	}
	st := State{
		NodeID: rr.NodeID, OverlayIP: rr.OverlayIP, ServerKey: rr.ServerKey,
		ServerEndpoint: rr.ServerEndpoint, OverlayNet: rr.OverlayNet,
		AdvertisedRoutes: rr.AdvertisedRoutes,
	}
	if err := saveState(stateDir, st); err != nil {
		return State{}, err
	}
	return st, nil
}
