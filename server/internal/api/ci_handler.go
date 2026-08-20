package api

import (
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

const (
	ciTriggerPush  = "push"
	ciTriggerMR    = "mr"
	ciWorkflow     = "ci"
	ciWorkflowPath = ".xvpn-ci.sh"
	maxCiLogBytes  = 2 << 20
	ciURLHint      = "http://10.66.66.1:8080"
)

type ciJobStepJSON struct {
	Name   string            `json:"name"`
	Status store.CiJobStatus `json:"status"`
}

type ciWorkflowJSON struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type ciJobJSON struct {
	Number             uint              `json:"number"`
	Workflow           string            `json:"workflow"`
	Title              string            `json:"title"`
	Event              string            `json:"event"`
	Trigger            string            `json:"trigger"`
	Ref                string            `json:"ref"`
	Branch             string            `json:"branch"`
	SHA                string            `json:"sha"`
	Actor              string            `json:"actor,omitempty"`
	MergeRequestNumber *uint             `json:"merge_request_number,omitempty"`
	Status             store.CiJobStatus `json:"status"`
	Runner             string            `json:"runner,omitempty"`
	HasLog             bool              `json:"has_log"`
	HasArtifact        bool              `json:"has_artifact"`
	Error              string            `json:"error,omitempty"`
	Jobs               []ciJobStepJSON   `json:"jobs"`
	DurationMs         *int64            `json:"duration_ms,omitempty"`
	CanApprove         bool              `json:"can_approve,omitempty"`
	CanRerun           bool              `json:"can_rerun,omitempty"`
	CanCancel          bool              `json:"can_cancel,omitempty"`
	StartedAt          *time.Time        `json:"started_at,omitempty"`
	FinishedAt         *time.Time        `json:"finished_at,omitempty"`
	CreatedAt          time.Time         `json:"created_at"`
}

type ciRunnerJSON struct {
	Hostname string   `json:"hostname"`
	Name     string   `json:"name"`
	Status   string   `json:"status"`
	Labels   []string `json:"labels,omitempty"`
	WgIP     string   `json:"wg_ip,omitempty"`
}

type ciClaimJSON struct {
	ID uint `json:"id"`
	ciJobJSON
	Slug     string `json:"slug"`
	CloneURL string `json:"clone_url"`
}

type ciFinishRequest struct {
	Status string `json:"status"`
	Log    string `json:"log"`
	Error  string `json:"error"`
}

func (a *App) RequireVPN() gin.HandlerFunc {
	var subnet *net.IPNet
	if _, parsed, err := net.ParseCIDR(a.Config.WireGuardAllowedSubnet); err == nil {
		subnet = parsed
	}
	return func(c *gin.Context) {
		if subnet == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "sub-rede da VPN mal configurada no servidor"})
			return
		}
		ip := net.ParseIP(c.RemoteIP())
		if ip == nil || !subnet.Contains(ip) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "esta rota só responde na VPN"})
			return
		}
		c.Next()
	}
}

func ciEvent(trigger string) string {
	if trigger == ciTriggerMR {
		return "pull_request"
	}
	return "push"
}

func ciBranch(ref string) string {
	return strings.TrimPrefix(strings.TrimSpace(ref), "refs/heads/")
}

func (a *App) ciJobTitle(proj store.Project, job store.CiJob) string {
	if job.MergeRequestNumber != nil {
		var mr store.MergeRequest
		if a.Store.DB.Where("project_id = ? AND number = ?", proj.ID, *job.MergeRequestNumber).First(&mr).Error == nil && mr.Title != "" {
			return mr.Title
		}
	}
	if items, err := forge.ListCommits(a.gitDir(), a.projectRepo(proj), job.SHA, "", 1); err == nil && len(items) > 0 && items[0].Subject != "" {
		return items[0].Subject
	}
	if job.Trigger == ciTriggerMR && job.MergeRequestNumber != nil {
		return fmt.Sprintf("Merge request !%d", *job.MergeRequestNumber)
	}
	return fmt.Sprintf("ci #%d", job.Number)
}

func (a *App) ciJobDurationMs(job store.CiJob) *int64 {
	if job.StartedAt == nil {
		return nil
	}
	end := time.Now()
	if job.FinishedAt != nil {
		end = *job.FinishedAt
	}
	ms := end.Sub(*job.StartedAt).Milliseconds()
	if ms < 0 {
		ms = 0
	}
	return &ms
}

