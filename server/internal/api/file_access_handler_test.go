package api

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

// fakeUserProvisioner registra as chamadas feitas a cada método, pra
// o teste poder afirmar exatamente qual subcomando do binário foi
// invocado e em que ordem. Pode ser programado pra falhar num método
// específico (failOn) pra testar consistência DB↔sistema.
type fakeUserProvisioner struct {
	calls  []string
	failOn string
	err    error
}

func (f *fakeUserProvisioner) record(call string) error {
	if f.failOn != "" && strings.Contains(call, f.failOn) {
		return f.err
	}
	f.calls = append(f.calls, call)
	return nil
}

func (f *fakeUserProvisioner) Create(_ context.Context, username string) error {
	return f.record("Create(" + username + ")")
}
func (f *fakeUserProvisioner) EnableSFTP(_ context.Context, username, key string) error {
	return f.record("EnableSFTP(" + username + "," + key + ")")
}
func (f *fakeUserProvisioner) EnableSamba(_ context.Context, username string) error {
	return f.record("EnableSamba(" + username + ")")
}
func (f *fakeUserProvisioner) DisableSFTP(_ context.Context, username string) error {
	return f.record("DisableSFTP(" + username + ")")
}
func (f *fakeUserProvisioner) DisableSamba(_ context.Context, username string) error {
	return f.record("DisableSamba(" + username + ")")
}
func (f *fakeUserProvisioner) Disable(_ context.Context, username string) error {
	return f.record("Disable(" + username + ")")
}

// withProvisioner retorna um App de teste com o provisioner fake injetado.
// Reuso do newTestApp existente, só sobrescrevendo o campo novo.
func withProvisioner(t *testing.T, fp *fakeUserProvisioner) (*App, *fakePeerManager) {
	t.Helper()
	app, wg := newTestApp(t)
	app.UserProvisioner = fp
	return app, wg
}

