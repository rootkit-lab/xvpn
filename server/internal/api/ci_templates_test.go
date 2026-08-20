package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestListWorkflowTemplatesFilter(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	router := NewRouter(app)
	tok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")

	rec := doJSON(t, router, http.MethodGet, "/api/ci/workflow-templates?category=deployment", nil, tok)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Categories []workflowCategoryJSON `json:"categories"`
		Items      []workflowTemplateJSON `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out.Categories) != 6 {
		t.Fatalf("categories=%d", len(out.Categories))
	}
	if len(out.Items) == 0 {
		t.Fatal("deployment vazio")
	}
	for _, it := range out.Items {
		if it.Category != "deployment" {
			t.Fatalf("%+v", it)
		}
	}
}

func TestApplyWorkflowTemplateOnEmptyRepo(t *testing.T) {
	app, router, adminTok := setupGitApp(t)
	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Org: "xcorp", Slug: "lab", Name: "Lab"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodPost, "/api/projects/xcorp/lab/workflows", applyWorkflowRequest{TemplateID: "go"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("apply: %d %s", rec.Code, rec.Body.String())
	}
	if !forge.HasCommits(app.Config.GitDir, "xcorp/lab") {
		t.Fatal("esperava commit")
	}
	rec = doJSON(t, router, http.MethodPost, "/api/projects/xcorp/lab/workflows", applyWorkflowRequest{TemplateID: "go"}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("reapply: %d %s", rec.Code, rec.Body.String())
	}
}

func TestApplyPublishTemplateBakesOrgSlug(t *testing.T) {
	app, router, adminTok := setupGitApp(t)
	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Org: "xcorp", Slug: "lab", Name: "Lab"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodPost, "/api/projects/xcorp/lab/workflows", applyWorkflowRequest{TemplateID: "npm-xgit"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("apply: %d %s", rec.Code, rec.Body.String())
	}
	body, _, err := forge.ReadBlob(app.Config.GitDir, "xcorp/lab", "HEAD", ".xvpn-ci.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "api/packages/xcorp/lab/npm/") {
		t.Fatalf("esperava registry org/slug, veio:\n%s", body)
	}
	if strings.Contains(body, "{{REPO}}") || strings.Contains(body, "${PWD") {
		t.Fatalf("placeholder no script:\n%s", body)
	}
}
