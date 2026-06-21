package server

import (
	"testing"
	"time"
)

func TestMemStoreCreateAndGet(t *testing.T) {
	s := NewMemStore()
	n := Node{ID: "id1", PublicKey: "pk1", OverlayIP: "10.100.0.2", Hostname: "h"}
	if err := s.Create(n); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.GetByPublicKey("pk1")
	if err != nil || !ok {
		t.Fatalf("GetByPublicKey ok=%v err=%v", ok, err)
	}
	if got.OverlayIP != "10.100.0.2" {
		t.Fatalf("got %s", got.OverlayIP)
	}
}

func TestMemStoreTouch(t *testing.T) {
	s := NewMemStore()
	_ = s.Create(Node{ID: "id1", PublicKey: "pk1"})
	ts := time.Date(2026, 6, 21, 0, 0, 0, 0, time.UTC)
	if err := s.Touch("id1", ts); err != nil {
		t.Fatal(err)
	}
	list, _ := s.List()
	if !list[0].LastSeen.Equal(ts) {
		t.Fatalf("LastSeen = %v, want %v", list[0].LastSeen, ts)
	}
}
