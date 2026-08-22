package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/monitor"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type monitorCheckJSON struct {
	Slug      string    `json:"slug"`
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	Summary   string    `json:"summary"`
	Detail    string    `json:"detail,omitempty"`
	CheckedAt time.Time `json:"checked_at"`
}

type monitorNodeJSON struct {
	Hostname    string    `json:"hostname"`
	MeshID      uint      `json:"mesh_server_id"`
	Load1       float64   `json:"load1"`
	DiskUsedPct float64   `json:"disk_used_pct"`
	DiskAvailGB float64   `json:"disk_avail_gb"`
	ReportedAt  time.Time `json:"reported_at,omitempty"`
	WgIP        string    `json:"wg_ip,omitempty"`
	Role        string    `json:"role,omitempty"`
}

type monitorDashboardJSON struct {
	Checks    []monitorCheckJSON `json:"checks"`
	Nodes     []monitorNodeJSON  `json:"nodes"`
	UpdatedAt time.Time          `json:"updated_at"`
}

type monitorReportRequest struct {
	Load1       float64 `json:"load1"`
	DiskUsedPct float64 `json:"disk_used_pct"`
	DiskAvailGB float64 `json:"disk_avail_gb"`
}

func (a *App) handleXmonitorDashboard(c *gin.Context) {
	var checks []store.MonitorCheck
	_ = a.Store.DB.Order("name ASC").Find(&checks).Error
	out := monitorDashboardJSON{
		Checks: make([]monitorCheckJSON, 0, len(checks)),
		Nodes:  []monitorNodeJSON{},
	}
	var latest time.Time
	for _, row := range checks {
		out.Checks = append(out.Checks, monitorCheckJSON{
			Slug: row.Slug, Name: row.Name, Status: row.Status,
			Summary: row.Summary, Detail: row.Detail, CheckedAt: row.CheckedAt,
		})
		if row.CheckedAt.After(latest) {
			latest = row.CheckedAt
		}
	}
	var metrics []store.MonitorNodeMetric
	_ = a.Store.DB.Find(&metrics).Error
	metricByMesh := map[uint]store.MonitorNodeMetric{}
	for _, m := range metrics {
		metricByMesh[m.MeshServerID] = m
	}
	var mesh []store.MeshServer
	_ = a.Store.DB.Order("hostname ASC").Find(&mesh).Error
	for _, s := range mesh {
		node := monitorNodeJSON{
			Hostname: s.Hostname, MeshID: s.ID, WgIP: s.WgIP, Role: s.Role,
		}
		if m, ok := metricByMesh[s.ID]; ok {
			node.Load1 = m.Load1
			node.DiskUsedPct = m.DiskUsedPct
			node.DiskAvailGB = m.DiskAvailGB
			node.ReportedAt = m.ReportedAt
		}
		out.Nodes = append(out.Nodes, node)
	}
	if !latest.IsZero() {
		out.UpdatedAt = latest
	}
	c.JSON(http.StatusOK, out)
}

func (a *App) handleXmonitorRefresh(c *gin.Context) {
	runner := monitor.NewRunner(a.Store.DB, a.WG, a.gitDir())
	if _, err := runner.RunAll(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "xmonitor.refresh", "")
	a.handleXmonitorDashboard(c)
}

func (a *App) handleXmonitorReport(c *gin.Context) {
	srv, ok := a.authenticateSvcAgent(c)
	if !ok {
		return
	}
	var req monitorReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	row := store.MonitorNodeMetric{
		MeshServerID: srv.ID,
		Hostname:     srv.Hostname,
		Load1:        req.Load1,
		DiskUsedPct:  req.DiskUsedPct,
		DiskAvailGB:  req.DiskAvailGB,
		ReportedAt:   time.Now(),
	}
	var existing store.MonitorNodeMetric
	err := a.Store.DB.Where("mesh_server_id = ?", srv.ID).First(&existing).Error
	if err == nil {
		row.ID = existing.ID
		if err := a.Store.DB.Save(&row).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
			return
		}
	} else {
		if err := a.Store.DB.Create(&row).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) startMonitorPoller() {
	if a == nil || a.Store == nil {
		return
	}
	go func() {
		// primeira rodada após o boot estabilizar
		time.Sleep(15 * time.Second)
		ticker := time.NewTicker(2 * time.Minute)
		defer ticker.Stop()
		run := func() {
			runner := monitor.NewRunner(a.Store.DB, a.WG, a.gitDir())
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			if _, err := runner.RunAll(ctx); err != nil {
				// best-effort — painel mostra último estado bom
				return
			}
		}
		run()
		for range ticker.C {
			run()
		}
	}()
}
