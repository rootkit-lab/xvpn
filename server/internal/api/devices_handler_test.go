package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

// Chaves geradas só para fixtures de teste (wgtypes.GeneratePrivateKey),
// não correspondem a nenhum dispositivo real.
const testPublicKey = "roo2tgn2wL/ky+6+rFSXEnyRkx2frp1JIXc/VM6zPGk="
const testPublicKey2 = "pg1Kchi4g+xvp3pYsVBKvy2GMMA+F7lXTLd4Sq73DCY="

func createTestInvite(t *testing.T, app *App, userID uint) string {
	t.Helper()
	invite := store.InviteToken{
		UserID:    userID,
		Token:     "XVPN-TEST-0001",
		ExpiresAt: time.Now().Add(15 * time.Minute),
	}
	if err := app.Store.DB.Create(&invite).Error; err != nil {
		t.Fatalf("erro criando invite de teste: %v", err)
	}
	return invite.Token
}

func TestHandleDeviceEnroll_Success(t *testing.T) {
	app, wg := newTestApp(t)
	admin := createTestUser(t, app, "admin", "senha-admin-123")
	inviteToken := createTestInvite(t, app, admin.ID)
	router := NewRouter(app)

	req := enrollRequest{InviteToken: inviteToken, PublicKey: testPublicKey, DeviceName: "meu-notebook"}
	rec := doJSON(t, router, http.MethodPost, "/api/devices/enroll", req, "")

	if rec.Code != http.StatusCreated {
		t.Fatalf("esperado 201, obtido %d: %s", rec.Code, rec.Body.String())
	}
	var resp enrollResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("erro decodificando resposta: %v", err)
	}
	if resp.AssignedIP != "10.66.80.2/32" {
		t.Fatalf("esperado IP 10.66.80.2/32, obtido %s", resp.AssignedIP)
	}
	if resp.AllowedIPs != "0.0.0.0/0, ::/0" {
		t.Fatalf("users com exit deve ser full-tunnel, obtido %s", resp.AllowedIPs)
	}
	if resp.ServerPublicKey != app.ServerPublicKey {
		t.Fatalf("chave pública do servidor não confere")
	}

	peers, err := wg.ListPeers()
	if err != nil {
		t.Fatalf("erro listando peers: %v", err)
	}
	if len(peers) != 1 {
		t.Fatalf("esperava 1 peer registrado na interface, obtido %d", len(peers))
	}
}

func TestHandleDeviceEnroll_RejectsReusedToken(t *testing.T) {
	app, _ := newTestApp(t)
	admin := createTestUser(t, app, "admin", "senha-admin-123")
	inviteToken := createTestInvite(t, app, admin.ID)
	router := NewRouter(app)

	req := enrollRequest{InviteToken: inviteToken, PublicKey: testPublicKey, DeviceName: "device-1"}
	rec := doJSON(t, router, http.MethodPost, "/api/devices/enroll", req, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("primeiro enrollment deveria funcionar, obtido %d: %s", rec.Code, rec.Body.String())
	}

	req2 := enrollRequest{InviteToken: inviteToken, PublicKey: testPublicKey2, DeviceName: "device-2"}
	rec2 := doJSON(t, router, http.MethodPost, "/api/devices/enroll", req2, "")
	if rec2.Code != http.StatusUnauthorized {
		t.Fatalf("esperado 401 ao reusar convite, obtido %d", rec2.Code)
	}
}

func TestHandleDeviceEnroll_RejectsInvalidToken(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)

	req := enrollRequest{InviteToken: "nao-existe", PublicKey: testPublicKey, DeviceName: "device"}
	rec := doJSON(t, router, http.MethodPost, "/api/devices/enroll", req, "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("esperado 401 para convite inexistente, obtido %d", rec.Code)
	}
}

