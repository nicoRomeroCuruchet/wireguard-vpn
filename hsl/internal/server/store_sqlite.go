package server

import (
	"database/sql"
	"fmt"
	"sort"
	"time"

	_ "modernc.org/sqlite"
)

// SQLiteStore is a Store backed by a SQLite file (pure-Go driver, no CGO).
type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(path string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // serialize writes; SQLite single-writer
	const schema = `
CREATE TABLE IF NOT EXISTS nodes (
	id         TEXT PRIMARY KEY,
	public_key TEXT NOT NULL UNIQUE,
	overlay_ip TEXT NOT NULL UNIQUE,
	hostname   TEXT NOT NULL DEFAULT '',
	last_seen  INTEGER NOT NULL DEFAULT 0
);`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Close() error { return s.db.Close() }

func (s *SQLiteStore) GetByPublicKey(pubkey string) (Node, bool, error) {
	row := s.db.QueryRow(
		`SELECT id, public_key, overlay_ip, hostname, last_seen FROM nodes WHERE public_key = ?`, pubkey)
	n, err := scanNode(row)
	if err == sql.ErrNoRows {
		return Node{}, false, nil
	}
	if err != nil {
		return Node{}, false, err
	}
	return n, true, nil
}

func (s *SQLiteStore) Create(n Node) error {
	_, err := s.db.Exec(
		`INSERT INTO nodes (id, public_key, overlay_ip, hostname, last_seen) VALUES (?, ?, ?, ?, ?)`,
		n.ID, n.PublicKey, n.OverlayIP, n.Hostname, n.LastSeen.Unix())
	return err
}

func (s *SQLiteStore) List() ([]Node, error) {
	rows, err := s.db.Query(
		`SELECT id, public_key, overlay_ip, hostname, last_seen FROM nodes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Node
	for rows.Next() {
		n, err := scanNode(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	// Sort in Go (not SQL ORDER BY) so ordering is numeric by overlay IP,
	// identical to MemStore.List. SQL string ordering would put .10 before .2.
	sort.Slice(out, func(i, j int) bool { return lessOverlayIP(out[i].OverlayIP, out[j].OverlayIP) })
	return out, nil
}

func (s *SQLiteStore) Touch(id string, t time.Time) error {
	_, err := s.db.Exec(`UPDATE nodes SET last_seen = ? WHERE id = ?`, t.Unix(), id)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanNode(sc scanner) (Node, error) {
	var n Node
	var lastSeen int64
	if err := sc.Scan(&n.ID, &n.PublicKey, &n.OverlayIP, &n.Hostname, &lastSeen); err != nil {
		return Node{}, err
	}
	n.LastSeen = time.Unix(lastSeen, 0)
	return n, nil
}
