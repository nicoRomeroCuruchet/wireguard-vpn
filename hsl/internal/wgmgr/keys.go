// Package wgmgr wraps all kernel WireGuard interaction: key management,
// interface bring-up via netlink, and device/peer configuration via wgctrl.
// It is the only package that imports netlink or wgctrl.
package wgmgr

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// LoadOrCreateKey returns the WireGuard private key stored at path, generating
// and persisting a new one (mode 0600) if the file does not exist.
func LoadOrCreateKey(path string) (wgtypes.Key, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		key, perr := wgtypes.ParseKey(strings.TrimSpace(string(data)))
		if perr != nil {
			return wgtypes.Key{}, fmt.Errorf("parse key %s: %w", path, perr)
		}
		return key, nil
	}
	if !os.IsNotExist(err) {
		return wgtypes.Key{}, err
	}
	key, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return wgtypes.Key{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return wgtypes.Key{}, err
	}
	if err := os.WriteFile(path, []byte(key.String()+"\n"), 0o600); err != nil {
		return wgtypes.Key{}, err
	}
	return key, nil
}
