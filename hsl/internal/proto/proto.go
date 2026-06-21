// Package proto defines the JSON wire contract between the hsl server and
// client. JSON is snake_case; Go fields are PascalCase via struct tags.
package proto

type RegisterRequest struct {
	PublicKey string `json:"public_key"`
	Hostname  string `json:"hostname"`
}

type RegisterResponse struct {
	NodeID         string `json:"node_id"`
	OverlayIP      string `json:"overlay_ip"`
	ServerKey      string `json:"server_key"`
	ServerEndpoint string `json:"server_endpoint"`
	OverlayNet     string `json:"overlay_net"`
}

type Peer struct {
	ID        string `json:"id"`
	PublicKey string `json:"public_key"`
	OverlayIP string `json:"overlay_ip"`
	Hostname  string `json:"hostname"`
	LastSeen  string `json:"last_seen"`
}

type PeersResponse struct {
	Peers []Peer `json:"peers"`
}

type HeartbeatResponse struct {
	OK bool `json:"ok"`
}
