package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestProjectCodespaceEnvs_SecretWriteOnly(t *testing.T) {
	app, _ := newTestApp(t)
	app.Config.DriverProjectsDir = t.TempDir()
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	dev := createTestUserWithRole(t, app, "dev", "senha-dev-okxx", store.RoleMember)
	router := NewRouter(app)
	adminTok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	devTok := loginAndGetToken(t, app, router, "dev", "senha-dev-okxx")

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
		},
	}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("members: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPut, "/api/projects/lab/codespaces/envs", putProjectEnvsRequest{
		Items: []putProjectEnvItem{
			{Name: "APP_URL", Value: "https://xgit.corp", Secret: false},
			{Name: "XCS_LLM_KEY", Value: "sk-secret-key", Secret: true},
		},
	}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("put: %d %s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Items []projectEnvJSON `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 2 {
		t.Fatalf("items: %+v", listed.Items)
	}
	for _, it := range listed.Items {
		if it.Name == "XCS_LLM_KEY" {
			if it.Value != "" || !it.HasValue || !it.Secret {
				t.Fatalf("secret vazou: %+v", it)
			}
		}
		if it.Name == "APP_URL" && it.Value != "https://xgit.corp" {
			t.Fatalf("plaintext: %+v", it)
		}
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects/lab/codespaces/envs", nil, devTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("get dev: %d %s", rec.Code, rec.Body.String())
	}
	listed = struct {
		Items []projectEnvJSON `json:"items"`
	}{}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	for _, it := range listed.Items {
		if it.Value != "" {
			t.Fatalf("developer não vê valor: %+v", it)
		}
	}

	rec = doJSON(t, router, http.MethodPut, "/api/projects/lab/codespaces/envs", putProjectEnvsRequest{
		Items: []putProjectEnvItem{{Name: "PATH", Value: "/bin"}},
	}, adminTok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("PATH deveria 400, veio %d", rec.Code)
	}

	var proj store.Project
	if err := app.Store.DB.Where("slug = ?", "lab").First(&proj).Error; err != nil {
		t.Fatal(err)
	}
	runtime := app.codespaceRuntimeEnvs(proj.ID)
	if runtime["APP_URL"] != "https://xgit.corp" {
		t.Fatalf("runtime: %+v", runtime)
	}
	if _, ok := runtime["XCS_LLM_KEY"]; ok {
		t.Fatal("key LLM não entra no container")
	}
	_, _, _, key := app.loadProjectLLMConfig(proj.ID)
	if key != "sk-secret-key" {
		t.Fatalf("proxy deve ler a key: %q", key)
	}

	var logs []store.AuditLog
	_ = app.Store.DB.Where("action = ?", "project.codespaces.envs").Find(&logs).Error
	for _, l := range logs {
		if strings.Contains(l.Detail, "sk-secret-key") {
			t.Fatal("audit não pode ter o valor")
		}
	}
}

func TestProjectCodespaceEnvs_MaintainerOnlyWrite(t *testing.T) {
	app, _ := newTestApp(t)
	app.Config.DriverProjectsDir = t.TempDir()
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	dev := createTestUserWithRole(t, app, "dev", "senha-dev-okxx", store.RoleMember)
	router := NewRouter(app)
	adminTok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	devTok := loginAndGetToken(t, app, router, "dev", "senha-dev-okxx")

	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Slug: "lab", Name: "Lab"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d", rec.Code)
	}
	var created projectResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	rec = doJSON(t, router, http.MethodPut, "/api/projects/lab/members", setProjectMembersRequest{
		Members: []projectMemberIn{
			{UserID: created.Members[0].UserID, Role: store.ProjectRoleOwner},
			{UserID: dev.ID, Role: store.ProjectRoleDeveloper},
		},
	}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatal(rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodPut, "/api/projects/lab/codespaces/envs", putProjectEnvsRequest{
		Items: []putProjectEnvItem{{Name: "APP_URL", Value: "x"}},
	}, devTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("developer write: %d", rec.Code)
	}
}
