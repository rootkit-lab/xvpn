package auth

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// SessionCookieName é o cookie HttpOnly do SSO web (PLAN.md §6.13).
// Desktop nunca o envia — usa só Authorization: Bearer em memória.
const SessionCookieName = "ihuull_session"

const (
	xauthHost        = "xauth.ihuull.com"
	xauthLocalHost   = "xauth.localhost"
	sessionCookieDom = ".ihuull.com"
)

func requestHostName(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	return strings.ToLower(host)
}

// IsXAuthHost é o único vhost que grava o cookie de sessão. Login no
// painel/loja (e o cliente desktop em xvpn.ihuull.com) devolve só o JWE.
func IsXAuthHost(host string) bool {
	h := requestHostName(host)
	return h == xauthHost || h == xauthLocalHost
}

func isIhuullHost(host string) bool {
	h := requestHostName(host)
	return h == "ihuull.com" || strings.HasSuffix(h, ".ihuull.com")
}

func isLocalDevHost(host string) bool {
	h := requestHostName(host)
	return h == "localhost" || h == "127.0.0.1" || strings.HasSuffix(h, ".localhost")
}

func bearerToken(c *gin.Context) string {
	header := c.GetHeader("Authorization")
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func cookieToken(c *gin.Context) string {
	ck, err := c.Request.Cookie(SessionCookieName)
	if err != nil || ck == nil {
		return ""
	}
	return strings.TrimSpace(ck.Value)
}

// TokenFromRequest lê Bearer e, se ausente, o cookie de SSO.
func TokenFromRequest(c *gin.Context) string {
	if t := bearerToken(c); t != "" {
		return t
	}
	return cookieToken(c)
}

func sessionCookie(token string, maxAge int, host string) *http.Cookie {
	ck := &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   !isLocalDevHost(host),
		SameSite: http.SameSiteLaxMode,
	}
	if isIhuullHost(host) {
		ck.Domain = sessionCookieDom
	}
	return ck
}

func cookieMaxAge(ttl time.Duration) int {
	maxAge := int(ttl.Seconds())
	if maxAge < 1 {
		return 1
	}
	return maxAge
}

// SetSessionCookie grava o JWE só quando o login veio do xauth.
func SetSessionCookie(c *gin.Context, token string, ttl time.Duration) {
	if !IsXAuthHost(c.Request.Host) {
		return
	}
	http.SetCookie(c.Writer, sessionCookie(token, cookieMaxAge(ttl), c.Request.Host))
}

// SetSessionCookieOnHost grava o JWE em qualquer host ihuull — handoff
// SSO (POST /api/auth/session no destino). Login normal continua só no
// xauth para o desktop não receber Set-Cookie no enroll em xvpn.
func SetSessionCookieOnHost(c *gin.Context, token string, ttl time.Duration) {
	host := c.Request.Host
	if !isIhuullHost(host) && !isLocalDevHost(host) {
		return
	}
	http.SetCookie(c.Writer, sessionCookie(token, cookieMaxAge(ttl), host))
}

// ClearSessionCookie apaga o cookie no host atual e, se for ihuull, no
// Domain=.ihuull.com (o browser só remove com os mesmos atributos).
func ClearSessionCookie(c *gin.Context) {
	http.SetCookie(c.Writer, sessionCookie("", -1, c.Request.Host))
}
