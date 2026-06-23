package server

import (
	"fmt"
	"os/exec"
)

// SetupSNAT adds iptables POSTROUTING MASQUERADE rules for each advertised
// route so that overlay traffic destined for the LAN is NATed on the hub.
// It is idempotent: duplicate rules are harmless and skipped.
func SetupSNAT(overlayCIDR string, routes []string) error {
	if len(routes) == 0 {
		return nil
	}
	for _, r := range routes {
		cmd := exec.Command("iptables", "-t", "nat", "-C", "POSTROUTING",
			"-s", overlayCIDR, "-d", r, "-j", "MASQUERADE")
		if err := cmd.Run(); err == nil {
			continue // already present
		}
		add := exec.Command("iptables", "-t", "nat", "-A", "POSTROUTING",
			"-s", overlayCIDR, "-d", r, "-j", "MASQUERADE")
		if out, err := add.CombinedOutput(); err != nil {
			return fmt.Errorf("iptables add POSTROUTING for %s: %w | %s", r, err, out)
		}
	}
	return nil
}

// TeardownSNAT removes the iptables rules added by SetupSNAT.
func TeardownSNAT(overlayCIDR string, routes []string) error {
	if len(routes) == 0 {
		return nil
	}
	for _, r := range routes {
		cmd := exec.Command("iptables", "-t", "nat", "-D", "POSTROUTING",
			"-s", overlayCIDR, "-d", r, "-j", "MASQUERADE")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("iptables del POSTROUTING for %s: %w | %s", r, err, out)
		}
	}
	return nil
}
