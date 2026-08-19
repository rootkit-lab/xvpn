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

func TestIssueFiltersMilestoneAndLabels(t *testing.T) {
	app, router, adminTok := setupGitApp(t)
	reporter := createTestUserWithRole(t, app, "rep", "senha-rep-ok-1", store.RoleMember)
	repTok := loginAndGetToken(t, app, router, "rep", "senha-rep-ok-1")

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
		},
	}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("members: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/milestones", createMilestoneRequest{Title: "v1"}, repTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("milestone: %d %s", rec.Code, rec.Body.String())
	}
	var ms milestoneJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &ms); err != nil {
		t.Fatal(err)
	}
	if ms.Number != 1 {
		t.Fatalf("milestone number: %+v", ms)
	}

	one := uint(1)
	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/issues", createIssueRequest{
		Title: "Bug no login", Body: "ping @rep", Labels: []string{"bug"}, Assignee: []string{"rep"}, Milestone: &one,
	}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("issue1: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/issues", createIssueRequest{
		Title: "Docs", Body: "sem menção", Labels: []string{"docs"},
	}, repTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("issue2: %d %s", rec.Code, rec.Body.String())
	}

	type listed struct {
		Items       []issueJSON `json:"items"`
		OpenCount   int         `json:"open_count"`
		ClosedCount int         `json:"closed_count"`
	}
	get := func(qs string) listed {
		t.Helper()
		r := doJSON(t, router, http.MethodGet, "/api/projects/lab/issues"+qs, nil, repTok)
		if r.Code != http.StatusOK {
			t.Fatalf("list %s: %d %s", qs, r.Code, r.Body.String())
		}
		var out listed
		if err := json.Unmarshal(r.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}

	all := get("")
	if len(all.Items) != 2 || all.OpenCount != 2 {
		t.Fatalf("all: %+v", all)
	}
	assigned := get("?assignee=me")
	if len(assigned.Items) != 1 || assigned.Items[0].Title != "Bug no login" {
		t.Fatalf("assignee: %+v", assigned.Items)
	}
	authored := get("?author=rep")
	if len(authored.Items) != 1 || authored.Items[0].Title != "Docs" {
		t.Fatalf("author: %+v", authored.Items)
	}
	labeled := get("?label=bug")
	if len(labeled.Items) != 1 {
		t.Fatalf("label: %+v", labeled.Items)
	}
	mentioned := get("?mentioned=me")
	if len(mentioned.Items) != 1 || mentioned.Items[0].Number != 1 {
		t.Fatalf("mentioned: %+v", mentioned.Items)
	}
	byMS := get("?milestone=1")
	if len(byMS.Items) != 1 || byMS.Items[0].Milestone == nil || *byMS.Items[0].Milestone != 1 {
		t.Fatalf("milestone filter: %+v", byMS.Items)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects/lab/labels", nil, repTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("labels: %d %s", rec.Code, rec.Body.String())
	}
}
