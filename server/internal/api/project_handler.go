package api

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/pkgexamples"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type createProjectRequest struct {
	Org          string              `json:"org"`
	Slug         string              `json:"slug"`
	Name         string              `json:"name"`
	Description  string              `json:"description"`
	FilesEnabled bool                `json:"files_enabled"`
	Visibility   store.AppVisibility `json:"visibility"`
	Network      store.AppNetwork    `json:"network"`
	Runners      []string            `json:"runners"`
	Team         string              `json:"team"`
}

type updateProjectRequest struct {
	Name         *string              `json:"name"`
	Description  *string              `json:"description"`
	FilesEnabled *bool                `json:"files_enabled"`
	Visibility   *store.AppVisibility `json:"visibility"`
	Network      *store.AppNetwork    `json:"network"`
	Runners      *[]string            `json:"runners"`
}

type projectMemberIn struct {
	UserID uint              `json:"user_id"`
	Role   store.ProjectRole `json:"role"`
}

type setProjectMembersRequest struct {
	Members []projectMemberIn `json:"members"`
}

type projectMemberResponse struct {
	UserID   uint              `json:"user_id"`
	Username string            `json:"username"`
	Role     store.ProjectRole `json:"role"`
}

type projectResponse struct {
	Org           string                  `json:"org"`
	Slug          string                  `json:"slug"`
	FullName      string                  `json:"full_name"`
	Name          string                  `json:"name"`
	Description   string                  `json:"description"`
	AppID         *uint                   `json:"app_id,omitempty"`
	SocialGroupID uint                    `json:"social_group_id"`
	FilesEnabled  bool                    `json:"files_enabled"`
	Visibility    store.AppVisibility     `json:"visibility"`
	Network       store.AppNetwork        `json:"network"`
	Runners       []string                `json:"runners"`
	MemberCount   int                     `json:"member_count"`
	Members       []projectMemberResponse `json:"members,omitempty"`
	CreatedAt     time.Time               `json:"created_at"`
	UpdatedAt     time.Time               `json:"updated_at"`
	Language      string                  `json:"language,omitempty"`
	LastCommitAt  *time.Time              `json:"last_commit_at,omitempty"`
	Starred       bool                    `json:"starred"`
	StarCount     int64                   `json:"star_count"`
	Spark         []int                   `json:"spark,omitempty"`
	Team          string                  `json:"team,omitempty"`
}

func (a *App) canSeeProject(user store.User, proj store.Project) bool {
	if user.Role.Rank() >= store.RoleViewer.Rank() {
		return true
	}
	var n int64
	_ = a.Store.DB.Model(&store.ProjectMember{}).
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

func (a *App) canCreateInOrg(user store.User, orgID uint) bool {
	if store.HasProduct(user.Role, user.Products, store.ProductForge) {
		return true
	}
	return a.isOrgMember(user, orgID)
}

func (a *App) projectMemberRole(user store.User, proj store.Project) (store.ProjectRole, bool) {
	var m store.ProjectMember
	if err := a.Store.DB.Where("project_id = ? AND user_id = ?", proj.ID, user.ID).First(&m).Error; err != nil {
		return "", false
	}
	return m.Role, true
}

// canAccessProjectFiles: forge write (admin/super_admin) ou membro.
// Mutação (mkdir/upload/rm) exige developer+; guest/reporter só leem.
func (a *App) canAccessProjectFiles(user store.User, proj store.Project, write bool) bool {
	if store.HasProduct(user.Role, user.Products, store.ProductForge) {
		return true
	}
	role, ok := a.projectMemberRole(user, proj)
	if !ok {
		return false
	}
	if write {
		return role.Rank() >= store.ProjectRoleDeveloper.Rank()
	}
	return true
}

func (a *App) loadProjectBySlug(c *gin.Context) (store.Project, store.User, bool) {
	orgSlug, slug, ok := parseRepoParams(c)
	var user store.User
	if err := a.Store.DB.First(&user, callerUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return store.Project{}, user, false
	}
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "projeto não encontrado"})
		return store.Project{}, user, false
	}
	proj, found := a.findProject(orgSlug, slug)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "projeto não encontrado"})
		return store.Project{}, user, false
	}
	if !a.canSeeProject(user, proj) {
		c.JSON(http.StatusNotFound, gin.H{"error": "projeto não encontrado"})
		return store.Project{}, user, false
	}
	return proj, user, true
}

