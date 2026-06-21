package server

import "time"

// Node is a registered overlay member, including the hub itself.
type Node struct {
	ID        string
	PublicKey string
	OverlayIP string
	Hostname  string
	LastSeen  time.Time
}

// Store persists nodes. Implementations must be safe for concurrent use.
type Store interface {
	GetByPublicKey(pubkey string) (Node, bool, error)
	Create(n Node) error
	List() ([]Node, error)
	Touch(id string, t time.Time) error
}
