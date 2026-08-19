// Package corpdns valida e gera a zona da intranet (dnsmasq em wg0).
// Bind é sempre 10.66.66.1:53 — nunca 0.0.0.0/eth0 (PLAN.md §5).
package corpdns

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
)

const (
	Zone             = "corp.ihuull.com"
	ListenIP         = "10.66.66.1"
	CorpNet          = "10.66.66.0/24"
	RecordsHostsPath = "/etc/xvpn/dnsmasq-records.hosts"
	DemoHostsPath    = "/etc/xvpn/demo.hosts"
)

var corpCIDR *net.IPNet

func init() {
	_, n, err := net.ParseCIDR(CorpNet)
	if err != nil {
		panic(err)
	}
	corpCIDR = n
}

type Record struct {
	Hostname string `json:"hostname"`
	IPv4     string `json:"ipv4"`
}

type ApplyPayload struct {
	Forwarders    []string `json:"forwarders"`
	CacheSize     int      `json:"cache_size"`
	CatchAll      bool     `json:"catch_all"`
	Records       []Record `json:"records"`
	StackRecords  []Record `json:"stack_records,omitempty"`
	SplitSuffixes []string `json:"split_suffixes,omitempty"`
}

func NormalizeHostname(raw string) (string, error) {
	h := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(raw, ".")))
	if h == "" {
		return "", fmt.Errorf("hostname vazio")
	}
	if h == Zone {
		return h, nil
	}
	if !strings.HasSuffix(h, "."+Zone) {
		return "", fmt.Errorf("hostname deve ser %s ou *.%s", Zone, Zone)
	}
	label := strings.TrimSuffix(h, "."+Zone)
	if label == "" || strings.Contains(label, ".") || strings.ContainsAny(label, " \t/") {
		return "", fmt.Errorf("use um único rótulo sob %s (ex.: app.%s)", Zone, Zone)
	}
	for _, c := range label {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			continue
		}
		return "", fmt.Errorf("rótulo inválido: %q", label)
	}
	if label[0] == '-' || label[len(label)-1] == '-' {
		return "", fmt.Errorf("rótulo inválido: %q", label)
	}
	return h, nil
}

func ValidateIPv4(raw string) (string, error) {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil || ip.To4() == nil {
		return "", fmt.Errorf("IPv4 inválido")
	}
	ip = ip.To4()
	if !corpCIDR.Contains(ip) {
		return "", fmt.Errorf("IPv4 deve estar em %s", CorpNet)
	}
	return ip.String(), nil
}

func ValidateForwarder(raw string) (string, error) {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil || ip.To4() == nil {
		return "", fmt.Errorf("forwarder inválido")
	}
	ip = ip.To4()
	if ip.IsLoopback() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() {
		return "", fmt.Errorf("forwarder %s não é um resolvedor público", ip)
	}
	if corpCIDR.Contains(ip) {
		return "", fmt.Errorf("forwarder não pode ser o gateway da VPN")
	}
	return ip.String(), nil
}

func ParseForwarders(raw string) ([]string, error) {
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == ';' })
	if len(parts) == 0 {
		return nil, fmt.Errorf("informe ao menos um forwarder")
	}
	out := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, p := range parts {
		ip, err := ValidateForwarder(p)
		if err != nil {
			return nil, err
		}
		if seen[ip] {
			continue
		}
		seen[ip] = true
		out = append(out, ip)
	}
	return out, nil
}

func ValidateCacheSize(n int) error {
	if n < 0 || n > 10000 {
		return fmt.Errorf("cache_size deve estar entre 0 e 10000")
	}
	return nil
}

