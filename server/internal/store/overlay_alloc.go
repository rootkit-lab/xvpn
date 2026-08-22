package store

import (
	"fmt"
	"net"
)

// allocateOverlayIP is the same walk as wireguard.AllocateIP (network,
// first host, broadcast, VIP .254 on infra). Kept here so seed/rehome
// does not import the kernel package.
func allocateOverlayIP(subnetCIDR string, used []string) (string, error) {
	ip, ipNet, err := net.ParseCIDR(subnetCIDR)
	if err != nil {
		return "", fmt.Errorf("sub-rede %q inválida: %w", subnetCIDR, err)
	}
	usedSet := make(map[string]bool, len(used))
	for _, u := range used {
		usedSet[normalizeOverlayIP(u)] = true
	}
	network := cloneIP4(ip.Mask(ipNet.Mask))
	broadcast := lastIPv4(ipNet)
	usedSet[normalizeOverlayIP(network.String())] = true
	usedSet[normalizeOverlayIP(nextIPv4(network).String())] = true
	usedSet[normalizeOverlayIP(broadcast.String())] = true
	usedSet[CodespaceVIP] = true

	for candidate := nextIPv4(nextIPv4(network)); cmpIPv4(candidate, broadcast) < 0; candidate = nextIPv4(candidate) {
		if !usedSet[normalizeOverlayIP(candidate.String())] {
			return candidate.String() + "/32", nil
		}
	}
	return "", fmt.Errorf("nenhum IP livre disponível em %s", subnetCIDR)
}

func normalizeOverlayIP(s string) string {
	if ip, _, err := net.ParseCIDR(s); err == nil {
		return ip.String()
	}
	if ip := net.ParseIP(s); ip != nil {
		return ip.String()
	}
	return s
}

func cloneIP4(ip net.IP) net.IP {
	ip4 := ip.To4()
	dup := make(net.IP, len(ip4))
	copy(dup, ip4)
	return dup
}

func nextIPv4(ip net.IP) net.IP {
	dup := cloneIP4(ip)
	for i := len(dup) - 1; i >= 0; i-- {
		dup[i]++
		if dup[i] != 0 {
			break
		}
	}
	return dup
}

func lastIPv4(ipNet *net.IPNet) net.IP {
	ip := cloneIP4(ipNet.IP)
	mask := ipNet.Mask
	for i := range ip {
		ip[i] |= ^mask[i]
	}
	return ip
}

func cmpIPv4(a, b net.IP) int {
	a4, b4 := a.To4(), b.To4()
	for i := 0; i < 4; i++ {
		if a4[i] != b4[i] {
			if a4[i] < b4[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}