func (a *App) canApproveCi(user store.User, proj store.Project) bool {
	if store.HasProduct(user.Role, user.Products, store.ProductForge) {
		return true
	}
	role, ok := a.projectMemberRole(user, proj)
	return ok && role.Rank() >= store.ProjectRoleMaintainer.Rank()
}

func (a *App) ciJobJSON(job store.CiJob) ciJobJSON {
	return a.ciJobJSONFor(store.Project{}, store.User{}, job)
}

func (a *App) ciJobJSONFor(proj store.Project, user store.User, job store.CiJob) ciJobJSON {
	workflow := strings.TrimSpace(job.Workflow)
	if workflow == "" {
		workflow = ciWorkflow
	}
	out := ciJobJSON{
		Number:             job.Number,
		Workflow:           workflow,
		Event:              ciEvent(job.Trigger),
		Trigger:            job.Trigger,
		Ref:                job.Ref,
		Branch:             ciBranch(job.Ref),
		SHA:                job.SHA,
		Actor:              job.Actor,
		MergeRequestNumber: job.MergeRequestNumber,
		Status:             job.Status,
		HasLog:             job.LogRel != "",
		HasArtifact:        job.ArtifactRel != "",
		Error:              job.Error,
		Jobs:               []ciJobStepJSON{{Name: ciWorkflow, Status: job.Status}},
		DurationMs:         a.ciJobDurationMs(job),
		StartedAt:          job.StartedAt,
		FinishedAt:         job.FinishedAt,
		CreatedAt:          job.CreatedAt,
	}
	if proj.ID != 0 {
		out.Title = a.ciJobTitle(proj, job)
	} else {
		out.Title = fmt.Sprintf("ci #%d", job.Number)
	}
	if user.ID != 0 {
		ok := a.canApproveCi(user, proj)
		out.CanApprove = ok && job.Status == store.CiAwaitingApproval
		out.CanRerun = ok && job.Status.Terminal()
		out.CanCancel = ok && !job.Status.Terminal()
	}
	if job.RunnerID != nil {
		var s store.MeshServer
		if a.Store.DB.First(&s, *job.RunnerID).Error == nil {
			out.Runner = s.Hostname
		}
	}
	return out
}

func (a *App) enqueuePushJobs(proj store.Project, updates []forge.RefUpdate, actor string) {
	for _, u := range updates {
		if forge.IsZeroOID(u.NewHex) || !strings.HasPrefix(u.Ref, "refs/heads/") {
			continue
		}
		a.enqueueCiJobAs(proj, ciTriggerPush, u.Ref, u.NewHex, nil, actor, store.CiPending)
	}
}

func (a *App) enqueueCiJob(proj store.Project, trigger, ref, sha string, mrNumber *uint) {
	a.enqueueCiJobAs(proj, trigger, ref, sha, mrNumber, "", store.CiPending)
}

func (a *App) enqueueCiJobAs(proj store.Project, trigger, ref, sha string, mrNumber *uint, actor string, status store.CiJobStatus) *store.CiJob {
	sha = strings.TrimSpace(sha)
	ref = strings.TrimSpace(ref)
	if sha == "" || ref == "" || forge.IsZeroOID(sha) {
		return nil
	}
	if !status.Valid() {
		status = store.CiPending
	}
	var last store.CiJob
	number := uint(1)
	if err := a.Store.DB.Where("project_id = ?", proj.ID).Order("number desc").First(&last).Error; err == nil {
		number = last.Number + 1
	}
	job := store.CiJob{
		ProjectID:          proj.ID,
		Number:             number,
		Trigger:            trigger,
		Ref:                ref,
		SHA:                sha,
		MergeRequestNumber: mrNumber,
		Workflow:           ciWorkflow,
		Actor:              strings.TrimSpace(actor),
		Status:             status,
	}
	if err := a.Store.DB.Create(&job).Error; err != nil {
		return nil
	}
	_ = a.ensureProjectFilesDir(a.projectRepo(proj))
	_ = a.Store.LogAudit("system", "project.ci.enqueue", fmt.Sprintf("%s#%d %s", proj.Slug, job.Number, trigger))
	return &job
}

func (a *App) runnerMatchesProject(srv store.MeshServer, proj store.Project) bool {
	if srv.Role != store.ServerRoleRunner {
		return false
	}
	wanted := proj.Runners
	if len(wanted) == 0 {
		return true
	}
	labels := append([]string{}, srv.Labels...)
	labels = append(labels, srv.Hostname, srv.Name)
	have := map[string]struct{}{}
	for _, l := range labels {
		have[strings.ToLower(strings.TrimSpace(l))] = struct{}{}
	}
	for _, w := range wanted {
		w = strings.ToLower(strings.TrimSpace(w))
		if w == "" {
			continue
		}
		if _, ok := have[w]; ok {
			return true
		}
	}
	return false
}

