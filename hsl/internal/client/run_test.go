package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/nromero/hsl/internal/proto"
)

func writeJSONResp(w http.ResponseWriter, v any) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(v)
}

func TestFetchPeersSendsNodeID(t *testing.T) {
	var gotID atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID.Store(r.Header.Get("X-Node-ID"))
		_ = writeJSONResp(w, proto.PeersResponse{Peers: []proto.Peer{{OverlayIP: "10.100.0.1"}}})
	}))
	defer srv.Close()

	resp, err := fetchPeers(http.DefaultClient, srv.URL, "node-xyz")
	if err != nil {
		t.Fatal(err)
	}
	if gotID.Load() != "node-xyz" {
		t.Fatalf("X-Node-ID = %v, want node-xyz", gotID.Load())
	}
	if len(resp.Peers) != 1 {
		t.Fatalf("got %d peers", len(resp.Peers))
	}
}

func TestHeartbeatSendsNodeID(t *testing.T) {
	var gotID atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID.Store(r.Header.Get("X-Node-ID"))
		_ = writeJSONResp(w, proto.HeartbeatResponse{OK: true})
	}))
	defer srv.Close()

	if err := heartbeat(http.DefaultClient, srv.URL, "node-xyz"); err != nil {
		t.Fatal(err)
	}
	if gotID.Load() != "node-xyz" {
		t.Fatalf("X-Node-ID = %v", gotID.Load())
	}
}
