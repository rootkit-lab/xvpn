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

// auditLogLimit é o número máximo de entradas devolvidas por requisição.
// Não há paginação ainda — suficiente para o volume de ações administrativas
// de um painel de uso interno; revisar se isso crescer.
const auditLogLimit = 200

// handleListAudit lista as entradas de auditoria mais recentes.
// GET /api/audit
func (a *App) handleListAudit(c *gin.Context) {
	var logs []store.AuditLog
	if err := a.Store.DB.Order("id desc").Limit(auditLogLimit).Find(&logs).Error; err != nil {
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
	c.JSON(http.StatusOK, resp)
}
