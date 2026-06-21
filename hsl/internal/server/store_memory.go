package server

import (
	"sort"
	"sync"
	"time"
)

// MemStore is an in-memory Store used during early development and tests.
type MemStore struct {
	mu   sync.RWMutex
	byID map[string]Node
}

func NewMemStore() *MemStore {
	return &MemStore{byID: make(map[string]Node)}
}

func (s *MemStore) GetByPublicKey(pubkey string) (Node, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, n := range s.byID {
		if n.PublicKey == pubkey {
			return n, true, nil
		}
	}
	return Node{}, false, nil
}

func (s *MemStore) Create(n Node) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[n.ID] = n
	return nil
}

func (s *MemStore) List() ([]Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Node, 0, len(s.byID))
	for _, n := range s.byID {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return lessOverlayIP(out[i].OverlayIP, out[j].OverlayIP) })
	return out, nil
}

func (s *MemStore) Touch(id string, t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if n, ok := s.byID[id]; ok {
		n.LastSeen = t
		s.byID[id] = n
	}
	return nil
}
