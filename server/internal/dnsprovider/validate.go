package dnsprovider

import (
	"fmt"
	"net"
	"strings"
)

const blockedSuffix = "appapisip.com"

func NormalizeZone(raw string) (string, error) {
	z := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(raw, ".")))
	if z == "" || strings.ContainsAny(z, " \t/") || strings.Contains(z, "..") {
		return "", fmt.Errorf("zona inválida")
	}
	if !strings.Contains(z, ".") {
		return "", fmt.Errorf("zona precisa de um TLD")
	}
	if z == blockedSuffix || strings.HasSuffix(z, "."+blockedSuffix) {
		return "", fmt.Errorf("ldpops/appapisip.com não entra no stack")
	}
	if z == "corp.ihuull.com" || strings.HasSuffix(z, ".corp.ihuull.com") {
		return "", fmt.Errorf("zona corp só existe no dnsmasq")
	}
	return z, nil
}

func NormalizeRecordName(zone, raw string) (string, error) {
	n := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(raw, ".")))
	if n == "" || n == "@" {
		return zone, nil
	}
	if strings.HasSuffix(n, "."+zone) {
		// ok
	} else if n == zone {
		// ok
	} else if !strings.Contains(n, ".") {
		n = n + "." + zone
	} else {
		return "", fmt.Errorf("nome deve ser relativo à zona %s", zone)
	}
	if n == "corp.ihuull.com" || strings.HasSuffix(n, ".corp.ihuull.com") {
		return "", fmt.Errorf("não publique *.corp na internet")
	}
	return n, nil
}

func ValidatePublicContent(rrType, content string) error {
	rrType = strings.ToUpper(strings.TrimSpace(rrType))
	content = strings.TrimSpace(content)
	if content == "" {
		return fmt.Errorf("content é obrigatório")
	}
	switch rrType {
	case "A":
		ip := net.ParseIP(content)
		if ip == nil || ip.To4() == nil {
			return fmt.Errorf("A precisa de IPv4 público")
		}
		ip = ip.To4()
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() {
			return fmt.Errorf("A público não pode ser RFC1918/loopback")
		}
	case "AAAA":
		ip := net.ParseIP(content)
		if ip == nil || ip.To4() != nil {
			return fmt.Errorf("AAAA inválido")
		}
		if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() {
			return fmt.Errorf("AAAA público não pode ser privado")
		}
	case "CNAME", "TXT", "MX":
		if strings.ContainsAny(content, "\n\r") {
			return fmt.Errorf("content inválido")
		}
	default:
		return fmt.Errorf("tipo %s não suportado (A, AAAA, CNAME, TXT, MX)", rrType)
	}
	return nil
}

func AllowedType(rrType string) bool {
	switch strings.ToUpper(strings.TrimSpace(rrType)) {
	case "A", "AAAA", "CNAME", "TXT", "MX":
		return true
	default:
		return false
	}
}
