package server

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteStorePersistsAcrossReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hsl.db")

	s1, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	n := Node{ID: "id1", PublicKey: "pk1", OverlayIP: "10.100.0.2", Hostname: "h", LastSeen: time.Now()}
	if err := s1.Create(n); err != nil {
		t.Fatal(err)
	}
	_ = s1.Close()

	s2, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	got, ok, err := s2.GetByPublicKey("pk1")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	if got.OverlayIP != "10.100.0.2" {
		t.Fatalf("got %s after reopen", got.OverlayIP)
	}
}

func TestSQLiteStoreList(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hsl.db")
	s, _ := NewSQLiteStore(path)
	defer s.Close()
	_ = s.Create(Node{ID: "a", PublicKey: "pka", OverlayIP: "10.100.0.10"})
	_ = s.Create(Node{ID: "b", PublicKey: "pkb", OverlayIP: "10.100.0.2"})
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	// Numeric order: .2 before .10 (lexical/SQL string order would invert this).
	if len(list) != 2 || list[0].OverlayIP != "10.100.0.2" || list[1].OverlayIP != "10.100.0.10" {
		t.Fatalf("list not numerically sorted by ip: %+v", list)
	}
}
