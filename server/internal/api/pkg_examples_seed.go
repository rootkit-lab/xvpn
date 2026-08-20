package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"path"
	"sort"
	"time"

	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/pkgexamples"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

// SeedLanguagePackageExamples cria os repos hello-* no XGIT e publica
// o artefacto de cada linguagem (Fase 45.3). Idempotente: se o slug ou
// a versão já existem, só garante membros guest e o git vazio.
func (a *App) SeedLanguagePackageExamples() error {
	if a == nil || a.Store == nil || a.Packages == nil {
		return nil
	}
	owner, ok := a.firstProjectOwner()
	if !ok {
		return nil
	}
	var last error
	for _, spec := range pkgexamples.Specs {
		if err := a.seedOneLanguageExample(owner, spec); err != nil {
			slog.Error("seed do exemplo XGIT falhou", "slug", spec.Slug, "err", err)
			last = err
		}
	}
	return last
}

func (a *App) seedOneLanguageExample(owner store.User, spec pkgexamples.Spec) error {
	files, err := pkgexamples.Files(spec.Lang)
	if err != nil {
		return fmt.Errorf("embed %s: %w", spec.Lang, err)
	}
	proj, err := a.ensureExampleProject(owner, spec)
	if err != nil {
		return err
	}
	a.ensureExampleGuests(proj)
	if a.gitDir() != "" {
		if err := a.ensureGitRepo(proj.Slug); err != nil {
			return err
		}
		if !forge.HasCommits(a.gitDir(), proj.Slug) {
			list := make([]forge.FileContent, 0, len(files))
			for p, body := range files {
				list = append(list, forge.FileContent{Path: p, Content: body})
			}
			sort.Slice(list, func(i, j int) bool { return list[i].Path < list[j].Path })
			if _, err := forge.CommitFiles(a.gitDir(), proj.Slug, forge.CommitFilesOpts{
				Files:       list,
				Message:     "chore: seed exemplo " + spec.Slug,
				AuthorName:  owner.Username,
				AuthorEmail: owner.Username + "@corp.ihuull.com",
			}); err != nil {
				return fmt.Errorf("git %s: %w", spec.Slug, err)
			}
		}
	}
	blob, err := packageTarball(spec, files)
	if err != nil {
		return err
	}
	kind := store.ForgePackageKind(spec.Kind)
	_, err = a.publishPackageBytes(owner, proj, kind, spec.PkgName, spec.Version, spec.Filename, spec.Description, blob)
	if err == errPackageExists {
		return nil
	}
	return err
}

func (a *App) ensureExampleProject(owner store.User, spec pkgexamples.Spec) (store.Project, error) {
	var existing store.Project
	err := a.Store.DB.Where("slug = ?", spec.Slug).First(&existing).Error
	if err == nil {
		return existing, nil
	}
	return a.createProject(owner.ID, spec.Slug, spec.Title, spec.Description,
		store.AppVisibilityGlobal, store.AppNetworkVPN, nil, false)
}

func (a *App) ensureExampleGuests(proj store.Project) {
	var users []store.User
	if err := a.Store.DB.Find(&users).Error; err != nil {
		return
	}
	for _, u := range users {
		row := store.ProjectMember{ProjectID: proj.ID, UserID: u.ID, Role: store.ProjectRoleGuest}
		_ = a.Store.DB.Where("project_id = ? AND user_id = ?", proj.ID, u.ID).FirstOrCreate(&row).Error
	}
	_ = a.syncProjectGroupMembers(proj)
}

func packageTarball(spec pkgexamples.Spec, files map[string]string) ([]byte, error) {
	prefix := spec.Slug + "-" + spec.Version
	switch spec.Kind {
	case "npm":
		prefix = "package"
	case "pypi":
		prefix = "hello_ihuull-" + spec.Version
	}
	keys := make([]string, 0, len(files))
	for k := range files {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	now := time.Now()
	for _, rel := range keys {
		body := files[rel]
		name := path.Join(prefix, rel)
		hdr := &tar.Header{
			Name:    name,
			Mode:    0o644,
			Size:    int64(len(body)),
			ModTime: now,
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, err
		}
		if _, err := io.WriteString(tw, body); err != nil {
			return nil, err
		}
	}
	if err := tw.Close(); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
