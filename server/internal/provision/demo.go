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
	DemoVIP          = "10.66.66.254"
	demoDnsmasqConf  = "/etc/dnsmasq.d/xvpn-demo.conf"
	demoNatChain     = "XVPN-DEMO-NAT"
	demoFwdChain     = "XVPN-DEMO-FWD"
	demoPrefix       = "demo-"
	demoNginxSnippet = "/etc/nginx/snippets/xvpn-demo-vip.conf"
	demoOfflineHTML  = "/var/www/xvpn-demo/demo-offline.html"
	demoOfflineRoot  = "/var/www/xvpn-demo"
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

// DefaultDemoName pairs cs-<id>.corp with demo-cs-<id>.corp (Fase 56).
func DefaultDemoName(codespaceID string) string {
	n, err := ValidDemoName("cs-" + codespaceID)
	if err != nil {
		return ""
	}
	return n
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
	if err := r.HostCmd("systemctl", "reload", "dnsmasq"); err != nil {
		return err
	}
	return applyDemoNginx(r, host)
}

func clearDemoForward(r CsRunner) error {
	_ = applyDemoNAT(r, "")
	_ = r.WriteFile(demoDnsmasqConf, "# sem demo ativo\n", 0644)
	_ = r.HostCmd("systemctl", "reload", "dnsmasq")
	return applyDemoNginx(r, "")
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
	if strings.Contains(low, "file exists") ||
		strings.Contains(low, "already assigned") ||
		strings.Contains(low, "exists") {
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
	if err := r.HostCmd("iptables", "-t", "nat", "-A", demoNatChain, "-p", "tcp", "-m", "multiport", "!", "--dports", "80,443", "-j", "DNAT", "--to-destination", containerIP); err != nil {
		return err
	}
	if err := r.HostCmd("iptables", "-t", "nat", "-A", demoNatChain, "-p", "udp", "-j", "DNAT", "--to-destination", containerIP); err != nil {
		return err
	}
	if err := r.HostCmd("iptables", "-A", demoFwdChain, "-m", "conntrack", "--ctstate", "RELATED,ESTABLISHED", "-j", "ACCEPT"); err != nil {
		return err
	}
	if err := r.HostCmd("iptables", "-A", demoFwdChain, "-s", "10.66.66.0/24", "-d", containerIP, "-j", "ACCEPT"); err != nil {
		return err
	}
	if err := r.HostCmd("iptables", "-t", "nat", "-C", "POSTROUTING", "-s", "10.66.66.0/24", "-d", containerIP, "-j", "MASQUERADE"); err != nil {
		return r.HostCmd("iptables", "-t", "nat", "-A", "POSTROUTING", "-s", "10.66.66.0/24", "-d", containerIP, "-j", "MASQUERADE")
	}
	return nil
}

func applyDemoNginx(r CsRunner, host string) error {
	if err := r.MkdirAll(demoOfflineRoot, 0755); err != nil {
		return fmt.Errorf("mkdir demo offline: %w", err)
	}
	if err := r.WriteFile(demoOfflineHTML, demoOfflinePage, 0644); err != nil {
		return fmt.Errorf("gravando demo-offline.html: %w", err)
	}
	snippet := "# sem demo vip\n"
	if host != "" {
		snippet = demoNginxVIP(host)
	}
	if err := r.WriteFile(demoNginxSnippet, snippet, 0644); err != nil {
		return fmt.Errorf("gravando nginx demo: %w", err)
	}
	if err := r.HostCmd("nginx", "-t"); err != nil {
		return err
	}
	return r.HostCmd("systemctl", "reload", "nginx")
}

func demoNginxVIP(host string) string {
	return fmt.Sprintf(`# Gerado por cs-apply (Fase 56). Não edite.
server {
    listen %s:80;
    server_name %s;
    allow 10.66.66.0/24;
    deny all;
    root /var/www/xvpn-demo;
    default_type text/html;
    location / { try_files /demo-offline.html =503; }
}
server {
    listen %s:443 ssl;
    http2 on;
    server_name %s;
    ssl_certificate     /etc/letsencrypt/live/corp.ihuull.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/corp.ihuull.com/privkey.pem;
    allow 10.66.66.0/24;
    deny all;
    root /var/www/xvpn-demo;
    default_type text/html;
    location / { try_files /demo-offline.html =503; }
}
`, DemoVIP, host, DemoVIP, host)
}

const demoOfflinePage = `<!DOCTYPE html>
<html lang="pt-BR"><head><meta charset="utf-8"><title>XCODESPACES — preview offline</title>
<style>body{margin:0;min-height:100vh;display:flex;align-items:center;justify-content:center;font-family:system-ui,sans-serif;background:#111;color:#ddd}.box{text-align:center;max-width:28rem;padding:2rem}h1{font-size:1.1rem;color:#c8f}p{line-height:1.5;color:#aaa}</style></head>
<body><div class="box"><h1>Preview offline</h1><p>Nenhum serviço escuta nesta porta no codespace.</p><p>Suba o app em <code>0.0.0.0:&lt;porta&gt;</code> e use a aba <strong>Ports</strong> no XCODESPACES.</p></div></body></html>
`
