package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type projectEnvJSON struct {
	Name     string `json:"name"`
	Secret   bool   `json:"secret"`
	Value    string `json:"value,omitempty"`
	HasValue bool   `json:"has_value"`
}

type putProjectEnvsRequest struct {
	Items []putProjectEnvItem `json:"items"`
}

type putProjectEnvItem struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Secret bool   `json:"secret"`
}

func (a *App) handleGetProjectCodespaceEnvs(c *gin.Context) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	items, err := a.listProjectEnvJSON(proj, a.canMaintainerWrite(user, proj))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao ler ENVs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) handlePutProjectCodespaceEnvs(c *gin.Context) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	if !a.canMaintainerWrite(user, proj) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão"})
		return
	}
	var req putProjectEnvsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payload inválido"})
		return
	}
	if len(req.Items) > store.MaxProjectEnvs {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no máximo 32 ENVs"})
		return
	}
	var existing []store.ProjectEnv
	if err := a.Store.DB.Where("project_id = ?", proj.ID).Find(&existing).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao ler ENVs"})
		return
	}
	old := map[string]store.ProjectEnv{}
	for _, e := range existing {
		old[e.Name] = e
	}
	seen := map[string]struct{}{}
	next := make([]store.ProjectEnv, 0, len(req.Items))
	for _, item := range req.Items {
		name := strings.TrimSpace(item.Name)
		if !store.ValidProjectEnvName(name) || store.BlockedProjectEnvName(name) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "nome de ENV inválido"})
			return
		}
		if _, dup := seen[name]; dup {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ENV duplicado"})
			return
		}
		seen[name] = struct{}{}
		val := item.Value
		if item.Secret && strings.TrimSpace(val) == "" {
			if prev, ok := old[name]; ok {
				val = prev.Value
			} else {
				c.JSON(http.StatusBadRequest, gin.H{"error": "secret sem valor"})
				return
			}
		}
		if !store.ValidProjectEnvValue(val) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "valor de ENV inválido"})
			return
		}
		next = append(next, store.ProjectEnv{
			ProjectID: proj.ID,
			Name:      name,
			Value:     val,
			Secret:    item.Secret || store.IsLLMProjectEnv(name),
		})
	}
	if err := a.Store.DB.Where("project_id = ?", proj.ID).Delete(&store.ProjectEnv{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao gravar ENVs"})
		return
	}
	if len(next) > 0 {
		if err := a.Store.DB.Create(&next).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao gravar ENVs"})
			return
		}
	}
	_ = a.Store.LogAudit(callerUsername(c), "project.codespaces.envs", proj.Slug)
	items, err := a.listProjectEnvJSON(proj, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao ler ENVs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) listProjectEnvJSON(proj store.Project, reveal bool) ([]projectEnvJSON, error) {
	var rows []store.ProjectEnv
	if err := a.Store.DB.Where("project_id = ?", proj.ID).Order("name asc").Find(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]projectEnvJSON, 0, len(rows))
	for _, e := range rows {
		item := projectEnvJSON{Name: e.Name, Secret: e.Secret, HasValue: e.Value != ""}
		if reveal && !e.Secret {
			item.Value = e.Value
		}
		out = append(out, item)
	}
	return out, nil
}

func (a *App) codespaceRuntimeEnvs(projectID uint) map[string]string {
	var rows []store.ProjectEnv
	if err := a.Store.DB.Where("project_id = ?", projectID).Find(&rows).Error; err != nil {
		return nil
	}
	out := map[string]string{}
	for _, e := range rows {
		if store.IsLLMProjectEnv(e.Name) || store.BlockedProjectEnvName(e.Name) || !store.ValidProjectEnvName(e.Name) {
			continue
		}
		if !store.ValidProjectEnvValue(e.Value) {
			continue
		}
		out[e.Name] = e.Value
	}
	return out
}

func (a *App) loadProjectLLMConfig(projectID uint) (provider, baseURL, model, key string) {
	var rows []store.ProjectEnv
	if err := a.Store.DB.Where("project_id = ?", projectID).Find(&rows).Error; err != nil {
		return "", "", "", ""
	}
	for _, e := range rows {
		switch e.Name {
		case "XCS_LLM_PROVIDER":
			provider = e.Value
		case "XCS_LLM_BASE_URL":
			baseURL = e.Value
		case "XCS_LLM_MODEL":
			model = e.Value
		case "XCS_LLM_KEY":
			key = e.Value
		}
	}
	return provider, baseURL, model, key
}
