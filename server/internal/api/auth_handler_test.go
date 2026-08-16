package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func doJSON(t *testing.T, router http.Handler, method, path string, body any, token string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("erro codificando corpo da requisição: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// createTestUser cria um usuário de teste com papel super_admin — a
// grande maioria dos testes escritos antes da Fase 10 (RBAC) assume que
// "admin" consegue fazer qualquer operação autenticada, o que só
// super_admin garante sem exceção (ex.: gerenciar outro super_admin). Para
// testar a matriz de papéis (viewer/admin/member com permissões restritas),
// use createTestUserWithRole.
func createTestUser(t *testing.T, app *App, username, password string) store.User {
	t.Helper()
	return createTestUserWithRole(t, app, username, password, store.RoleSuperAdmin)
}

func createTestUserWithRole(t *testing.T, app *App, username, password string, role store.Role) store.User {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("erro gerando hash: %v", err)
	}
	user := store.User{Username: username, PasswordHash: hash, Role: role}
	if err := app.Store.DB.Create(&user).Error; err != nil {
		t.Fatalf("erro criando usuário de teste: %v", err)
	}
	return user
}

func TestHandleLogin_Success(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "alice", "senha-forte-123")
	router := NewRouter(app)

	rec := doJSON(t, router, http.MethodPost, "/api/auth/login", loginRequest{Username: "alice", Password: "senha-forte-123"}, "")

	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d: %s", rec.Code, rec.Body.String())
	}
	var resp loginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("erro decodificando resposta: %v", err)
	}
	if resp.Token == "" {
		t.Fatalf("esperava um token não vazio")
	}
}

func TestHandleLogin_WrongPassword(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "bob", "senha-correta")
	router := NewRouter(app)

	rec := doJSON(t, router, http.MethodPost, "/api/auth/login", loginRequest{Username: "bob", Password: "senha-errada"}, "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperado 401, obtido %d", rec.Code)
	}
}

func TestHandleLogin_UnknownUser(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)

	rec := doJSON(t, router, http.MethodPost, "/api/auth/login", loginRequest{Username: "ninguem", Password: "qualquer"}, "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperado 401, obtido %d", rec.Code)
	}
}

func TestHandleLogin_RateLimited(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "alice", "senha-forte-123")
	router := NewRouter(app)

	var lastCode int
	for i := 0; i < loginRateLimitMax+1; i++ {
		rec := doJSON(t, router, http.MethodPost, "/api/auth/login", loginRequest{Username: "alice", Password: "senha-errada"}, "")
		lastCode = rec.Code
	}
	if lastCode != http.StatusTooManyRequests {
		t.Fatalf("esperava a última tentativa do mesmo IP em 429, obtido %d", lastCode)
	}
}

func TestHandleLogin_SetsCookieOnlyOnXAuth(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "sso", "senha-forte-123")
	router := NewRouter(app)

	rec := doJSONHost(t, router, http.MethodPost, "/api/auth/login",
		loginRequest{Username: "sso", Password: "senha-forte-123"}, "", "xauth.ihuull.com")
	if rec.Code != http.StatusOK {
		t.Fatalf("login xauth: %d %s", rec.Code, rec.Body.String())
	}
	var got *http.Cookie
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == auth.SessionCookieName {
			got = ck
		}
	}
	if got == nil || got.Value == "" || (got.Domain != ".ihuull.com" && got.Domain != "ihuull.com") {
		t.Fatalf("esperado cookie SSO em xauth, got %+v", rec.Result().Cookies())
	}

	rec = doJSONHost(t, router, http.MethodPost, "/api/auth/login",
		loginRequest{Username: "sso", Password: "senha-forte-123"}, "", "xvpn.ihuull.com")
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == auth.SessionCookieName {
			t.Fatal("login em xvpn não deve gravar cookie")
		}
	}
}

