package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestCiJobEnqueueClaimAndArtifact(t *testing.T) {
	app, router, adminTok := setupGitApp(t)
	app.Config.DriverProjectsDir = t.TempDir()
	app.Config.WireGuardAllowedSubnet = "10.66.66.0/24"

	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Org: "xcorp", Slug: "lab", Name: "Lab"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var proj projectResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &proj); err != nil {
		t.Fatal(err)
	}
	var row store.Project
	if err := app.Store.DB.Where("slug = ?", "lab").First(&row).Error; err != nil {
		t.Fatal(err)
	}

	token := "runner-token-ci-test-ok"
	hash, err := auth.HashPassword(token)
	if err != nil {
		t.Fatal(err)
	}
	srv := store.MeshServer{
		BitLaunchID: "ci-1", Name: "runner-1", Hostname: "runner1",
		Role: store.ServerRoleRunner, WgIP: "10.66.66.9", Labels: []string{"runner"},
		RunnerTokenHash: hash, Status: "active",
	}
	if err := app.Store.DB.Create(&srv).Error; err != nil {
		t.Fatal(err)
	}

	app.enqueueCiJob(row, ciTriggerPush, "refs/heads/main", strings.Repeat("a", 40), nil)

	rec = doJSON(t, router, http.MethodGet, "/api/projects/xcorp/lab/jobs", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	var listed struct {
		Items []ciJobJSON `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 1 || listed.Items[0].Status != store.CiPending {
		t.Fatalf("jobs: %+v", listed.Items)
	}

	rec = doJSONFrom(t, router, http.MethodGet, "/api/ci/jobs/next", nil, "203.0.113.9:9", map[string]string{
		"Authorization": "Bearer " + token,
	})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("fora da VPN deveria 403, veio %d", rec.Code)
	}

	rec = doRunner(t, router, http.MethodGet, "/api/ci/jobs/next", nil, token, "10.66.66.9:9")
	if rec.Code != http.StatusOK {
		t.Fatalf("claim: %d %s", rec.Code, rec.Body.String())
	}
	var claim ciClaimJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &claim); err != nil {
		t.Fatal(err)
	}
	if claim.ID == 0 || claim.Slug != "lab" || claim.Status != store.CiRunning {
		t.Fatalf("claim: %+v", claim)
	}

	rec = doRunner(t, router, http.MethodPost, "/api/ci/jobs/"+itoa(claim.ID)+"/log", nil, token, "10.66.66.9:9")
	// empty body via doRunner JSON — use raw instead
	req := httptest.NewRequest(http.MethodPost, "/api/ci/jobs/"+itoa(claim.ID)+"/log", strings.NewReader("hello ci\n"))
	req.RemoteAddr = "10.66.66.9:9"
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("log: %d %s", rec.Code, rec.Body.String())
	}

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("file", "out.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = io.WriteString(fw, "artifact")
	_ = w.Close()
	req = httptest.NewRequest(http.MethodPost, "/api/ci/jobs/"+itoa(claim.ID)+"/artifact", &body)
	req.RemoteAddr = "10.66.66.9:9"
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("artifact: %d %s", rec.Code, rec.Body.String())
	}

	rec = doRunner(t, router, http.MethodPost, "/api/ci/jobs/"+itoa(claim.ID)+"/finish", ciFinishRequest{Status: "success", Log: "hello ci\ndone\n"}, token, "10.66.66.9:9")
	if rec.Code != http.StatusOK {
		t.Fatalf("finish: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects/xcorp/lab/jobs/1", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("get: %d", rec.Code)
	}
	var job ciJobJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &job); err != nil {
		t.Fatal(err)
	}
	if job.Status != store.CiSuccess || !job.HasLog || !job.HasArtifact || job.Runner != "runner1" {
		t.Fatalf("job: %+v", job)
	}
	rec = doJSON(t, router, http.MethodGet, "/api/projects/xcorp/lab/jobs/1/log", nil, adminTok)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "hello ci") {
		t.Fatalf("read log: %d %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(app.Config.DriverProjectsDir, "xcorp", "lab", "ci", "1", "out.txt")); err != nil {
		t.Fatalf("artifact no XDRIVER: %v", err)
	}
}

func TestCiEnqueueOnMergeAndSkipDelete(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git não está no PATH")
	}
	app, router, adminTok := setupGitApp(t)
	app.Config.DriverProjectsDir = t.TempDir()
	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Org: "xcorp", Slug: "lab", Name: "Lab"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d", rec.Code)
	}
	seedProjectBranches(t, app.Config.GitDir, "xcorp/lab")
	rec = doJSON(t, router, http.MethodPost, "/api/projects/xcorp/lab/merge-requests", createMRRequest{
		Title: "x", SourceBranch: "feat", TargetBranch: "main",
	}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("mr: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodPost, "/api/projects/xcorp/lab/merge-requests/1/merge", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("merge: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodGet, "/api/projects/xcorp/lab/jobs", nil, adminTok)
	var listed struct {
		Items []ciJobJSON `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) < 2 {
		t.Fatalf("esperado job ao abrir e ao mergear o MR: %+v", listed.Items)
	}
	for _, it := range listed.Items {
		if it.Trigger != ciTriggerMR {
			t.Fatalf("esperado trigger mr: %+v", it)
		}
	}

	var row store.Project
	_ = app.Store.DB.Where("slug = ?", "lab").First(&row).Error
	before := int64(0)
	_ = app.Store.DB.Model(&store.CiJob{}).Where("project_id = ?", row.ID).Count(&before).Error
	app.enqueuePushJobs(row, []forge.RefUpdate{{
		OldHex: strings.Repeat("a", 40), NewHex: strings.Repeat("0", 40), Ref: "refs/heads/gone",
	}}, "admin")
	after := int64(0)
	_ = app.Store.DB.Model(&store.CiJob{}).Where("project_id = ?", row.ID).Count(&after).Error
	if after != before {
		t.Fatal("delete não deveria enfileirar job")
	}
}

