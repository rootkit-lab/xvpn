package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type auditLogResponse struct {
	ID        uint      `json:"id"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}

// handleListAudit lista as entradas de auditoria mais recentes.
// GET /api/audit?page=&per_page=&q=  — q filtra actor/action, nunca o detalhe
// (pode conter e-mail/ids; não é campo de busca).
func (a *App) handleListAudit(c *gin.Context) {
	p := parsePage(c)
	q := a.Store.DB.Model(&store.AuditLog{})
	if p.Q != "" {
		like := p.like()
		q = q.Where("actor LIKE ? OR action LIKE ?", like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	var logs []store.AuditLog
	if err := p.apply(q.Order("id desc")).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	resp := make([]auditLogResponse, 0, len(logs))
	for _, l := range logs {
		resp = append(resp, auditLogResponse{
			ID:        l.ID,
			Actor:     l.Actor,
			Action:    l.Action,
			Detail:    l.Detail,
			CreatedAt: l.CreatedAt,
		})
	}
	writePage(c, resp, total, p)
}
