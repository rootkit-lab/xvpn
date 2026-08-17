package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestIssueLifecycleRBAC(t *testing.T) {
	app, router, adminTok := setupGitApp(t)
	reporter := createTestUserWithRole(t, app, "rep", "senha-rep-ok-1", store.RoleMember)
	repTok := loginAndGetToken(t, app, router, "rep", "senha-rep-ok-1")
	guest := createTestUserWithRole(t, app, "gst", "senha-gst-ok-1", store.RoleMember)
	gstTok := loginAndGetToken(t, app, router, "gst", "senha-gst-ok-1")

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
			{UserID: reporter.ID, Role: store.ProjectRoleReporter},
			{UserID: guest.ID, Role: store.ProjectRoleGuest},
		},
	}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("members: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/issues", createIssueRequest{
		Title: "Primeira", Body: "detalhe", Labels: []string{"bug"},
	}, gstTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("guest create: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/issues", createIssueRequest{
		Title: "Primeira", Body: "detalhe", Labels: []string{"bug"}, Assignee: []string{"rep"},
	}, repTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("reporter create: %d %s", rec.Code, rec.Body.String())
	}
	var issue issueJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &issue); err != nil {
		t.Fatal(err)
	}
	if issue.Number != 1 || issue.Status != store.IssueOpen || issue.ThreadID == 0 || issue.Author != "rep" {
		t.Fatalf("issue: %+v", issue)
	}
	if len(issue.Assignees) != 1 || issue.Assignees[0] != "rep" {
		t.Fatalf("assignees: %v", issue.Assignees)
	}

	var th store.DirectThread
	if err := app.Store.DB.First(&th, issue.ThreadID).Error; err != nil {
		t.Fatal(err)
	}
	if th.Kind != store.ThreadKindIssue {
		t.Fatalf("thread kind: %q", th.Kind)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects/lab/issues", nil, gstTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("guest list: %d", rec.Code)
	}
	var listed struct {
		Items []issueJSON `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if len(listed.Items) != 1 {
		t.Fatalf("list: %+v", listed.Items)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects/lab/issues/1", nil, gstTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("guest get: %d", rec.Code)
	}

	closed := "closed"
	rec = doJSON(t, router, http.MethodPatch, "/api/projects/lab/issues/1", patchIssueRequest{Status: &closed}, gstTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("guest patch: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPatch, "/api/projects/lab/issues/1", patchIssueRequest{Status: &closed}, repTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("close: %d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &issue); err != nil {
		t.Fatal(err)
	}
	if issue.Status != store.IssueClosed || issue.ClosedBy != "rep" {
		t.Fatalf("closed: %+v", issue)
	}

	open := "open"
	rec = doJSON(t, router, http.MethodPatch, "/api/projects/lab/issues/1", patchIssueRequest{Status: &open}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("reopen: %d %s", rec.Code, rec.Body.String())
	}
	issue = issueJSON{}
	if err := json.Unmarshal(rec.Body.Bytes(), &issue); err != nil {
		t.Fatal(err)
	}
	if issue.Status != store.IssueOpen || issue.ClosedBy != "" {
		t.Fatalf("reopened: %+v", issue)
	}
}
