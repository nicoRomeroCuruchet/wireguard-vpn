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

func TestMemStoreListNumericOrder(t *testing.T) {
	s := NewMemStore()
	_ = s.Create(Node{ID: "a", PublicKey: "pka", OverlayIP: "10.100.0.10"})
	_ = s.Create(Node{ID: "b", PublicKey: "pkb", OverlayIP: "10.100.0.2"})
	list, _ := s.List()
	// .2 must come before .10 — lexical order would put .10 first.
	if list[0].OverlayIP != "10.100.0.2" || list[1].OverlayIP != "10.100.0.10" {
		t.Fatalf("List not numerically ordered: %s, %s", list[0].OverlayIP, list[1].OverlayIP)
	}
}
