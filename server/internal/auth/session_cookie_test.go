package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func cookieCoversIhuull(domain string) bool {
	return domain == ".ihuull.com" || domain == "ihuull.com"
}

func TestIsXAuthHost(t *testing.T) {
	if !IsXAuthHost("xauth.ihuull.com") || !IsXAuthHost("xauth.ihuull.com:443") {
		t.Fatal("xauth.ihuull.com deveria ser o host SSO")
	}
	if !IsXAuthHost("xauth.localhost") {
		t.Fatal("xauth.localhost é o equivalente de teste")
	}
	if IsXAuthHost("xvpn.ihuull.com") || IsXAuthHost("marketplace.ihuull.com") {
		t.Fatal("outros hosts ihuull não gravam o cookie")
	}
}

func TestSetSessionCookie_OnlyOnXAuth(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	c.Request.Host = "xvpn.ihuull.com"
	SetSessionCookie(c, "jwe-token", time.Hour)
	if rec.Result().Cookies() != nil && len(rec.Result().Cookies()) > 0 {
		t.Fatal("login em xvpn não deve gravar cookie (desktop / enroll)")
	}

	rec = httptest.NewRecorder()
	c, _ = gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	c.Request.Host = "xauth.ihuull.com"
	SetSessionCookie(c, "jwe-token", time.Hour)
	cks := rec.Result().Cookies()
	if len(cks) != 1 || cks[0].Name != SessionCookieName || cks[0].Value != "jwe-token" {
		t.Fatalf("cookie SSO: %+v", cks)
	}
	if !cookieCoversIhuull(cks[0].Domain) || !cks[0].HttpOnly || cks[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("atributos do cookie: %+v", cks[0])
	}
	if !cks[0].Secure {
		t.Fatal("cookie em ihuull deve ser Secure")
	}
}

func TestSetSessionCookieOnHost_Panel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodPost, "/api/auth/session", nil)
	c.Request.Host = "xvpn.ihuull.com"
	SetSessionCookieOnHost(c, "jwe-token", time.Hour)
	cks := rec.Result().Cookies()
	if len(cks) != 1 || cks[0].Name != SessionCookieName || cks[0].Value != "jwe-token" {
		t.Fatalf("handoff no painel: %+v", cks)
	}
	if !cookieCoversIhuull(cks[0].Domain) || !cks[0].HttpOnly {
		t.Fatalf("atributos: %+v", cks[0])
	}
}

func TestTokenFromRequest_BearerWinsOverCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	c.Request.Header.Set("Authorization", "Bearer from-header")
	c.Request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "from-cookie"})
	if got := TokenFromRequest(c); got != "from-header" {
		t.Fatalf("Bearer deveria prevalecer, got %q", got)
	}
}

func TestRequireAuth_StaleBearerFallsBackToCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tm := NewTokenManager("um-segredo-de-teste-com-pelo-menos-32-bytes", time.Hour)
	good, err := tm.Issue(9, "rootkit", store.RoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	var uid uint
	var okReq bool
	r := gin.New()
	r.GET("/me", RequireAuth(tm), func(c *gin.Context) {
		okReq = true
		v, _ := c.Get(ContextUserIDKey)
		uid, _ = v.(uint)
		c.Status(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodGet, "/me", nil)
	req.Header.Set("Authorization", "Bearer token-velho-invalido")
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: good})
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent || !okReq {
		t.Fatalf("esperava cookie válido, got %d", rec.Code)
	}
	if uid != uint(9) {
		t.Fatalf("uid %v", uid)
	}
}

func TestTokenFromRequest_CookieFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	c.Request.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "from-cookie"})
	if got := TokenFromRequest(c); got != "from-cookie" {
		t.Fatalf("got %q", got)
	}
}
