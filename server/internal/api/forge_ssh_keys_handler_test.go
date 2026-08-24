package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func testMemberToken(t *testing.T, router http.Handler, username, password string) string {
	t.Helper()
	rec := doJSON(t, router, http.MethodPost, "/api/auth/login", loginRequest{Username: username, Password: password}, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("login: %d %s", rec.Code, rec.Body.String())
	}
	var resp loginResponse
	decodeJSON(t, rec, &resp)
	return resp.Token
}

func decodeJSON(t *testing.T, rec *httptest.ResponseRecorder, out any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

func TestForgeSSHKeys_CRUDAndRender(t *testing.T) {
	fp := &fakeUserProvisioner{}
	app, _ := withProvisioner(t, fp)
	router := NewRouter(app)

	_ = createTestUserWithRole(t, app, "dev", "pass-dev", store.RoleMember)
	tok := testMemberToken(t, router, "dev", "pass-dev")

	key := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIGZvcmdlLXRlc3Qta2V5IGZvcmdlLXRlc3Q="

	rec := doJSON(t, router, http.MethodPost, "/api/me/forge-ssh-keys",
		createForgeSSHKeyRequest{Title: "laptop", PublicKey: key}, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/me/forge-ssh-keys", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "laptop") {
		t.Fatalf("expected laptop in list: %s", rec.Body.String())
	}

	content, err := app.renderForgeAuthorizedKeys()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(content, "xvpn-git-shell") || !strings.Contains(content, "dev") {
		t.Fatalf("render missing shell/user: %q", content)
	}
	if len(fp.calls) == 0 || !strings.Contains(fp.calls[len(fp.calls)-1], "ApplyGitSSHKeys") {
		t.Fatalf("expected ApplyGitSSHKeys call, got %v", fp.calls)
	}

	var row store.ForgeSSHKey
	if err := app.Store.DB.Where("title = ?", "laptop").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	rec = doJSON(t, router, http.MethodDelete, "/api/me/forge-ssh-keys/"+strconv.FormatUint(uint64(row.ID), 10), nil, tok)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
}

func TestRegisterDeviceSSHKey_SyncsForgeKey(t *testing.T) {
	fp := &fakeUserProvisioner{}
	app, _ := withProvisioner(t, fp)
	admin := createTestUser(t, app, "admin", "senha-admin-123")
	router := NewRouter(app)
	device := enrollDeviceFor(t, app, router, admin, "notebook", testPublicKey)

	ip := device.AllowedIP
	if len(ip) > 3 && ip[len(ip)-3:] == "/32" {
		ip = ip[:len(ip)-3]
	}

	key := "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIHN5bmMtZm9yZ2Uta2V5IHN5bmM="
	rec := doJSONFrom(t, router, http.MethodPost, "/api/me/ssh-key",
		registerSSHKeyRequest{PublicKey: key}, ip+":12345", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("register: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"forge_registered":true`) {
		t.Fatalf("expected forge_registered true: %s", rec.Body.String())
	}

	var count int64
	if err := app.Store.DB.Model(&store.ForgeSSHKey{}).Where("user_id = ?", admin.ID).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("expected 1 forge key, got %d", count)
	}
}