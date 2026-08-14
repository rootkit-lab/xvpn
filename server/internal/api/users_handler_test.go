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
	createTestUser(t, app, "boss", "senha-admin-123")
	// O alvo da exclusão precisa de um papel abaixo de super_admin — do
	// contrário cairia na guarda de "não remover o único super_admin"
	// (ver TestHandleDeleteUser_LastSuperAdminGuard), o que não é o que
	// este teste quer exercitar.
	target := createTestUserWithRole(t, app, "member1", "senha-membro-123", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "boss", "senha-admin-123")

	pub := "liZAlmaFUyITHHF1GIqBv1yoSVbs5rF+l151paxtOFA="
	device := store.Device{UserID: target.ID, Name: "notebook", PublicKey: pub, AllowedIP: "10.66.66.5/32"}
	if err := app.Store.DB.Create(&device).Error; err != nil {
		t.Fatalf("erro criando device de teste: %v", err)
	}
	if err := wg.AddPeer(wireguard.PeerSpec{PublicKey: pub, AllowedIP: "10.66.66.5/32"}); err != nil {
		t.Fatalf("erro preparando peer de teste: %v", err)
	}

	rec := doJSON(t, router, http.MethodDelete, "/api/users/"+strconv.FormatUint(uint64(target.ID), 10), nil, token)
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

// TestHandleDeleteUser_CompensatesWireGuardWhenDBFails cobre um bug em que
// uma falha de banco *depois* dos peers do usuário já terem sido removidos
// do WG deixava o kernel "à frente" do banco (que ainda tem usuário e
// devices intactos, já que a transação é revertida) — ver ROADMAP.md
// Fase 9.
func TestHandleDeleteUser_CompensatesWireGuardWhenDBFails(t *testing.T) {
	app, wg := newTestApp(t)
	createTestUser(t, app, "boss", "senha-admin-123")
	target := createTestUserWithRole(t, app, "member1", "senha-membro-123", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "boss", "senha-admin-123")

	pub := "liZAlmaFUyITHHF1GIqBv1yoSVbs5rF+l151paxtOFA="
	device := store.Device{UserID: target.ID, Name: "notebook", PublicKey: pub, AllowedIP: "10.66.66.5/32"}
	if err := app.Store.DB.Create(&device).Error; err != nil {
		t.Fatalf("erro criando device de teste: %v", err)
	}
	if err := wg.AddPeer(wireguard.PeerSpec{PublicKey: pub, AllowedIP: "10.66.66.5/32"}); err != nil {
		t.Fatalf("erro preparando peer de teste: %v", err)
	}

	// Simula falha de banco no delete final do usuário, depois que os
	// devices dele já foram apagados dentro da mesma transação — GORM
	// reverte a transação inteira, mas o WG (fora da transação) já
	// tinha sido mexido antes dela nem começar.
	if err := app.Store.DB.Exec("CREATE TRIGGER block_user_delete BEFORE DELETE ON users BEGIN SELECT RAISE(ABORT, 'falha simulada de banco'); END;").Error; err != nil {
		t.Fatalf("erro criando trigger de teste: %v", err)
	}

	rec := doJSON(t, router, http.MethodDelete, "/api/users/"+strconv.FormatUint(uint64(target.ID), 10), nil, token)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("esperado 500 (falha de banco), obtido %d: %s", rec.Code, rec.Body.String())
	}

	peers, err := wg.ListPeers()
	if err != nil {
		t.Fatalf("erro listando peers: %v", err)
	}
	if len(peers) != 1 {
		t.Fatalf("esperava que o peer fosse re-adicionado (compensação) após falha do banco, obtido %d peers", len(peers))
	}

	var remaining int64
	app.Store.DB.Model(&store.Device{}).Count(&remaining)
	if remaining != 1 {
		t.Fatalf("esperava que o device continuasse no banco (transação revertida), obtido %d", remaining)
	}
}

// TestHandleDeleteUser_LastSuperAdminGuard cobre a guarda da Fase 10 (ver
// PLAN.md §6.7): o sistema nunca pode ficar sem nenhum super_admin, ou
// ninguém mais conseguiria promover/gerenciar contas.
func TestHandleDeleteUser_LastSuperAdminGuard(t *testing.T) {
	app, _ := newTestApp(t)
	boss := createTestUser(t, app, "boss", "senha-admin-123")
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "boss", "senha-admin-123")

	rec := doJSON(t, router, http.MethodDelete, "/api/users/"+strconv.FormatUint(uint64(boss.ID), 10), nil, token)
	if rec.Code != http.StatusConflict {
		t.Fatalf("esperado 409 ao tentar remover o único super_admin, obtido %d: %s", rec.Code, rec.Body.String())
	}

	var remaining int64
	app.Store.DB.Model(&store.User{}).Count(&remaining)
	if remaining != 1 {
		t.Fatalf("usuário não deveria ter sido removido, restam %d", remaining)
	}
}

