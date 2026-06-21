// Package client implements the hsl node agent: registration and the
// reconciliation run loop.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/nromero/hsl/internal/proto"
	"github.com/nromero/hsl/internal/wgmgr"
)

// State is the persisted client identity + assignment (node.json).
type State struct {
	NodeID         string `json:"node_id"`
	OverlayIP      string `json:"overlay_ip"`
	ServerKey      string `json:"server_key"`
	ServerEndpoint string `json:"server_endpoint"`
	OverlayNet     string `json:"overlay_net"`
}

func defaultStateDir() string {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return filepath.Join(d, "hsl")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "hsl")
}

func keyPath(stateDir string) string   { return filepath.Join(stateDir, "identity.key") }
func statePath(stateDir string) string { return filepath.Join(stateDir, "node.json") }

func loadState(stateDir string) (State, error) {
	var st State
	data, err := os.ReadFile(statePath(stateDir))
	if err != nil {
		return st, err
	}
	return st, json.Unmarshal(data, &st)
}

func saveState(stateDir string, st State) error {
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(statePath(stateDir), data, 0o600)
}

// Register loads-or-creates the node identity, registers with the server, and
// persists the returned assignment.
func Register(serverURL, hostname, stateDir string) (State, error) {
	if stateDir == "" {
		stateDir = defaultStateDir()
	}
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return State{}, err
	}
	key, err := wgmgr.LoadOrCreateKey(keyPath(stateDir))
	if err != nil {
		return State{}, err
	}
	if hostname == "" {
		hostname, _ = os.Hostname()
	}
	body, _ := json.Marshal(proto.RegisterRequest{
		PublicKey: key.PublicKey().String(), Hostname: hostname,
	})
	httpClient := &http.Client{Timeout: 10 * time.Second}
	resp, err := httpClient.Post(serverURL+"/register", "application/json", bytes.NewReader(body))
	if err != nil {
		return State{}, fmt.Errorf("register request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return State{}, fmt.Errorf("register: server returned %s", resp.Status)
	}
	var rr proto.RegisterResponse
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return State{}, fmt.Errorf("decode register response: %w", err)
	}
	st := State{
		NodeID: rr.NodeID, OverlayIP: rr.OverlayIP, ServerKey: rr.ServerKey,
		ServerEndpoint: rr.ServerEndpoint, OverlayNet: rr.OverlayNet,
	}
	if err := saveState(stateDir, st); err != nil {
		return State{}, err
	}
	return st, nil
}
