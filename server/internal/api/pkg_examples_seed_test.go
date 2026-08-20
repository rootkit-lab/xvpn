package api

import (
	"net/http"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/pkgexamples"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func TestSeedLanguagePackageExamplesIdempotent(t *testing.T) {
	app, _ := newTestApp(t)
	app.Config.GitDir = t.TempDir()
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)

	if err := app.SeedLanguagePackageExamples(); err != nil {
		t.Fatal(err)
	}
	if err := app.SeedLanguagePackageExamples(); err != nil {
		t.Fatal(err)
	}

	var n int64
	if err := app.Store.DB.Model(&store.Project{}).Where("slug LIKE ?", "hello-%").Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n != int64(len(pkgexamples.Specs)) {
		t.Fatalf("projetos=%d want %d", n, len(pkgexamples.Specs))
	}
	var pkgs int64
	if err := app.Store.DB.Model(&store.ForgePackage{}).Count(&pkgs).Error; err != nil {
		t.Fatal(err)
	}
	if pkgs != int64(len(pkgexamples.Specs)) {
		t.Fatalf("packages=%d", pkgs)
	}
	var alice store.User
	if err := app.Store.DB.Where("username = ?", "alice").First(&alice).Error; err != nil {
		t.Fatal(err)
	}
	var guests int64
	if err := app.Store.DB.Model(&store.ProjectMember{}).
		Where("user_id = ? AND role = ?", alice.ID, store.ProjectRoleGuest).Count(&guests).Error; err != nil {
		t.Fatal(err)
	}
	if guests < int64(len(pkgexamples.Specs)) {
		t.Fatalf("alice guest em %d exemplos", guests)
	}
}

func TestSeedSkipsForeignHelloSlug(t *testing.T) {
	app, _ := newTestApp(t)
	app.Config.GitDir = t.TempDir()
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	alice := createTestUserWithRole(t, app, "alice", "senha-alice-ok", store.RoleMember)
	if _, err := app.createProject(alice.ID, "hello-js", "mine", "repo da alice",
		store.AppVisibilityRestricted, store.AppNetworkVPN, nil, false); err != nil {
		t.Fatal(err)
	}
	if err := app.SeedLanguagePackageExamples(); err != nil {
		t.Fatal(err)
	}
	var proj store.Project
	if err := app.Store.DB.Where("slug = ?", "hello-js").First(&proj).Error; err != nil {
		t.Fatal(err)
	}
	if proj.Description != "repo da alice" {
		t.Fatalf("seed sobrescreveu: %q", proj.Description)
	}
	var guests int64
	if err := app.Store.DB.Model(&store.ProjectMember{}).
		Where("project_id = ? AND role = ?", proj.ID, store.ProjectRoleGuest).Count(&guests).Error; err != nil {
		t.Fatal(err)
	}
	if guests != 0 {
		t.Fatalf("não deveria alargar guests: %d", guests)
	}
	var pkgs int64
	if err := app.Store.DB.Model(&store.ForgePackage{}).Where("name = ?", "@ihuull/hello-js").Count(&pkgs).Error; err != nil {
		t.Fatal(err)
	}
	if pkgs != 0 {
		t.Fatal("não deveria publicar no repo alheio")
	}
}

func TestCreateProjectRejectsExampleSlug(t *testing.T) {
	app, _ := newTestApp(t)
	createTestUserWithRole(t, app, "admin", "senha-admin-ok", store.RoleSuperAdmin)
	router := NewRouter(app)
	tok := loginAndGetToken(t, app, router, "admin", "senha-admin-ok")
	rec := doJSON(t, router, http.MethodPost, "/api/projects", createProjectRequest{Slug: "hello-js", Name: "x"}, tok)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("create hello-js: %d %s", rec.Code, rec.Body.String())
	}
}

func TestPackageTarballNpmPrefix(t *testing.T) {
	files, err := pkgexamples.Files(pkgexamples.JavaScript)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := files["package.json"]; !ok {
		t.Fatal("package.json")
	}
	blob, err := packageTarball(pkgexamples.Specs[0], files)
	if err != nil || len(blob) < 100 {
		t.Fatalf("tarball: %v len=%d", err, len(blob))
	}
}
