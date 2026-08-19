package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestReservedSlugRejected(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	router := NewRouter(app)
	tok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Slug: "repositories", Name: "Nope"}, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("slug reservado deveria 400, veio %d: %s", rec.Code, rec.Body.String())
	}
}

func TestProjectStarAndOverview(t *testing.T) {
	app, _ := newTestApp(t)
	app.Config.GitDir = t.TempDir()
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	alice := createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	router := NewRouter(app)
	adminTok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	aliceTok := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")

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
			{UserID: alice.ID, Role: store.ProjectRoleDeveloper},
		},
	}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("members: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/xgit/overview", nil, aliceTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("overview: %d %s", rec.Code, rec.Body.String())
	}
	var ov xgitOverviewResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &ov); err != nil {
		t.Fatal(err)
	}
	if ov.Profile.Username != "alice" || ov.RepoCount != 1 {
		t.Fatalf("overview: %+v", ov)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/star", nil, aliceTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("star: %d %s", rec.Code, rec.Body.String())
	}
	var starred projectResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &starred); err != nil {
		t.Fatal(err)
	}
	if !starred.Starred || starred.StarCount != 1 {
		t.Fatalf("star resp: %+v", starred)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/xgit/stars", nil, aliceTok)
	var listed struct {
		Items []projectResponse `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 1 || listed.Items[0].Slug != "lab" {
		t.Fatalf("stars: %+v", listed.Items)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/star", nil, aliceTok)
	if err := json.Unmarshal(rec.Body.Bytes(), &starred); err != nil {
		t.Fatal(err)
	}
	if starred.Starred {
		t.Fatal("toggle deveria remover a estrela")
	}
}
