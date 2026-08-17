package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

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
