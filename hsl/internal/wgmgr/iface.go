package wgmgr

import (
	"errors"
	"fmt"

	"github.com/vishvananda/netlink"
)

// EnsureInterface creates (if needed) a WireGuard interface named name,
// assigns addrCIDR (e.g. "10.100.0.1/24"), sets the MTU, and brings it up.
// It is idempotent. Requires CAP_NET_ADMIN.
func EnsureInterface(name, addrCIDR string, mtu int) error {
	link, err := netlink.LinkByName(name)
	if err != nil {
		var notFound netlink.LinkNotFoundError
		if !errors.As(err, &notFound) {
			return fmt.Errorf("look up %s: %w", name, err)
		}
		la := netlink.NewLinkAttrs()
		la.Name = name
		wg := &netlink.Wireguard{LinkAttrs: la}
		if err := netlink.LinkAdd(wg); err != nil {
			return fmt.Errorf("create %s (is the wireguard kernel module loaded?): %w", name, err)
		}
		link, err = netlink.LinkByName(name)
		if err != nil {
			return fmt.Errorf("re-look up %s: %w", name, err)
		}
	}

	addr, err := netlink.ParseAddr(addrCIDR)
	if err != nil {
		return fmt.Errorf("parse addr %q: %w", addrCIDR, err)
	}
	if err := netlink.AddrReplace(link, addr); err != nil {
		return fmt.Errorf("assign %s to %s: %w", addrCIDR, name, err)
	}
	if err := netlink.LinkSetMTU(link, mtu); err != nil {
		return fmt.Errorf("set mtu on %s: %w", name, err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("bring up %s: %w", name, err)
	}
	return nil
}