func TestHandleMe_AcceptsSessionCookie(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "cookie-user", "senha-forte-123")
	router := NewRouter(app)
	loginRec := doJSONHost(t, router, http.MethodPost, "/api/auth/login",
		loginRequest{Username: "cookie-user", Password: "senha-forte-123"}, "", "xauth.ihuull.com")
	var token string
	for _, ck := range loginRec.Result().Cookies() {
		if ck.Name == auth.SessionCookieName {
			token = ck.Value
		}
	}
	if token == "" {
		t.Fatal("login xauth sem cookie")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: token})
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /auth/me com cookie: %d %s", rec.Code, rec.Body.String())
	}
}

func TestHandleEstablishSession_SetsCookieOnPanelAndRedirects(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "handoff", "senha-forte-123")
	router := NewRouter(app)
	loginRec := doJSONHost(t, router, http.MethodPost, "/api/auth/login",
		loginRequest{Username: "handoff", Password: "senha-forte-123"}, "", "xauth.ihuull.com")
	var token string
	for _, ck := range loginRec.Result().Cookies() {
		if ck.Name == auth.SessionCookieName {
			token = ck.Value
		}
	}
	if token == "" {
		t.Fatal("login xauth sem token")
	}

	form := "token=" + token + "&return=https%3A%2F%2Fxvpn.ihuull.com%2Fadmin"
	req := httptest.NewRequest(http.MethodPost, "/api/auth/session", strings.NewReader(form))
	req.Host = "xvpn.ihuull.com"
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Origin", "https://xauth.ihuull.com")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("handoff: %d %s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); loc != "https://xvpn.ihuull.com/admin" {
		t.Fatalf("Location=%q", loc)
	}
	var got *http.Cookie
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == auth.SessionCookieName {
			got = ck
		}
	}
	if got == nil || got.Value != token {
		t.Fatalf("cookie no destino: %+v", rec.Result().Cookies())
	}

	bad := httptest.NewRequest(http.MethodPost, "/api/auth/session", strings.NewReader("token=nao-e-jwe&return=https://evil.example/"))
	bad.Host = "xvpn.ihuull.com"
	bad.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	bad.Header.Set("Origin", "https://xauth.ihuull.com")
	badRec := httptest.NewRecorder()
	router.ServeHTTP(badRec, bad)
	if badRec.Code != http.StatusUnauthorized {
		t.Fatalf("token lixo deveria ser 401, got %d", badRec.Code)
	}

	csrf := httptest.NewRequest(http.MethodPost, "/api/auth/session", strings.NewReader(form))
	csrf.Host = "xvpn.ihuull.com"
	csrf.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	csrf.Header.Set("Origin", "https://evil.example")
	csrfRec := httptest.NewRecorder()
	router.ServeHTTP(csrfRec, csrf)
	if csrfRec.Code != http.StatusForbidden {
		t.Fatalf("Origin estranha deveria ser 403, got %d", csrfRec.Code)
	}

	sameSite := httptest.NewRequest(http.MethodPost, "/api/auth/session", strings.NewReader(form))
	sameSite.Host = "xvpn.ihuull.com"
	sameSite.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	sameSite.Header.Set("Sec-Fetch-Site", "same-site")
	sameRec := httptest.NewRecorder()
	router.ServeHTTP(sameRec, sameSite)
	if sameRec.Code != http.StatusSeeOther {
		t.Fatalf("same-site sem Origin deveria passar, got %d %s", sameRec.Code, sameRec.Body.String())
	}
}