func TestHandleDeviceEnroll_RejectsInvalidPublicKey(t *testing.T) {
	app, _ := newTestApp(t)
	admin := createTestUser(t, app, "admin", "senha-admin-123")
	inviteToken := createTestInvite(t, app, admin.ID)
	router := NewRouter(app)

	req := enrollRequest{InviteToken: inviteToken, PublicKey: "chave-invalida", DeviceName: "device"}
	rec := doJSON(t, router, http.MethodPost, "/api/devices/enroll", req, "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperado 400 para chave pública inválida, obtido %d", rec.Code)
	}
}

func TestHandleListDevices_And_Delete(t *testing.T) {
	app, wg := newTestApp(t)
	admin := createTestUser(t, app, "admin", "senha-admin-123")
	inviteToken := createTestInvite(t, app, admin.ID)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	enrollReq := enrollRequest{InviteToken: inviteToken, PublicKey: testPublicKey, DeviceName: "notebook"}
	rec := doJSON(t, router, http.MethodPost, "/api/devices/enroll", enrollReq, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("erro no enrollment de setup: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/devices", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d", rec.Code)
	}
	devices := pageItems[deviceResponse](t, decodePage[deviceResponse](t, rec.Body.Bytes()))
	if len(devices) != 1 {
		t.Fatalf("esperava 1 device, obtido %d", len(devices))
	}

	rec = doJSON(t, router, http.MethodDelete, "/api/devices/"+strconv.FormatUint(uint64(devices[0].ID), 10), nil, token)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("esperado 204, obtido %d: %s", rec.Code, rec.Body.String())
	}

	peers, err := wg.ListPeers()
	if err != nil {
		t.Fatalf("erro listando peers: %v", err)
	}
	if len(peers) != 0 {
		t.Fatalf("esperava 0 peers após revogar o device, obtido %d", len(peers))
	}
}

func TestHandleDeleteDevice_NotFound(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUser(t, app, "admin", "senha-admin-123")
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doJSON(t, router, http.MethodDelete, "/api/devices/9999", nil, token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("esperado 404, obtido %d", rec.Code)
	}
}

// TestHandleDeviceEnroll_RestoresInviteOnWireGuardFailure cobre um bug em
// que uma falha do wgctrl/kernel depois do banco já ter marcado o convite
// como usado deixava o código "queimado" para sempre, mesmo o enrollment
// tendo falhado por completo (ver ROADMAP.md Fase 9).
func TestHandleDeviceEnroll_RestoresInviteOnWireGuardFailure(t *testing.T) {
	app, wg := newTestApp(t)
	admin := createTestUser(t, app, "admin", "senha-admin-123")
	inviteToken := createTestInvite(t, app, admin.ID)
	router := NewRouter(app)

	wg.failNextAdd = true
	req := enrollRequest{InviteToken: inviteToken, PublicKey: testPublicKey, DeviceName: "device-1"}
	rec := doJSON(t, router, http.MethodPost, "/api/devices/enroll", req, "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("esperado 500 (falha simulada no WireGuard), obtido %d: %s", rec.Code, rec.Body.String())
	}

	var invite store.InviteToken
	if err := app.Store.DB.Where("token = ?", inviteToken).First(&invite).Error; err != nil {
		t.Fatalf("erro lendo invite: %v", err)
	}
	if invite.UsedAt != nil {
		t.Fatalf("esperava convite restaurado (used_at nulo) após falha no WireGuard, mas continua marcado como usado")
	}

	var deviceCount int64
	app.Store.DB.Model(&store.Device{}).Count(&deviceCount)
	if deviceCount != 0 {
		t.Fatalf("esperava 0 devices após rollback, obtido %d", deviceCount)
	}

	// O bug original "queimava" o convite mesmo com o enrollment tendo
	// falhado — confirma que o mesmo código pode ser reusado agora.
	req2 := enrollRequest{InviteToken: inviteToken, PublicKey: testPublicKey2, DeviceName: "device-2"}
	rec2 := doJSON(t, router, http.MethodPost, "/api/devices/enroll", req2, "")
	if rec2.Code != http.StatusCreated {
		t.Fatalf("esperava que o convite pudesse ser reusado após o rollback, obtido %d: %s", rec2.Code, rec2.Body.String())
	}
}

