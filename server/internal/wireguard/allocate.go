package wireguard

import (
	"fmt"
	"net"
)

// AllocateIP encontra o primeiro endereço livre dentro de subnetCIDR (ex.:
// "10.66.66.0/24"), pulando o endereço de rede, o broadcast, e o primeiro
// host (reservado ao próprio servidor — ver PLAN.md §5), além de qualquer
// IP já em used (endereços de "/32" ou IPs simples, ambos aceitos).
//
// Função pura (sem I/O), pensada para ser fácil de testar isoladamente e
// para ser chamada dentro de uma transação no handler de enrollment,
// evitando corrida entre dois enrollments simultâneos.
func AllocateIP(subnetCIDR string, used []string) (string, error) {
	ip, ipNet, err := net.ParseCIDR(subnetCIDR)
	if err != nil {
		return "", fmt.Errorf("sub-rede %q inválida: %w", subnetCIDR, err)
	}

	usedSet := make(map[string]bool, len(used))
	for _, u := range used {
		usedSet[normalizeIP(u)] = true
	}

	serverIP := ip.Mask(ipNet.Mask)
	network := cloneIP(serverIP)
	broadcast := lastAddr(ipNet)

	// Reserva .0 (rede), o primeiro host (servidor) e o broadcast.
	usedSet[normalizeIP(network.String())] = true
	usedSet[normalizeIP(nextIP(network).String())] = true
	usedSet[normalizeIP(broadcast.String())] = true
	// VIP do preview de codespace (Fase 56) — nunca peer.
	usedSet["10.66.66.254"] = true

	for candidate := nextIP(nextIP(network)); compareIPs(candidate, broadcast) < 0; candidate = nextIP(candidate) {
		if !usedSet[normalizeIP(candidate.String())] {
			return fmt.Sprintf("%s/32", candidate.String()), nil
		}
	}

	return "", fmt.Errorf("nenhum IP livre disponível em %s", subnetCIDR)
}

// normalizeIP aceita tanto "10.66.66.5" quanto "10.66.66.5/32" e retorna só
// o endereço, para comparar de forma consistente.
func normalizeIP(s string) string {
	if ip, _, err := net.ParseCIDR(s); err == nil {
		return ip.String()
	}
	if ip := net.ParseIP(s); ip != nil {
		return ip.String()
	}
	return s
}

func cloneIP(ip net.IP) net.IP {
	dup := make(net.IP, len(ip))
	copy(dup, ip)
	return dup
}

func nextIP(ip net.IP) net.IP {
	dup := cloneIP(ip.To4())
	for i := len(dup) - 1; i >= 0; i-- {
		dup[i]++
		if dup[i] != 0 {
			break
		}
	}
	return dup
}

func lastAddr(ipNet *net.IPNet) net.IP {
	ip := cloneIP(ipNet.IP.To4())
	mask := ipNet.Mask
	for i := range ip {
		ip[i] |= ^mask[i]
	}
	return ip
}

func compareIPs(a, b net.IP) int {
	a4, b4 := a.To4(), b.To4()
	for i := 0; i < len(a4); i++ {
		if a4[i] != b4[i] {
			if a4[i] < b4[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}
