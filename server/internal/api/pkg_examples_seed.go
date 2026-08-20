package api

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
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

var errForeignExampleSlug = errors.New("slug já pertence a outro projeto")

// SeedLanguagePackageExamples cria os repos hello-* no XGIT e publica
// o artefacto de cada linguagem (Fase 45.3). Idempotente. Se o slug já
// existir e não for o exemplo do seed (descrição + owner), o boot
// não toca no repo nem alarga guests.
func (a *App) SeedLanguagePackageExamples() error {
	if a == nil || a.Store == nil || a.Packages == nil {
		return nil
	}
	if err := store.SeedXcorp(a.Store.DB); err != nil {
		return err
	}
	a.remountProjectsToDefaultOrg()
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
	if errors.Is(err, errForeignExampleSlug) {
		slog.Warn("seed pulou slug ocupado por outro projeto", "slug", spec.Slug)
		return nil
	}
	if err != nil {
		return err
	}
	if a.gitDir() != "" {
		if err := a.ensureGitRepo(proj); err != nil {
			return err
		}
		if !forge.HasCommits(a.gitDir(), a.projectRepo(proj)) {
			list := make([]forge.FileContent, 0, len(files))
			for p, body := range files {
				list = append(list, forge.FileContent{Path: p, Content: body})
			}
			sort.Slice(list, func(i, j int) bool { return list[i].Path < list[j].Path })
			if _, err := forge.CommitFiles(a.gitDir(), a.projectRepo(proj), forge.CommitFilesOpts{
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
	org, ok := a.defaultOrganization()
	if !ok {
		return store.Project{}, errors.New("org xcorp ausente")
	}
	var existing store.Project
	err := a.Store.DB.Where("organization_id = ? AND slug = ?", org.ID, spec.Slug).First(&existing).Error
	if err == nil {
		if !a.isOwnedSeedExample(existing, owner, spec) {
			return store.Project{}, errForeignExampleSlug
		}
		if existing.OrganizationID == 0 {
			existing.OrganizationID = org.ID
			_ = a.Store.DB.Save(&existing).Error
		}
		existing.Organization = org
		return existing, nil
	}
	teamID := (*uint)(nil)
	if team, ok := a.orgTeam(org.ID, "packages"); ok {
		teamID = &team.ID
	}
	return a.createProject(owner.ID, org, spec.Slug, spec.Title, spec.Description,
		store.AppVisibilityGlobal, store.AppNetworkVPN, nil, false, teamID)
}

func (a *App) isOwnedSeedExample(proj store.Project, owner store.User, spec pkgexamples.Spec) bool {
	if proj.Description != spec.Description {
		return false
	}
	var n int64
	_ = a.Store.DB.Model(&store.ProjectMember{}).
		Where("project_id = ? AND user_id = ? AND role = ?", proj.ID, owner.ID, store.ProjectRoleOwner).
		Count(&n).Error
	return n > 0
}

// ensureExampleGuests removido: o finding da #166 alargava guest a
// toda a VPN. Quem vê o exemplo é OrgMember da xcorp / time packages.

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
