//go:build windows

package windows

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

// addHostRouteException instala serverIP/32 via o gateway padrão atual,
// espelhando addHostRouteException do Linux (engine_linux.go). Sem isso,
// com AllowedIPs=0.0.0.0/0 o handshake UDP para o VPS entra na própria
// interface XVPN (loop) — suspeita da Fase 15 no ROADMAP.
func addHostRouteException(serverIP net.IP) error {
	if serverIP == nil {
		return fmt.Errorf("IP do servidor ausente")
	}
	ip4 := serverIP.To4()
	if ip4 == nil {
		return fmt.Errorf("rota de exceção Windows só implementada para IPv4 (%s)", serverIP)
	}
	gw, err := findIPv4DefaultGateway()
	if err != nil {
		return err
	}
	// `route ADD` é idempotente o suficiente: se a rota já existe, o
	// comando falha com mensagem conhecida — tratamos como sucesso.
	out, err := exec.Command("route", "ADD", ip4.String(), "MASK", "255.255.255.255", gw.String()).CombinedOutput()
	if err != nil {
		msg := strings.ToLower(string(out))
		if strings.Contains(msg, "object already exists") || strings.Contains(msg, "já existe") {
			return nil
		}
		return fmt.Errorf("route ADD %s via %s: %w (%s)", ip4, gw, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func removeHostRouteException(serverIP net.IP) {
	if serverIP == nil {
		return
	}
	ip4 := serverIP.To4()
	if ip4 == nil {
		return
	}
	_ = exec.Command("route", "DELETE", ip4.String()).Run()
}

// findIPv4DefaultGateway lê `route print -4` e devolve o NextHop da
// primeira rota 0.0.0.0/0. Preferimos isso a PowerShell para manter a
// dependência igual ao restante do engine (`netsh`/`route`).
func findIPv4DefaultGateway() (net.IP, error) {
	out, err := exec.Command("route", "print", "-4").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("route print -4: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return parseIPv4DefaultGateway(string(out))
}