// TestHandleDeleteDevice_CompensatesWireGuardWhenDBDeleteFails cobre um bug
// em que uma falha de banco *depois* do peer já ter sido removido do WG
// deixava o kernel "à frente" do banco — um restart do servidor chamaria
// ReconcilePeers com o device ainda listado e o peer "ressuscitaria"
// sozinho (ver ROADMAP.md Fase 9).
func TestHandleDeleteDevice_CompensatesWireGuardWhenDBDeleteFails(t *testing.T) {
	app, wg := newTestApp(t)
	admin := createTestUser(t, app, "admin", "senha-admin-123")
	inviteToken := createTestInvite(t, app, admin.ID)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	enrollReq := enrollRequest{InviteToken: inviteToken, PublicKey: testPublicKey, DeviceName: "notebook"}
	rec := doJSON(t, router, http.MethodPost, "/api/devices/enroll", enrollReq, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("erro no enrollment de setup: %d %s", rec.Code, rec.Body.String())
	}
	var device store.Device
	if err := app.Store.DB.First(&device).Error; err != nil {
		t.Fatalf("erro lendo device de setup: %v", err)
	}

	// Simula falha de banco só no DELETE (o SELECT que o próprio handler
	// usa para localizar o device continua funcionando normalmente).
	if err := app.Store.DB.Exec("CREATE TRIGGER block_device_delete BEFORE DELETE ON devices BEGIN SELECT RAISE(ABORT, 'falha simulada de banco'); END;").Error; err != nil {
		t.Fatalf("erro criando trigger de teste: %v", err)
	}

	rec = doJSON(t, router, http.MethodDelete, "/api/devices/"+strconv.FormatUint(uint64(device.ID), 10), nil, token)
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
}

// TestHandleListMyDevices_OnlyOwnDevices cobre o autosserviço da Fase 10
// (PLAN.md §6.7): /api/me/devices nunca deve vazar dispositivos de outro
// usuário, mesmo listando sem nenhum filtro explícito no request.
func TestHandleListMyDevices_OnlyOwnDevices(t *testing.T) {
	app, _ := newTestApp(t)
	owner := createTestUserWithRole(t, app, "member1", "senha-membro-123", store.RoleMember)
	other := createTestUserWithRole(t, app, "member2", "senha-membro-456", store.RoleMember)
	router := NewRouter(app)

	ownerInvite := createTestInvite(t, app, owner.ID)
	otherInvite := &store.InviteToken{UserID: other.ID, Token: "XVPN-TEST-0002", ExpiresAt: time.Now().Add(15 * time.Minute)}
	if err := app.Store.DB.Create(otherInvite).Error; err != nil {
		t.Fatalf("erro criando segundo invite de teste: %v", err)
	}

	rec := doJSON(t, router, http.MethodPost, "/api/devices/enroll", enrollRequest{InviteToken: ownerInvite, PublicKey: testPublicKey, DeviceName: "meu-notebook"}, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("erro no enrollment do owner: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodPost, "/api/devices/enroll", enrollRequest{InviteToken: otherInvite.Token, PublicKey: testPublicKey2, DeviceName: "notebook-do-outro"}, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("erro no enrollment do other: %d %s", rec.Code, rec.Body.String())
	}

	token := loginAndGetToken(t, app, router, "member1", "senha-membro-123")
	rec = doJSON(t, router, http.MethodGet, "/api/me/devices", nil, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d: %s", rec.Code, rec.Body.String())
	}
	var devices []deviceResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &devices); err != nil {
		t.Fatalf("erro decodificando devices: %v", err)
	}
	if len(devices) != 1 || devices[0].PublicKey != testPublicKey {
		t.Fatalf("esperava só o device do próprio usuário, obtido %+v", devices)
	}
}

