package api

import (
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
