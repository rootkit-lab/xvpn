package api

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/pkgexamples"
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

func (a *App) orgRole(user store.User, orgID uint) (store.OrgRole, bool) {
	var row store.OrgMember
	if err := a.Store.DB.Where("organization_id = ? AND user_id = ?", orgID, user.ID).First(&row).Error; err != nil {
		return "", false
	}
	return row.Role, true
}

func (a *App) canManageOrg(user store.User, orgID uint) bool {
	if user.Role.Rank() >= store.RoleAdmin.Rank() {
		return true
	}
	role, ok := a.orgRole(user, orgID)
	return ok && (role == store.OrgRoleOwner || role == store.OrgRoleAdmin)
}

func (a *App) canSeeTeam(user store.User, org store.ForgeOrganization, team store.OrgTeam) bool {
	if user.Role.Rank() >= store.RoleViewer.Rank() {
		return true
	}
	if a.isOrgMember(user, org.ID) {
		return true
	}
	want := map[uint]struct{}{team.ID: {}}
	if team.ParentID != nil {
		want[*team.ParentID] = struct{}{}
	}
	for _, id := range a.readableTeamIDs(user.ID) {
		if _, ok := want[id]; ok {
			return true
		}
	}
	return false
}

func (a *App) ensureTeamMember(teamID, userID uint) {
	if teamID == 0 || userID == 0 {
		return
	}
	row := store.OrgTeamMember{TeamID: teamID, UserID: userID}
	_ = a.Store.DB.Where("team_id = ? AND user_id = ?", teamID, userID).FirstOrCreate(&row).Error
}

// readableTeamIDs is the team the user belongs to plus one level of children
// (exemplos → packages / workflows).
func (a *App) readableTeamIDs(userID uint) []uint {
	var mine []uint
	_ = a.Store.DB.Model(&store.OrgTeamMember{}).Where("user_id = ?", userID).Pluck("team_id", &mine).Error
	if len(mine) == 0 {
		return nil
	}
	var kids []uint
	_ = a.Store.DB.Model(&store.OrgTeam{}).Where("parent_id IN ?", mine).Pluck("id", &kids).Error
	seen := map[uint]struct{}{}
	out := make([]uint, 0, len(mine)+len(kids))
	for _, id := range append(mine, kids...) {
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (a *App) canSeeViaTeam(user store.User, proj store.Project) bool {
	if proj.TeamID == nil || *proj.TeamID == 0 {
		return false
	}
	for _, id := range a.readableTeamIDs(user.ID) {
		if id == *proj.TeamID {
			return true
		}
	}
	return false
}

func (a *App) teamSlug(id *uint) string {
	if id == nil || *id == 0 {
		return ""
	}
	var team store.OrgTeam
	if err := a.Store.DB.First(&team, *id).Error; err != nil {
		return ""
	}
	return team.Slug
}

func (a *App) assignDefaultTeams() {
	org, ok := a.defaultOrganization()
	if !ok {
		return
	}
	if pkg, ok := a.orgTeam(org.ID, "packages"); ok {
		slugs := make([]string, 0, len(pkgexamples.Specs))
		for _, spec := range pkgexamples.Specs {
			slugs = append(slugs, spec.Slug)
		}
		if len(slugs) > 0 {
			_ = a.Store.DB.Model(&store.Project{}).
				Where("organization_id = ? AND slug IN ? AND (team_id IS NULL OR team_id = 0)", org.ID, slugs).
				Update("team_id", pkg.ID)
		}
	}
	if ex, ok := a.orgTeam(org.ID, "exemplos"); ok {
		var owner store.User
		err := a.Store.DB.Where("role = ?", store.RoleSuperAdmin).Order("id").First(&owner).Error
		if err != nil {
			err = a.Store.DB.Where("role = ?", store.RoleAdmin).Order("id").First(&owner).Error
		}
		if err == nil {
			a.ensureTeamMember(ex.ID, owner.ID)
		}
	}
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
	a.assignDefaultTeams()
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
