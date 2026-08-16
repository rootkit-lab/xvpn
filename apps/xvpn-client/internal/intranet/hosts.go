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

func block() string {
	return MarkerBegin + "\n" + ServerIP + " " + strings.Join(Names, " ") + "\n" + MarkerEnd + "\n"
}

// Apply insere ou substitui o bloco xvpn no arquivo hosts.
func Apply(path string) error {
	raw, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("lendo %s: %w", path, err)
	}
	body := stripBlock(string(raw))
	if body != "" && !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return os.WriteFile(path, []byte(body+block()), 0644)
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
