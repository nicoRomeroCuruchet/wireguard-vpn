package server

import (
	"strconv"
	"testing"
)

func TestNextFreeIPEmpty(t *testing.T) {
	got, err := nextFreeIP("10.100.0.0/24", map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	if got != "10.100.0.2" {
		t.Fatalf("got %s, want 10.100.0.2 (skip .0 net and .1 hub)", got)
	}
}

func TestNextFreeIPSkipsUsed(t *testing.T) {
	used := map[string]bool{"10.100.0.2": true, "10.100.0.3": true}
	got, err := nextFreeIP("10.100.0.0/24", used)
	if err != nil {
		t.Fatal(err)
	}
	if got != "10.100.0.4" {
		t.Fatalf("got %s, want 10.100.0.4", got)
	}
}

func TestNextFreeIPExhausted(t *testing.T) {
	used := map[string]bool{}
	for i := 2; i <= 254; i++ {
		used["10.100.0."+strconv.Itoa(i)] = true
	}
	if _, err := nextFreeIP("10.100.0.0/24", used); err == nil {
		t.Fatal("expected error when pool exhausted, got nil")
	}
}