func TestValidSSHPublicKey(t *testing.T) {
	cases := []struct {
		key  string
		want bool
	}{
		{"", true},
		{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIE5x alice@host", true},
		{"ssh-rsa AAAAC3NzaC1lZDI1NTE5AAAA alice", true},
		{"# comentário\nssh-ed25519 AAAAC3NzaC1lZDI1NTE5 alice", true},
		{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 alice\nssh-rsa AAAAC3NzaC1lZDI1NTE5AAAA bob", true},
		{"ssh-ed25519", false},      // sem base64
		{"bogus AAAA alice", false}, // tipo desconhecido
		{"ssh-ed25519 " + strings.Repeat("A", 9000) + " alice", false}, // base64 absurdo (>8192)
	}
	for _, c := range cases {
		if got := validSSHPublicKey(c.key); got != c.want {
			t.Errorf("validSSHPublicKey(%q) = %v, want %v", c.key, got, c.want)
		}
	}
}

func TestHandleSetFileAccess_EnableSFTPRequiresKey(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	admin := createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doJSON(t, router, http.MethodPut, "/api/users/"+uidStr(admin)+"/file-access",
		fileAccessRequest{SFTPEnabled: true}, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperava 400 (SFTP sem chave), obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleSetFileAccess_EnableSFTPAndSamba(t *testing.T) {
	fp := &fakeUserProvisioner{}
	app, _ := withProvisioner(t, fp)
	router := NewRouter(app)
	admin := createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")
	key := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIE5x alice@host"

	rec := doJSON(t, router, http.MethodPut, "/api/users/"+uidStr(admin)+"/file-access",
		fileAccessRequest{SFTPEnabled: true, SambaEnabled: true, SSHPublicKey: key}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperava 200, obtido %d: %s", rec.Code, rec.Body.String())
	}
	// EnableSFTP e EnableSamba foram chamados (ordem: desligar antes de
	// ligar, mas aqui nada estava ligado, então só os dois enables).
	wantCalls := []string{
		"EnableSFTP(admin," + key + ")",
		"EnableSamba(admin)",
	}
	if len(fp.calls) != len(wantCalls) {
		t.Fatalf("esperava %d calls, obtido %d: %v", len(wantCalls), len(fp.calls), fp.calls)
	}
	for i, w := range wantCalls {
		if fp.calls[i] != w {
			t.Errorf("call[%d] = %q, want %q", i, fp.calls[i], w)
		}
	}
	// DB reflete o estado aplicado.
	var u store.User
	if err := app.Store.DB.First(&u, admin.ID).Error; err != nil {
		t.Fatal(err)
	}
	if !u.SFTPEnabled || !u.SambaEnabled || u.SSHPublicKey != key {
		t.Errorf("DB não reflete o estado aplicado: %+v", u)
	}
}

func TestHandleSetFileAccess_DisableBothCallsDisableOnce(t *testing.T) {
	fp := &fakeUserProvisioner{}
	app, _ := withProvisioner(t, fp)
	router := NewRouter(app)
	admin := createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")
	key := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 alice"

	// Primeiro liga os dois.
	rec := doJSON(t, router, http.MethodPut, "/api/users/"+uidStr(admin)+"/file-access",
		fileAccessRequest{SFTPEnabled: true, SambaEnabled: true, SSHPublicKey: key}, token)
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	fp.calls = nil // reseta pra medir só o segundo PUT

	// Agora desliga os dois → uma chamada Disable só.
	rec = doJSON(t, router, http.MethodPut, "/api/users/"+uidStr(admin)+"/file-access",
		fileAccessRequest{}, token)
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	if len(fp.calls) != 1 || fp.calls[0] != "Disable(admin)" {
		t.Fatalf("esperava 1 chamada Disable(admin), obtido %v", fp.calls)
	}
	var u store.User
	if err := app.Store.DB.First(&u, admin.ID).Error; err != nil {
		t.Fatal(err)
	}
	if u.SFTPEnabled || u.SambaEnabled || u.SSHPublicKey != "" {
		t.Errorf("DB deveria estar tudo off/vazio: %+v", u)
	}
}

func TestHandleSetFileAccess_ToggleSFTPOffKeepsSamba(t *testing.T) {
	fp := &fakeUserProvisioner{}
	app, _ := withProvisioner(t, fp)
	router := NewRouter(app)
	admin := createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")
	key := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 alice"

	// Liga ambos.
	_ = doJSON(t, router, http.MethodPut, "/api/users/"+uidStr(admin)+"/file-access",
		fileAccessRequest{SFTPEnabled: true, SambaEnabled: true, SSHPublicKey: key}, token)
	fp.calls = nil

	// Desliga SFTP, mantém Samba → DisableSFTP só (não toca Samba).
	rec := doJSON(t, router, http.MethodPut, "/api/users/"+uidStr(admin)+"/file-access",
		fileAccessRequest{SambaEnabled: true}, token)
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	if len(fp.calls) != 1 || fp.calls[0] != "DisableSFTP(admin)" {
		t.Fatalf("esperava só DisableSFTP, obtido %v", fp.calls)
	}
	var u store.User
	if err := app.Store.DB.First(&u, admin.ID).Error; err != nil {
		t.Fatal(err)
	}
	if u.SFTPEnabled {
		t.Error("SFTP deveria estar off no DB")
	}
	if !u.SambaEnabled {
		t.Error("Samba deveria estar on no DB")
	}
}

func TestHandleSetFileAccess_KeyUpdateReappliesEnableSFTP(t *testing.T) {
	fp := &fakeUserProvisioner{}
	app, _ := withProvisioner(t, fp)
	router := NewRouter(app)
	admin := createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")
	oldKey := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAold alice@old"
	newKey := "ssh-ed25519 C3NzaC1lZDI1NTE5AAAAnew alice@new"

	// Liga SFTP com oldKey.
	_ = doJSON(t, router, http.MethodPut, "/api/users/"+uidStr(admin)+"/file-access",
		fileAccessRequest{SFTPEnabled: true, SSHPublicKey: oldKey}, token)
	fp.calls = nil

	// Atualiza a chave (SFTP continua on) → re-aplica EnableSFTP.
	rec := doJSON(t, router, http.MethodPut, "/api/users/"+uidStr(admin)+"/file-access",
		fileAccessRequest{SFTPEnabled: true, SSHPublicKey: newKey}, token)
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	if len(fp.calls) != 1 || fp.calls[0] != "EnableSFTP(admin,"+newKey+")" {
		t.Fatalf("esperava EnableSFTP com a chave nova, obtido %v", fp.calls)
	}
	var u store.User
	if err := app.Store.DB.First(&u, admin.ID).Error; err != nil {
		t.Fatal(err)
	}
	if u.SSHPublicKey != newKey {
		t.Errorf("DB deveria ter a chave nova, tem %q", u.SSHPublicKey)
	}
}

func TestHandleSetFileAccess_ProvisionerFailureKeepsDBConsistent(t *testing.T) {
	fp := &fakeUserProvisioner{failOn: "EnableSFTP", err: errors.New("sshd -t falhou")}
	app, _ := withProvisioner(t, fp)
	router := NewRouter(app)
	admin := createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	// EnableSFTP falha → 500 e DB NÃO marca SFTP como ligado.
	rec := doJSON(t, router, http.MethodPut, "/api/users/"+uidStr(admin)+"/file-access",
		fileAccessRequest{SFTPEnabled: true, SSHPublicKey: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 alice"}, token)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("esperava 500, obtido %d: %s", rec.Code, rec.Body.String())
	}
	var u store.User
	if err := app.Store.DB.First(&u, admin.ID).Error; err != nil {
		t.Fatal(err)
	}
	if u.SFTPEnabled {
		t.Error("DB não deveria marcar SFTP como ligado quando o provisionador falhou")
	}
}

func TestHandleSetFileAccess_NilProvisionerReturns503(t *testing.T) {
	app, _ := newTestApp(t) // UserProvisioner fica nil
	router := NewRouter(app)
	admin := createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doJSON(t, router, http.MethodPut, "/api/users/"+uidStr(admin)+"/file-access",
		fileAccessRequest{SambaEnabled: true}, token)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("esperava 503 com provisioner nil, obtido %d", rec.Code)
	}
}

func TestHandleSetFileAccess_RBAC(t *testing.T) {
	// member não pode gerenciar acesso a arquivos de ninguém (nem de si).
	app, _ := withProvisioner(t, &fakeUserProvisioner{})
	router := NewRouter(app)
	member := createTestUserWithRole(t, app, "member", "senha-member-123", store.RoleMember)
	memberToken := loginAndGetToken(t, app, router, "member", "senha-member-123")

	rec := doJSON(t, router, http.MethodPut, "/api/users/"+uidStr(member)+"/file-access",
		fileAccessRequest{SambaEnabled: true}, memberToken)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("esperava 403 (member não pode), obtido %d", rec.Code)
	}
}

func TestHandleSetFileAccess_InvalidKeyRejected(t *testing.T) {
	app, _ := withProvisioner(t, &fakeUserProvisioner{})
	router := NewRouter(app)
	admin := createTestUserWithRole(t, app, "admin", "senha-admin-123", store.RoleAdmin)
	token := loginAndGetToken(t, app, router, "admin", "senha-admin-123")

	rec := doJSON(t, router, http.MethodPut, "/api/users/"+uidStr(admin)+"/file-access",
		fileAccessRequest{SFTPEnabled: true, SSHPublicKey: "bogus AAAA alice"}, token)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("esperava 400 (chave malformada), obtido %d", rec.Code)
	}
}

// itoa evita importar strconv só pra isso aqui.
func itoa(n uint) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// uidStr é helper pra formatar o ID de um store.User pra string (pra
// montar path params nas rotas). Evita importar strconv no teste.
func uidStr(u store.User) string { return itoa(u.ID) }