func (a *App) projectResponse(proj store.Project, withMembers bool) projectResponse {
	runners := proj.Runners
	if runners == nil {
		runners = []string{}
	}
	var count int64
	_ = a.Store.DB.Model(&store.ProjectMember{}).Where("project_id = ?", proj.ID).Count(&count).Error
	org := a.projectOrgSlug(proj)
	out := projectResponse{
		Org:           org,
		Slug:          proj.Slug,
		FullName:      forge.RepoName(org, proj.Slug),
		Name:          proj.Name,
		Description:   proj.Description,
		AppID:         proj.AppID,
		SocialGroupID: proj.SocialGroupID,
		FilesEnabled:  proj.FilesEnabled,
		Visibility:    proj.Visibility,
		Network:       proj.Network,
		Runners:       runners,
		MemberCount:   int(count),
		CreatedAt:     proj.CreatedAt,
		UpdatedAt:     proj.UpdatedAt,
		Team:          a.teamSlug(proj.TeamID),
	}
	if !withMembers {
		return out
	}
	var rows []store.ProjectMember
	_ = a.Store.DB.Where("project_id = ?", proj.ID).Order("id").Find(&rows).Error
	out.Members = make([]projectMemberResponse, 0, len(rows))
	for _, m := range rows {
		var u store.User
		if err := a.Store.DB.First(&u, m.UserID).Error; err != nil {
			continue
		}
		out.Members = append(out.Members, projectMemberResponse{
			UserID: u.ID, Username: u.Username, Role: m.Role,
		})
	}
	return out
}

func (a *App) applyMemberProjectScope(q *gorm.DB, user store.User) (*gorm.DB, bool) {
	var ids []uint
	_ = a.Store.DB.Model(&store.ProjectMember{}).Where("user_id = ?", user.ID).Pluck("project_id", &ids).Error
	var orgIDs []uint
	_ = a.Store.DB.Model(&store.OrgMember{}).Where("user_id = ?", user.ID).Pluck("organization_id", &orgIDs).Error
	teamIDs := a.readableTeamIDs(user.ID)
	switch {
	case len(ids) == 0 && len(orgIDs) == 0 && len(teamIDs) == 0:
		return q, true
	case len(orgIDs) == 0 && len(teamIDs) == 0:
		return q.Where("id IN ?", ids), false
	case len(ids) == 0 && len(teamIDs) == 0:
		return q.Where("organization_id IN ? AND visibility <> ?", orgIDs, store.AppVisibilityRestricted), false
	case len(ids) == 0 && len(orgIDs) == 0:
		return q.Where("team_id IN ? AND visibility <> ?", teamIDs, store.AppVisibilityRestricted), false
	case len(teamIDs) == 0:
		return q.Where("id IN ? OR (organization_id IN ? AND visibility <> ?)", ids, orgIDs, store.AppVisibilityRestricted), false
	case len(orgIDs) == 0:
		return q.Where("id IN ? OR (team_id IN ? AND visibility <> ?)", ids, teamIDs, store.AppVisibilityRestricted), false
	case len(ids) == 0:
		return q.Where("(organization_id IN ? OR team_id IN ?) AND visibility <> ?", orgIDs, teamIDs, store.AppVisibilityRestricted), false
	default:
		return q.Where("id IN ? OR ((organization_id IN ? OR team_id IN ?) AND visibility <> ?)", ids, orgIDs, teamIDs, store.AppVisibilityRestricted), false
	}
}

