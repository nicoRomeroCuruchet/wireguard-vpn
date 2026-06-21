package wgmgr

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrCreateKeyIsStable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "identity.key")

	k1, err := LoadOrCreateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("key file perm = %v, want 0600", info.Mode().Perm())
	}
	k2, err := LoadOrCreateKey(path)
	if err != nil {
		t.Fatal(err)
	}
	if k1.String() != k2.String() {
		t.Fatalf("key not stable: %s != %s", k1, k2)
	}
}
