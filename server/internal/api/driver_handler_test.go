package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func doJSONHost(t *testing.T, router http.Handler, method, path string, body any, token, host string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatal(err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Host = host
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestDriverListRequiresCorpHost(t *testing.T) {
	app, _ := newTestApp(t)
	shared := t.TempDir()
	app.Config.DriverSharedDir = shared
	app.Config.DriverHomeRoot = t.TempDir()
	if err := os.WriteFile(filepath.Join(shared, "readme.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")

	rec := doJSON(t, router, http.MethodGet, "/api/driver/ls?root=shared", nil, token)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("sem Host corp deveria 404, veio %d %s", rec.Code, rec.Body.String())
	}

	rec2 := doJSONHost(t, router, http.MethodGet, "/api/driver/ls?root=shared", nil, token, "xdriver.corp.ihuull.com")
	if rec2.Code != http.StatusOK {
		t.Fatalf("com Host corp: %d %s", rec2.Code, rec2.Body.String())
	}
	var body struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Items) != 1 || body.Items[0].Name != "readme.txt" {
		t.Fatalf("lista: %+v", body)
	}
}

func TestDriverProjectRootRequiresMembership(t *testing.T) {
	app, _ := newTestApp(t)
	app.Config.DriverSharedDir = t.TempDir()
	app.Config.DriverHomeRoot = t.TempDir()
	app.Config.DriverProjectsDir = t.TempDir()
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	createTestUserWithRole(t, app, "bob", "senha-bob-okxx", store.RoleMember)
	router := NewRouter(app)
	adminTok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	bobTok := loginAndGetToken(t, app, router, "bob", "senha-bob-okxx")

	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{
		Slug: "xchat", Name: "XCHAT", FilesEnabled: true,
	}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create project: %d %s", rec.Code, rec.Body.String())
	}
	if err := os.WriteFile(filepath.Join(app.Config.DriverProjectsDir, "xchat", "readme.md"), []byte("wiki"), 0o644); err != nil {
		t.Fatal(err)
	}

	rec = doJSONHost(t, router, http.MethodGet, "/api/driver/ls?root=project:xchat", nil, bobTok, "xdriver.corp.ihuull.com")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("não-membro deveria 404, veio %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSONHost(t, router, http.MethodGet, "/api/driver/ls?root=project:xchat", nil, adminTok, "xdriver.corp.ihuull.com")
	if rec.Code != http.StatusOK {
		t.Fatalf("owner/admin: %d %s", rec.Code, rec.Body.String())
	}
}

func TestDriverProjectHidesSlugFromStrangers(t *testing.T) {
	app, _ := newTestApp(t)
	app.Config.DriverSharedDir = t.TempDir()
	app.Config.DriverHomeRoot = t.TempDir()
	app.Config.DriverProjectsDir = t.TempDir()
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	createTestUserWithRole(t, app, "viewer", "senha-viewer-ok", store.RoleViewer)
	createTestUserWithRole(t, app, "guest", "senha-guest-okx", store.RoleMember)
	router := NewRouter(app)
	adminTok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	viewerTok := loginAndGetToken(t, app, router, "viewer", "senha-viewer-ok")
	guestTok := loginAndGetToken(t, app, router, "guest", "senha-guest-okx")

	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Slug: "lab", Name: "Lab"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created projectResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}

	rec = doJSONHost(t, router, http.MethodGet, "/api/driver/ls?root=project:lab", nil, viewerTok, "xdriver.corp.ihuull.com")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("viewer sem membership deveria 404 (não 403), veio %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPut, "/api/projects/lab/members", setProjectMembersRequest{
		Members: []projectMemberIn{
			{UserID: created.Members[0].UserID, Role: store.ProjectRoleOwner},
			{UserID: mustUserID(t, app, "guest"), Role: store.ProjectRoleGuest},
		},
	}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("members: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPatch, "/api/projects/lab", updateProjectRequest{FilesEnabled: boolPtr(true)}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("enable files: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSONHost(t, router, http.MethodGet, "/api/driver/ls?root=project:lab", nil, guestTok, "xdriver.corp.ihuull.com")
	if rec.Code != http.StatusOK {
		t.Fatalf("guest deveria listar, veio %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSONHost(t, router, http.MethodPost, "/api/driver/mkdir", map[string]string{
		"root": "project:lab", "path": "", "name": "wiki",
	}, guestTok, "xdriver.corp.ihuull.com")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("guest não deveria mkdir, veio %d %s", rec.Code, rec.Body.String())
	}
}

