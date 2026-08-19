package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestHandleChangeMyPassword_Success(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "membro", "senha-antiga-123", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "membro", "senha-antiga-123")

	rec := doJSON(t, router, http.MethodPatch, "/api/me/password",
		changeMyPasswordRequest{CurrentPassword: "senha-antiga-123", NewPassword: "senha-nova-456"}, token)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("esperado 204, obtido %d: %s", rec.Code, rec.Body.String())
	}

	oldLogin := doJSON(t, router, http.MethodPost, "/api/auth/login",
		loginRequest{Username: "membro", Password: "senha-antiga-123"}, "")
	if oldLogin.Code != http.StatusUnauthorized {
		t.Fatalf("senha antiga deveria falhar, obtido %d: %s", oldLogin.Code, oldLogin.Body.String())
	}

	newLogin := doJSON(t, router, http.MethodPost, "/api/auth/login",
		loginRequest{Username: "membro", Password: "senha-nova-456"}, "")
	if newLogin.Code != http.StatusOK {
		t.Fatalf("esperava logar com a senha nova, obtido %d: %s", newLogin.Code, newLogin.Body.String())
	}

	var logs []store.AuditLog
	if err := app.Store.DB.Where("action = ?", "me.password_change").Find(&logs).Error; err != nil {
		t.Fatalf("erro lendo audit: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("esperado 1 audit me.password_change, obtido %d", len(logs))
	}
	if logs[0].Actor != "membro" {
		t.Fatalf("ator inesperado: %q", logs[0].Actor)
	}
}

func TestHandleChangeMyPassword_WrongCurrent(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "membro", "senha-certa-123", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "membro", "senha-certa-123")

	rec := doJSON(t, router, http.MethodPatch, "/api/me/password",
		changeMyPasswordRequest{CurrentPassword: "senha-errada-999", NewPassword: "senha-nova-456"}, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400 (não 401 — sessão continua válida), obtido %d: %s", rec.Code, rec.Body.String())
	}

	stillWorks := doJSON(t, router, http.MethodPost, "/api/auth/login",
		loginRequest{Username: "membro", Password: "senha-certa-123"}, "")
	if stillWorks.Code != http.StatusOK {
		t.Fatalf("senha não deveria ter mudado, login obtido %d: %s", stillWorks.Code, stillWorks.Body.String())
	}
}

func TestHandleChangeMyPassword_RejectsShort(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "membro", "senha-certa-123", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "membro", "senha-certa-123")

	rec := doJSON(t, router, http.MethodPatch, "/api/me/password",
		changeMyPasswordRequest{CurrentPassword: "senha-certa-123", NewPassword: "curta"}, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400 para senha curta, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleChangeMyPassword_RejectsSameAsCurrent(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "membro", "senha-igual-123", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "membro", "senha-igual-123")

	rec := doJSON(t, router, http.MethodPatch, "/api/me/password",
		changeMyPasswordRequest{CurrentPassword: "senha-igual-123", NewPassword: "senha-igual-123"}, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400 quando a nova senha é igual à atual, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleChangeMyPassword_Unauthenticated(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)

	rec := doJSON(t, router, http.MethodPatch, "/api/me/password",
		changeMyPasswordRequest{CurrentPassword: "x", NewPassword: "senha-nova-456"}, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperado 401 sem token, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleChangeMyPassword_MissingFields(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "membro", "senha-certa-123", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "membro", "senha-certa-123")

	rec := doJSON(t, router, http.MethodPatch, "/api/me/password",
		changeMyPasswordRequest{CurrentPassword: "senha-certa-123"}, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400 sem nova senha, obtido %d: %s", rec.Code, rec.Body.String())
	}

	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("erro decodificando: %v", err)
	}
	if body["error"] == "" {
		t.Fatalf("esperava mensagem de erro no corpo")
	}
}