// TestHandleDeleteUser_ForbiddenAcrossRank garante que um admin não
// consegue apagar um super_admin (store.Role.CanManage) — sem essa
// checagem, "admin" gerenciaria contas acima do próprio nível.
func TestHandleDeleteUser_ForbiddenAcrossRank(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "boss", "senha-admin-123", store.RoleSuperAdmin)
	plainAdmin := createTestUserWithRole(t, app, "gerente", "senha-gerente-123", store.RoleAdmin)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "gerente", "senha-gerente-123")

	rec := doJSON(t, router, http.MethodDelete, "/api/users/"+strconv.FormatUint(uint64(mustFindUserID(t, app, "boss")), 10), nil, token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("esperado 403 (admin não gerencia super_admin), obtido %d: %s", rec.Code, rec.Body.String())
	}

	// Sanidade: o mesmo admin consegue apagar quem está no próprio nível
	// ou abaixo.
	member := createTestUserWithRole(t, app, "novato", "senha-novato-123", store.RoleMember)
	rec = doJSON(t, router, http.MethodDelete, "/api/users/"+strconv.FormatUint(uint64(member.ID), 10), nil, token)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("esperado 204 removendo um member, obtido %d: %s", rec.Code, rec.Body.String())
	}
	_ = plainAdmin
}

func mustFindUserID(t *testing.T, app *App, username string) uint {
	t.Helper()
	var u store.User
	if err := app.Store.DB.Where("username = ?", username).First(&u).Error; err != nil {
		t.Fatalf("erro buscando usuário %q: %v", username, err)
	}
	return u.ID
}

func TestHandleCreateUser_DefaultsToMemberRole(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "admin", "senha-admin-123")
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doJSON(t, router, http.MethodPost, "/api/users", createUserRequest{Username: "novo", Password: "senha-do-novo"}, token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("esperado 201, obtido %d: %s", rec.Code, rec.Body.String())
	}
	var created userResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("erro decodificando resposta: %v", err)
	}
	if created.Role != store.RoleMember {
		t.Fatalf("esperava role padrão member, obtido %q", created.Role)
	}
}

