package api

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestXgitSettingsAndMemberCreate(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	router := NewRouter(app)
	adminTok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	aliceTok := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")

	rec := doJSON(t, router, http.MethodGet, "/api/xgit/settings", nil, aliceTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("settings member: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/xgit/repos", createProjectRequest{Slug: "lab", Name: "Lab"}, aliceTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("member sem flag deveria 403, veio %d: %s", rec.Code, rec.Body.String())
	}

	allow := true
	rec = doJSON(t, router, http.MethodPatch, "/api/xgit/settings", updateForgeSettingsRequest{
		AllowMemberCreate: &allow,
	}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch settings: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/xgit/repos", createProjectRequest{Slug: "lab", Name: "Lab"}, aliceTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("member com flag deveria criar, veio %d: %s", rec.Code, rec.Body.String())
	}
}

func TestProjectTreeAPI(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git não está no PATH")
	}
	app, _ := newTestApp(t)
	app.Config.GitDir = t.TempDir()
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	router := NewRouter(app)
	tok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Slug: "lab", Name: "Lab"}, tok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	seedProjectBranches(t, app.Config.GitDir, "lab")

	rec = doJSON(t, router, http.MethodGet, "/api/projects/lab/tree?ref=main", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("tree: %d %s", rec.Code, rec.Body.String())
	}
	var tree struct {
		Items []struct {
			Name       string `json:"name"`
			Type       string `json:"type"`
			LastCommit *struct {
				Subject string `json:"subject"`
			} `json:"last_commit"`
		} `json:"items"`
		CommitCount int `json:"commit_count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &tree); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range tree.Items {
		if e.Name == "README" && e.Type == "blob" {
			found = true
			if e.LastCommit == nil || e.LastCommit.Subject == "" {
				t.Fatalf("README sem last_commit: %s", rec.Body.String())
			}
		}
	}
	if !found {
		t.Fatalf("README ausente: %s", rec.Body.String())
	}
	if tree.CommitCount < 1 {
		t.Fatalf("commit_count: %d", tree.CommitCount)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects/lab/blob?path=README&ref=main", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("blob: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects/lab/commits?ref=main", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("commits: %d %s", rec.Code, rec.Body.String())
	}
}
