package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type orgTeamJSON struct {
	Slug      string                 `json:"slug"`
	Name      string                 `json:"name"`
	Parent    string                 `json:"parent,omitempty"`
	Repos     []projectResponse      `json:"repos"`
	Templates []workflowTemplateJSON `json:"templates,omitempty"`
}

type orgHomeJSON struct {
	Slug        string            `json:"slug"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Teams       []orgTeamJSON     `json:"teams"`
	Root        []projectResponse `json:"root"`
}

type orgTeamMemberJSON struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
}

func (a *App) loadOrgParam(c *gin.Context) (store.ForgeOrganization, store.User, bool) {
	var user store.User
	if err := a.Store.DB.First(&user, callerUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return store.ForgeOrganization{}, user, false
	}
	org, ok := a.loadOrganization(c.Param("org"))
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "organização não encontrada"})
		return store.ForgeOrganization{}, user, false
	}
	return org, user, true
}

func (a *App) handleGetOrg(c *gin.Context) {
	org, user, ok := a.loadOrgParam(c)
	if !ok {
		return
	}
	var teams []store.OrgTeam
	_ = a.Store.DB.Where("organization_id = ?", org.ID).Order("id").Find(&teams).Error
	parentSlug := map[uint]string{}
	for i := range teams {
		parentSlug[teams[i].ID] = teams[i].Slug
	}
	out := orgHomeJSON{
		Slug: org.Slug, Name: org.Name, Description: org.Description,
		Teams: make([]orgTeamJSON, 0, len(teams)),
		Root:  []projectResponse{},
	}
	var projs []store.Project
	_ = a.Store.DB.Where("organization_id = ? AND archived_at IS NULL", org.ID).Order("slug").Find(&projs).Error
	byTeam := map[uint][]projectResponse{}
	for i := range projs {
		if !a.canSeeProject(user, projs[i]) {
			continue
		}
		card := a.decorateProjectCard(user, projs[i], true)
		if projs[i].TeamID == nil || *projs[i].TeamID == 0 {
			out.Root = append(out.Root, card)
			continue
		}
		byTeam[*projs[i].TeamID] = append(byTeam[*projs[i].TeamID], card)
	}
	for i := range teams {
		item := orgTeamJSON{
			Slug: teams[i].Slug, Name: teams[i].Name,
			Repos: byTeam[teams[i].ID],
		}
		if item.Repos == nil {
			item.Repos = []projectResponse{}
		}
		if teams[i].ParentID != nil {
			item.Parent = parentSlug[*teams[i].ParentID]
		}
		if teams[i].Slug == "workflows" {
			item.Templates = openWorkflowTemplates()
		}
		out.Teams = append(out.Teams, item)
	}
	c.JSON(http.StatusOK, out)
}

func (a *App) handleListTeamMembers(c *gin.Context) {
	org, user, ok := a.loadOrgParam(c)
	if !ok {
		return
	}
	team, found := a.orgTeam(org.ID, c.Param("team"))
	if !found || !a.canSeeTeam(user, org, team) {
		c.JSON(http.StatusNotFound, gin.H{"error": "time não encontrado"})
		return
	}
	var rows []store.OrgTeamMember
	_ = a.Store.DB.Where("team_id = ?", team.ID).Order("id").Find(&rows).Error
	items := make([]orgTeamMemberJSON, 0, len(rows))
	for _, row := range rows {
		var u store.User
		if err := a.Store.DB.First(&u, row.UserID).Error; err != nil {
			continue
		}
		items = append(items, orgTeamMemberJSON{UserID: u.ID, Username: u.Username})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) handleAddTeamMember(c *gin.Context) {
	org, user, ok := a.loadOrgParam(c)
	if !ok {
		return
	}
	if !a.canManageOrg(user, org.ID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para gerir o time"})
		return
	}
	team, found := a.orgTeam(org.ID, c.Param("team"))
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "time não encontrado"})
		return
	}
	var req struct {
		UserID uint `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.UserID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id obrigatório"})
		return
	}
	var member store.User
	if err := a.Store.DB.First(&member, req.UserID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "usuário não encontrado"})
		return
	}
	a.ensureTeamMember(team.ID, member.ID)
	c.JSON(http.StatusCreated, orgTeamMemberJSON{UserID: member.ID, Username: member.Username})
}

func (a *App) handleRemoveTeamMember(c *gin.Context) {
	org, user, ok := a.loadOrgParam(c)
	if !ok {
		return
	}
	if !a.canManageOrg(user, org.ID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para gerir o time"})
		return
	}
	team, found := a.orgTeam(org.ID, c.Param("team"))
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "time não encontrado"})
		return
	}
	uid, err := strconv.ParseUint(strings.TrimSpace(c.Param("userID")), 10, 64)
	if err != nil || uid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id inválido"})
		return
	}
	if err := a.Store.DB.Where("team_id = ? AND user_id = ?", team.ID, uint(uid)).
		Delete(&store.OrgTeamMember{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
