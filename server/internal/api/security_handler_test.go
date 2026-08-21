package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestSecurityNpmAuditBecomesAlert(t *testing.T) {
	app, router, adminTok := setupGitApp(t)
	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Org: "xcorp", Slug: "lab", Name: "Lab"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var proj store.Project
	if err := app.Store.DB.Where("slug = ?", "lab").First(&proj).Error; err != nil {
		t.Fatal(err)
	}
	app.recordSecAlerts(proj, nil, "npm audit\n3 vulnerabilities (1 high)")
	rec = doJSON(t, router, http.MethodGet, "/api/projects/xcorp/lab/security", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("sec: %d %s", rec.Code, rec.Body.String())
	}
	var out struct {
		Alerts []secAlertJSON    `json:"alerts"`
		Setup  map[string]string `json:"setup"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || len(out.Alerts) == 0 {
		t.Fatalf("alerts: %s", rec.Body.String())
	}
	if out.Setup["secret"] != "enabled" {
		t.Fatalf("secret setup: %v", out.Setup)
	}
}

func TestSecurityPrivateReportHidden(t *testing.T) {
	app, _ := newTestApp(t)
	app.Config.GitDir = t.TempDir()
	router := NewRouter(app)
	adminTok, aliceTok, _ := seedLabWithAlice(t, app, router, store.ProjectRoleDeveloper)
	rec := doJSON(t, router, http.MethodPost, "/api/projects/xcorp/lab/security/report", securityReportRequest{Title: "vuln", Body: "detalhe"}, aliceTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("report: %d %s", rec.Code, rec.Body.String())
	}
	createTestUserWithRole(t, app, "bob", "senha-bob-okkkk", store.RoleMember)
	// bob as reporter on project
	var alice store.User
	_ = app.Store.DB.Where("username = ?", "alice").First(&alice).Error
	var bob store.User
	_ = app.Store.DB.Where("username = ?", "bob").First(&bob).Error
	var created projectResponse
	_ = json.Unmarshal(doJSON(t, router, http.MethodGet, "/api/projects/xcorp/lab", nil, adminTok).Body.Bytes(), &created)
	rec = doJSON(t, router, http.MethodPut, "/api/projects/xcorp/lab/members", setProjectMembersRequest{
		Members: []projectMemberIn{
			{UserID: created.Members[0].UserID, Role: store.ProjectRoleOwner},
			{UserID: alice.ID, Role: store.ProjectRoleDeveloper},
			{UserID: bob.ID, Role: store.ProjectRoleReporter},
		},
	}, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("members: %d %s", rec.Code, rec.Body.String())
	}
	bobTok := loginAndGetToken(t, app, router, "bob", "senha-bob-okkkk")
	rec = doJSON(t, router, http.MethodGet, "/api/projects/xcorp/lab/issues", nil, bobTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("issues: %d", rec.Code)
	}
	if rec.Body.String() != "" && containsIssueTitle(rec.Body.String(), "vuln") {
		t.Fatal("reporter não deveria ver issue restrita de outro")
	}
}

func TestScanPackRejectsPrivateKey(t *testing.T) {
	reject, titles := scanPackSecrets([]byte("foo\n-----BEGIN " + "OPENSSH PRIVATE KEY-----\nbar"))
	if !reject || len(titles) == 0 {
		t.Fatalf("reject=%v titles=%v", reject, titles)
	}
}

func TestRevHasPrivateKeyOnCommit(t *testing.T) {
	app, router, adminTok := setupGitApp(t)
	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Org: "xcorp", Slug: "lab", Name: "Lab"}, adminTok)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d", rec.Code)
	}
	if _, err := forge.CommitFiles(app.Config.GitDir, "xcorp/lab", forge.CommitFilesOpts{
		Files:   []forge.FileContent{{Path: "id", Content: "-----BEGIN " + "OPENSSH PRIVATE KEY-----\nb\n-----END " + "OPENSSH PRIVATE KEY-----\n"}},
		Message: "key",
	}); err != nil {
		t.Fatal(err)
	}
	if !forge.RevHasPrivateKey(app.Config.GitDir, "xcorp/lab", "HEAD") {
		t.Fatal("esperava detectar chave no tree")
	}
}

func containsIssueTitle(raw, title string) bool {
	return json.Valid([]byte(raw)) && (func() bool {
		var wrap struct {
			Items []struct {
				Title string `json:"title"`
			} `json:"items"`
		}
		if json.Unmarshal([]byte(raw), &wrap) != nil {
			return false
		}
		for _, it := range wrap.Items {
			if it.Title == title {
				return true
			}
		}
		return false
	})()
}
