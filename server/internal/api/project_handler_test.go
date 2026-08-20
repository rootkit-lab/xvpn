package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestProjectCRUDAndMembers(t *testing.T) {
	app, _ := newTestApp(t)
	app.Config.DriverProjectsDir = t.TempDir()
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	member := createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	router := NewRouter(app)
	adminTok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	aliceTok := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{
		Org: "xcorp", Slug: "xchat", Name: "XCHAT", Description: "messenger", FilesEnabled: true,
	}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created projectResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Slug != "xchat" || created.SocialGroupID == 0 || created.MemberCount != 1 {
		t.Fatalf("create resp: %+v", created)
	}
	if _, err := os.Stat(filepath.Join(app.Config.DriverProjectsDir, "xcorp", "xchat")); err != nil {
		t.Fatalf("dir do projeto: %v", err)
	}
	var group store.SocialGroup
	if err := app.Store.DB.First(&group, created.SocialGroupID).Error; err != nil {
		t.Fatalf("grupo XGROUP: %v", err)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects", nil, aliceTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("list member: %d", rec.Code)
	}
	var listed struct {
		Items []projectResponse `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 0 {
		t.Fatalf("member não deveria ver projeto alheio: %+v", listed.Items)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects/xcorp/xchat", nil, aliceTok)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("get alheio deveria 404, veio %d", rec.Code)
	}

	rec = doJSON(t, router, http.MethodPut, "/api/projects/xcorp/xchat/members", setProjectMembersRequest{
		Members: []projectMemberIn{
			{UserID: created.Members[0].UserID, Role: store.ProjectRoleOwner},
			{UserID: member.ID, Role: store.ProjectRoleDeveloper},
		},
	}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("members: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects", nil, aliceTok)
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 1 || listed.Items[0].Slug != "xchat" {
		t.Fatalf("member deveria ver o próprio projeto: %+v", listed.Items)
	}

	rec = doJSON(t, router, http.MethodPatch, "/api/projects/xcorp/xchat", updateProjectRequest{
		Network: ptrNetwork(store.AppNetworkVPN),
	}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/social/posts", createPostRequest{
		Body: "issue do xchat", ProjectSlug: strPtr("xchat"),
	}, aliceTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("post ligado: %d %s", rec.Code, rec.Body.String())
	}
	var post socialPostResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &post); err != nil {
		t.Fatal(err)
	}
	if post.ProjectSlug == nil || *post.ProjectSlug != "xchat" || post.SocialGroupID != created.SocialGroupID {
		t.Fatalf("post sem projeto: %+v", post)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/social/posts", createPostRequest{
		Body: "slug mentiroso", ProjectSlug: strPtr("nope"),
	}, aliceTok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("slug inválido deveria 400, veio %d", rec.Code)
	}
}

func TestAdminWithoutForgeScopeCannotCreateProject(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductCore})
	rec := doJSON(t, f.router, http.MethodPost, "/api/projects", createProjectRequest{Org: "xcorp", Slug: "lab", Name: "Lab"}, f.token)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("admin só-core não deveria criar projeto, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminWithForgeScopeCanCreateProject(t *testing.T) {
	f := setupScopedAdmin(t, []store.Product{store.ProductForge})
	rec := doJSON(t, f.router, http.MethodPost, "/api/projects", createProjectRequest{Org: "xcorp", Slug: "lab", Name: "Lab"}, f.token)
	if rec.Code != http.StatusCreated {
		t.Fatalf("admin forge deveria criar, obtido %d: %s", rec.Code, rec.Body.String())
	}
}

func TestEnsureProjectForAppOnSync(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "root", "senha-root-okxx", store.RoleSuperAdmin)
	market := store.App{
		Slug: "xchat", Name: "XCHAT", Visibility: store.AppVisibilityGlobal,
		Network: store.AppNetworkVPN, Kind: store.AppKindDesktop,
		Source: store.AppSourceBuild, SourcePath: "apps/xvpn-chat",
	}
	if err := app.Store.DB.Create(&market).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.ensureProjectForApp(market); err != nil {
		t.Fatal(err)
	}
	var proj store.Project
	if err := app.Store.DB.Where("slug = ?", "xchat").First(&proj).Error; err != nil {
		t.Fatalf("projeto do sync: %v", err)
	}
	if proj.AppID == nil || *proj.AppID != market.ID || proj.SocialGroupID == 0 {
		t.Fatalf("projeto incompleto: %+v", proj)
	}
	if err := app.ensureProjectForApp(market); err != nil {
		t.Fatal(err)
	}
	var n int64
	_ = app.Store.DB.Model(&store.Project{}).Where("slug = ?", "xchat").Count(&n).Error
	if n != 1 {
		t.Fatalf("ensure deveria ser idempotente, n=%d", n)
	}
}

func TestListProjectsScopeAllForbiddenForMember(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	router := NewRouter(app)
	adminTok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	aliceTok := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Org: "xcorp", Slug: "lab", Name: "Lab"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects?scope=all", nil, aliceTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("member scope=all deveria 403, veio %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects?scope=all", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin scope=all: %d %s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Items []projectResponse `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 1 || listed.Items[0].Slug != "lab" {
		t.Fatalf("xadmin deveria ver todos: %+v", listed.Items)
	}
}

func TestMeXgitEnabledByMembershipOrACL(t *testing.T) {
	app, _ := newTestApp(t)
	if err := store.SeedXgitApp(app.Store.DB); err != nil {
		t.Fatal(err)
	}
	alice := createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	bob := createTestUserWithRole(t, app, "bob", "senha-bob-okxx", store.RoleMember)
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	router := NewRouter(app)
	adminTok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	aliceTok := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")
	bobTok := loginAndGetToken(t, app, router, "bob", "senha-bob-okxx")

	rec := doJSON(t, router, http.MethodGet, "/api/auth/me", nil, bobTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("me bob: %d %s", rec.Code, rec.Body.String())
	}
	var me userResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &me); err != nil {
		t.Fatal(err)
	}
	if me.XgitEnabled {
		t.Fatal("sem ProjectMember nem ACL o waffle não mostra XGIT")
	}

	rec = doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Org: "xcorp", Slug: "lab", Name: "Lab"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created projectResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	rec = doJSON(t, router, http.MethodPut, "/api/projects/xcorp/lab/members", setProjectMembersRequest{
		Members: []projectMemberIn{
			{UserID: created.Members[0].UserID, Role: store.ProjectRoleOwner},
			{UserID: alice.ID, Role: store.ProjectRoleDeveloper},
		},
	}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("members: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/auth/me", nil, aliceTok)
	if err := json.Unmarshal(rec.Body.Bytes(), &me); err != nil {
		t.Fatal(err)
	}
	if !me.XgitEnabled {
		t.Fatal("ProjectMember deveria ligar xgit_enabled")
	}

	var xgit store.App
	if err := app.Store.DB.Where("slug = ?", "xgit").First(&xgit).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.Store.DB.Create(&store.AppAccess{AppID: xgit.ID, UserID: bob.ID}).Error; err != nil {
		t.Fatal(err)
	}
	rec = doJSON(t, router, http.MethodGet, "/api/auth/me", nil, bobTok)
	if err := json.Unmarshal(rec.Body.Bytes(), &me); err != nil {
		t.Fatal(err)
	}
	if !me.XgitEnabled {
		t.Fatal("ACL do app xgit deveria ligar xgit_enabled")
	}
}

func TestOrgMemberSeesGlobalNotRestricted(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	alice := createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	router := NewRouter(app)
	adminTok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	aliceTok := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{
		Org: "xcorp", Slug: "open", Name: "Open", Visibility: store.AppVisibilityGlobal,
	}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("open: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{
		Org: "xcorp", Slug: "secret", Name: "Secret", Visibility: store.AppVisibilityRestricted,
	}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("secret: %d %s", rec.Code, rec.Body.String())
	}

	org, ok := app.defaultOrganization()
	if !ok {
		t.Fatal("xcorp")
	}
	app.ensureOrgMember(org.ID, alice.ID, store.OrgRoleMember)

	rec = doJSON(t, router, http.MethodGet, "/api/projects/xcorp/open", nil, aliceTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("global deveria 200, veio %d", rec.Code)
	}
	rec = doJSON(t, router, http.MethodGet, "/api/projects/xcorp/secret", nil, aliceTok)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("restricted deveria 404, veio %d", rec.Code)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects", nil, aliceTok)
	var listed struct {
		Items []projectResponse `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 1 || listed.Items[0].Slug != "open" {
		t.Fatalf("lista deveria só o global: %+v", listed.Items)
	}
}

func ptrNetwork(n store.AppNetwork) *store.AppNetwork { return &n }
