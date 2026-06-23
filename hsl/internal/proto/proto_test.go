package proto

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestRegisterRequestJSONTags(t *testing.T) {
	b, err := json.Marshal(RegisterRequest{PublicKey: "abc", Hostname: "h1"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"public_key":"abc","hostname":"h1"}`
	if string(b) != want {
		t.Fatalf("got %s, want %s", b, want)
	}
}

func TestRegisterResponseRoundTrip(t *testing.T) {
	in := RegisterResponse{
		NodeID: "id", OverlayIP: "10.100.0.2", ServerKey: "k",
		ServerEndpoint: "1.2.3.4:51820", OverlayNet: "10.100.0.0/24",
		AdvertisedRoutes: []string{"192.168.1.0/24"},
	}
	b, _ := json.Marshal(in)
	var out RegisterResponse
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Fatalf("round trip mismatch: %+v != %+v", out, in)
	}
}

func TestRegisterResponseAdvertisedRoutesJSON(t *testing.T) {
	in := RegisterResponse{
		NodeID: "id", OverlayIP: "10.100.0.2", ServerKey: "k",
		ServerEndpoint: "1.2.3.4:51820", OverlayNet: "10.100.0.0/24",
		AdvertisedRoutes: []string{"192.168.1.0/24", "10.0.0.0/8"},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"advertised_routes"`) {
		t.Fatalf("json missing advertised_routes key: %s", b)
	}
	if !strings.Contains(string(b), `"192.168.1.0/24"`) {
		t.Fatalf("json missing route value: %s", b)
	}
}

func TestPeerAdvertisedRoutesJSON(t *testing.T) {
	p := Peer{ID: "hub", OverlayIP: "10.100.0.1", AdvertisedRoutes: []string{"192.168.1.0/24"}}
	b, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"advertised_routes"`) {
		t.Fatalf("json missing advertised_routes key: %s", b)
	}
}

func TestAdvertisedRoutesOmitWhenNil(t *testing.T) {
	in := RegisterResponse{
		NodeID: "id", OverlayIP: "10.100.0.2", ServerKey: "k",
		ServerEndpoint: "1.2.3.4:51820", OverlayNet: "10.100.0.0/24",
	}
	b, _ := json.Marshal(in)
	// nil slice should still serialize as null/empty — verify roundtrip works.
	var out RegisterResponse
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if len(out.AdvertisedRoutes) != 0 {
		t.Fatalf("expected empty advertised_routes, got %v", out.AdvertisedRoutes)
	}
}
