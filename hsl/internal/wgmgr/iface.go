package wgmgr

import (
	"errors"
	"fmt"
	"net"

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

// EnsureRoutes installs routes for the given CIDRs through the named interface.
// It is idempotent and ignores routes that already exist. Requires CAP_NET_ADMIN.
func EnsureRoutes(iface string, routes []string) error {
	if len(routes) == 0 {
		return nil
	}
	link, err := netlink.LinkByName(iface)
	if err != nil {
		return fmt.Errorf("look up %s: %w", iface, err)
	}
	for _, r := range routes {
		_, dst, err := net.ParseCIDR(r)
		if err != nil {
			return fmt.Errorf("parse route %q: %w", r, err)
		}
		route := netlink.Route{LinkIndex: link.Attrs().Index, Dst: dst}
		if err := netlink.RouteReplace(&route); err != nil {
			return fmt.Errorf("add route %s via %s: %w", r, iface, err)
		}
	}
	return nil
}
