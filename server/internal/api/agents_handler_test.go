package api

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestProjectAgentsMineVsMaintainer(t *testing.T) {
	app, _ := newTestApp(t)
	router := NewRouter(app)
	adminTok, aliceTok, alice := seedLabWithAlice(t, app, router, store.ProjectRoleDeveloper)
	var lab store.Project
	if err := app.Store.DB.Where("slug = ?", "lab").First(&lab).Error; err != nil {
		t.Fatal(err)
	}
	var admin store.User
	if err := app.Store.DB.Where("username = ?", "admin").First(&admin).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.Store.DB.Create(&store.CodeSpace{
		PublicID: "aabbccddeefe", UserID: admin.ID, ProjectID: lab.ID, Branch: "main",
		RelPath: "a", Kind: store.CodespaceKindQuick, Status: store.CodespaceRunning,
	}).Error; err != nil {
		t.Fatal(err)
	}
	if err := app.Store.DB.Create(&store.CodeSpace{
		PublicID: "bbccddeeffaa", UserID: alice.ID, ProjectID: lab.ID, Branch: "feat",
		RelPath: "b", Kind: store.CodespaceKindQuick, Status: store.CodespaceStopped,
	}).Error; err != nil {
		t.Fatal(err)
	}
	rec := doJSON(t, router, http.MethodGet, "/api/projects/xcorp/lab/agents", nil, aliceTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("alice: %d %s", rec.Code, rec.Body.String())
	}
	var aliceOut struct {
		Items []codespaceJSON `json:"items"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &aliceOut); err != nil || len(aliceOut.Items) != 1 || aliceOut.Items[0].Author != "alice" {
		t.Fatalf("alice items: %s", rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodGet, "/api/projects/xcorp/lab/agents", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin: %d %s", rec.Code, rec.Body.String())
	}
	var adminOut struct {
		Items  []codespaceJSON `json:"items"`
		SeeAll bool            `json:"see_all"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &adminOut); err != nil || len(adminOut.Items) != 2 || !adminOut.SeeAll {
		t.Fatalf("admin items: %s", rec.Body.String())
	}
	rec = doJSON(t, router, http.MethodGet, "/api/projects/xcorp/lab/agents?filter=active", nil, adminTok)
	if rec.Code != http.StatusOK {
		t.Fatalf("active: %d", rec.Code)
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &adminOut); err != nil || len(adminOut.Items) != 1 {
		t.Fatalf("active items: %s", rec.Body.String())
	}
}
