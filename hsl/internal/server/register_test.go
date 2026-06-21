package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nromero/hsl/internal/proto"
)

func postRegister(t *testing.T, s *Server, pk, host string) proto.RegisterResponse {
	t.Helper()
	body, _ := json.Marshal(proto.RegisterRequest{PublicKey: pk, Hostname: host})
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("register status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp proto.RegisterResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

// validKey is a base64 of 32 bytes (a syntactically valid WireGuard key).
const validKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEU="
const validKey2 = "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBEU="

func TestRegisterAssignsFirstClientIP(t *testing.T) {
	s := testServer(t)
	resp := postRegister(t, s, validKey, "node-a")
	if resp.OverlayIP != "10.100.0.2" {
		t.Fatalf("overlay_ip = %s, want 10.100.0.2", resp.OverlayIP)
	}
	if resp.OverlayNet != "10.100.0.0/24" {
		t.Fatalf("overlay_net = %s", resp.OverlayNet)
	}
	if resp.ServerEndpoint != "1.2.3.4:51820" {
		t.Fatalf("server_endpoint = %s", resp.ServerEndpoint)
	}
	if resp.NodeID == "" {
		t.Fatal("node_id empty")
	}
}

func TestRegisterIsIdempotentPerPubkey(t *testing.T) {
	s := testServer(t)
	r1 := postRegister(t, s, validKey, "node-a")
	r2 := postRegister(t, s, validKey, "node-a-renamed")
	if r1.NodeID != r2.NodeID || r1.OverlayIP != r2.OverlayIP {
		t.Fatalf("not idempotent: %+v vs %+v", r1, r2)
	}
}

func TestRegisterSecondNodeGetsNextIP(t *testing.T) {
	s := testServer(t)
	_ = postRegister(t, s, validKey, "a")
	r2 := postRegister(t, s, validKey2, "b")
	if r2.OverlayIP != "10.100.0.3" {
		t.Fatalf("second node ip = %s, want 10.100.0.3", r2.OverlayIP)
	}
}

func TestRegisterRejectsBadKey(t *testing.T) {
	s := testServer(t)
	body, _ := json.Marshal(proto.RegisterRequest{PublicKey: "not-base64!!", Hostname: "x"})
	req := httptest.NewRequest(http.MethodPost, "/register", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
