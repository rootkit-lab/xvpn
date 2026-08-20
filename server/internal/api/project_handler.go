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

	"github.com/rootkit-lab/xvpn/server/internal/pkgexamples"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type createProjectRequest struct {
	Slug         string              `json:"slug"`
	Name         string              `json:"name"`
	Description  string              `json:"description"`
	FilesEnabled bool                `json:"files_enabled"`
	Visibility   store.AppVisibility `json:"visibility"`
	Network      store.AppNetwork    `json:"network"`
	Runners      []string            `json:"runners"`
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
	Slug          string                  `json:"slug"`
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
}

func (a *App) canSeeProject(user store.User, proj store.Project) bool {
	if user.Role.Rank() >= store.RoleViewer.Rank() {
		return true
	}
	var n int64
	_ = a.Store.DB.Model(&store.ProjectMember{}).
		Where("project_id = ? AND user_id = ?", proj.ID, user.ID).Count(&n).Error
	return n > 0
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
	slug := strings.TrimSpace(c.Param("slug"))
	var user store.User
	if err := a.Store.DB.First(&user, callerUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return store.Project{}, user, false
	}
	var proj store.Project
	if err := a.Store.DB.Where("slug = ?", slug).First(&proj).Error; err != nil {
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
	out := projectResponse{
		Slug:          proj.Slug,
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
		var ids []uint
		_ = a.Store.DB.Model(&store.ProjectMember{}).Where("user_id = ?", user.ID).Pluck("project_id", &ids).Error
		if len(ids) == 0 {
			c.JSON(http.StatusOK, gin.H{"items": []projectResponse{}})
			return
		}
		q = q.Where("id IN ?", ids)
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
	var existing store.Project
	if err := a.Store.DB.Where("slug = ?", slug).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "já existe um projeto com este slug"})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	proj, err := a.createProject(user.ID, slug, name, strings.TrimSpace(req.Description), vis, net, req.Runners, req.FilesEnabled)
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
		_ = a.ensureProjectFilesDir(proj.Slug)
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

func (a *App) createProject(ownerID uint, slug, name, desc string, vis store.AppVisibility, net store.AppNetwork, runners []string, files bool) (store.Project, error) {
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
		Slug:          slug,
		Name:          name,
		Description:   desc,
		AppID:         appID,
		SocialGroupID: g.ID,
		FilesEnabled:  files,
		Visibility:    vis,
		Network:       net,
		Runners:       runners,
	}
	if err := a.Store.DB.Create(&proj).Error; err != nil {
		return store.Project{}, err
	}
	if err := a.Store.DB.Create(&store.ProjectMember{
		ProjectID: proj.ID, UserID: ownerID, Role: store.ProjectRoleOwner,
	}).Error; err != nil {
		return store.Project{}, err
	}
	if files {
		_ = a.ensureProjectFilesDir(slug)
	}
	a.ensureDefaultProtected(proj.ID)
	_ = a.ensureGitRepo(slug)
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
	var existing store.Project
	err := a.Store.DB.Where("slug = ?", slug).First(&existing).Error
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
	_, err = a.createProject(owner.ID, slug, name, strings.TrimSpace(app.Description), app.Visibility, app.Network, nil, false)
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

func (a *App) ensureProjectFilesDir(slug string) error {
	if !store.ValidProjectSlug(slug) {
		return errors.New("slug inválido")
	}
	roots := a.driverRoots()
	if roots.ProjectsDir == "" {
		return nil
	}
	return os.MkdirAll(filepath.Join(roots.ProjectsDir, slug), 0o750)
}
