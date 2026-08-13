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

func createTestUser(t *testing.T, app *App, username, password string) store.User {
	t.Helper()
	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("erro gerando hash: %v", err)
	}
	user := store.User{Username: username, PasswordHash: hash}
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