func mustUserID(t *testing.T, app *App, username string) uint {
	t.Helper()
	var u store.User
	if err := app.Store.DB.Where("username = ?", username).First(&u).Error; err != nil {
		t.Fatal(err)
	}
	return u.ID
}

func boolPtr(v bool) *bool { return &v }

func TestDriverRejectsEmptyDelete(t *testing.T) {
	app, _ := newTestApp(t)
	app.Config.DriverSharedDir = t.TempDir()
	app.Config.DriverHomeRoot = t.TempDir()
	createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")
	rec := doJSONHost(t, router, http.MethodDelete, "/api/driver/rm", map[string]string{"root": "shared", "path": ""}, token, "xdriver.corp.ihuull.com")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("delete vazio deveria 400, veio %d", rec.Code)
	}
}

func TestDriverWriteAndInlineDownload(t *testing.T) {
	app, _ := newTestApp(t)
	shared := t.TempDir()
	app.Config.DriverSharedDir = shared
	app.Config.DriverHomeRoot = t.TempDir()
	if err := os.WriteFile(filepath.Join(shared, "nota.txt"), []byte("velho"), 0o644); err != nil {
		t.Fatal(err)
	}
	createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")
	host := "xdriver.corp.ihuull.com"

	rec := doJSONHost(t, router, http.MethodPut, "/api/driver/write", map[string]string{
		"root": "shared", "path": "nota.txt", "content": "novo",
	}, token, host)
	if rec.Code != http.StatusOK {
		t.Fatalf("write: %d %s", rec.Code, rec.Body.String())
	}
	got, err := os.ReadFile(filepath.Join(shared, "nota.txt"))
	if err != nil || string(got) != "novo" {
		t.Fatalf("disco=%q err=%v", got, err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/driver/download?root=shared&path=nota.txt&inline=1", nil)
	req.Host = host
	req.Header.Set("Authorization", "Bearer "+token)
	dl := httptest.NewRecorder()
	router.ServeHTTP(dl, req)
	if dl.Code != http.StatusOK {
		t.Fatalf("download: %d", dl.Code)
	}
	if !strings.Contains(dl.Header().Get("Content-Disposition"), "inline") {
		t.Fatalf("disposition=%q", dl.Header().Get("Content-Disposition"))
	}
	if dl.Body.String() != "novo" {
		t.Fatalf("body=%q", dl.Body.String())
	}
	if dl.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("faltou nosniff")
	}
}

func TestDriverInlineHTMLIsNotExecutable(t *testing.T) {
	app, _ := newTestApp(t)
	shared := t.TempDir()
	app.Config.DriverSharedDir = shared
	app.Config.DriverHomeRoot = t.TempDir()
	if err := os.WriteFile(filepath.Join(shared, "x.html"), []byte("<script>alert(1)</script>"), 0o644); err != nil {
		t.Fatal(err)
	}
	createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")
	req := httptest.NewRequest(http.MethodGet, "/api/driver/download?root=shared&path=x.html&inline=1", nil)
	req.Host = "xdriver.corp.ihuull.com"
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("download: %d", rec.Code)
	}
	ct := rec.Header().Get("Content-Type")
	if strings.Contains(ct, "html") {
		t.Fatalf("HTML inline não pode ser text/html: %q", ct)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("faltou nosniff")
	}
}

func TestDriverExtractRejectsUnsupported(t *testing.T) {
	app, _ := newTestApp(t)
	app.Config.DriverSharedDir = t.TempDir()
	app.Config.DriverHomeRoot = t.TempDir()
	createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	router := NewRouter(app)
	token := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")
	rec := doJSONHost(t, router, http.MethodPost, "/api/driver/extract", map[string]string{
		"root": "shared", "path": "x.rar",
	}, token, "xdriver.corp.ihuull.com")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("rar deveria 400, veio %d %s", rec.Code, rec.Body.String())
	}
}
