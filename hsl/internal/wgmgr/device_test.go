package wgmgr

import (
	"testing"
	"time"
)

func TestBuildPeerConfigsParses(t *testing.T) {
	const pk = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEU="
	peers := []PeerConfig{{
		PublicKey:  pk,
		Endpoint:   "1.2.3.4:51820",
		AllowedIPs: []string{"10.100.0.0/24"},
		Keepalive:  25 * time.Second,
	}}
	got, err := buildPeerConfigs(peers)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d peers, want 1", len(got))
	}
	if got[0].Endpoint == nil || got[0].Endpoint.Port != 51820 {
		t.Fatalf("endpoint not parsed: %+v", got[0].Endpoint)
	}
	if len(got[0].AllowedIPs) != 1 || got[0].AllowedIPs[0].String() != "10.100.0.0/24" {
		t.Fatalf("allowedips not parsed: %+v", got[0].AllowedIPs)
	}
	if got[0].PersistentKeepaliveInterval == nil || *got[0].PersistentKeepaliveInterval != 25*time.Second {
		t.Fatalf("keepalive not set")
	}
}

func TestBuildPeerConfigsNoEndpoint(t *testing.T) {
	const pk = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAEU="
	got, err := buildPeerConfigs([]PeerConfig{{PublicKey: pk, AllowedIPs: []string{"10.100.0.2/32"}}})
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Endpoint != nil {
		t.Fatalf("endpoint should be nil when empty, got %+v", got[0].Endpoint)
	}
}