func (a *App) runnerByToken(token string) (store.MeshServer, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return store.MeshServer{}, false
	}
	var rows []store.MeshServer
	if err := a.Store.DB.Where("role = ? AND runner_token_hash <> ''", store.ServerRoleRunner).Find(&rows).Error; err != nil {
		return store.MeshServer{}, false
	}
	for _, s := range rows {
		ok, err := auth.VerifyPassword(s.RunnerTokenHash, token)
		if err == nil && ok {
			return s, true
		}
	}
	return store.MeshServer{}, false
}

func (a *App) authenticateRunner(c *gin.Context) (store.MeshServer, bool) {
	raw := strings.TrimSpace(c.GetHeader("Authorization"))
	token := ""
	if strings.HasPrefix(strings.ToLower(raw), "bearer ") {
		token = strings.TrimSpace(raw[7:])
	}
	if token == "" {
		if _, pass, ok := c.Request.BasicAuth(); ok {
			token = pass
		}
	}
	srv, ok := a.runnerByToken(token)
	if !ok {
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "credenciais inválidas"})
		return store.MeshServer{}, false
	}
	ip := net.ParseIP(c.RemoteIP())
	if ip == nil || !a.runnerIPOK(srv, ip) {
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "runner fora do peer esperado"})
		return store.MeshServer{}, false
	}
	return srv, true
}

func (a *App) runnerIPOK(srv store.MeshServer, ip net.IP) bool {
	if wg := net.ParseIP(strings.TrimSpace(srv.WgIP)); wg != nil && wg.Equal(ip) {
		return true
	}
	if srv.DeviceID == nil {
		return false
	}
	var d store.Device
	if err := a.Store.DB.First(&d, *srv.DeviceID).Error; err != nil {
		return false
	}
	host, _, _ := net.ParseCIDR(d.AllowedIP)
	if host == nil {
		host = net.ParseIP(strings.TrimSuffix(d.AllowedIP, "/32"))
	}
	return host != nil && host.Equal(ip)
}

func (a *App) authenticateGitRunner(c *gin.Context) (store.MeshServer, bool) {
	user, pass, ok := c.Request.BasicAuth()
	if !ok || !strings.EqualFold(user, "runner") {
		return store.MeshServer{}, false
	}
	return a.runnerByToken(pass)
}

func (a *App) handleListCiJobs(c *gin.Context) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	q := a.Store.DB.Where("project_id = ?", proj.ID)
	if raw := strings.TrimSpace(c.Query("mr")); raw != "" {
		n, err := strconv.ParseUint(raw, 10, 32)
		if err != nil || n == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "mr inválido"})
			return
		}
		q = q.Where("merge_request_number = ?", n)
	}
	if wf := strings.TrimSpace(c.Query("workflow")); wf != "" && wf != ciWorkflow {
		c.JSON(http.StatusOK, gin.H{
			"items":     []ciJobJSON{},
			"workflows": []ciWorkflowJSON{{Name: ciWorkflow, Path: ciWorkflowPath}},
		})
		return
	}
	var rows []store.CiJob
	if err := q.Order("number desc").Limit(80).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]ciJobJSON, 0, len(rows))
	for _, j := range rows {
		items = append(items, a.ciJobJSONFor(proj, user, j))
	}
	c.JSON(http.StatusOK, gin.H{
		"items":     items,
		"workflows": []ciWorkflowJSON{{Name: ciWorkflow, Path: ciWorkflowPath}},
	})
}

func (a *App) loadCiJob(c *gin.Context) (store.Project, store.User, store.CiJob, bool) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return store.Project{}, store.User{}, store.CiJob{}, false
	}
	var n uint
	if _, err := parseUintParam(c, "n", &n); err != nil || n == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "job inválido"})
		return proj, user, store.CiJob{}, false
	}
	var job store.CiJob
	if err := a.Store.DB.Where("project_id = ? AND number = ?", proj.ID, n).First(&job).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job não encontrado"})
		return proj, user, store.CiJob{}, false
	}
	return proj, user, job, true
}

func (a *App) handleGetCiJob(c *gin.Context) {
	proj, user, job, ok := a.loadCiJob(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, a.ciJobJSONFor(proj, user, job))
}