func ParseApplyPayload(raw []byte) (ApplyPayload, error) {
	var p ApplyPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		return ApplyPayload{}, fmt.Errorf("payload DNS inválido")
	}
	if err := ValidateCacheSize(p.CacheSize); err != nil {
		return ApplyPayload{}, err
	}
	fwd := make([]string, 0, len(p.Forwarders))
	for _, f := range p.Forwarders {
		ip, err := ValidateForwarder(f)
		if err != nil {
			return ApplyPayload{}, err
		}
		fwd = append(fwd, ip)
	}
	if len(fwd) == 0 {
		return ApplyPayload{}, fmt.Errorf("informe ao menos um forwarder")
	}
	p.Forwarders = fwd
	recs := make([]Record, 0, len(p.Records))
	seen := map[string]bool{}
	for _, r := range p.Records {
		h, err := NormalizeHostname(r.Hostname)
		if err != nil {
			return ApplyPayload{}, err
		}
		ip, err := ValidateIPv4(r.IPv4)
		if err != nil {
			return ApplyPayload{}, fmt.Errorf("%s: %w", h, err)
		}
		if seen[h] {
			return ApplyPayload{}, fmt.Errorf("hostname duplicado: %s", h)
		}
		seen[h] = true
		recs = append(recs, Record{Hostname: h, IPv4: ip})
	}
	p.Records = recs
	stack := make([]Record, 0, len(p.StackRecords))
	for _, r := range p.StackRecords {
		h := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(r.Hostname, ".")))
		if h == "" || strings.ContainsAny(h, " \t/") {
			return ApplyPayload{}, fmt.Errorf("hostname de stack inválido")
		}
		ip, err := ValidateIPv4(r.IPv4)
		if err != nil {
			return ApplyPayload{}, fmt.Errorf("%s: %w", h, err)
		}
		if seen[h] {
			continue
		}
		seen[h] = true
		stack = append(stack, Record{Hostname: h, IPv4: ip})
	}
	p.StackRecords = stack
	return p, nil
}

func RenderMain(p ApplyPayload) string {
	var b strings.Builder
	b.WriteString("# Gerado pelo painel (/admin/dns). Não edite à mão.\n")
	b.WriteString("# Bind só em wg0 — nunca 0.0.0.0/eth0.\n")
	b.WriteString("bind-interfaces\n")
	b.WriteString("listen-address=" + ListenIP + "\n")
	b.WriteString("except-interface=eth0\n")
	b.WriteString("except-interface=eth1\n")
	b.WriteString("interface=wg0\n")
	b.WriteString("no-dhcp-interface=wg0\n")
	b.WriteString("domain-needed\n")
	b.WriteString("bogus-priv\n")
	b.WriteString("no-resolv\n")
	b.WriteString(fmt.Sprintf("cache-size=%d\n", p.CacheSize))
	b.WriteString("addn-hosts=" + RecordsHostsPath + "\n")
	// SIGHUP relê addn-hosts; não relê host-record= em /etc/dnsmasq.d/.
	b.WriteString("addn-hosts=" + DemoHostsPath + "\n")
	if p.CatchAll {
		b.WriteString("address=/" + Zone + "/" + ListenIP + "\n")
	}
	for _, f := range p.Forwarders {
		b.WriteString("server=" + f + "\n")
	}
	return b.String()
}

func RenderHosts(records []Record) string {
	var b strings.Builder
	b.WriteString("# Gerado pelo painel (/admin/dns). A records da zona corp + stack interno.\n")
	for _, r := range records {
		b.WriteString(r.IPv4 + " " + r.Hostname + "\n")
	}
	return b.String()
}

func RenderRecursor(suffixes []string) string {
	var b strings.Builder
	b.WriteString("# Recursor da malha — só nos hosts mesh, nunca na eth0 do controle.\n")
	b.WriteString("server=/corp.ihuull.com/" + ListenIP + "\n")
	seen := map[string]bool{"corp.ihuull.com": true}
	for _, s := range suffixes {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		b.WriteString("server=/" + s + "/" + ListenIP + "\n")
	}
	return b.String()
}

func AssertSafeMain(conf string) error {
	if strings.Contains(conf, "listen-address=0.0.0.0") || strings.Contains(conf, "listen-address=::") {
		return fmt.Errorf("recusado: bind em 0.0.0.0")
	}
	if !strings.Contains(conf, "listen-address="+ListenIP) {
		return fmt.Errorf("recusado: listen-address deve ser %s", ListenIP)
	}
	return nil
}
