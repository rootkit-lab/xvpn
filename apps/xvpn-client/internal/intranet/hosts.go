// Package intranet grava os nomes *.corp no /etc/hosts (ou equivalente)
// enquanto o túnel está no ar. O Chrome com DNS-over-HTTPS ignora o
// resolvectl da xvpn0 — sem isto, xdriver.corp dá NXDOMAIN mesmo com a
// VPN conectada (PLAN.md §6.9).
package intranet

import (
	"fmt"
	"os"
	"strings"
)

const (
	MarkerBegin = "# xvpn-intranet BEGIN"
	MarkerEnd   = "# xvpn-intranet END"
	ServerIP    = "10.66.66.1"
)

// Names são os hostnames da intranet que o helper aponta para a wg0.
var Names = []string{
	"corp.ihuull.com",
	"xchat.corp.ihuull.com",
	"xgroup.corp.ihuull.com",
	"xdriver.corp.ihuull.com",
}

type HostEntry struct {
	Hostname string `json:"hostname"`
	IPv4     string `json:"ipv4"`
}

func block() string {
	return blockEntries(nil)
}

func blockEntries(entries []HostEntry) string {
	if len(entries) == 0 {
		return MarkerBegin + "\n" + ServerIP + " " + strings.Join(Names, " ") + "\n" + MarkerEnd + "\n"
	}
	var b strings.Builder
	b.WriteString(MarkerBegin + "\n")
	grouped := map[string][]string{}
	var order []string
	for _, e := range entries {
		h := strings.TrimSpace(e.Hostname)
		ip := strings.TrimSpace(e.IPv4)
		if h == "" || ip == "" {
			continue
		}
		if _, ok := grouped[ip]; !ok {
			order = append(order, ip)
		}
		grouped[ip] = append(grouped[ip], h)
	}
	if len(order) == 0 {
		return MarkerBegin + "\n" + ServerIP + " " + strings.Join(Names, " ") + "\n" + MarkerEnd + "\n"
	}
	for _, ip := range order {
		b.WriteString(ip + " " + strings.Join(grouped[ip], " ") + "\n")
	}
	b.WriteString(MarkerEnd + "\n")
	return b.String()
}

// Apply insere ou substitui o bloco xvpn no arquivo hosts.
func Apply(path string) error {
	return ApplyEntries(path, nil)
}

// ApplyEntries grava os A publicados pelo /admin/dns, sempre incluindo
// os oficiais (Names). Lista vazia do servidor não pode omitir xchat.
func ApplyEntries(path string, entries []HostEntry) error {
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("lendo %s: %w", path, err)
	}
	body := stripBlock(string(raw))
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return os.WriteFile(path, []byte(body+blockEntries(withOfficial(entries))), 0644)
}

func withOfficial(entries []HostEntry) []HostEntry {
	byHost := make(map[string]string, len(Names)+len(entries))
	order := make([]string, 0, len(Names)+len(entries))
	for _, n := range Names {
		byHost[n] = ServerIP
		order = append(order, n)
	}
	for _, e := range entries {
		h := strings.TrimSpace(e.Hostname)
		ip := strings.TrimSpace(e.IPv4)
		if h == "" || ip == "" {
			continue
		}
		if _, ok := byHost[h]; !ok {
			order = append(order, h)
		}
		byHost[h] = ip
	}
	out := make([]HostEntry, 0, len(order))
	for _, h := range order {
		out = append(out, HostEntry{Hostname: h, IPv4: byHost[h]})
	}
	return out
}

// Revert remove o bloco xvpn do arquivo hosts. Sem bloco, é no-op.
func Revert(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("lendo %s: %w", path, err)
	}
	body := stripBlock(string(raw))
	return os.WriteFile(path, []byte(body), 0644)
}

func stripBlock(src string) string {
	start := strings.Index(src, MarkerBegin)
	if start < 0 {
		return src
	}
	end := strings.Index(src[start:], MarkerEnd)
	if end < 0 {
		return src[:start]
	}
	end = start + end + len(MarkerEnd)
	if end < len(src) && src[end] == '\n' {
		end++
	}
	return src[:start] + src[end:]
}
