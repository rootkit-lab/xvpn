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
	if resp.AssignedIP != "10.66.66.2/32" {
		t.Fatalf("esperado IP 10.66.66.2/32, obtido %s", resp.AssignedIP)
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
	var devices []deviceResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &devices); err != nil {
		t.Fatalf("erro decodificando lista de devices: %v", err)
	}
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