func (a *App) handleListProjects(c *gin.Context) {
	var user store.User
	if err := a.Store.DB.First(&user, callerUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	q := a.Store.DB.Model(&store.Project{}).Where("archived_at IS NULL")
	scope := strings.TrimSpace(c.Query("scope"))
	wantAll := scope == "all" || (scope == "" && user.Role.Rank() >= store.RoleViewer.Rank())
	if scope == "all" && user.Role.Rank() < store.RoleViewer.Rank() {
		c.JSON(http.StatusForbidden, gin.H{"error": "lista completa só no xadmin"})
		return
	}
	if scope == "mine" || !wantAll {
		scoped, empty := a.applyMemberProjectScope(q, user)
		if empty {
			c.JSON(http.StatusOK, gin.H{"items": []projectResponse{}})
			return
		}
		q = scoped
	}
	if orgSlug := forge.NormalizeSlug(c.Query("org")); store.ValidOrgSlug(orgSlug) {
		if org, ok := a.loadOrganization(orgSlug); ok {
			q = q.Where("organization_id = ?", org.ID)
			if team := strings.TrimSpace(c.Query("team")); team == "root" {
				q = q.Where("team_id IS NULL OR team_id = 0")
			} else if team != "" {
				if t, ok := a.orgTeam(org.ID, team); ok {
					q = q.Where("team_id = ?", t.ID)
				}
			}
		}
	}
	var rows []store.Project
	if err := q.Order("slug").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	cards := c.Query("cards") == "1"
	items := make([]projectResponse, 0, len(rows))
	for _, p := range rows {
		items = append(items, a.decorateProjectCard(user, p, cards))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) handleCreateProject(c *gin.Context) {
	var req createProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	orgSlug := strings.ToLower(strings.TrimSpace(req.Org))
	if orgSlug == "" || !store.ValidOrgSlug(orgSlug) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "org obrigatória (2–20, a-z 0-9 hífen)"})
		return
	}
	if store.ReservedOrgSlug(orgSlug) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "org reservada (rota da home XGIT)"})
		return
	}
	org, ok := a.loadOrganization(orgSlug)
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "organização não encontrada"})
		return
	}
	slug := strings.ToLower(strings.TrimSpace(req.Slug))
	if !store.ValidProjectSlug(slug) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug inválido (2–20, a-z 0-9 hífen)"})
		return
	}
	if store.ReservedProjectSlug(slug) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug reservado (rota da home XGIT)"})
		return
	}
	if pkgexamples.IsExampleSlug(slug) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug reservado para exemplos do XGIT"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = slug
	}
	defaults := a.loadForgeSettings()
	vis := req.Visibility
	if vis == "" {
		vis = defaults.DefaultVisibility
	}
	if vis == "" {
		vis = store.AppVisibilityGlobal
	}
	if !vis.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "visibility inválida"})
		return
	}
	net := req.Network
	if net == "" {
		net = defaults.DefaultNetwork
	}
	if net == "" {
		net = store.AppNetworkVPN
	}
	if !net.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "network inválida"})
		return
	}
	var user store.User
	if err := a.Store.DB.First(&user, callerUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	if !a.canCreateInOrg(user, org.ID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "não é membro desta organização"})
		return
	}
	var existing store.Project
	if err := a.Store.DB.Where("organization_id = ? AND slug = ?", org.ID, slug).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "já existe um projeto com este slug nesta org"})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	var teamID *uint
	if t := strings.TrimSpace(req.Team); t != "" {
		if team, ok := a.orgTeam(org.ID, t); ok {
			teamID = &team.ID
		}
	}
	proj, err := a.createProject(user.ID, org, slug, name, strings.TrimSpace(req.Description), vis, net, req.Runners, req.FilesEnabled, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "project.create", slug)
	c.JSON(http.StatusCreated, a.projectResponse(proj, true))
}

func (a *App) handleGetProject(c *gin.Context) {
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, a.projectResponse(proj, true))
}

func (a *App) handleUpdateProject(c *gin.Context) {
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	var req updateProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name inválido"})
			return
		}
		proj.Name = name
	}
	if req.Description != nil {
		proj.Description = strings.TrimSpace(*req.Description)
	}
	if req.Visibility != nil {
		if !req.Visibility.Valid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "visibility inválida"})
			return
		}
		proj.Visibility = *req.Visibility
	}
	if req.Network != nil {
		if !req.Network.Valid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "network inválida"})
			return
		}
		proj.Network = *req.Network
	}
	if req.Runners != nil {
		proj.Runners = *req.Runners
	}
	enableFiles := req.FilesEnabled != nil && *req.FilesEnabled && !proj.FilesEnabled
	if req.FilesEnabled != nil {
		proj.FilesEnabled = *req.FilesEnabled
	}
	if err := a.Store.DB.Save(&proj).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if enableFiles {
		_ = a.ensureProjectFilesDir(a.projectRepo(proj))
	}
	_ = a.Store.LogAudit(callerUsername(c), "project.update", proj.Slug)
	c.JSON(http.StatusOK, a.projectResponse(proj, true))
}

func (a *App) handleSetProjectMembers(c *gin.Context) {
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	var req setProjectMembersRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	owners := 0
	seen := map[uint]struct{}{}
	for _, m := range req.Members {
		if m.UserID == 0 || !m.Role.Valid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "membro inválido"})
			return
		}
		if _, dup := seen[m.UserID]; dup {
			c.JSON(http.StatusBadRequest, gin.H{"error": "membro duplicado"})
			return
		}
		seen[m.UserID] = struct{}{}
		var u store.User
		if err := a.Store.DB.First(&u, m.UserID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "usuário não encontrado"})
			return
		}
		if m.Role == store.ProjectRoleOwner {
			owners++
		}
	}
	if owners < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "o projeto precisa de pelo menos um owner"})
		return
	}
	if err := a.Store.DB.Where("project_id = ?", proj.ID).Delete(&store.ProjectMember{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	for _, m := range req.Members {
		row := store.ProjectMember{ProjectID: proj.ID, UserID: m.UserID, Role: m.Role}
		if err := a.Store.DB.Create(&row).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
			return
		}
	}
	if err := a.syncProjectGroupMembers(proj); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "project.members", proj.Slug)
	c.JSON(http.StatusOK, a.projectResponse(proj, true))
}