func (a *App) handleGetCiJobLog(c *gin.Context) {
	proj, _, job, ok := a.loadCiJob(c)
	if !ok {
		return
	}
	body, err := a.readCiFile(a.projectRepo(proj), job.LogRel)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "log indisponível"})
		return
	}
	c.Header("Content-Type", "text/plain; charset=utf-8")
	c.String(http.StatusOK, body)
}

func (a *App) handleGetCiJobArtifact(c *gin.Context) {
	proj, _, job, ok := a.loadCiJob(c)
	if !ok {
		return
	}
	path, err := a.ciAbs(a.projectRepo(proj), job.ArtifactRel)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "artifact indisponível"})
		return
	}
	c.FileAttachment(path, filepath.Base(path))
}

func (a *App) handleCancelCiJob(c *gin.Context) {
	proj, user, job, ok := a.loadCiJob(c)
	if !ok {
		return
	}
	if !store.HasProduct(user.Role, user.Products, store.ProductForge) {
		role, member := a.projectMemberRole(user, proj)
		if !member || role.Rank() < store.ProjectRoleMaintainer.Rank() {
			c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para cancelar"})
			return
		}
	}
	if job.Status.Terminal() {
		c.JSON(http.StatusConflict, gin.H{"error": "job já encerrado"})
		return
	}
	now := time.Now()
	job.Status = store.CiCanceled
	job.FinishedAt = &now
	if err := a.Store.DB.Save(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "project.ci.cancel", fmt.Sprintf("%s#%d", proj.Slug, job.Number))
	c.JSON(http.StatusOK, a.ciJobJSONFor(proj, user, job))
}

func (a *App) handleApproveCiJob(c *gin.Context) {
	proj, user, job, ok := a.loadCiJob(c)
	if !ok {
		return
	}
	if !a.canApproveCi(user, proj) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para aprovar"})
		return
	}
	if job.Status != store.CiAwaitingApproval {
		c.JSON(http.StatusConflict, gin.H{"error": "run não está aguardando aprovação"})
		return
	}
	job.Status = store.CiPending
	if err := a.Store.DB.Save(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "project.ci.approve", fmt.Sprintf("%s#%d", proj.Slug, job.Number))
	c.JSON(http.StatusOK, a.ciJobJSONFor(proj, user, job))
}

func (a *App) handleRerunCiJob(c *gin.Context) {
	proj, user, job, ok := a.loadCiJob(c)
	if !ok {
		return
	}
	if !a.canApproveCi(user, proj) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para reexecutar"})
		return
	}
	if !job.Status.Terminal() {
		c.JSON(http.StatusConflict, gin.H{"error": "só é possível reexecutar um run encerrado"})
		return
	}
	next := a.enqueueCiJobAs(proj, job.Trigger, job.Ref, job.SHA, job.MergeRequestNumber, user.Username, store.CiPending)
	if next == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "project.ci.rerun", fmt.Sprintf("%s#%d→%d", proj.Slug, job.Number, next.Number))
	c.JSON(http.StatusCreated, a.ciJobJSONFor(proj, user, *next))
}

func (a *App) handleListProjectRunners(c *gin.Context) {
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	var rows []store.MeshServer
	if err := a.Store.DB.Where("role = ?", store.ServerRoleRunner).Order("hostname").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]ciRunnerJSON, 0)
	for _, s := range rows {
		if !a.runnerMatchesProject(s, proj) {
			continue
		}
		items = append(items, ciRunnerJSON{
			Hostname: s.Hostname,
			Name:     s.Name,
			Status:   s.Status,
			Labels:   s.Labels,
			WgIP:     s.WgIP,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) handleCiClaim(c *gin.Context) {
	srv, ok := a.authenticateRunner(c)
	if !ok {
		return
	}
	var pending []store.CiJob
	if err := a.Store.DB.Where("status = ?", store.CiPending).Order("id").Limit(50).Find(&pending).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	for _, job := range pending {
		var proj store.Project
		if err := a.Store.DB.First(&proj, job.ProjectID).Error; err != nil {
			continue
		}
		if !a.runnerMatchesProject(srv, proj) {
			continue
		}
		now := time.Now()
		rid := srv.ID
		job.Status = store.CiRunning
		job.RunnerID = &rid
		job.StartedAt = &now
		if err := a.Store.DB.Save(&job).Error; err != nil {
			continue
		}
		c.JSON(http.StatusOK, ciClaimJSON{
			ID:        job.ID,
			ciJobJSON: a.ciJobJSON(job),
			Slug:      proj.Slug,
			CloneURL:  a.projectCloneURL(proj),
		})
		return
	}
	c.JSON(http.StatusNoContent, nil)
}

func (a *App) loadRunnerJob(c *gin.Context, srv store.MeshServer) (store.Project, store.CiJob, bool) {
	var id uint
	if _, err := parseUintParam(c, "id", &id); err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return store.Project{}, store.CiJob{}, false
	}
	var job store.CiJob
	if err := a.Store.DB.First(&job, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "job não encontrado"})
		return store.Project{}, store.CiJob{}, false
	}
	if job.RunnerID == nil || *job.RunnerID != srv.ID || job.Status != store.CiRunning {
		c.JSON(http.StatusForbidden, gin.H{"error": "job de outro runner"})
		return store.Project{}, store.CiJob{}, false
	}
	var proj store.Project
	if err := a.Store.DB.First(&proj, job.ProjectID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "projeto não encontrado"})
		return store.Project{}, store.CiJob{}, false
	}
	return proj, job, true
}

