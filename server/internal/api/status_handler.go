package api

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

type statusResponse struct {
	APIVersion     int   `json:"api_version"`
	UptimeSeconds  int64 `json:"uptime_seconds"`
	ConnectedPeers int   `json:"connected_peers"`
	TotalPeers     int   `json:"total_peers"`
	// ReceiveBytesTotal / TransmitBytesTotal são o agregado de tráfego
	// reportado pelo kernel WireGuard em todos os peers (métrica básica
	// da Fase 8 — o painel já soma por dispositivo em /api/devices).
	ReceiveBytesTotal  int64 `json:"receive_bytes_total"`
	TransmitBytesTotal int64 `json:"transmit_bytes_total"`
}

// handleStatus expõe a saúde do servidor e o contrato de versão da API
// (ver PLAN.md §13.3). Público de propósito: o cliente desktop precisa
// checar a versão antes mesmo de ter feito login/enrollment.
// GET /api/status
func (a *App) handleStatus(c *gin.Context) {
	peers, err := a.WG.ListPeers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro consultando estado da interface WireGuard"})
		return
	}

	connected := 0
	var rx, tx int64
	recentThreshold := 3 * time.Minute
	for _, p := range peers {
		if p.LastHandshake != nil && time.Since(*p.LastHandshake) < recentThreshold {
			connected++
		}
		rx += p.ReceiveBytes
		tx += p.TransmitBytes
	}

	c.JSON(http.StatusOK, statusResponse{
		APIVersion:         APIVersion,
		UptimeSeconds:      int64(time.Since(StartedAt).Seconds()),
		ConnectedPeers:     connected,
		TotalPeers:         len(peers),
		ReceiveBytesTotal:  rx,
		TransmitBytesTotal: tx,
	})
}
