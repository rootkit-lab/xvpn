package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestHandleUpdateMySSHPublicKey(t *testing.T) {
	fp := &fakeUserProvisioner{}
	app, _ := withProvisioner(t, fp)
	member := createTestUserWithRole(t, app, "member1", "senha-membro-123", store.RoleMember)
	member.SFTPEnabled = true
	if err := app.Store.DB.Save(&member).Error; err != nil {
		t.Fatal(err)
	}
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "member1", "senha-membro-123")

	key := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIE5x alice@phone"
	rec := doJSON(t, router, http.MethodPut, "/api/me/ssh-public-key",
		updateMySSHPublicKeyRequest{SSHPublicKey: key}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d: %s", rec.Code, rec.Body.String())
	}
	var resp userResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.SSHPublicKey != key {
		t.Errorf("ssh_public_key=%q", resp.SSHPublicKey)
	}
	if len(fp.calls) == 0 || !stringsHasPrefix(fp.calls[0], "EnableSFTP(member1,") {
		t.Errorf("esperava EnableSFTP com SFTP ligado, calls=%v", fp.calls)
	}
}

func TestHandleUpdateMySSHPublicKey_NoSFTPSkipsProvisioner(t *testing.T) {
	fp := &fakeUserProvisioner{}
	app, _ := withProvisioner(t, fp)
	_ = createTestUserWithRole(t, app, "member2", "senha-membro-123", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "member2", "senha-membro-123")

	key := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIE5x bob@phone"
	rec := doJSON(t, router, http.MethodPut, "/api/me/ssh-public-key",
		updateMySSHPublicKeyRequest{SSHPublicKey: key}, token)
	if rec.Code != http.StatusOK {
		t.Fatalf("esperado 200, obtido %d: %s", rec.Code, rec.Body.String())
	}
	if len(fp.calls) != 0 {
		t.Errorf("SFTP off não deveria chamar provisioner, calls=%v", fp.calls)
	}
}

func stringsHasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
