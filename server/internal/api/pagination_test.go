package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func decodePage[T any](t *testing.T, body []byte) pageEnvelope {
	t.Helper()
	var env pageEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		t.Fatalf("envelope inválido: %v (%s)", err, body)
	}
	raw, err := json.Marshal(env.Items)
	if err != nil {
		t.Fatalf("items inválidos: %v", err)
	}
	var items []T
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("items não decodificam: %v (%s)", err, raw)
	}
	env.Items = items
	return env
}

func pageItems[T any](t *testing.T, env pageEnvelope) []T {
	t.Helper()
	items, ok := env.Items.([]T)
	if !ok {
		t.Fatalf("items não são []T")
	}
	return items
}

func TestParsePage_ClampsPerPageAndEmptyPage(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "admin", "senha-admin-123")
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doJSON(t, router, http.MethodGet, "/api/users?page=0&per_page=999", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d: %s", rec.Code, rec.Body.String())
	}
	env := decodePage[userResponse](t, rec.Body.Bytes())
	if env.Page != 1 {
		t.Fatalf("page=0 deveria virar 1, obtido %d", env.Page)
	}
	if env.PerPage != maxPerPage {
		t.Fatalf("per_page=999 deveria capar em %d, obtido %d", maxPerPage, env.PerPage)
	}
	if env.Total != 1 {
		t.Fatalf("esperava total 1, obtido %d", env.Total)
	}
}

func TestListUsers_QDoesNotMatchPasswordHash(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "admin", "senha-admin-123")
	createTestUser(t, app, "alice", "senha-alice-ok")
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doJSON(t, router, http.MethodGet, "/api/users?q=alice", nil, token)
	env := decodePage[userResponse](t, rec.Body.Bytes())
	items := pageItems[userResponse](t, env)
	if env.Total != 1 || len(items) != 1 || items[0].Username != "alice" {
		t.Fatalf("q=alice deveria achar só alice, obtido %+v", env)
	}

	// Hash Argon2 contém caracteres aleatórios; buscar um pedaço da senha
	// em claro não pode vazar o hash nem achar o usuário pela senha.
	rec = doJSON(t, router, http.MethodGet, "/api/users?q=senha-alice-ok", nil, token)
	env = decodePage[userResponse](t, rec.Body.Bytes())
	if env.Total != 0 {
		t.Fatalf("q pela senha não pode casar hash, total=%d", env.Total)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/users?page=2&per_page=25", nil, token)
	env = decodePage[userResponse](t, rec.Body.Bytes())
	items = pageItems[userResponse](t, env)
	if env.Total != 2 || len(items) != 0 {
		t.Fatalf("página vazia deveria ter items=[] e total=2, obtido items=%d total=%d", len(items), env.Total)
	}
}

func TestListUsers_RoleFilter(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "admin", "senha-admin-123")
	createTestUserWithRole(t, app, "bob", "senha-bob-ok", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doJSON(t, router, http.MethodGet, "/api/users?role=member", nil, token)
	env := decodePage[userResponse](t, rec.Body.Bytes())
	items := pageItems[userResponse](t, env)
	if env.Total != 1 || items[0].Username != "bob" {
		t.Fatalf("filtro role=member falhou: %+v", env)
	}
}
