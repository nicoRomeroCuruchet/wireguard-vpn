package server

import (
	"fmt"
	"net"
)

// nextFreeIP returns the lowest host IP in cidr not present in used,
// skipping the network address and .1 (reserved for the hub).
func nextFreeIP(cidr string, used map[string]bool) (string, error) {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return "", fmt.Errorf("parse cidr %q: %w", cidr, err)
	}
	ip := ipnet.IP.Mask(ipnet.Mask).To4()
	if ip == nil {
		return "", fmt.Errorf("only IPv4 overlays supported: %q", cidr)
	}
	// Start at network+2 (skip .0 network and .1 hub).
	cur := make(net.IP, len(ip))
	copy(cur, ip)
	incIP(cur) // .1
	incIP(cur) // .2
	for ipnet.Contains(cur) {
		s := cur.String()
		// Skip the broadcast-style all-ones host for a /24 (.255).
		if cur[len(cur)-1] != 255 && !used[s] {
			return s, nil
		}
		incIP(cur)
	}
	return "", fmt.Errorf("overlay %q exhausted", cidr)
}

func incIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			break
		}
	}
}
