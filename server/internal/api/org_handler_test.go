package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestTeamMemberSeesTeamRepoNotOrgRoot(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	alice := createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	router := NewRouter(app)
	adminTok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	aliceTok := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{
		Org: "xcorp", Slug: "hello-js-x", Name: "pkg", Visibility: store.AppVisibilityGlobal, Team: "packages",
	}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("pkg: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{
		Org: "xcorp", Slug: "xchat-x", Name: "root", Visibility: store.AppVisibilityGlobal,
	}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("root: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects/xcorp/hello-js-x", nil, aliceTok)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("sem time deveria 404, veio %d", rec.Code)
	}

	org, ok := app.defaultOrganization()
	if !ok {
		t.Fatal("xcorp")
	}
	pkg, ok := app.orgTeam(org.ID, "packages")
	if !ok {
		t.Fatal("packages")
	}
	app.ensureTeamMember(pkg.ID, alice.ID)

	rec = doJSON(t, router, http.MethodGet, "/api/projects/xcorp/hello-js-x", nil, aliceTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("membro do time packages deveria ver o repo: %d", rec.Code)
	}
	rec = doJSON(t, router, http.MethodGet, "/api/projects/xcorp/xchat-x", nil, aliceTok)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("raiz da org não é do time: %d", rec.Code)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/orgs/xcorp", nil, aliceTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("org home: %d %s", rec.Code, rec.Body.String())
	}
	var home orgHomeJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &home); err != nil {
		t.Fatal(err)
	}
	if len(home.Root) != 0 {
		t.Fatalf("alice não deveria ver a raiz: %+v", home.Root)
	}
	var sawPkg bool
	for _, team := range home.Teams {
		if team.Slug == "packages" {
			for _, p := range team.Repos {
				if p.Slug == "hello-js-x" {
					sawPkg = true
				}
			}
		}
		if team.Slug == "workflows" && len(team.Templates) == 0 {
			t.Fatal("workflows deveria listar templates abertos")
		}
	}
	if !sawPkg {
		t.Fatal("hello-js-x deveria aparecer em packages")
	}
}

func TestParentTeamSeesChildRepos(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	bob := createTestUserWithRole(t, app, "bob", "senha-bob-okkk", store.RoleMember)
	router := NewRouter(app)
	adminTok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	bobTok := loginAndGetToken(t, app, router, "bob", "senha-bob-okkk")

	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{
		Org: "xcorp", Slug: "hello-py-x", Name: "py", Visibility: store.AppVisibilityGlobal, Team: "packages",
	}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	org, _ := app.defaultOrganization()
	ex, ok := app.orgTeam(org.ID, "exemplos")
	if !ok {
		t.Fatal("exemplos")
	}
	app.ensureTeamMember(ex.ID, bob.ID)

	rec = doJSON(t, router, http.MethodGet, "/api/projects/xcorp/hello-py-x", nil, bobTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("pai exemplos deveria ver packages: %d", rec.Code)
	}
}

func TestAddTeamMemberRequiresManager(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	alice := createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	bob := createTestUserWithRole(t, app, "bob", "senha-bob-okkk", store.RoleMember)
	router := NewRouter(app)
	aliceTok := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")
	adminTok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/orgs/xcorp/teams/packages/members", map[string]uint{"user_id": bob.ID}, aliceTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("member não gere time: %d", rec.Code)
	}
	rec = doJSON(t, router, http.MethodPost, "/api/orgs/xcorp/teams/packages/members", map[string]uint{"user_id": alice.ID}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("admin adiciona: %d %s", rec.Code, rec.Body.String())
	}
}
