package api

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestHandleListAudit_RequiresAuth(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)

	rec := doJSON(t, router, http.MethodGet, "/api/audit", nil, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperado 401 sem token, obtido %d", rec.Code)
	}
}

func TestHandleListAudit_ListsMostRecentFirst(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "admin", "senha-admin-123")
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	// O próprio login acima já gera uma entrada de auditoria. Criamos mais
	// uma ação para confirmar a ordenação (mais recente primeiro).
	rec := doJSON(t, router, http.MethodPost, "/api/users", createUserRequest{Username: "novo", Password: "senha-do-novo"}, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("esperado 201 criando usuário, obtido %d: %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/audit", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d: %s", rec.Code, rec.Body.String())
	}

	var logs []auditLogResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &logs); err != nil {
		t.Fatalf("erro decodificando resposta: %v", err)
	}
	if len(logs) < 2 {
		t.Fatalf("esperava ao menos 2 entradas de auditoria (login + user.create), obtido %d", len(logs))
	}
	if logs[0].Action != "user.create" {
		t.Fatalf("esperava a entrada mais recente ser user.create, obtido %q", logs[0].Action)
	}
}