// TestHandleDeleteMyDevice_NotFoundForOthersDevice garante que tentar
// revogar o device de outro usuário responde 404 (não 403) — não confirma a
// um usuário comum que aquele ID existe, mesmo negando a operação.
func TestHandleDeleteMyDevice_NotFoundForOthersDevice(t *testing.T) {
	app, _ := newTestApp(t)
	owner := createTestUserWithRole(t, app, "member1", "senha-membro-123", store.RoleMember)
	createTestUserWithRole(t, app, "member2", "senha-membro-456", store.RoleMember)
	router := NewRouter(app)

	ownerInvite := createTestInvite(t, app, owner.ID)
	rec := doJSON(t, router, http.MethodPost, "/api/devices/enroll", enrollRequest{InviteToken: ownerInvite, PublicKey: testPublicKey, DeviceName: "meu-notebook"}, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("erro no enrollment do owner: %d %s", rec.Code, rec.Body.String())
	}
	var device store.Device
	if err := app.Store.DB.First(&device).Error; err != nil {
		t.Fatalf("erro lendo device de setup: %v", err)
	}

	token := loginAndGetToken(t, app, router, "member2", "senha-membro-456")
	rec = doJSON(t, router, http.MethodDelete, "/api/me/devices/"+strconv.FormatUint(uint64(device.ID), 10), nil, token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("esperado 404 ao tentar revogar device de outro usuário, obtido %d: %s", rec.Code, rec.Body.String())
	}

	var remaining int64
	app.Store.DB.Model(&store.Device{}).Count(&remaining)
	if remaining != 1 {
		t.Fatalf("device do owner não deveria ter sido afetado, restam %d", remaining)
	}
}

// TestHandleDeleteMyDevice_Success confirma que o dono consegue revogar o
// próprio device por /api/me/devices sem precisar de papel administrativo.
func TestHandleDeleteMyDevice_Success(t *testing.T) {
	app, wg := newTestApp(t)
	owner := createTestUserWithRole(t, app, "member1", "senha-membro-123", store.RoleMember)
	router := NewRouter(app)

	ownerInvite := createTestInvite(t, app, owner.ID)
	rec := doJSON(t, router, http.MethodPost, "/api/devices/enroll", enrollRequest{InviteToken: ownerInvite, PublicKey: testPublicKey, DeviceName: "meu-notebook"}, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("erro no enrollment do owner: %d %s", rec.Code, rec.Body.String())
	}
	var device store.Device
	if err := app.Store.DB.First(&device).Error; err != nil {
		t.Fatalf("erro lendo device de setup: %v", err)
	}

	token := loginAndGetToken(t, app, router, "member1", "senha-membro-123")
	rec = doJSON(t, router, http.MethodDelete, "/api/me/devices/"+strconv.FormatUint(uint64(device.ID), 10), nil, token)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("esperado 204, obtido %d: %s", rec.Code, rec.Body.String())
	}

	peers, err := wg.ListPeers()
	if err != nil {
		t.Fatalf("erro listando peers: %v", err)
	}
	if len(peers) != 0 {
		t.Fatalf("esperava 0 peers após o dono revogar o próprio device, obtido %d", len(peers))
	}
}

func TestHandleDeviceEnroll_RateLimited(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)

	var lastCode int
	for i := 0; i < enrollRateLimitMax+1; i++ {
		rec := doJSON(t, router, http.MethodPost, "/api/devices/enroll", enrollRequest{
			InviteToken: "nao-existe",
			PublicKey:   testPublicKey,
			DeviceName:  "d",
		}, "")
		lastCode = rec.Code
	}
	if lastCode != http.StatusTooManyRequests {
		t.Fatalf("esperava a última tentativa do mesmo IP em 429, obtido %d", lastCode)
	}
}