func (a *App) handleCiLog(c *gin.Context) {
	srv, ok := a.authenticateRunner(c)
	if !ok {
		return
	}
	proj, job, ok := a.loadRunnerJob(c, srv)
	if !ok {
		return
	}
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, maxCiLogBytes+1))
	if err != nil || len(body) > maxCiLogBytes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "log inválido"})
		return
	}
	rel := fmt.Sprintf("ci/%d/job.log", job.Number)
	if err := a.writeCiFile(a.projectRepo(proj), rel, body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	job.LogRel = rel
	_ = a.Store.DB.Save(&job).Error
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) handleCiFinish(c *gin.Context) {
	srv, ok := a.authenticateRunner(c)
	if !ok {
		return
	}
	proj, job, ok := a.loadRunnerJob(c, srv)
	if !ok {
		return
	}
	var req ciFinishRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	st := store.CiJobStatus(strings.TrimSpace(req.Status))
	if st != store.CiSuccess && st != store.CiFailed {
		c.JSON(http.StatusBadRequest, gin.H{"error": "status inválido"})
		return
	}
	if req.Log != "" {
		rel := fmt.Sprintf("ci/%d/job.log", job.Number)
		if err := a.writeCiFile(a.projectRepo(proj), rel, []byte(req.Log)); err == nil {
			job.LogRel = rel
		}
	}
	now := time.Now()
	job.Status = st
	job.FinishedAt = &now
	job.Error = strings.TrimSpace(req.Error)
	if err := a.Store.DB.Save(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.JSON(http.StatusOK, a.ciJobJSON(job))
}

func (a *App) handleCiArtifact(c *gin.Context) {
	srv, ok := a.authenticateRunner(c)
	if !ok {
		return
	}
	proj, job, ok := a.loadRunnerJob(c, srv)
	if !ok {
		return
	}
	fh, err := c.FormFile("file")
	if err != nil || fh == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "arquivo obrigatório"})
		return
	}
	src, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "arquivo inválido"})
		return
	}
	defer src.Close()
	body, err := io.ReadAll(io.LimitReader(src, 32<<20+1))
	if err != nil || len(body) > 32<<20 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "artifact grande demais"})
		return
	}
	name := filepath.Base(fh.Filename)
	if name == "." || name == "/" || strings.Contains(name, "..") {
		name = "artifact.bin"
	}
	rel := fmt.Sprintf("ci/%d/%s", job.Number, name)
	if err := a.writeCiFile(a.projectRepo(proj), rel, body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	job.ArtifactRel = rel
	_ = a.Store.DB.Save(&job).Error
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) handleIssueRunnerToken(c *gin.Context) {
	s, ok := a.loadMeshServer(c)
	if !ok {
		return
	}
	if s.Role != store.ServerRoleRunner {
		c.JSON(http.StatusBadRequest, gin.H{"error": "só host com papel runner"})
		return
	}
	plain, err := auth.GenerateRandomPassword()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	hash, err := auth.HashPassword(plain)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	s.RunnerTokenHash = hash
	if err := a.Store.DB.Save(&s).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "compute.runner-token", s.Hostname)
	c.JSON(http.StatusOK, gin.H{"runner_token": plain, "ci_url": ciURLHint})
}

func (a *App) ciAbs(slug, rel string) (string, error) {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return "", errors.New("vazio")
	}
	return a.driverRoots().ResolveProject(slug, rel)
}

func (a *App) readCiFile(slug, rel string) (string, error) {
	path, err := a.ciAbs(slug, rel)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (a *App) writeCiFile(slug, rel string, body []byte) error {
	path, err := a.ciAbs(slug, rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, body, 0o640)
}
