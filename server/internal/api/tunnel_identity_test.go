package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

// doJSONFrom simula uma requisição chegando de remoteAddr (peer TCP).
// É o que distingue o caminho do túnel (10.66.66.x) do caminho do Nginx
// (127.0.0.1): RemoteIP() lê Request.RemoteAddr, não headers.
func doJSONFrom(t *testing.T, router http.Handler, method, path string, body any, remoteAddr string, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("erro codificando corpo: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	req.RemoteAddr = remoteAddr
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func enrollDeviceFor(t *testing.T, app *App, router http.Handler, user store.User, deviceName, pubKey string) store.Device {
	t.Helper()
	invite := createTestInvite(t, app, user.ID)
	rec := doJSON(t, router, http.MethodPost, "/api/devices/enroll",
		enrollRequest{InviteToken: invite, PublicKey: pubKey, DeviceName: deviceName}, "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("enrollment falhou: %d %s", rec.Code, rec.Body.String())
	}
	var device store.Device
	if err := app.Store.DB.Where("name = ?", deviceName).First(&device).Error; err != nil {
		t.Fatalf("device não persistido: %v", err)
	}
	return device
}

func TestTunnelIdentity_RejectsNginxPathEvenWithForgedXFF(t *testing.T) {
	app, _ := newTestApp(t)
	admin := createTestUser(t, app, "admin", "senha-admin-123")
	router := NewRouter(app)
	_ = enrollDeviceFor(t, app, router, admin, "notebook", testPublicKey)

	// Simula o Nginx: peer TCP = 127.0.0.1, header forjado apontando
	// para o IP do peer. Se o middleware usasse ClientIP(), isto
	// devolveria 200 — exatamente o furo que a revisão de segurança
	// apontou.
	rec := doJSONFrom(t, router, http.MethodGet, "/api/me", nil, "127.0.0.1:54321",
		map[string]string{"X-Forwarded-For": "10.66.66.2"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("esperava 403 pelo caminho do Nginx, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestTunnelIdentity_AcceptsPeerInsideTunnel(t *testing.T) {
	app, _ := newTestApp(t)
	admin := createTestUser(t, app, "admin", "senha-admin-123")
	router := NewRouter(app)
	device := enrollDeviceFor(t, app, router, admin, "notebook", testPublicKey)

	// AllowedIP é "10.66.66.N/32" — RemoteAddr precisa do IP sem máscara.
	ip := device.AllowedIP
	if len(ip) > 3 && ip[len(ip)-3:] == "/32" {
		ip = ip[:len(ip)-3]
	}

	rec := doJSONFrom(t, router, http.MethodGet, "/api/me", nil, ip+":12345", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperava 200 de dentro do túnel, obtido %d: %s", rec.Code, rec.Body.String())
	}
	var body tunnelIdentityResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Username != "admin" {
		t.Errorf("username = %q, esperado admin", body.Username)
	}
}

func TestRegisterDeviceSSHKey_IdempotentAndDBOnlyWhenSFTPOff(t *testing.T) {
	fp := &fakeUserProvisioner{}
	app, _ := withProvisioner(t, fp)
	admin := createTestUser(t, app, "admin", "senha-admin-123")
	router := NewRouter(app)
	device := enrollDeviceFor(t, app, router, admin, "notebook", testPublicKey)

	ip := device.AllowedIP
	if len(ip) > 3 && ip[len(ip)-3:] == "/32" {
		ip = ip[:len(ip)-3]
	}
	const key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIE5x xvpn-notebook"

	rec := doJSONFrom(t, router, http.MethodPost, "/api/me/ssh-key",
		registerSSHKeyRequest{PublicKey: key}, ip+":12345", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("primeiro registro: %d %s", rec.Code, rec.Body.String())
	}
	var first registerSSHKeyResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &first)
	if !first.Changed {
		t.Fatal("primeira chamada deveria reportar changed=true")
	}
	if first.SFTPEnabled {
		t.Fatal("SFTP ainda está desligado — resposta não podia dizer o contrário")
	}
	if len(fp.calls) != 0 {
		t.Fatalf("SFTP off: não podia chamar o provisionador, calls=%v", fp.calls)
	}

	// Idempotência: mesma chave de novo.
	rec = doJSONFrom(t, router, http.MethodPost, "/api/me/ssh-key",
		registerSSHKeyRequest{PublicKey: key}, ip+":12345", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("segundo registro: %d %s", rec.Code, rec.Body.String())
	}
	var second registerSSHKeyResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &second)
	if second.Changed {
		t.Fatal("segunda chamada com a mesma chave deveria ser no-op")
	}
	if len(fp.calls) != 0 {
		t.Fatalf("no-op ainda não podia chamar o provisionador, calls=%v", fp.calls)
	}

	var stored store.Device
	if err := app.Store.DB.First(&stored, device.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.SSHPublicKey != key {
		t.Errorf("chave no DB = %q", stored.SSHPublicKey)
	}
	if stored.SSHKeyUpdatedAt == nil || stored.SSHKeyUpdatedAt.IsZero() {
		t.Error("SSHKeyUpdatedAt deveria ter sido preenchido")
	}
}

func TestRegisterDeviceSSHKey_AppliesWhenSFTPOn(t *testing.T) {
	fp := &fakeUserProvisioner{}
	app, _ := withProvisioner(t, fp)
	admin := createTestUser(t, app, "admin", "senha-admin-123")
	app.Store.DB.Model(&admin).Updates(map[string]any{"sftp_enabled": true})
	router := NewRouter(app)
	device := enrollDeviceFor(t, app, router, admin, "notebook", testPublicKey)

	ip := device.AllowedIP
	if len(ip) > 3 && ip[len(ip)-3:] == "/32" {
		ip = ip[:len(ip)-3]
	}
	const key = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIE5x xvpn-notebook"

	rec := doJSONFrom(t, router, http.MethodPost, "/api/me/ssh-key",
		registerSSHKeyRequest{PublicKey: key}, ip+":12345", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("registro: %d %s", rec.Code, rec.Body.String())
	}
	if len(fp.calls) != 1 || fp.calls[0] != "EnableSFTP(admin,"+key+")" {
		t.Fatalf("esperava EnableSFTP com a chave do device, obtido %v", fp.calls)
	}
}

func TestReconcileUnixAccounts_PreservesDeviceKeyUnion(t *testing.T) {
	fp := &fakeUserProvisioner{}
	app, _ := withProvisioner(t, fp)
	alice := createTestUserWithRole(t, app, "alice", "senha-alice-123", store.RoleMember)
	app.Store.DB.Model(&alice).Updates(map[string]any{
		"sftp_enabled":   true,
		"ssh_public_key": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 manual",
	})
	now := time.Now()
	if err := app.Store.DB.Create(&store.Device{
		UserID:          alice.ID,
		Name:            "notebook",
		PublicKey:       testPublicKey,
		AllowedIP:       "10.66.66.9/32",
		SSHPublicKey:    "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA device",
		SSHKeyUpdatedAt: &now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	if err := app.ReconcileUnixAccounts(context.Background()); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if len(fp.calls) != 1 {
		t.Fatalf("esperava 1 call, obtido %v", fp.calls)
	}
	want := "EnableSFTP(alice,ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA device\nssh-ed25519 AAAAC3NzaC1lZDI1NTE5 manual)"
	if fp.calls[0] != want {
		t.Errorf("call = %q\nwant = %q", fp.calls[0], want)
	}
}
