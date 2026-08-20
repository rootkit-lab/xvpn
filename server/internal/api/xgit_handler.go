package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type forgeSettingsJSON struct {
	DefaultVisibility store.AppVisibility `json:"default_visibility"`
	DefaultNetwork    store.AppNetwork    `json:"default_network"`
	AllowMemberCreate bool                `json:"allow_member_create"`
	CloneHost         string              `json:"clone_host"`
}

type updateForgeSettingsRequest struct {
	DefaultVisibility *store.AppVisibility `json:"default_visibility"`
	DefaultNetwork    *store.AppNetwork    `json:"default_network"`
	AllowMemberCreate *bool                `json:"allow_member_create"`
}

func (a *App) loadForgeSettings() store.ForgeSettings {
	var s store.ForgeSettings
	if err := a.Store.DB.First(&s, 1).Error; err != nil {
		s = store.ForgeSettings{
			ID: 1, DefaultVisibility: store.AppVisibilityGlobal,
			DefaultNetwork: store.AppNetworkVPN,
		}
		_ = a.Store.DB.Create(&s).Error
	}
	return s
}

func (a *App) handleGetForgeSettings(c *gin.Context) {
	s := a.loadForgeSettings()
	c.JSON(http.StatusOK, forgeSettingsJSON{
		DefaultVisibility: s.DefaultVisibility,
		DefaultNetwork:    s.DefaultNetwork,
		AllowMemberCreate: s.AllowMemberCreate,
		CloneHost:         gitCloneHost,
	})
}

func (a *App) handleUpdateForgeSettings(c *gin.Context) {
	var req updateForgeSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	s := a.loadForgeSettings()
	if req.DefaultVisibility != nil {
		if !req.DefaultVisibility.Valid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "visibility inválida"})
			return
		}
		s.DefaultVisibility = *req.DefaultVisibility
	}
	if req.DefaultNetwork != nil {
		if !req.DefaultNetwork.Valid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "network inválida"})
			return
		}
		s.DefaultNetwork = *req.DefaultNetwork
	}
	if req.AllowMemberCreate != nil {
		s.AllowMemberCreate = *req.AllowMemberCreate
	}
	if err := a.Store.DB.Save(&s).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "xgit.settings", "")
	a.handleGetForgeSettings(c)
}

func (a *App) canCreateProject(user store.User) bool {
	if store.HasProduct(user.Role, user.Products, store.ProductForge) {
		return true
	}
	if user.Role == store.RoleMember && a.loadForgeSettings().AllowMemberCreate {
		return true
	}
	return false
}

func (a *App) handleListTree(c *gin.Context) {
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	ref := strings.TrimSpace(c.DefaultQuery("ref", "HEAD"))
	path := strings.TrimSpace(c.Query("path"))
	ents, err := forge.ListTree(a.gitDir(), a.projectRepo(proj), ref, path)
	if err != nil {
		if errors.Is(err, forge.ErrEmptyRepo) || errors.Is(err, forge.ErrBranchMissing) {
			c.JSON(http.StatusOK, gin.H{"items": []forge.TreeEntry{}, "ref": ref, "path": path, "commit_count": 0, "tags": []string{}, "languages": []forge.LangStat{}})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "ref ou path inválido"})
		return
	}
	payload := gin.H{"items": ents, "ref": ref, "path": path}
	if strings.Trim(path, "/") == "" {
		if n, err := forge.CountCommits(a.gitDir(), a.projectRepo(proj), ref); err == nil {
			payload["commit_count"] = n
		}
		if tags, err := forge.ListTags(a.gitDir(), a.projectRepo(proj)); err == nil {
			payload["tags"] = tags
		}
		if langs, err := forge.LanguageStats(a.gitDir(), a.projectRepo(proj), ref); err == nil {
			payload["languages"] = langs
		}
	}
	c.JSON(http.StatusOK, payload)
}

func (a *App) handleGetBlob(c *gin.Context) {
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	ref := strings.TrimSpace(c.DefaultQuery("ref", "HEAD"))
	path := strings.TrimSpace(c.Query("path"))
	body, binary, err := forge.ReadBlob(a.gitDir(), a.projectRepo(proj), ref, path)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "arquivo não encontrado"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": path, "ref": ref, "binary": binary, "content": body})
}

func (a *App) handleListCommits(c *gin.Context) {
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	ref := strings.TrimSpace(c.DefaultQuery("ref", "HEAD"))
	path := strings.TrimSpace(c.Query("path"))
	n, _ := strconv.Atoi(c.Query("n"))
	items, err := forge.ListCommits(a.gitDir(), a.projectRepo(proj), ref, path, n)
	if err != nil {
		if errors.Is(err, forge.ErrEmptyRepo) || errors.Is(err, forge.ErrBranchMissing) {
			c.JSON(http.StatusOK, gin.H{"items": []forge.CommitInfo{}})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "ref inválida"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) handleCreateProjectAuthed(c *gin.Context) {
	var user store.User
	if err := a.Store.DB.First(&user, callerUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	if !a.canCreateProject(user) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para criar repositório"})
		return
	}
	a.handleCreateProject(c)
}
