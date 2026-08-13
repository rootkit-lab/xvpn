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

// statusCacheTTL é por quanto tempo uma resposta calculada é reaproveitada
// antes de consultar o wgctrl/kernel de novo — GET /api/status é público e
// chamado em polling (painel + cliente desktop), então sem isso cada poll
// simultâneo batia direto no kernel à toa. 2s é imperceptível para quem
// olha o painel, mas já absorve rajadas de várias abas/dispositivos
// consultando quase ao mesmo tempo (ver ROADMAP.md Fase 9).
const statusCacheTTL = 2 * time.Second

// handleStatus expõe a saúde do servidor e o contrato de versão da API
// (ver PLAN.md §13.3). Público de propósito: o cliente desktop precisa
// checar a versão antes mesmo de ter feito login/enrollment.
// GET /api/status
func (a *App) handleStatus(c *gin.Context) {
	if resp, ok := a.cachedStatus(); ok {
		c.JSON(http.StatusOK, resp)
		return
	}

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

	resp := statusResponse{
		APIVersion:         APIVersion,
		UptimeSeconds:      int64(time.Since(StartedAt).Seconds()),
		ConnectedPeers:     connected,
		TotalPeers:         len(peers),
		ReceiveBytesTotal:  rx,
		TransmitBytesTotal: tx,
	}
	a.setStatusCache(resp)
	c.JSON(http.StatusOK, resp)
}

func (a *App) cachedStatus() (statusResponse, bool) {
	a.statusCacheMu.Lock()
	defer a.statusCacheMu.Unlock()
	if a.statusCacheAt.IsZero() || time.Since(a.statusCacheAt) > statusCacheTTL {
		return statusResponse{}, false
	}
	return a.statusCacheResp, true
}

func (a *App) setStatusCache(resp statusResponse) {
	a.statusCacheMu.Lock()
	defer a.statusCacheMu.Unlock()
	a.statusCacheAt = time.Now()
	a.statusCacheResp = resp
}