func TestHandleHandoffContinue_OnlyOnXAuth(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "alice", "senha-forte-123")
	router := NewRouter(app)

	login := doJSONHost(t, router, http.MethodPost, "/api/auth/login", loginRequest{Username: "alice", Password: "senha-forte-123"}, "", "xauth.ihuull.com")
	if login.Code != http.StatusOK {
		t.Fatalf("login: %d %s", login.Code, login.Body.String())
	}
	var resp loginResponse
	if err := json.Unmarshal(login.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	fetch := httptest.NewRequest(http.MethodGet, "/api/auth/handoff-continue?return=https://xvpn.ihuull.com/", nil)
	fetch.Host = "xauth.ihuull.com"
	fetch.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: resp.Token})
	fetch.Header.Set("Sec-Fetch-Dest", "empty")
	fetch.Header.Set("Sec-Fetch-Mode", "cors")
	fetchRec := httptest.NewRecorder()
	router.ServeHTTP(fetchRec, fetch)
	if fetchRec.Code != http.StatusForbidden {
		t.Fatalf("fetch XSS deveria ser 403, got %d", fetchRec.Code)
	}

	ok := httptest.NewRequest(http.MethodGet, "/api/auth/handoff-continue?return=https://xvpn.ihuull.com/", nil)
	ok.Host = "xauth.ihuull.com"
	ok.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: resp.Token})
	ok.Header.Set("Sec-Fetch-Dest", "document")
	ok.Header.Set("Sec-Fetch-Mode", "navigate")
	okRec := httptest.NewRecorder()
	router.ServeHTTP(okRec, ok)
	if okRec.Code != http.StatusSeeOther {
		t.Fatalf("xauth cookie: %d %s", okRec.Code, okRec.Body.String())
	}
	loc := okRec.Header().Get("Location")
	if !strings.Contains(loc, "https://xvpn.ihuull.com/api/auth/redeem") || !strings.Contains(loc, "ticket=") {
		t.Fatalf("location: %s", loc)
	}
	if strings.Contains(loc, resp.Token) || strings.Contains(okRec.Body.String(), resp.Token) {
		t.Fatal("JWE vazou no redirect")
	}

	redeemURL, err := url.Parse(loc)
	if err != nil {
		t.Fatal(err)
	}
	redeem := httptest.NewRequest(http.MethodGet, redeemURL.RequestURI(), nil)
	redeem.Host = "xvpn.ihuull.com"
	redeem.Header.Set("Sec-Fetch-Dest", "document")
	redeem.Header.Set("Sec-Fetch-Mode", "navigate")
	redeemRec := httptest.NewRecorder()
	router.ServeHTTP(redeemRec, redeem)
	if redeemRec.Code != http.StatusSeeOther {
		t.Fatalf("redeem: %d %s", redeemRec.Code, redeemRec.Body.String())
	}
	found := false
	for _, ck := range redeemRec.Result().Cookies() {
		if ck.Name == auth.SessionCookieName && ck.Value == resp.Token {
			found = true
		}
	}
	if !found {
		t.Fatal("redeem não plantou cookie")
	}

	panel := httptest.NewRequest(http.MethodGet, "/api/auth/handoff-continue", nil)
	panel.Host = "xvpn.ihuull.com"
	panel.AddCookie(&http.Cookie{Name: auth.SessionCookieName, Value: resp.Token})
	panel.Header.Set("Sec-Fetch-Dest", "document")
	panel.Header.Set("Sec-Fetch-Mode", "navigate")
	panelRec := httptest.NewRecorder()
	router.ServeHTTP(panelRec, panel)
	if panelRec.Code != http.StatusForbidden {
		t.Fatalf("xvpn deveria ser 403, got %d", panelRec.Code)
	}
}

func TestHandleLogout_ClearsCookie(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	rec := doJSONHost(t, router, http.MethodPost, "/api/auth/logout", nil, "", "xauth.ihuull.com")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("logout: %d", rec.Code)
	}
	found := false
	for _, ck := range rec.Result().Cookies() {
		if ck.Name == auth.SessionCookieName && ck.MaxAge < 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("logout deveria expirar o cookie, got %+v", rec.Result().Cookies())
	}
}

func TestProtectedRoute_RequiresAuth(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)

	rec := doJSON(t, router, http.MethodGet, "/api/users", nil, "")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperado 401 sem token, obtido %d", rec.Code)
	}
}

func TestProtectedRoute_RejectsInvalidToken(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)

	rec := doJSON(t, router, http.MethodGet, "/api/users", nil, "token-invalido")

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperado 401 com token inválido, obtido %d", rec.Code)
	}
}
