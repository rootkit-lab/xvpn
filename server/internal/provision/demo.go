package provision

import (
	"fmt"
	"net"
	"strings"

	"github.com/rootkit-lab/xvpn/server/internal/corpdns"
)

// DemoVIP is a /32 on wg0 reserved for codespace TCP/UDP preview (Fase 56).
// Not 10.66.66.1 — DNAT of :* there would steal 53/443/445/8080.
const (
	DemoVIP         = "10.66.66.254"
	demoDnsmasqConf = "/etc/dnsmasq.d/xvpn-demo.conf"
	demoNatChain    = "XVPN-DEMO-NAT"
	demoFwdChain    = "XVPN-DEMO-FWD"
	demoPrefix      = "demo-"
)

func ValidDemoName(raw string) (string, error) {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.TrimPrefix(s, demoPrefix)
	if s == "" {
		return "", fmt.Errorf("nome vazio")
	}
	if len(s) > 24 {
		return "", fmt.Errorf("nome longo")
	}
	host, err := corpdns.NormalizeHostname(demoPrefix + s + "." + corpdns.Zone)
	if err != nil {
		return "", err
	}
	label := strings.TrimSuffix(host, "."+corpdns.Zone)
	if !strings.HasPrefix(label, demoPrefix) {
		return "", fmt.Errorf("hostname inválido")
	}
	return strings.TrimPrefix(label, demoPrefix), nil
}

func DemoHostname(name string) string {
	n, err := ValidDemoName(name)
	if err != nil {
		return ""
	}
	return demoPrefix + n + "." + corpdns.Zone
}

func DemoHTTPBase(name string) string {
	h := DemoHostname(name)
	if h == "" {
		return ""
	}
	return "http://" + h
}

func validContainerIP(raw string) (string, error) {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil || ip.To4() == nil {
		return "", fmt.Errorf("ip do container inválido")
	}
	ip = ip.To4()
	if ip[0] != 172 || ip[1] != 17 {
		return "", fmt.Errorf("ip do container fora da bridge docker")
	}
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip[3] < 2 {
		return "", fmt.Errorf("ip do container inválido")
	}
	return ip.String(), nil
}

func applyDemoForward(r CsRunner, spec CsSpec) error {
	if strings.TrimSpace(spec.DemoName) == "" {
		return nil
	}
	name, err := ValidDemoName(spec.DemoName)
	if err != nil {
		return err
	}
	cip, err := containerBridgeIP(r, spec.ID)
	if err != nil {
		return err
	}
	if err := ensureDemoVIP(r); err != nil {
		return err
	}
	if err := applyDemoNAT(r, cip); err != nil {
		return err
	}
	host := DemoHostname(name)
	body := "address=/" + host + "/" + DemoVIP + "\n"
	if err := r.WriteFile(demoDnsmasqConf, body, 0644); err != nil {
		return fmt.Errorf("gravando dnsmasq demo: %w", err)
	}
	return r.HostCmd("systemctl", "reload", "dnsmasq")
}

func clearDemoForward(r CsRunner) error {
	_ = applyDemoNAT(r, "")
	_ = r.WriteFile(demoDnsmasqConf, "# sem demo ativo\n", 0644)
	_ = r.HostCmd("systemctl", "reload", "dnsmasq")
	return nil
}

func containerBridgeIP(r CsRunner, id string) (string, error) {
	out, err := r.DockerOutput("inspect", "-f", "{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}", containerName(id))
	if err != nil {
		return "", err
	}
	first := strings.Fields(strings.TrimSpace(out))
	if len(first) == 0 {
		return "", fmt.Errorf("container sem IP")
	}
	return validContainerIP(first[0])
}

func ensureDemoVIP(r CsRunner) error {
	err := r.HostCmd("ip", "addr", "add", DemoVIP+"/32", "dev", "wg0")
	if err == nil {
		return nil
	}
	low := strings.ToLower(err.Error())
	if strings.Contains(low, "file exists") || strings.Contains(low, "exists") {
		return nil
	}
	return err
}

func jumpIfMissing(r CsRunner, table string, args ...string) error {
	check := append([]string{}, args...)
	if table != "" {
		if err := r.HostCmd("iptables", append([]string{"-t", table, "-C"}, check...)...); err == nil {
			return nil
		}
		return r.HostCmd("iptables", append([]string{"-t", table, "-I", check[0], "1"}, check[1:]...)...)
	}
	if err := r.HostCmd("iptables", append([]string{"-C"}, check...)...); err == nil {
		return nil
	}
	return r.HostCmd("iptables", append([]string{"-I", check[0], "1"}, check[1:]...)...)
}

func applyDemoNAT(r CsRunner, containerIP string) error {
	_ = r.HostCmd("iptables", "-t", "nat", "-N", demoNatChain)
	_ = r.HostCmd("iptables", "-N", demoFwdChain)
	_ = r.HostCmd("iptables", "-t", "nat", "-F", demoNatChain)
	_ = r.HostCmd("iptables", "-F", demoFwdChain)
	if err := jumpIfMissing(r, "nat", "PREROUTING", "-s", "10.66.66.0/24", "-d", DemoVIP, "-j", demoNatChain); err != nil {
		return err
	}
	if err := jumpIfMissing(r, "", "FORWARD", "-j", demoFwdChain); err != nil {
		return err
	}
	if containerIP == "" {
		return nil
	}
	if err := r.HostCmd("iptables", "-t", "nat", "-A", demoNatChain, "-p", "tcp", "-j", "DNAT", "--to-destination", containerIP); err != nil {
		return err
	}
	if err := r.HostCmd("iptables", "-t", "nat", "-A", demoNatChain, "-p", "udp", "-j", "DNAT", "--to-destination", containerIP); err != nil {
		return err
	}
	if err := r.HostCmd("iptables", "-A", demoFwdChain, "-s", "10.66.66.0/24", "-d", containerIP, "-j", "ACCEPT"); err != nil {
		return err
	}
	if err := r.HostCmd("iptables", "-A", demoFwdChain, "-s", containerIP, "-d", "10.66.66.0/24", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err := r.HostCmd("iptables", "-t", "nat", "-C", "POSTROUTING", "-s", "10.66.66.0/24", "-d", containerIP, "-j", "MASQUERADE"); err != nil {
		return r.HostCmd("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", "10.66.66.0/24", "-d", containerIP, "-j", "MASQUERADE")
	}
	return nil
}
