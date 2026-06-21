package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/nromero/hsl/internal/proto"
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
