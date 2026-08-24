package gitssh

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/store"
	"gorm.io/gorm"
)

var (
	ErrAccessDenied = errors.New("acesso negado")
	ErrNotFound     = errors.New("repositório não encontrado")
)

var sshGitCmd = regexp.MustCompile(`^git-(upload-pack|receive-pack)\s+'([^']+\.git)'\s*$`)

// ParsedCommand é o resultado de SSH_ORIGINAL_COMMAND validado.
type ParsedCommand struct {
	Service string // upload-pack | receive-pack
	Repo    string // org/slug
}

// ParseOriginalCommand valida SSH_ORIGINAL_COMMAND do git client.
func ParseOriginalCommand(raw string) (ParsedCommand, error) {
	raw = strings.TrimSpace(raw)
	m := sshGitCmd.FindStringSubmatch(raw)
	if m == nil {
		return ParsedCommand{}, fmt.Errorf("comando git inválido")
	}
	repo := strings.TrimPrefix(strings.TrimSpace(m[2]), "/")
	repo = strings.TrimSuffix(repo, ".git")
	if _, _, err := forge.SplitRepo(repo); err != nil {
		return ParsedCommand{}, err
	}
	return ParsedCommand{Service: m[1], Repo: repo}, nil
}

// Access encapsula ACL de git espelhando git_handler.go.
type Access struct {
	DB     *gorm.DB
	GitDir string
}

func (a *Access) FindProject(orgSlug, slug string) (store.Project, bool) {
	orgSlug = forge.NormalizeSlug(orgSlug)
	if !store.ValidOrgSlug(orgSlug) {
		return store.Project{}, false
	}
	var org store.ForgeOrganization
	if err := a.DB.Where("slug = ?", orgSlug).First(&org).Error; err != nil {
		return store.Project{}, false
	}
	slug = forge.NormalizeSlug(slug)
	if !store.ValidProjectSlug(slug) {
		return store.Project{}, false
	}
	var proj store.Project
	if err := a.DB.Where("organization_id = ? AND slug = ?", org.ID, slug).First(&proj).Error; err != nil {
		return store.Project{}, false
	}
	proj.Organization = org
	return proj, true
}

func (a *Access) LoadUser(username string) (store.User, error) {
	var user store.User
	if err := a.DB.Where("username = ?", username).First(&user).Error; err != nil {
		return store.User{}, err
	}
	if user.Role == store.RoleBot || user.Username == "xbot" {
		return store.User{}, gorm.ErrRecordNotFound
	}
	return user, nil
}

func (a *Access) CanGitRead(user store.User, proj store.Project) bool {
	return a.canSeeProject(user, proj)
}

func (a *Access) CanGitPush(user store.User, proj store.Project) bool {
	if store.HasProduct(user.Role, user.Products, store.ProductForge) {
		return true
	}
	role, ok := a.projectMemberRole(user, proj)
	if !ok {
		return false
	}
	return role.Rank() >= store.ProjectRoleDeveloper.Rank()
}

func (a *Access) CanGitPushProtected(user store.User, proj store.Project, min store.ProjectRole) bool {
	if store.HasProduct(user.Role, user.Products, store.ProductForge) {
		return true
	}
	role, ok := a.projectMemberRole(user, proj)
	if !ok {
		return false
	}
	if !min.Valid() {
		min = store.ProjectRoleMaintainer
	}
	return role.Rank() >= min.Rank()
}

func (a *Access) canSeeProject(user store.User, proj store.Project) bool {
	if user.Role.Rank() >= store.RoleViewer.Rank() {
		return true
	}
	var n int64
	_ = a.DB.Model(&store.ProjectMember{}).
		Where("project_id = ? AND user_id = ?", proj.ID, user.ID).Count(&n).Error
	if n > 0 {
		return true
	}
	if proj.Visibility == store.AppVisibilityRestricted {
		return false
	}
	if a.isOrgMember(user, proj.OrganizationID) {
		return true
	}
	return a.canSeeViaTeam(user, proj)
}

func (a *Access) projectMemberRole(user store.User, proj store.Project) (store.ProjectRole, bool) {
	var m store.ProjectMember
	if err := a.DB.Where("project_id = ? AND user_id = ?", proj.ID, user.ID).First(&m).Error; err != nil {
		return "", false
	}
	return m.Role, true
}

func (a *Access) isOrgMember(user store.User, orgID uint) bool {
	if orgID == 0 {
		return false
	}
	var n int64
	_ = a.DB.Model(&store.OrgMember{}).
		Where("organization_id = ? AND user_id = ?", orgID, user.ID).Count(&n).Error
	return n > 0
}

func (a *Access) canSeeViaTeam(user store.User, proj store.Project) bool {
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

func (a *Access) readableTeamIDs(userID uint) []uint {
	var mine []uint
	_ = a.DB.Model(&store.OrgTeamMember{}).Where("user_id = ?", userID).Pluck("team_id", &mine).Error
	if len(mine) == 0 {
		return nil
	}
	var children []uint
	_ = a.DB.Model(&store.OrgTeam{}).Where("parent_id IN ?", mine).Pluck("id", &children).Error
	seen := make(map[uint]struct{}, len(mine)+len(children))
	out := make([]uint, 0, len(mine)+len(children))
	for _, id := range append(mine, children...) {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func (a *Access) ProtectedBranchRules(projectID uint) []store.ProtectedBranch {
	var rows []store.ProtectedBranch
	_ = a.DB.Where("project_id = ?", projectID).Order("id").Find(&rows).Error
	return rows
}

func (a *Access) ProjectRepo(proj store.Project) string {
	org := proj.Organization.Slug
	if org == "" && proj.OrganizationID != 0 {
		var o store.ForgeOrganization
		if err := a.DB.First(&o, proj.OrganizationID).Error; err == nil {
			org = o.Slug
		}
	}
	if org == "" || proj.Slug == "" {
		return ""
	}
	return forge.RepoName(org, proj.Slug)
}