func (a *App) createProject(ownerID uint, org store.ForgeOrganization, slug, name, desc string, vis store.AppVisibility, net store.AppNetwork, runners []string, files bool, teamID *uint) (store.Project, error) {
	g := store.SocialGroup{
		Name:        name,
		Description: "Projeto " + slug,
		OwnerUserID: ownerID,
	}
	if err := a.Store.DB.Create(&g).Error; err != nil {
		return store.Project{}, err
	}
	_ = a.Store.DB.Where("group_id = ? AND user_id = ?", g.ID, ownerID).
		FirstOrCreate(&store.SocialGroupMember{GroupID: g.ID, UserID: ownerID}).Error

	var appID *uint
	var app store.App
	if err := a.Store.DB.Where("slug = ?", slug).First(&app).Error; err == nil {
		appID = &app.ID
	}

	if runners == nil {
		runners = []string{}
	}
	proj := store.Project{
		OrganizationID: org.ID,
		TeamID:         teamID,
		Slug:           slug,
		Name:           name,
		Description:    desc,
		AppID:          appID,
		SocialGroupID:  g.ID,
		FilesEnabled:   files,
		Visibility:     vis,
		Network:        net,
		Runners:        runners,
	}
	proj.Organization = org
	if err := a.Store.DB.Create(&proj).Error; err != nil {
		return store.Project{}, err
	}
	if err := a.Store.DB.Create(&store.ProjectMember{
		ProjectID: proj.ID, UserID: ownerID, Role: store.ProjectRoleOwner,
	}).Error; err != nil {
		return store.Project{}, err
	}
	a.ensureOrgMember(org.ID, ownerID, store.OrgRoleMember)
	if files {
		_ = a.ensureProjectFilesDir(a.projectRepo(proj))
	}
	a.ensureDefaultProtected(proj.ID)
	_ = a.ensureGitRepo(proj)
	return proj, nil
}

// ensureProjectForApp cria o projeto do slug se ainda não existir. Sem
// admin no banco (sync de CI em teste vazio) é no-op — o próximo sync
// com owner, ou um POST no xadmin, completa.
func (a *App) ensureProjectForApp(app store.App) error {
	slug := strings.TrimSpace(app.Slug)
	if !store.ValidProjectSlug(slug) {
		return nil
	}
	org, ok := a.defaultOrganization()
	if !ok {
		return nil
	}
	var existing store.Project
	err := a.Store.DB.Where("organization_id = ? AND slug = ?", org.ID, slug).First(&existing).Error
	if err == nil {
		if existing.AppID == nil {
			existing.AppID = &app.ID
			return a.Store.DB.Save(&existing).Error
		}
		return nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	owner, ok := a.firstProjectOwner()
	if !ok {
		return nil
	}
	name := strings.TrimSpace(app.Name)
	if name == "" {
		name = slug
	}
	_, err = a.createProject(owner.ID, org, slug, name, strings.TrimSpace(app.Description), app.Visibility, app.Network, nil, false, nil)
	return err
}

func (a *App) firstProjectOwner() (store.User, bool) {
	var u store.User
	if err := a.Store.DB.Where("role = ?", store.RoleSuperAdmin).Order("id").First(&u).Error; err == nil {
		return u, true
	}
	if err := a.Store.DB.Where("role = ?", store.RoleAdmin).Order("id").First(&u).Error; err == nil {
		return u, true
	}
	return store.User{}, false
}

func (a *App) syncProjectGroupMembers(proj store.Project) error {
	var members []store.ProjectMember
	if err := a.Store.DB.Where("project_id = ?", proj.ID).Find(&members).Error; err != nil {
		return err
	}
	want := map[uint]struct{}{}
	for _, m := range members {
		want[m.UserID] = struct{}{}
		row := store.SocialGroupMember{GroupID: proj.SocialGroupID, UserID: m.UserID}
		if err := a.Store.DB.Where("group_id = ? AND user_id = ?", proj.SocialGroupID, m.UserID).
			FirstOrCreate(&row).Error; err != nil {
			return err
		}
	}
	var group store.SocialGroup
	if err := a.Store.DB.First(&group, proj.SocialGroupID).Error; err == nil {
		want[group.OwnerUserID] = struct{}{}
	}
	var current []store.SocialGroupMember
	if err := a.Store.DB.Where("group_id = ?", proj.SocialGroupID).Find(&current).Error; err != nil {
		return err
	}
	for _, m := range current {
		if _, ok := want[m.UserID]; ok {
			continue
		}
		if err := a.Store.DB.Delete(&m).Error; err != nil {
			return err
		}
	}
	return nil
}

func (a *App) ensureProjectFilesDir(repo string) error {
	org, slug, err := forge.SplitRepo(repo)
	if err != nil {
		return err
	}
	roots := a.driverRoots()
	if roots.ProjectsDir == "" {
		return nil
	}
	return os.MkdirAll(filepath.Join(roots.ProjectsDir, org, slug), 0o750)
}
