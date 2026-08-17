package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestCodespaceRuntimeHost(t *testing.T) {
	if got := codespaceRuntimeHost("cs-aabbccddeeff.corp.ihuull.com"); got != "aabbccddeeff" {
		t.Fatalf("got %q", got)
	}
	if codespaceRuntimeHost("evil.xcodespaces.corp.ihuull.com") != "" {
		t.Fatal("dois rótulos não podem casar")
	}
	if codespaceRuntimeHost("xcodespaces.corp.ihuull.com") != "" {
		t.Fatal("catálogo não é runtime")
	}
}

func TestCodespaceLifecycle(t *testing.T) {
	app, router, adminTok := setupGitApp(t)
	app.Config.CodespacesDir = t.TempDir()
	if err := store.SeedXcodespacesApp(app.Store.DB); err != nil {
		t.Fatal(err)
	}

	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Slug: "lab", Name: "Lab"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	seedProjectBranches(t, app.Config.GitDir, "lab")

	rec = doJSON(t, router, http.MethodPost, "/api/xcodespaces", createCodespaceRequest{Slug: "lab", Branch: "main"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create cs: %d %s", rec.Code, rec.Body.String())
	}
	var cs codespaceJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &cs); err != nil {
		t.Fatal(err)
	}
	if cs.ID == "" || cs.Slug != "lab" || cs.Branch != "main" {
		t.Fatalf("cs: %+v", cs)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/xcodespaces?slug=lab", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d", rec.Code)
	}
	var listed struct {
		Items []codespaceJSON `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 1 {
		t.Fatalf("list: %+v", listed.Items)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/xcodespaces/"+cs.ID+"/tree", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("tree: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPut, "/api/xcodespaces/"+cs.ID+"/contents", writeCodespaceRequest{
		Path: "hello.txt", Content: "oi",
	}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("write: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/xcodespaces/"+cs.ID+"/commit", commitCodespaceRequest{
		Message: "add hello",
	}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("commit: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodDelete, "/api/xcodespaces/"+cs.ID, nil, adminTok)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
}

func TestCodespaceRejectsGitMetadataAndSymlinks(t *testing.T) {
	app, router, adminTok := setupGitApp(t)
	app.Config.CodespacesDir = t.TempDir()
	if err := store.SeedXcodespacesApp(app.Store.DB); err != nil {
		t.Fatal(err)
	}

	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Slug: "lab", Name: "Lab"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	seedProjectBranches(t, app.Config.GitDir, "lab")

	rec = doJSON(t, router, http.MethodPost, "/api/xcodespaces", createCodespaceRequest{Slug: "lab", Branch: "main"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create cs: %d %s", rec.Code, rec.Body.String())
	}
	var cs codespaceJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &cs); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{".git", ".git/config", "nested/.git/config"} {
		rec = doJSON(t, router, http.MethodPut, "/api/xcodespaces/"+cs.ID+"/contents", writeCodespaceRequest{
			Path: path, Content: "gitdir: /tmp/evil",
		}, adminTok)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("write %s: %d %s", path, rec.Code, rec.Body.String())
		}
	}

	root := filepath.Join(app.Config.CodespacesDir, "admin", "lab", cs.ID)
	if err := os.Symlink("/etc", filepath.Join(root, "etc-link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/etc/passwd", filepath.Join(root, "passwd-link")); err != nil {
		t.Fatal(err)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/xcodespaces/"+cs.ID+"/tree?path=etc-link", nil, adminTok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("tree symlink: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodGet, "/api/xcodespaces/"+cs.ID+"/blob?path=passwd-link", nil, adminTok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("blob symlink: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodPut, "/api/xcodespaces/"+cs.ID+"/contents", writeCodespaceRequest{
		Path: "passwd-link", Content: "pwn",
	}, adminTok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("write symlink: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/xcodespaces/"+cs.ID+"/tree", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("tree root: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "etc-link") || strings.Contains(rec.Body.String(), "passwd-link") {
		t.Fatalf("tree leaked symlink: %s", rec.Body.String())
	}
}

func TestCodespaceRemoteCreateUsesHelper(t *testing.T) {
	fp := &fakeUserProvisioner{}
	app, _ := withProvisioner(t, fp)
	app.Config.CodespacesDir = t.TempDir()
	app.Config.GitDir = t.TempDir()
	if err := store.SeedXcodespacesApp(app.Store.DB); err != nil {
		t.Fatal(err)
	}
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	router := NewRouter(app)
	adminTok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Slug: "lab", Name: "Lab"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	seedProjectBranches(t, app.Config.GitDir, "lab")

	rec = doJSON(t, router, http.MethodPost, "/api/xcodespaces", createCodespaceRequest{Slug: "lab", Branch: "main", Kind: "remote"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create remote: %d %s", rec.Code, rec.Body.String())
	}
	var cs codespaceJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &cs); err != nil {
		t.Fatal(err)
	}
	if cs.Kind != store.CodespaceKindRemote || cs.Status != store.CodespaceRunning {
		t.Fatalf("cs: %+v", cs)
	}
	if !strings.HasPrefix(cs.RuntimeURL, "https://cs-") {
		t.Fatalf("runtime: %s", cs.RuntimeURL)
	}
	joined := strings.Join(fp.calls, "\n")
	if !strings.Contains(joined, "ApplyCodespace(") {
		t.Fatalf("helper não chamado: %v", fp.calls)
	}
	if strings.Contains(joined, "worktree") {
		t.Fatal("remote não pode usar worktree")
	}

	rec = doJSON(t, router, http.MethodPost, "/api/xcodespaces/"+cs.ID+"/stop", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("stop: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodDelete, "/api/xcodespaces/"+cs.ID, nil, adminTok)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d", rec.Code)
	}
}

func TestCodespaceHostDoesNotExposeControlAPI(t *testing.T) {
	_, router, adminTok := setupGitApp(t)
	host := "cs-aabbccddeeff.corp.ihuull.com"

	rec := doJSONHost(t, router, http.MethodGet, "/api/xcodespaces", nil, adminTok, host)
	if rec.Code == http.StatusOK && strings.Contains(rec.Body.String(), `"items"`) {
		t.Fatalf("API de controle vazou no host cs-*: %s", rec.Body.String())
	}
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("host cs-* /api/xcodespaces: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSONHost(t, router, http.MethodGet, "/api/auth/redeem", nil, "", host)
	if rec.Code == http.StatusNotFound || rec.Code == http.StatusServiceUnavailable {
		t.Fatalf("SSO redeem no cs-* deve chegar ao Gin: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPrepareCodespaceUpstreamStripsSession(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:19000/?tkn=old", nil)
	req.Header.Set("Cookie", "ihuull_session=secreto; vscode-session=keep")
	req.Header.Set("Authorization", "Bearer jwe")
	prepareCodespaceUpstream(req, "tokentokentoken1")
	if req.Header.Get("Authorization") != "" {
		t.Fatal("authorization não pode ir ao IDE")
	}
	if strings.Contains(req.Header.Get("Cookie"), "ihuull_session") {
		t.Fatal("cookie de sessão não pode ir ao IDE")
	}
	if !strings.Contains(req.Header.Get("Cookie"), "vscode-session=keep") {
		t.Fatal("cookie do IDE foi descartado")
	}
	if !strings.Contains(req.Header.Get("Cookie"), "vscode-tkn=tokentokentoken1") {
		t.Fatal("vscode-tkn ausente")
	}
	if req.Header.Get("X-Connection-Token") != "tokentokentoken1" {
		t.Fatal("connection token ausente")
	}
	if req.URL.Query().Get("tkn") != "" {
		t.Fatal("tkn na query causa redirect loop no openvscode")
	}
}

func TestFilterCodespaceSetCookie(t *testing.T) {
	resp := &http.Response{Header: make(http.Header)}
	resp.Header.Add("Set-Cookie", "vscode-tkn=abc; Path=/")
	resp.Header.Add("Set-Cookie", "ihuull_session=jwe; Path=/")
	resp.Header.Add("Set-Cookie", "evil=1; Path=/")
	if err := filterCodespaceSetCookie(resp); err != nil {
		t.Fatal(err)
	}
	got := strings.Join(resp.Header.Values("Set-Cookie"), "\n")
	if !strings.Contains(got, "vscode-tkn=") {
		t.Fatalf("vscode-tkn sumiu: %s", got)
	}
	if strings.Contains(got, "ihuull_session") || strings.Contains(got, "evil=") {
		t.Fatalf("cookie estranho vazou: %s", got)
	}
}
