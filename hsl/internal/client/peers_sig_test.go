package client

import (
	"testing"

	"github.com/nromero/hsl/internal/proto"
)

func TestPeersSignatureOrderIndependent(t *testing.T) {
	a := proto.PeersResponse{Peers: []proto.Peer{{OverlayIP: "10.100.0.3"}, {OverlayIP: "10.100.0.1"}}}
	b := proto.PeersResponse{Peers: []proto.Peer{{OverlayIP: "10.100.0.1"}, {OverlayIP: "10.100.0.3"}}}
	if peersSignature(a) != peersSignature(b) {
		t.Fatalf("signature should be order-independent: %q vs %q", peersSignature(a), peersSignature(b))
	}
}

func TestPeersSignatureDetectsChange(t *testing.T) {
	two := proto.PeersResponse{Peers: []proto.Peer{{OverlayIP: "10.100.0.1"}, {OverlayIP: "10.100.0.3"}}}
	one := proto.PeersResponse{Peers: []proto.Peer{{OverlayIP: "10.100.0.1"}}}
	if peersSignature(two) == peersSignature(one) {
		t.Fatal("signature should differ when the peer set differs")
	}
}
