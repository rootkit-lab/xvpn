package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestGitSmartHTTPHostAndAuth(t *testing.T) {
	app, router, adminTok := setupGitApp(t)

	rec := doJSONHost(t, router, http.MethodGet, "/lab/info/refs?service=git-upload-pack", nil, adminTok, "xvpn.ihuull.com")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("host público deveria 404, veio %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSONHost(t, router, http.MethodGet, "/lab/info/refs?service=git-upload-pack", nil, "", "xgit.corp.ihuull.com")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("sem credencial deveria 401, veio %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("WWW-Authenticate"), `Basic realm="xgit"`) {
		t.Fatalf("WWW-Authenticate ausente: %q", rec.Header().Get("WWW-Authenticate"))
	}

	rec = doGitBasic(t, router, http.MethodGet, "/lab/info/refs?service=git-upload-pack", nil, "admin", adminTok)
	if rec.Code != http.StatusOK && rec.Code != http.StatusNotFound {
		t.Fatalf("basic JWE: %d %s", rec.Code, rec.Body.String())
	}

	rec = doGitBasic(t, router, http.MethodGet, "/lab/info/refs?service=git-upload-pack", nil, "outro", adminTok)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("user Basic divergente deveria 401, veio %d", rec.Code)
	}

	rec = doGitBasic(t, router, http.MethodGet, "/lab/info/refs?service=git-upload-pack", nil, "admin", "senha-admin-ok")
	if rec.Code != http.StatusOK && rec.Code != http.StatusNotFound {
		t.Fatalf("basic senha da conta: %d %s", rec.Code, rec.Body.String())
	}

	rec = doGitBasic(t, router, http.MethodGet, "/lab/info/refs?service=git-upload-pack", nil, "admin", "senha-errada")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("senha errada deveria 401, veio %d", rec.Code)
	}

	_ = app
}

func TestProjectGitAPIAndProtectedBranches(t *testing.T) {
	app, router, adminTok := setupGitApp(t)
	createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	aliceTok := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Slug: "lab", Name: "Lab"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects/lab/git", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("git get: %d %s", rec.Code, rec.Body.String())
	}
	var git projectGitResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &git); err != nil {
		t.Fatal(err)
	}
	if git.CloneURL != "https://xgit.corp.ihuull.com/lab" {
		t.Fatalf("clone_url: %q", git.CloneURL)
	}
	if len(git.ProtectedBranches) < 2 {
		t.Fatalf("defaults: %+v", git.ProtectedBranches)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects/lab/git", nil, aliceTok)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("member alheio deveria 404, veio %d", rec.Code)
	}

	rec = doJSON(t, router, http.MethodPut, "/api/projects/lab/protected-branches", setProtectedBranchesRequest{
		Branches: []protectedBranchJSON{{Pattern: "main", MinPushRole: store.ProjectRoleMaintainer}},
	}, aliceTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("member não escreve protect: %d", rec.Code)
	}

	rec = doJSON(t, router, http.MethodPut, "/api/projects/lab/protected-branches", setProtectedBranchesRequest{
		Branches: []protectedBranchJSON{
			{Pattern: "main", MinPushRole: store.ProjectRoleMaintainer},
			{Pattern: "release/*", MinPushRole: store.ProjectRoleMaintainer},
		},
	}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("protect: %d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &git); err != nil {
		t.Fatal(err)
	}
	if len(git.ProtectedBranches) != 2 || git.ProtectedBranches[1].Pattern != "release/*" {
		t.Fatalf("protect resp: %+v", git.ProtectedBranches)
	}

	rec = doJSON(t, router, http.MethodPut, "/api/projects/lab/protected-branches", setProtectedBranchesRequest{
		Branches: []protectedBranchJSON{{Pattern: "../etc", MinPushRole: store.ProjectRoleMaintainer}},
	}, adminTok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("padrão perigoso deveria 400, veio %d", rec.Code)
	}
}

