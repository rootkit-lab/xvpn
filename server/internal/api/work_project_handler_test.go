package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestWorkProjectKanbanLifecycle(t *testing.T) {
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

	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/issues", createIssueRequest{Title: "Primeira"}, repTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("issue: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/work-projects", createWorkProjectRequest{
		Title: "Sprint", Template: "kanban",
	}, gstTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("guest create: %d %s", rec.Code, rec.Body.String())
	}

	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/work-projects", createWorkProjectRequest{
		Title: "Sprint", Template: "kanban",
	}, repTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create board: %d %s", rec.Code, rec.Body.String())
	}
	var wp workProjectJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &wp); err != nil {
		t.Fatal(err)
	}
	if wp.Number != 1 || wp.Layout != "board" || len(wp.Columns) != 3 || wp.Columns[0] != "Todo" {
		t.Fatalf("board: %+v", wp)
	}

	one := uint(1)
	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/work-projects/1/items", createWorkItemRequest{
		Issue: &one,
	}, repTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("item: %d %s", rec.Code, rec.Body.String())
	}
	var item workItemJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if item.Title != "Primeira" || item.Column != "Todo" || item.Issue == nil {
		t.Fatalf("item: %+v", item)
	}

	col := "In Progress"
	rec = doJSON(t, router, http.MethodPatch, "/api/projects/lab/work-projects/1/items/1", patchWorkItemRequest{
		Column: &col,
	}, repTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("move: %d %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &item); err != nil {
		t.Fatal(err)
	}
	if item.Column != "In Progress" {
		t.Fatalf("moved: %+v", item)
	}

	rec = doJSON(t, router, http.MethodGet, "/api/projects/lab/work-projects/1", nil, gstTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("guest get: %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &wp); err != nil {
		t.Fatal(err)
	}
	if len(wp.Items) != 1 || wp.Items[0].Column != "In Progress" {
		t.Fatalf("detail: %+v", wp.Items)
	}

	rec = doJSON(t, router, http.MethodPost, "/api/projects/lab/work-projects/1/items", createWorkItemRequest{
		Title: "draft card",
	}, gstTok)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("guest item: %d", rec.Code)
	}
}
