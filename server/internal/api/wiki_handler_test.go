package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestWikiHomePutGet(t *testing.T) {
	app, router, adminTok := setupGitApp(t)
	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Org: "xcorp", Slug: "lab", Name: "Lab"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	seedProjectBranches(t, app.Config.GitDir, "xcorp/lab")

	rec = doJSON(t, router, http.MethodPut, "/api/projects/xcorp/lab/wiki/Home", putWikiRequest{Content: "# Casa\n", Message: "wiki home"}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("put: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodGet, "/api/projects/xcorp/lab/wiki/Home", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d %s", rec.Code, rec.Body.String())
	}
	var page wikiPageJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil || page.Content != "# Casa\n" {
		t.Fatalf("body: %s", rec.Body.String())
	}
	if _, err := forge.ReadWiki(app.Config.GitDir, "xcorp/lab", "Home"); err != nil {
		t.Fatal(err)
	}

	createTestUserWithRole(t, app, "eve", "senha-eve-okkk", store.RoleMember)
	eveTok := loginAndGetToken(t, app, router, "eve", "senha-eve-okkk")
	rec = doJSON(t, router, http.MethodGet, "/api/projects/xcorp/lab/wiki/Home", nil, eveTok)
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusForbidden {
		t.Fatalf("eve: %d", rec.Code)
	}
}

func TestWikiPutRequiresDeveloper(t *testing.T) {
	app, _ := newTestApp(t)
	app.Config.GitDir = t.TempDir()
	router := NewRouter(app)
	_, aliceTok, _ := seedLabWithAlice(t, app, router, store.ProjectRoleReporter)
	seedProjectBranches(t, app.Config.GitDir, "xcorp/lab")
	rec := doJSON(t, router, http.MethodPut, "/api/projects/xcorp/lab/wiki/Home", putWikiRequest{Content: "x", Message: "no"}, aliceTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("reporter put: %d %s", rec.Code, rec.Body.String())
	}
}
