package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
	"github.com/rootkit-lab/xvpn/server/internal/wireguard"
)

func loginAndGetToken(t *testing.T, app *App, router http.Handler, username, password string) string {
	t.Helper()
	rec := doJSON(t, router, http.MethodPost, "/api/auth/login", loginRequest{Username: username, Password: password}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login falhou: %d %s", rec.Code, rec.Body.String())
	}
	var resp loginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("erro decodificando resposta de login: %v", err)
	}
	return resp.Token
}

func TestHandleCreateUser_And_List(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "admin", "senha-admin-123")
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doJSON(t, router, http.MethodPost, "/api/users", createUserRequest{Username: "novo", Password: "senha-do-novo"}, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("esperado 201, obtido %d: %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/users", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d", rec.Code)
	}
	var users []userResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &users); err != nil {
		t.Fatalf("erro decodificando lista de usuários: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("esperava 2 usuários (admin + novo), obtido %d", len(users))
	}
}

func TestHandleCreateUser_ShortPasswordRejected(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "admin", "senha-admin-123")
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doJSON(t, router, http.MethodPost, "/api/users", createUserRequest{Username: "curto", Password: "123"}, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400 para senha curta, obtido %d", rec.Code)
	}
}

func TestHandleCreateUser_DuplicateUsernameRejected(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "admin", "senha-admin-123")
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	body := createUserRequest{Username: "duplicado", Password: "senha-valida-123"}
	rec := doJSON(t, router, http.MethodPost, "/api/users", body, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("primeira criação deveria funcionar, obtido %d", rec.Code)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/users", body, token)
	if rec.Code != http.StatusConflict {
		t.Fatalf("esperado 409 para username duplicado, obtido %d", rec.Code)
	}
}

func TestHandleCreateInvite(t *testing.T) {
	app, _ := newTestApp(t)
	admin := createTestUser(t, app, "admin", "senha-admin-123")
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doJSON(t, router, http.MethodPost, "/api/users/"+strconv.FormatUint(uint64(admin.ID), 10)+"/invite", nil, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("esperado 201, obtido %d: %s", rec.Code, rec.Body.String())
	}
	var resp inviteResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("erro decodificando resposta de convite: %v", err)
	}
	if resp.Token == "" {
		t.Fatalf("esperava um token de convite não vazio")
	}
}

func TestHandleDeleteUser_RevokesDevicesToo(t *testing.T) {
	app, wg := newTestApp(t)
	admin := createTestUser(t, app, "admin", "senha-admin-123")
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	pub := "liZAlmaFUyITHHF1GIqBv1yoSVbs5rF+l151paxtOFA="
	device := store.Device{UserID: admin.ID, Name: "notebook", PublicKey: pub, AllowedIP: "10.66.66.5/32"}
	if err := app.Store.DB.Create(&device).Error; err != nil {
		t.Fatalf("erro criando device de teste: %v", err)
	}
	if err := wg.AddPeer(wireguard.PeerSpec{PublicKey: pub, AllowedIP: "10.66.66.5/32"}); err != nil {
		t.Fatalf("erro preparando peer de teste: %v", err)
	}

	rec := doJSON(t, router, http.MethodDelete, "/api/users/"+strconv.FormatUint(uint64(admin.ID), 10), nil, token)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("esperado 204, obtido %d: %s", rec.Code, rec.Body.String())
	}

	peers, err := wg.ListPeers()
	if err != nil {
		t.Fatalf("erro listando peers: %v", err)
	}
	if len(peers) != 0 {
		t.Fatalf("esperava 0 peers após revogar o usuário, obtido %d", len(peers))
	}

	var remaining int64
	app.Store.DB.Model(&store.Device{}).Count(&remaining)
	if remaining != 0 {
		t.Fatalf("esperava 0 devices após deletar o usuário, obtido %d", remaining)
	}
}
