package proto

import (
	"encoding/json"
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
	}
	b, _ := json.Marshal(in)
	var out RegisterResponse
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out != in {
		t.Fatalf("round trip mismatch: %+v != %+v", out, in)
	}
}