func TestGitPushHonorsRolesAndProtectedBranch(t *testing.T) {
	app, router, adminTok := setupGitApp(t)
	dev := createTestUserWithRole(t, app, "dev", "senha-dev-ok-1", store.RoleMember)
	guest := createTestUserWithRole(t, app, "guest", "senha-guest-ok", store.RoleMember)
	devTok := loginAndGetToken(t, app, router, "dev", "senha-dev-ok-1")
	guestTok := loginAndGetToken(t, app, router, "guest", "senha-guest-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Slug: "lab", Name: "Lab"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created projectResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	rec = doJSON(t, router, http.MethodPut, "/api/projects/lab/members", setProjectMembersRequest{
		Members: []projectMemberIn{
			{UserID: created.Members[0].UserID, Role: store.ProjectRoleOwner},
			{UserID: dev.ID, Role: store.ProjectRoleDeveloper},
			{UserID: guest.ID, Role: store.ProjectRoleGuest},
		},
	}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("members: %d %s", rec.Code, rec.Body.String())
	}

	if _, err := exec.LookPath("git"); err == nil && app.Config.GitDir != "" {
		rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/git", nil, adminTok)
		if rec.Code != http.StatusOK {
			t.Fatalf("init: %d %s", rec.Code, rec.Body.String())
		}
		if !forge.Exists(app.Config.GitDir, "lab") {
			t.Fatal("bare repo deveria existir")
		}
	}

	pack := receivePackBody("refs/heads/main")
	rec = doGitBasic(t, router, http.MethodPost, "/lab/git-receive-pack", pack, "guest", guestTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("guest push: %d %s", rec.Code, rec.Body.String())
	}

	rec = doGitBasic(t, router, http.MethodPost, "/lab/git-receive-pack", pack, "dev", devTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("developer em main deveria 403, veio %d %s", rec.Code, rec.Body.String())
	}

	feature := receivePackBody("refs/heads/feature")
	rec = doGitBasic(t, router, http.MethodPost, "/lab/git-receive-pack", feature, "dev", devTok)
	if rec.Code == http.StatusForbidden {
		t.Fatalf("developer em feature não deveria ser bloqueado por protected: %s", rec.Body.String())
	}

	rec = doGitBasic(t, router, http.MethodPost, "/lab/git-receive-pack", pack, "admin", adminTok)
	if rec.Code == http.StatusForbidden {
		t.Fatalf("admin/forge não deveria levar 403 de protected: %s", rec.Body.String())
	}
}

func TestCodespaceGitTokenScopedToProject(t *testing.T) {
	app, router, adminTok := setupGitApp(t)
	var admin store.User
	if err := app.Store.DB.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"lab", "other"} {
		rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Slug: slug, Name: slug}, adminTok)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create %s: %d %s", slug, rec.Code, rec.Body.String())
		}
		seedProjectBranches(t, app.Config.GitDir, slug)
	}
	var lab, other store.Project
	if err := app.Store.DB.Where("slug = ?", "lab").First(&lab).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.Store.DB.Where("slug = ?", "other").First(&other).Error; err != nil {
		t.Fatal(err)
	}
	tok := "tokentokentoken1"
	cs := store.CodeSpace{
		PublicID:     "aabbccddeeff",
		UserID:       admin.ID,
		ProjectID:    lab.ID,
		Branch:       "main",
		RelPath:      "admin/lab/aabbccddeeff",
		Kind:         store.CodespaceKindRemote,
		Status:       store.CodespaceRunning,
		GitTokenHash: hashCodespaceToken(tok),
	}
	if err := app.Store.DB.Create(&cs).Error; err != nil {
		t.Fatal(err)
	}

	rec := doGitBasic(t, router, http.MethodGet, "/lab/info/refs?service=git-upload-pack", nil, "codespace-aabbccddeeff", tok)
	if rec.Code == http.StatusUnauthorized || rec.Code == http.StatusNotFound {
		t.Fatalf("token do codespace deveria ler o próprio repo: %d %s", rec.Code, rec.Body.String())
	}
	rec = doGitBasic(t, router, http.MethodGet, "/other/info/refs?service=git-upload-pack", nil, "codespace-aabbccddeeff", tok)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("token não pode autenticar outro repo: %d %s", rec.Code, rec.Body.String())
	}

	if err := app.Store.DB.Model(&cs).Updates(map[string]any{"status": store.CodespaceStopped, "git_token_hash": ""}).Error; err != nil {
		t.Fatal(err)
	}
	rec = doGitBasic(t, router, http.MethodGet, "/lab/info/refs?service=git-upload-pack", nil, "codespace-aabbccddeeff", tok)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("token após stop deveria falhar: %d", rec.Code)
	}
}

func setupGitApp(t *testing.T) (*App, http.Handler, string) {
	t.Helper()
	app, _ := newTestApp(t)
	app.Config.GitDir = t.TempDir()
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	router := NewRouter(app)
	return app, router, loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
}

func doGitBasic(t *testing.T, router http.Handler, method, path string, body []byte, user, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(string(body)))
	req.Host = "xgit.corp.ihuull.com"
	req.SetBasicAuth(user, token)
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/x-git-receive-pack-request")
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func receivePackBody(ref string) []byte {
	old := strings.Repeat("a", 40)
	newSHA := strings.Repeat("b", 40)
	line := old + " " + newSHA + " " + ref + "\n"
	n := len(line) + 4
	hdr := [4]byte{}
	const hexdigits = "0123456789abcdef"
	size := n
	for i := 3; i >= 0; i-- {
		hdr[i] = hexdigits[size&0xf]
		size >>= 4
	}
	return []byte(string(hdr[:]) + line + "0000")
}
