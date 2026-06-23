package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/nromero/hsl/internal/proto"
	"github.com/nromero/hsl/internal/wgmgr"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

func TestRegisterPersistsState(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/register" || r.Method != http.MethodPost {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var req proto.RegisterRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.PublicKey == "" {
			t.Error("empty public_key sent")
		}
		_ = json.NewEncoder(w).Encode(proto.RegisterResponse{
			NodeID: "node-1", OverlayIP: "10.100.0.2", ServerKey: "srvkey",
			ServerEndpoint: "1.2.3.4:51820", OverlayNet: "10.100.0.0/24",
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	st, err := Register(srv.URL, "my-laptop", dir)
	if err != nil {
		t.Fatal(err)
	}
	if st.NodeID != "node-1" || st.OverlayIP != "10.100.0.2" {
		t.Fatalf("bad state %+v", st)
	}
	// identity.key and node.json must exist.
	if _, err := loadState(dir); err != nil {
		t.Fatalf("state not persisted: %v", err)
	}
	if _, err := readFileExists(filepath.Join(dir, "identity.key")); err != nil {
		t.Fatalf("identity.key missing: %v", err)
	}
}

func readFileExists(path string) (bool, error) {
	if _, err := os.Stat(path); err != nil {
		return false, err
	}
	return true, nil
}

func TestRegisterPersistsAdvertisedRoutes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(proto.RegisterResponse{
			NodeID: "node-1", OverlayIP: "10.100.0.2", ServerKey: "srvkey",
			ServerEndpoint: "1.2.3.4:51820", OverlayNet: "10.100.0.0/24",
			AdvertisedRoutes: []string{"192.168.1.0/24"},
		})
	}))
	defer srv.Close()

	dir := t.TempDir()
	st, err := Register(srv.URL, "my-laptop", dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.AdvertisedRoutes) != 1 || st.AdvertisedRoutes[0] != "192.168.1.0/24" {
		t.Fatalf("advertised routes not persisted: %v", st.AdvertisedRoutes)
	}
	loaded, err := loadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded.AdvertisedRoutes) != 1 {
		t.Fatalf("loaded advertised routes mismatch: %v", loaded.AdvertisedRoutes)
	}
}

func TestConfigureWGMergesAllowedIPs(t *testing.T) {
	var captured []wgmgr.PeerConfig
	configureWG = func(st State, priv wgtypes.Key) error {
		allowedIPs := append([]string{st.OverlayNet}, st.AdvertisedRoutes...)
		captured = []wgmgr.PeerConfig{{
			PublicKey:  st.ServerKey,
			Endpoint:   st.ServerEndpoint,
			AllowedIPs: allowedIPs,
			Keepalive:  keepalive,
		}}
		return nil
	}
	defer func() { configureWG = realConfigureWG }()

	st := State{
		NodeID: "node-1", OverlayIP: "10.100.0.2", ServerKey: "srvkey",
		ServerEndpoint: "1.2.3.4:51820", OverlayNet: "10.100.0.0/24",
		AdvertisedRoutes: []string{"192.168.1.0/24"},
	}
	if err := configureWG(st, wgtypes.Key{}); err != nil {
		t.Fatal(err)
	}
	if len(captured) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(captured))
	}
	want := []string{"10.100.0.0/24", "192.168.1.0/24"}
	if !reflect.DeepEqual(captured[0].AllowedIPs, want) {
		t.Fatalf("AllowedIPs = %v, want %v", captured[0].AllowedIPs, want)
	}
}

func TestIfaceAddrCIDRUsesOverlayPrefix(t *testing.T) {
	got, err := ifaceAddrCIDR("10.100.0.2", "10.100.0.0/24")
	if err != nil {
		t.Fatal(err)
	}
	if got != "10.100.0.2/24" {
		t.Fatalf("got %q, want 10.100.0.2/24 (overlay prefix, not /32)", got)
	}
}

func TestIfaceAddrCIDRBadNet(t *testing.T) {
	if _, err := ifaceAddrCIDR("10.100.0.2", "not-a-cidr"); err == nil {
		t.Fatal("expected error for invalid overlay net")
	}
}
