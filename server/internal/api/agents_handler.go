package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

func (a *App) handleListProjectAgents(c *gin.Context) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	q := a.Store.DB.Where("project_id = ?", proj.ID)
	maintainer := a.canMaintainerWrite(user, proj)
	if !maintainer {
		q = q.Where("user_id = ?", user.ID)
	}
	filter := strings.TrimSpace(c.Query("filter"))
	switch filter {
	case "mine":
		q = q.Where("user_id = ?", user.ID)
	case "attention":
		if !maintainer {
			q = q.Where("user_id = ?", user.ID)
		}
		cutoff := time.Now().Add(-24 * time.Hour)
		q = q.Where("status = ? OR (status = ? AND (last_active_at IS NULL OR last_active_at < ?))",
			store.CodespaceError, store.CodespaceRunning, cutoff)
	case "active":
		q = q.Where("status IN ?", []string{store.CodespaceRunning, store.CodespaceStarting})
	case "completed":
		q = q.Where("status = ?", store.CodespaceStopped)
	}
	var rows []store.CodeSpace
	if err := q.Order("created_at desc").Limit(80).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]codespaceJSON, 0, len(rows))
	mine := 0
	attention := 0
	cutoff := time.Now().Add(-24 * time.Hour)
	for i := range rows {
		if rows[i].UserID == user.ID {
			a.maybeIdleStop(&rows[i])
		}
		item := a.codespaceJSON(user, proj, rows[i])
		items = append(items, item)
		if rows[i].UserID == user.ID {
			mine++
		}
		if rows[i].Status == store.CodespaceError || (rows[i].Status == store.CodespaceRunning && (rows[i].LastActiveAt == nil || rows[i].LastActiveAt.Before(cutoff))) {
			attention++
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"items":         items,
		"mine":          mine,
		"attention":     attention,
		"see_all":       maintainer,
		"settings_path": "settings",
	})
}
