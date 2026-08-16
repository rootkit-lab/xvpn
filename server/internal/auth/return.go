package auth

import (
	"net/url"
	"strings"
)

// PanelOrigin é o portal /admin — PLAN.md §5.1.
const PanelOrigin = "https://xvpn.ihuull.com"

var safeReturnHosts = map[string]struct{}{
	"xauth.ihuull.com":        {},
	"xvpn.ihuull.com":         {},
	"marketplace.ihuull.com":  {},
	"xdriver.ihuull.com":      {},
	"xdriver.corp.ihuull.com": {},
	"xgroup.ihuull.com":       {},
	"xgroup.corp.ihuull.com":  {},
	"xchat.ihuull.com":        {},
	"xchat.corp.ihuull.com":   {},
	"corp.ihuull.com":         {},
	"www.ihuull.com":          {},
	"ihuull.com":              {},
	"xauth.localhost":         {},
	"xvpn.localhost":          {},
	"marketplace.localhost":   {},
	"xdriver.localhost":       {},
	"xdriver.corp.localhost":  {},
	"xgroup.localhost":        {},
	"xgroup.corp.localhost":   {},
	"xchat.localhost":         {},
	"xchat.corp.localhost":    {},
	"corp.localhost":          {},
	"localhost":               {},
	"127.0.0.1":               {},
}

func isPanelReturnHost(host string) bool {
	return host == "xvpn.ihuull.com" || host == "xvpn.localhost" || host == "localhost" || host == "127.0.0.1"
}

func allowedReturnScheme(scheme, host string) bool {
	if isLocalDevHost(host) {
		return scheme == "https" || scheme == "http"
	}
	return scheme == "https"
}

// TrustedHandoffOrigin aceita Origin/Referer só de hosts ihuull (https, ou http no localhost).
func TrustedHandoffOrigin(raw string) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" || strings.EqualFold(raw, "null") {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	if _, ok := safeReturnHosts[host]; !ok {
		return false
	}
	return allowedReturnScheme(u.Scheme, host)
}

// HandoffAllowed decide se o POST /api/auth/session veio de um host ihuull.
// xauth manda Referrer-Policy same-origin, então o Referer some no POST
// cruzado; o Chrome às vezes omite Origin no POST same-site. Sec-Fetch-Site
// same-site/same-origin no host de destino fecha o CSRF (evil.com é cross-site).
func HandoffAllowed(origin, referer, fetchSite, requestHost string) bool {
	origin = strings.TrimSpace(origin)
	if strings.EqualFold(origin, "null") {
		return false
	}
	if origin != "" {
		return TrustedHandoffOrigin(origin)
	}
	if referer != "" {
		return TrustedHandoffOrigin(referer)
	}
	site := strings.ToLower(strings.TrimSpace(fetchSite))
	if site != "same-site" && site != "same-origin" {
		return false
	}
	host := requestHostName(requestHost)
	_, ok := safeReturnHosts[host]
	return ok
}

// SafeReturnURL aceita só hosts ihuull conhecidos — bloqueia open redirect.
// Espelha server/web/src/lib/product-host.ts.
func SafeReturnURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Hostname())
	if !allowedReturnScheme(u.Scheme, host) {
		return ""
	}
	if host == "vpn.ihuull.com" || host == "vpn.localhost" {
		if strings.HasSuffix(host, ".localhost") {
			u.Host = "xvpn.localhost"
		} else {
			u.Host = "xvpn.ihuull.com"
		}
		host = strings.ToLower(u.Hostname())
	}
	if _, ok := safeReturnHosts[host]; !ok {
		return ""
	}
	if !isPanelReturnHost(host) && strings.HasPrefix(u.Path, "/admin") {
		return PanelOrigin + "/admin"
	}
	return u.String()
}
