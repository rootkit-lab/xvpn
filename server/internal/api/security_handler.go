package api

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type secAlertJSON struct {
	ID        uint   `json:"id"`
	Kind      string `json:"kind"`
	Severity  string `json:"severity"`
	Title     string `json:"title"`
	Tool      string `json:"tool"`
	Status    string `json:"status"`
	JobNumber *uint  `json:"job_number,omitempty"`
}

type securityReportRequest struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

func (a *App) handleGetProjectSecurity(c *gin.Context) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	var rows []store.SecAlert
	_ = a.Store.DB.Where("project_id = ? AND status = ?", proj.ID, store.SecStatusOpen).Order("id desc").Limit(200).Find(&rows).Error
	items := make([]secAlertJSON, 0, len(rows))
	kinds := map[string]int{}
	for _, r := range rows {
		items = append(items, secAlertJSON{
			ID: r.ID, Kind: r.Kind, Severity: r.Severity, Title: r.Title, Tool: r.Tool, Status: r.Status, JobNumber: r.JobNumber,
		})
		kinds[r.Kind]++
	}
	policy := ""
	if body, _, err := forge.ReadBlob(a.gitDir(), a.projectRepo(proj), "HEAD", "SECURITY.md"); err == nil {
		policy = body
	}
	setup := func(kind, enabledIf string) string {
		if kinds[kind] > 0 {
			return "enabled"
		}
		if enabledIf != "" {
			return enabledIf
		}
		return "needs_setup"
	}
	c.JSON(http.StatusOK, gin.H{
		"alerts": items,
		"policy": policy,
		"setup": gin.H{
			"deps":   setup(store.SecKindDeps, ""),
			"code":   setup(store.SecKindCode, ""),
			"secret": "enabled",
		},
		"can_report": a.canReporterWrite(user, proj),
	})
}

func (a *App) handleCreateSecurityReport(c *gin.Context) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	if !a.canReporterWrite(user, proj) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão"})
		return
	}
	var req securityReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" || utf8.RuneCountInString(title) > maxIssueTitleRunes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "título inválido"})
		return
	}
	body := strings.TrimSpace(req.Body)
	if utf8.RuneCountInString(body) > maxIssueBodyRunes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo longo demais"})
		return
	}
	issue, err := a.insertIssue(proj, user, title, body, []string{"security"}, nil, nil, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.JSON(http.StatusCreated, a.issueJSON(proj, user, issue))
}

func (a *App) canSeeRestrictedIssue(user store.User, proj store.Project, issue store.Issue) bool {
	if !issue.Restricted {
		return true
	}
	if user.ID == issue.AuthorID {
		return true
	}
	return a.canMaintainerWrite(user, proj)
}

func (a *App) recordSecAlerts(proj store.Project, job *store.CiJob, log string) {
	alerts := parseCiSecurityLog(log, "")
	for i := range alerts {
		alerts[i].ProjectID = proj.ID
		if job != nil {
			n := job.Number
			alerts[i].JobNumber = &n
		}
		var n int64
		_ = a.Store.DB.Model(&store.SecAlert{}).Where(
			"project_id = ? AND tool = ? AND title = ? AND status = ?",
			proj.ID, alerts[i].Tool, alerts[i].Title, store.SecStatusOpen,
		).Count(&n).Error
		if n > 0 {
			continue
		}
		_ = a.Store.DB.Create(&alerts[i]).Error
	}
}

func (a *App) rejectSecretPush(proj store.Project, updates []forge.RefUpdate) bool {
	repo := a.projectRepo(proj)
	var titles []string
	reject := false
	for _, u := range updates {
		if forge.IsZeroOID(u.NewHex) {
			continue
		}
		if !forge.RevHasPrivateKey(a.gitDir(), repo, u.NewHex) {
			continue
		}
		titles = append(titles, "chave privada no push")
		reject = true
		_ = forge.ResetRef(a.gitDir(), repo, u.Ref, u.OldHex)
	}
	if len(titles) > 0 {
		a.recordSecretAlerts(proj, titles)
	}
	return reject
}

func (a *App) recordSecretAlerts(proj store.Project, titles []string) {
	for _, t := range titles {
		_ = a.Store.DB.Create(&store.SecAlert{
			ProjectID: proj.ID,
			Kind:      store.SecKindSecret,
			Severity:  "critical",
			Title:     t,
			Tool:      "receive-pack",
			Status:    store.SecStatusOpen,
		}).Error
	}
}
