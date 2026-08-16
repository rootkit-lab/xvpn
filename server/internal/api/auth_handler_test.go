package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
