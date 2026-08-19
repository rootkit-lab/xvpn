package api

import (
	"encoding/json"
	"net/http"
	"os/exec"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestPutContentsProtectedOpensPR(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git não está no PATH")
	}
	app, router, adminTok := setupGitApp(t)
	dev := createTestUserWithRole(t, app, "dev", "senha-dev-ok-1", store.RoleMember)
	devTok := loginAndGetToken(t, app, router, "dev", "senha-dev-ok-1")

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
	seedProjectBranches(t, app.Config.GitDir, "lab")

	rec = doJSON(t, router, http.MethodPut, "/api/projects/lab/contents", putContentsRequest{
		Path: "README", Ref: "main", Content: "hello from web\n", Message: "edit via web",
	}, devTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("dev put: %d %s", rec.Code, rec.Body.String())
	}
	var put putContentsJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &put); err != nil {
		t.Fatal(err)
	}
	if put.MergeRequestNumber == nil || *put.MergeRequestNumber == 0 {
		t.Fatalf("developer em main deveria abrir PR: %+v", put)
	}
	if put.Branch == "main" {
		t.Fatalf("não deveria commitar em main: %+v", put)
	}

	rec = doJSON(t, router, http.MethodPut, "/api/projects/lab/contents", putContentsRequest{
		Path: "README", Ref: "main", Content: "hello from maintainer\n", Message: "maintainer edit",
	}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin put: %d %s", rec.Code, rec.Body.String())
	}
	put = putContentsJSON{}
	if err := json.Unmarshal(rec.Body.Bytes(), &put); err != nil {
		t.Fatal(err)
	}
	if put.Branch != "main" || put.MergeRequestNumber != nil {
		t.Fatalf("maintainer deveria commitar em main: %+v", put)
	}

	rec = doJSON(t, router, http.MethodPut, "/api/projects/lab/contents", putContentsRequest{
		Path: "README", Ref: "main", Content: "hello from maintainer\n", Message: "noop",
	}, adminTok)
	if rec.Code != http.StatusConflict {
		t.Fatalf("unchanged: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects/lab/archive?ref=main", nil, devTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("archive: %d %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("content-type: %s", rec.Header().Get("Content-Type"))
	}
}
