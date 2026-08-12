package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestHandleGetConfig_RequiresAuth(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)

	rec := doJSON(t, router, http.MethodGet, "/api/config", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperado 401 sem token, obtido %d", rec.Code)
	}
}

func TestHandleGetConfig_NeverExposesSecrets(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "admin", "senha-admin-123")
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doJSON(t, router, http.MethodGet, "/api/config", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d: %s", rec.Code, rec.Body.String())
	}

	var resp configResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("erro decodificando resposta: %v", err)
	}
	if resp.WireGuardEndpoint != app.Config.WireGuardEndpoint {
		t.Fatalf("esperado endpoint %q, obtido %q", app.Config.WireGuardEndpoint, resp.WireGuardEndpoint)
	}

	// O corpo bruto da resposta nunca deve conter o segredo JWT nem a chave
	// privada WireGuard (ver AGENTS.md, invariante de segurança).
	if strings.Contains(rec.Body.String(), "segredo-de-teste-com-pelo-menos-32-bytes") {
		t.Fatalf("resposta de /api/config não deveria conter o JWTSecret")
	}
}
