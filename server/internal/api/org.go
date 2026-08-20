package api

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func (a *App) loadOrganization(slug string) (store.ForgeOrganization, bool) {
	slug = forge.NormalizeSlug(slug)
	if !store.ValidOrgSlug(slug) {
		return store.ForgeOrganization{}, false
	}
	var org store.ForgeOrganization
	if err := a.Store.DB.Where("slug = ?", slug).First(&org).Error; err != nil {
		return store.ForgeOrganization{}, false
	}
	return org, true
}

func (a *App) defaultOrganization() (store.ForgeOrganization, bool) {
	return a.loadOrganization(store.DefaultOrgSlug)
}

func (a *App) projectOrgSlug(proj store.Project) string {
	if proj.Organization.Slug != "" {
		return proj.Organization.Slug
	}
	if proj.OrganizationID == 0 {
		return ""
	}
	var org store.ForgeOrganization
	if err := a.Store.DB.First(&org, proj.OrganizationID).Error; err != nil {
		return ""
	}
	return org.Slug
}

func (a *App) projectRepo(proj store.Project) string {
	org := a.projectOrgSlug(proj)
	if org == "" || proj.Slug == "" {
		return ""
	}
	return forge.RepoName(org, proj.Slug)
}

func (a *App) projectCloneURL(proj store.Project) string {
	repo := a.projectRepo(proj)
	if repo == "" {
		return ""
	}
	return gitCloneHost + "/" + repo
}

func (a *App) findProject(orgSlug, slug string) (store.Project, bool) {
	org, ok := a.loadOrganization(orgSlug)
	if !ok {
		return store.Project{}, false
	}
	slug = forge.NormalizeSlug(slug)
	if !store.ValidProjectSlug(slug) {
		return store.Project{}, false
	}
	var proj store.Project
	if err := a.Store.DB.Where("organization_id = ? AND slug = ?", org.ID, slug).First(&proj).Error; err != nil {
		return store.Project{}, false
	}
	proj.Organization = org
	return proj, true
}

func (a *App) ensureOrgMember(orgID, userID uint, role store.OrgRole) {
	if orgID == 0 || userID == 0 {
		return
	}
	if !role.Valid() {
		role = store.OrgRoleMember
	}
	row := store.OrgMember{OrganizationID: orgID, UserID: userID, Role: role}
	_ = a.Store.DB.Where("organization_id = ? AND user_id = ?", orgID, userID).FirstOrCreate(&row).Error
}

func (a *App) orgTeam(orgID uint, slug string) (store.OrgTeam, bool) {
	var team store.OrgTeam
	if err := a.Store.DB.Where("organization_id = ? AND slug = ?", orgID, slug).First(&team).Error; err != nil {
		return store.OrgTeam{}, false
	}
	return team, true
}

func (a *App) isOrgMember(user store.User, orgID uint) bool {
	if orgID == 0 {
		return false
	}
	var n int64
	_ = a.Store.DB.Model(&store.OrgMember{}).
		Where("organization_id = ? AND user_id = ?", orgID, user.ID).Count(&n).Error
	return n > 0
}

func (a *App) remountProjectsToDefaultOrg() {
	org, ok := a.defaultOrganization()
	if !ok {
		return
	}
	var rows []store.Project
	if err := a.Store.DB.Where("organization_id = 0 OR organization_id IS NULL").Find(&rows).Error; err != nil {
		return
	}
	root := a.gitDir()
	for i := range rows {
		p := rows[i]
		p.OrganizationID = org.ID
		if err := a.Store.DB.Save(&p).Error; err != nil {
			continue
		}
		if root == "" || !store.ValidProjectSlug(p.Slug) {
			continue
		}
		old := filepath.Join(filepath.Clean(root), p.Slug+".git")
		if _, err := os.Stat(filepath.Join(old, "HEAD")); err != nil {
			continue
		}
		dest, err := forge.RepoPath(root, forge.RepoName(org.Slug, p.Slug))
		if err != nil {
			continue
		}
		if _, err := os.Stat(filepath.Join(dest, "HEAD")); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
			continue
		}
		_ = os.Rename(old, dest)
	}
}

func parseRepoParams(c *gin.Context) (org, slug string, ok bool) {
	org = forge.NormalizeSlug(c.Param("org"))
	slug = forge.NormalizeSlug(c.Param("slug"))
	if !store.ValidOrgSlug(org) || !store.ValidProjectSlug(slug) {
		return "", "", false
	}
	return org, slug, true
}

func parseRepoQuery(org, slug string) (string, string, bool) {
	org = forge.NormalizeSlug(strings.TrimSpace(org))
	slug = forge.NormalizeSlug(strings.TrimSpace(slug))
	if !store.ValidOrgSlug(org) || !store.ValidProjectSlug(slug) {
		return "", "", false
	}
	return org, slug, true
}