func TestIssueRunnerTokenRequiresRunnerRole(t *testing.T) {
	app, router, adminTok := setupGitApp(t)
	s := store.MeshServer{BitLaunchID: "m", Name: "mesh", Hostname: "mesh1", Role: store.ServerRoleMesh, Status: "active"}
	if err := app.Store.DB.Create(&s).Error; err != nil {
		t.Fatal(err)
	}
	rec := doJSON(t, router, http.MethodPost, "/api/servers/"+itoa(s.ID)+"/runner-token", nil, adminTok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("mesh token: %d %s", rec.Code, rec.Body.String())
	}
	s.Role = store.ServerRoleRunner
	if err := app.Store.DB.Save(&s).Error; err != nil {
		t.Fatal(err)
	}
	rec = doJSON(t, router, http.MethodPost, "/api/servers/"+itoa(s.ID)+"/runner-token", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("runner token: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "runner_token") {
		t.Fatalf("body: %s", rec.Body.String())
	}
}

func TestCiMRAwaitingApprovalAndApprove(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git não está no PATH")
	}
	app, router, adminTok := setupGitApp(t)
	app.Config.DriverProjectsDir = t.TempDir()
	app.Config.WireGuardAllowedSubnet = "10.66.66.0/24"
	alice := createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	aliceTok := loginAndGetToken(t, app, router, "alice", "senha-alice-ok")

	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Org: "xcorp", Slug: "lab", Name: "Lab"}, adminTok)
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
	seedProjectBranches(t, app.Config.GitDir, "xcorp/lab")

	rec = doJSON(t, router, http.MethodPost, "/api/projects/xcorp/lab/merge-requests", createMRRequest{
		Title: "feat x", SourceBranch: "feat", TargetBranch: "main",
	}, aliceTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("mr: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects/xcorp/lab/jobs", nil, adminTok)
	var listed struct {
		Items     []ciJobJSON      `json:"items"`
		Workflows []ciWorkflowJSON `json:"workflows"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 1 || listed.Items[0].Status != store.CiAwaitingApproval {
		t.Fatalf("esperado awaiting_approval: %+v", listed.Items)
	}
	if listed.Items[0].Event != "pull_request" || listed.Items[0].Actor != "alice" {
		t.Fatalf("run: %+v", listed.Items[0])
	}
	if len(listed.Workflows) != 1 || listed.Workflows[0].Name != "ci" {
		t.Fatalf("workflows: %+v", listed.Workflows)
	}

	token := "runner-token-approve-ok"
	hash, err := auth.HashPassword(token)
	if err != nil {
		t.Fatal(err)
	}
	srv := store.MeshServer{
		BitLaunchID: "ci-appr", Name: "runner-appr", Hostname: "runner-appr",
		Role: store.ServerRoleRunner, WgIP: "10.66.66.11", Labels: []string{"runner"},
		RunnerTokenHash: hash, Status: "active",
	}
	if err := app.Store.DB.Create(&srv).Error; err != nil {
		t.Fatal(err)
	}
	rec = doRunner(t, router, http.MethodGet, "/api/ci/jobs/next", nil, token, "10.66.66.11:9")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("runner não deveria reclamar awaiting_approval: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/projects/xcorp/lab/jobs/1/approve", nil, aliceTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("developer não aprova: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodPost, "/api/projects/xcorp/lab/jobs/1/approve", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("approve: %d %s", rec.Code, rec.Body.String())
	}
	var approved ciJobJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &approved); err != nil {
		t.Fatal(err)
	}
	if approved.Status != store.CiPending {
		t.Fatalf("depois do approve: %+v", approved)
	}

	rec = doRunner(t, router, http.MethodGet, "/api/ci/jobs/next", nil, token, "10.66.66.11:9")
	if rec.Code != http.StatusOK {
		t.Fatalf("claim após approve: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects/xcorp/lab/runners", nil, aliceTok)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "runner-appr") {
		t.Fatalf("runners: %d %s", rec.Code, rec.Body.String())
	}
}

func TestCiRerunCreatesNewJob(t *testing.T) {
	app, router, adminTok := setupGitApp(t)
	app.Config.DriverProjectsDir = t.TempDir()
	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Org: "xcorp", Slug: "lab", Name: "Lab"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d", rec.Code)
	}
	var row store.Project
	if err := app.Store.DB.Where("slug = ?", "lab").First(&row).Error; err != nil {
		t.Fatal(err)
	}
	job := app.enqueueCiJobAs(row, ciTriggerPush, "refs/heads/main", strings.Repeat("a", 40), nil, "admin", store.CiPending)
	if job == nil {
		t.Fatal("enqueue")
	}
	now := job.CreatedAt
	job.Status = store.CiSuccess
	job.FinishedAt = &now
	if err := app.Store.DB.Save(job).Error; err != nil {
		t.Fatal(err)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/projects/xcorp/lab/jobs/1/rerun", nil, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("rerun: %d %s", rec.Code, rec.Body.String())
	}
	var next ciJobJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &next); err != nil {
		t.Fatal(err)
	}
	if next.Number != 2 || next.Status != store.CiPending || next.Trigger != ciTriggerPush {
		t.Fatalf("rerun job: %+v", next)
	}
}

func doRunner(t *testing.T, router http.Handler, method, path string, body any, token, remote string) *httptest.ResponseRecorder {
	t.Helper()
	headers := map[string]string{"Authorization": "Bearer " + token}
	return doJSONFrom(t, router, method, path, body, remote, headers)
}
