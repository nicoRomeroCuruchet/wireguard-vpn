package wgmgr

import (
	"fmt"
	"net"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// PeerConfig is a high-level peer description translated to wgtypes.
type PeerConfig struct {
	PublicKey  string        // base64 public key
	Endpoint   string        // "host:port"; empty means no endpoint (NAT'd peer)
	AllowedIPs []string      // CIDRs
	Keepalive  time.Duration // 0 = disabled
}

func buildPeerConfigs(peers []PeerConfig) ([]wgtypes.PeerConfig, error) {
	out := make([]wgtypes.PeerConfig, 0, len(peers))
	for _, p := range peers {
		key, err := wgtypes.ParseKey(p.PublicKey)
		if err != nil {
			return nil, fmt.Errorf("parse peer key %q: %w", p.PublicKey, err)
		}
		allowed := make([]net.IPNet, 0, len(p.AllowedIPs))
		for _, c := range p.AllowedIPs {
			_, ipnet, err := net.ParseCIDR(c)
			if err != nil {
				return nil, fmt.Errorf("parse allowed ip %q: %w", c, err)
			}
			allowed = append(allowed, *ipnet)
		}
		pc := wgtypes.PeerConfig{
			PublicKey:         key,
			ReplaceAllowedIPs: true,
			AllowedIPs:        allowed,
		}
		if p.Endpoint != "" {
			udp, err := net.ResolveUDPAddr("udp", p.Endpoint)
			if err != nil {
				return nil, fmt.Errorf("resolve endpoint %q: %w", p.Endpoint, err)
			}
			pc.Endpoint = udp
		}
		if p.Keepalive > 0 {
			ka := p.Keepalive
			pc.PersistentKeepaliveInterval = &ka
		}
		out = append(out, pc)
	}
	return out, nil
}

// ConfigureDevice sets the private key, listen port, and full peer set on the
// named WireGuard device. listenPort <= 0 leaves the port unchanged.
// Requires CAP_NET_ADMIN.
func ConfigureDevice(name string, priv wgtypes.Key, listenPort int, peers []PeerConfig) error {
	pcs, err := buildPeerConfigs(peers)
	if err != nil {
		return err
	}
	c, err := wgctrl.New()
	if err != nil {
		return fmt.Errorf("open wgctrl: %w", err)
	}
	defer c.Close()

	cfg := wgtypes.Config{
		PrivateKey:   &priv,
		ReplacePeers: true,
		Peers:        pcs,
	}
	if listenPort > 0 {
		cfg.ListenPort = &listenPort
	}
	if err := c.ConfigureDevice(name, cfg); err != nil {
		return fmt.Errorf("configure %s: %w", name, err)
	}
	return nil
}
