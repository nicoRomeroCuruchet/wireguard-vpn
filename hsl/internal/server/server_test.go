package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	s, err := New(Config{
		Addr: ":0", Endpoint: "1.2.3.4:51820", OverlayCIDR: "10.100.0.0/24",
	}, NewMemStore(), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestHealthz(t *testing.T) {
	s := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != "ok" {
		t.Fatalf("body = %q, want ok", rec.Body.String())
	}
}
