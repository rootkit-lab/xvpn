package api

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestMergeRequestLifecycleAndDMIsolation(t *testing.T) {
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

	rec = doJSON(t, router, http.MethodGet, "/api/projects/lab/branches", nil, devTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("branches: %d %s", rec.Code, rec.Body.String())
	}
	var branches struct {
		Items []string `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &branches); err != nil {
		t.Fatal(err)
	}
	if len(branches.Items) < 2 {
		t.Fatalf("branches: %v", branches.Items)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/merge-requests", createMRRequest{
		Title: "Add feat", SourceBranch: "feat", TargetBranch: "main",
	}, devTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("open: %d %s", rec.Code, rec.Body.String())
	}
	var mr mergeRequestJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &mr); err != nil {
		t.Fatal(err)
	}
	if mr.Number != 1 || mr.Status != store.MROpen || mr.ThreadID == 0 || mr.SocialPostID == nil {
		t.Fatalf("mr: %+v", mr)
	}

	var th store.DirectThread
	if err := app.Store.DB.First(&th, mr.ThreadID).Error; err != nil {
		t.Fatal(err)
	}
	if th.Kind != store.ThreadKindMR {
		t.Fatalf("thread kind: %q", th.Kind)
	}
	var posts int64
	_ = app.Store.DB.Model(&store.SocialPost{}).Where("id = ?", *mr.SocialPostID).Count(&posts).Error
	if posts != 1 {
		t.Fatal("post XGROUP deveria existir")
	}

	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/merge-requests/1/merge", nil, devTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("developer merge em main protegida deveria 403, veio %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/merge-requests/1/merge", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("merge: %d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &mr); err != nil {
		t.Fatal(err)
	}
	if mr.Status != store.MRMerged || mr.MergedBy == "" {
		t.Fatalf("merged: %+v", mr)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/social/threads", openThreadRequest{Username: "admin"}, devTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("dm: %d %s", rec.Code, rec.Body.String())
	}
	var dm socialThreadResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &dm); err != nil {
		t.Fatal(err)
	}
	if dm.ID == 0 || dm.ID == mr.ThreadID {
		t.Fatalf("DM não pode ser a thread do MR: dm=%d mr=%d", dm.ID, mr.ThreadID)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/social/threads", nil, devTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("threads: %d", rec.Code)
	}
	var listed struct {
		Items []socialThreadResponse `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	var sawMR, sawDM bool
	for _, item := range listed.Items {
		if item.ID == mr.ThreadID && item.Title != "" && item.Title != "admin" {
			sawMR = true
		}
		if item.ID == dm.ID {
			sawDM = true
		}
	}
	if !sawMR || !sawDM {
		t.Fatalf("lista deveria ter MR e DM: %+v", listed.Items)
	}
}

func TestMergeRequestACLAndClose(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git não está no PATH")
	}
	app, router, adminTok := setupGitApp(t)
	_ = createTestUserWithRole(t, app, "out", "senha-out-ok-1", store.RoleMember)
	outTok := loginAndGetToken(t, app, router, "out", "senha-out-ok-1")
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
		t.Fatalf("members: %d", rec.Code)
	}
	seedProjectBranches(t, app.Config.GitDir, "lab")

	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/merge-requests", createMRRequest{
		Title: "x", SourceBranch: "feat", TargetBranch: "main",
	}, outTok)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("outsider create: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/merge-requests", createMRRequest{
		Title: "x", SourceBranch: "main", TargetBranch: "main",
	}, devTok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("mesmo branch: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/merge-requests", createMRRequest{
		Title: "x", SourceBranch: "feat", TargetBranch: "main",
	}, devTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("open: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/merge-requests/1/close", nil, outTok)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("outsider close: %d", rec.Code)
	}
	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/merge-requests/1/close", nil, devTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("author close: %d %s", rec.Code, rec.Body.String())
	}
	var mr mergeRequestJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &mr); err != nil {
		t.Fatal(err)
	}
	if mr.Status != store.MRClosed {
		t.Fatalf("status: %s", mr.Status)
	}
	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/merge-requests/1/merge", nil, adminTok)
	if rec.Code != http.StatusConflict {
		t.Fatalf("merge fechado: %d %s", rec.Code, rec.Body.String())
	}
}

func TestMergeRequestReviewDiffAndCIBlock(t *testing.T) {
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
		t.Fatalf("members: %d", rec.Code)
	}
	seedProjectBranches(t, app.Config.GitDir, "lab")

	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/merge-requests", createMRRequest{
		Title: "Add feat", SourceBranch: "feat", TargetBranch: "main",
	}, devTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("open: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects/lab/merge-requests/1/commits", nil, devTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("commits: %d %s", rec.Code, rec.Body.String())
	}
	var commits struct {
		Items []forge.CommitInfo `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &commits); err != nil {
		t.Fatal(err)
	}
	if len(commits.Items) == 0 {
		t.Fatal("esperava commits no PR")
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects/lab/merge-requests/1/diff", nil, devTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("diff: %d", rec.Code)
	}
	var diff struct {
		Diff string `json:"diff"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &diff); err != nil {
		t.Fatal(err)
	}
	if diff.Diff == "" || !strings.Contains(diff.Diff, "feat.txt") {
		t.Fatalf("diff: %q", diff.Diff)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/merge-requests/1/reviews", map[string]string{
		"state": "approve", "body": "lgtm",
	}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("review: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/merge-requests/1/reviews", map[string]string{
		"state": "approve",
	}, devTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("autor não aprova o próprio: %d %s", rec.Code, rec.Body.String())
	}

	var proj store.Project
	if err := app.Store.DB.Where("slug = ?", "lab").First(&proj).Error; err != nil {
		t.Fatal(err)
	}
	n := uint(1)
	if err := app.Store.DB.Model(&store.CiJob{}).Where("project_id = ? AND merge_request_number = ?", proj.ID, n).
		Update("status", store.CiFailed).Error; err != nil {
		t.Fatal(err)
	}
	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/merge-requests/1/merge", nil, adminTok)
	if rec.Code != http.StatusConflict {
		t.Fatalf("CI failed deveria bloquear merge: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects/lab/jobs?mr=1", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("jobs?mr=: %d", rec.Code)
	}
}

func seedProjectBranches(t *testing.T, root, slug string) {
	t.Helper()
	if err := forge.InitBare(root, slug); err != nil {
		t.Fatal(err)
	}
	dir, err := forge.RepoPath(root, slug)
	if err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(t.TempDir(), "w")
	runGitDir(t, "", "clone", dir, work)
	if err := os.WriteFile(filepath.Join(work, "README"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitDir(t, work, "add", "README")
	runGitDir(t, work, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "init")
	runGitDir(t, work, "branch", "-M", "main")
	runGitDir(t, work, "push", "-u", "origin", "main")
	runGitDir(t, work, "checkout", "-b", "feat")
	if err := os.WriteFile(filepath.Join(work, "feat.txt"), []byte("feat\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitDir(t, work, "add", "feat.txt")
	runGitDir(t, work, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-m", "feat")
	runGitDir(t, work, "push", "-u", "origin", "feat")
}

func runGitDir(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
