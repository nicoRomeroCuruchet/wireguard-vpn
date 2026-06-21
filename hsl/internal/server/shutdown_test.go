package server

import (
	"context"
	"log/slog"
	"testing"
	"time"
)

func TestRunReturnsOnContextCancel(t *testing.T) {
	s, err := New(Config{Addr: "127.0.0.1:0", Endpoint: "1.2.3.4:51820",
		OverlayCIDR: "10.100.0.0/24"}, NewMemStore(), slog.Default())
	if err != nil {
		t.Fatal(err)
	}
	// Skip kernel work: this test only exercises the HTTP lifecycle.
	s.skipWG = true

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- s.Run(ctx) }()
	time.Sleep(100 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(11 * time.Second):
		t.Fatal("Run did not return within shutdown timeout")
	}
}