// TestHandleCreateUser_CannotEscalatePrivilege garante que um admin nunca
// cria outro super_admin — só super_admin promove a super_admin (ver
// PLAN.md §6.7: "admin: sem promover a super_admin").
func TestHandleCreateUser_CannotEscalatePrivilege(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "gerente", "senha-gerente-123", store.RoleAdmin)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "gerente", "senha-gerente-123")

	rec := doJSON(t, router, http.MethodPost, "/api/users",
		createUserRequest{Username: "novo-boss", Password: "senha-valida-123", Role: store.RoleSuperAdmin}, token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("esperado 403 ao tentar criar super_admin sendo admin, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCreateUser_InvalidRoleRejected(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "admin", "senha-admin-123")
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doJSON(t, router, http.MethodPost, "/api/users",
		map[string]any{"username": "novo", "password": "senha-valida-123", "role": "onipotente"}, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400 para role inválido, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleUpdateUser_ChangesUsername(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "admin", "senha-admin-123")
	target := createTestUserWithRole(t, app, "antigo-nome", "senha-membro-123", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	newUsername := "novo-nome"
	rec := doJSON(t, router, http.MethodPatch, "/api/users/"+strconv.FormatUint(uint64(target.ID), 10),
		updateUserRequest{Username: &newUsername}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d: %s", rec.Code, rec.Body.String())
	}
	var updated userResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("erro decodificando resposta: %v", err)
	}
	if updated.Username != newUsername {
		t.Fatalf("esperava username %q, obtido %q", newUsername, updated.Username)
	}
	if updated.Role != store.RoleMember {
		t.Fatalf("trocar só o username não deveria mudar o papel, obtido %q", updated.Role)
	}
}

func TestHandleUpdateUser_ChangesRole(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "admin", "senha-admin-123")
	target := createTestUserWithRole(t, app, "promovido", "senha-membro-123", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	newRole := store.RoleViewer
	rec := doJSON(t, router, http.MethodPatch, "/api/users/"+strconv.FormatUint(uint64(target.ID), 10),
		updateUserRequest{Role: &newRole}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d: %s", rec.Code, rec.Body.String())
	}
	var updated userResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &updated); err != nil {
		t.Fatalf("erro decodificando resposta: %v", err)
	}
	if updated.Role != store.RoleViewer {
		t.Fatalf("esperava role viewer, obtido %q", updated.Role)
	}
}

// TestHandleUpdateUser_CannotChangeOwnRole evita auto-promoção/rebaixamento
// acidental (ou malicioso, caso um token vaze) — ver PLAN.md §6.7.
func TestHandleUpdateUser_CannotChangeOwnRole(t *testing.T) {
	app, _ := newTestApp(t)
	boss := createTestUser(t, app, "boss", "senha-admin-123")
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "boss", "senha-admin-123")

	newRole := store.RoleAdmin
	rec := doJSON(t, router, http.MethodPatch, "/api/users/"+strconv.FormatUint(uint64(boss.ID), 10),
		updateUserRequest{Role: &newRole}, token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("esperado 403 ao tentar alterar o próprio papel, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleUpdateUser_CannotEscalatePrivilege: um admin não promove
// ninguém a super_admin (mesma regra de handleCreateUser).
func TestHandleUpdateUser_CannotEscalatePrivilege(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "gerente", "senha-gerente-123", store.RoleAdmin)
	target := createTestUserWithRole(t, app, "novato", "senha-novato-123", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "gerente", "senha-gerente-123")

	newRole := store.RoleSuperAdmin
	rec := doJSON(t, router, http.MethodPatch, "/api/users/"+strconv.FormatUint(uint64(target.ID), 10),
		updateUserRequest{Role: &newRole}, token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("esperado 403 ao tentar promover a super_admin sendo admin, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleUpdateUser_CannotDemoteLastSuperAdmin verifica que dois
// super_admins conseguem se gerenciar mutuamente (rank igual, ver
// store.Role.CanManage) enquanto isso não reduzir o total a zero, e que a
// combinação "ninguém troca o próprio papel" + "CanManage exige rank igual
// ou superior" já torna impossível zerar os super_admins por esta rota:
// para rebaixar um super_admin restante, o ator também precisaria ser
// super_admin — mas se só existe um, ator e alvo são a mesma pessoa, e aí a
// guarda de auto-modificação barra primeiro (403, nunca chega no 409 de
// handleDeleteUser). O guard de contagem em handleUpdateUser continua no
// código como defesa em profundidade caso essa invariante mude no futuro.
func TestHandleUpdateUser_CannotDemoteLastSuperAdmin(t *testing.T) {
	app, _ := newTestApp(t)
	boss := createTestUser(t, app, "boss", "senha-admin-123")
	secondAdmin := createTestUserWithRole(t, app, "segundo", "senha-segundo-123", store.RoleAdmin)
	router := NewRouter(app)
	tokenSecond := loginAndGetToken(t, app, router, "segundo", "senha-segundo-123")

	newRole := store.RoleAdmin
	// "segundo" (admin) não gerencia "boss" (super_admin) de forma
	// alguma — a checagem de rank já barra antes de qualquer guarda de
	// contagem de super_admin.
	rec := doJSON(t, router, http.MethodPatch, "/api/users/"+strconv.FormatUint(uint64(boss.ID), 10),
		updateUserRequest{Role: &newRole}, tokenSecond)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("esperado 403 (admin não gerencia super_admin), obtido %d: %s", rec.Code, rec.Body.String())
	}

	// Promove "segundo" a super_admin usando o boss — dois super_admins
	// agora conseguem se gerenciar mutuamente.
	tokenBoss := loginAndGetToken(t, app, router, "boss", "senha-admin-123")
	promote := store.RoleSuperAdmin
	rec = doJSON(t, router, http.MethodPatch, "/api/users/"+strconv.FormatUint(uint64(secondAdmin.ID), 10),
		updateUserRequest{Role: &promote}, tokenBoss)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200 promovendo o segundo usuário a super_admin, obtido %d: %s", rec.Code, rec.Body.String())
	}

	tokenSecond = loginAndGetToken(t, app, router, "segundo", "senha-segundo-123")
	demote := store.RoleAdmin
	rec = doJSON(t, router, http.MethodPatch, "/api/users/"+strconv.FormatUint(uint64(boss.ID), 10),
		updateUserRequest{Role: &demote}, tokenSecond)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200 rebaixando o boss (não é mais o único super_admin), obtido %d: %s", rec.Code, rec.Body.String())
	}

	// Agora "segundo" é o único super_admin. Ele não consegue rebaixar a
	// si mesmo — não porque a contagem bloqueia, mas porque ninguém troca
	// o próprio papel (e não há mais ninguém apto a fazer isso por ele).
	rec = doJSON(t, router, http.MethodPatch, "/api/users/"+strconv.FormatUint(uint64(secondAdmin.ID), 10),
		updateUserRequest{Role: &demote}, tokenSecond)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("esperado 403 (auto-modificação de papel), obtido %d: %s", rec.Code, rec.Body.String())
	}

	var stillSuperAdmin store.User
	if err := app.Store.DB.First(&stillSuperAdmin, secondAdmin.ID).Error; err != nil {
		t.Fatalf("erro relendo usuário: %v", err)
	}
	if stillSuperAdmin.Role != store.RoleSuperAdmin {
		t.Fatalf("o único super_admin não deveria ter sido rebaixado, papel atual: %q", stillSuperAdmin.Role)
	}
}

func TestHandleResetPassword_GeneratesPasswordWhenOmitted(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "admin", "senha-admin-123")
	target := createTestUserWithRole(t, app, "usuario", "senha-antiga-123", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doJSON(t, router, http.MethodPost, "/api/users/"+strconv.FormatUint(uint64(target.ID), 10)+"/reset-password", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d: %s", rec.Code, rec.Body.String())
	}
	var resp resetPasswordResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("erro decodificando resposta: %v", err)
	}
	if resp.Password == "" {
		t.Fatalf("esperava uma senha gerada na resposta")
	}

	loginRec := doJSON(t, router, http.MethodPost, "/api/auth/login", loginRequest{Username: "usuario", Password: resp.Password}, "")
	if loginRec.Code != http.StatusOK {
		t.Fatalf("esperava conseguir logar com a senha gerada, obtido %d: %s", loginRec.Code, loginRec.Body.String())
	}
}

