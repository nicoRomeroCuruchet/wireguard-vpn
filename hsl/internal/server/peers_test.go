package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nromero/hsl/internal/proto"
)

func TestPeersRequiresAuth(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/peers", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestPeersListsRegisteredNodes(t *testing.T) {
	s := testServer(t)
	r := postRegister(t, s, validKey, "node-a")

	req := httptest.NewRequest(http.MethodGet, "/peers", nil)
	req.Header.Set("X-Node-ID", r.NodeID)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp proto.PeersResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	var ips []string
	for _, p := range resp.Peers {
		ips = append(ips, p.OverlayIP)
	}
	if !contains(ips, "10.100.0.2") {
		t.Fatalf("peers %v missing client 10.100.0.2", ips)
	}
}

func TestPeersHubAdvertisedRoutes(t *testing.T) {
	s := testServer(t)
	s.cfg.AdvertisedRoutes = []string{"192.168.1.0/24"}
	r := postRegister(t, s, validKey, "node-a")

	req := httptest.NewRequest(http.MethodGet, "/peers", nil)
	req.Header.Set("X-Node-ID", r.NodeID)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp proto.PeersResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	var hub *proto.Peer
	for i := range resp.Peers {
		if resp.Peers[i].ID == "hub" {
			hub = &resp.Peers[i]
			break
		}
	}
	if hub == nil {
		t.Fatal("hub peer missing")
	}
	if len(hub.AdvertisedRoutes) != 1 || hub.AdvertisedRoutes[0] != "192.168.1.0/24" {
		t.Fatalf("hub advertised routes mismatch: %v", hub.AdvertisedRoutes)
	}
}
func TestHeartbeatOK(t *testing.T) {
	s := testServer(t)
	r := postRegister(t, s, validKey, "node-a")
	req := httptest.NewRequest(http.MethodPost, "/heartbeat", nil)
	req.Header.Set("X-Node-ID", r.NodeID)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var hb proto.HeartbeatResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &hb)
	if !hb.OK {
		t.Fatal("heartbeat ok=false")
	}
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}
