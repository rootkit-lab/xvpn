package api

import (
	"regexp"
	"strings"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

var (
	rePrivateKey = regexp.MustCompile(`-----BEGIN (RSA |OPENSSH |EC |DSA )?PRIVATE KEY-----`)
	reNpmVuln    = regexp.MustCompile(`(?i)(\d+)\s+vulnerabilit`)
	reGovuln     = regexp.MustCompile(`(?m)^Vulnerability #`)
	reGosec      = regexp.MustCompile(`(?i)Severity:\s*(HIGH|MEDIUM|LOW)`)
	rePipAudit   = regexp.MustCompile(`(?i)Found\s+(\d+)\s+known vulnerabilit`)
)

func scanPackSecrets(pack []byte) (reject bool, titles []string) {
	if rePrivateKey.Match(pack) {
		return true, []string{"chave privada no push"}
	}
	return false, nil
}

func parseCiSecurityLog(log, toolHint string) []store.SecAlert {
	log = strings.TrimSpace(log)
	if log == "" {
		return nil
	}
	var out []store.SecAlert
	lower := strings.ToLower(log)
	if strings.Contains(lower, "npm audit") || reNpmVuln.MatchString(log) && strings.Contains(lower, "npm") {
		if m := reNpmVuln.FindStringSubmatch(log); len(m) > 1 && m[1] != "0" {
			out = append(out, store.SecAlert{
				Kind: store.SecKindDeps, Severity: "high", Title: "npm audit: " + m[1] + " vulnerabilidades",
				Tool: "npm-audit", Status: store.SecStatusOpen, Raw: clipRaw(log),
			})
		}
	}
	if reGovuln.MatchString(log) || strings.Contains(lower, "govulncheck") && strings.Contains(lower, "vulnerability") {
		n := len(reGovuln.FindAllString(log, 20))
		if n > 0 {
			out = append(out, store.SecAlert{
				Kind: store.SecKindCode, Severity: "high", Title: "govulncheck: findings",
				Tool: "govulncheck", Status: store.SecStatusOpen, Raw: clipRaw(log),
			})
		}
	}
	if reGosec.MatchString(log) || strings.Contains(lower, "gosec") {
		if reGosec.MatchString(log) {
			out = append(out, store.SecAlert{
				Kind: store.SecKindCode, Severity: "high", Title: "gosec: findings",
				Tool: "gosec", Status: store.SecStatusOpen, Raw: clipRaw(log),
			})
		}
	}
	if m := rePipAudit.FindStringSubmatch(log); len(m) > 1 && m[1] != "0" {
		out = append(out, store.SecAlert{
			Kind: store.SecKindDeps, Severity: "moderate", Title: "pip-audit: " + m[1] + " vulnerabilidades",
			Tool: "pip-audit", Status: store.SecStatusOpen, Raw: clipRaw(log),
		})
	}
	_ = toolHint
	return out
}

func clipRaw(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 2000 {
		return s[:2000]
	}
	return s
}