func TestHandleResetPassword_UsesProvidedPassword(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "admin", "senha-admin-123")
	target := createTestUserWithRole(t, app, "usuario", "senha-antiga-123", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doJSON(t, router, http.MethodPost, "/api/users/"+strconv.FormatUint(uint64(target.ID), 10)+"/reset-password",
		resetPasswordRequest{Password: "senha-escolhida-123"}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d: %s", rec.Code, rec.Body.String())
	}
	var resp resetPasswordResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("erro decodificando resposta: %v", err)
	}
	if resp.Password != "" {
		t.Fatalf("não deveria devolver senha quando o admin já informou uma, obtido %q", resp.Password)
	}

	loginRec := doJSON(t, router, http.MethodPost, "/api/auth/login", loginRequest{Username: "usuario", Password: "senha-escolhida-123"}, "")
	if loginRec.Code != http.StatusOK {
		t.Fatalf("esperava conseguir logar com a senha escolhida, obtido %d: %s", loginRec.Code, loginRec.Body.String())
	}
}

func TestHandleResetPassword_RejectsShortPassword(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "admin", "senha-admin-123")
	target := createTestUserWithRole(t, app, "usuario", "senha-antiga-123", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doJSON(t, router, http.MethodPost, "/api/users/"+strconv.FormatUint(uint64(target.ID), 10)+"/reset-password",
		resetPasswordRequest{Password: "123"}, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400 para senha curta, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

// TestHandleResetPassword_ForbiddenAcrossRank: mesma checagem de
// CanManage — um admin não reseta a senha de um super_admin.
func TestHandleResetPassword_ForbiddenAcrossRank(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "boss", "senha-admin-123", store.RoleSuperAdmin)
	createTestUserWithRole(t, app, "gerente", "senha-gerente-123", store.RoleAdmin)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "gerente", "senha-gerente-123")

	bossID := mustFindUserID(t, app, "boss")
	rec := doJSON(t, router, http.MethodPost, "/api/users/"+strconv.FormatUint(uint64(bossID), 10)+"/reset-password", nil, token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("esperado 403 (admin não gerencia super_admin), obtido %d: %s", rec.Code, rec.Body.String())
	}
}
