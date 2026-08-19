package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

const (
	maxMRTitleRunes = 120
	maxMRDescRunes  = 4000
)

type createMRRequest struct {
	Title        string `json:"title"`
	Description  string `json:"description"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
}

type mergeRequestJSON struct {
	Number       uint                     `json:"number"`
	Title        string                   `json:"title"`
	Description  string                   `json:"description"`
	SourceBranch string                   `json:"source_branch"`
	TargetBranch string                   `json:"target_branch"`
	AuthorID     uint                     `json:"author_id"`
	Author       string                   `json:"author"`
	Status       store.MergeRequestStatus `json:"status"`
	ThreadID     uint                     `json:"thread_id"`
	SocialPostID *uint                    `json:"social_post_id,omitempty"`
	MergedAt     *time.Time               `json:"merged_at,omitempty"`
	MergedBy     string                   `json:"merged_by,omitempty"`
	CreatedAt    time.Time                `json:"created_at"`
	UpdatedAt    time.Time                `json:"updated_at"`
	CanMerge     bool                     `json:"can_merge,omitempty"`
	CanEdit      bool                     `json:"can_edit,omitempty"`
	ChecksBlock  string                   `json:"checks_block,omitempty"`
}

func (a *App) canCreateMR(user store.User, proj store.Project) bool {
	return a.canGitPush(user, proj)
}

func (a *App) canMergeOnto(user store.User, proj store.Project, target string) bool {
	if !a.canGitPush(user, proj) {
		return false
	}
	ref := "refs/heads/" + strings.TrimSpace(target)
	for _, rule := range a.protectedBranchRules(proj.ID) {
		if !forge.MatchProtected([]string{rule.Pattern}, ref) {
			continue
		}
		return a.canGitPushProtected(user, proj, rule.MinPushRole)
	}
	return true
}

func (a *App) canCloseMR(user store.User, proj store.Project, mr store.MergeRequest) bool {
	if user.ID == mr.AuthorID {
		return true
	}
	if store.HasProduct(user.Role, user.Products, store.ProductForge) {
		return true
	}
	role, ok := a.projectMemberRole(user, proj)
	return ok && role.Rank() >= store.ProjectRoleMaintainer.Rank()
}

func (a *App) loadMR(c *gin.Context) (store.Project, store.User, store.MergeRequest, bool) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return store.Project{}, store.User{}, store.MergeRequest{}, false
	}
	var iid uint
	if _, err := parseUintParam(c, "iid", &iid); err != nil || iid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "iid inválido"})
		return store.Project{}, user, store.MergeRequest{}, false
	}
	var mr store.MergeRequest
	if err := a.Store.DB.Where("project_id = ? AND number = ?", proj.ID, iid).First(&mr).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "merge request não encontrado"})
		return proj, user, store.MergeRequest{}, false
	}
	return proj, user, mr, true
}

func (a *App) mrJSON(mr store.MergeRequest) mergeRequestJSON {
	out := mergeRequestJSON{
		Number:       mr.Number,
		Title:        mr.Title,
		Description:  mr.Description,
		SourceBranch: mr.SourceBranch,
		TargetBranch: mr.TargetBranch,
		AuthorID:     mr.AuthorID,
		Status:       mr.Status,
		ThreadID:     mr.ThreadID,
		SocialPostID: mr.SocialPostID,
		MergedAt:     mr.MergedAt,
		CreatedAt:    mr.CreatedAt,
		UpdatedAt:    mr.UpdatedAt,
	}
	var author store.User
	if a.Store.DB.First(&author, mr.AuthorID).Error == nil {
		out.Author = author.Username
	}
	if mr.MergedByID != nil {
		var merger store.User
		if a.Store.DB.First(&merger, *mr.MergedByID).Error == nil {
			out.MergedBy = merger.Username
		}
	}
	return out
}

func (a *App) handleListProjectBranches(c *gin.Context) {
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	heads, err := forge.ListBranches(a.gitDir(), proj.Slug)
	if err != nil {
		if errors.Is(err, forge.ErrGitMissing) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "git indisponível"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if heads == nil {
		heads = []string{}
	}
	c.JSON(http.StatusOK, gin.H{"items": heads})
}

func (a *App) handleListMergeRequests(c *gin.Context) {
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	q := a.Store.DB.Where("project_id = ?", proj.ID)
	if st := strings.TrimSpace(c.Query("status")); st != "" {
		if !store.MergeRequestStatus(st).Valid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status inválido"})
			return
		}
		q = q.Where("status = ?", st)
	}
	var rows []store.MergeRequest
	if err := q.Order("number desc").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]mergeRequestJSON, 0, len(rows))
	for _, mr := range rows {
		items = append(items, a.mrJSON(mr))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) handleGetMergeRequest(c *gin.Context) {
	proj, user, mr, ok := a.loadMR(c)
	if !ok {
		return
	}
	a.ensureMRThreadMember(proj, user, mr.ThreadID)
	out := a.mrJSON(mr)
	out.CanEdit = mr.Status == store.MROpen && (user.ID == mr.AuthorID || a.canMaintainerWrite(user, proj))
	if mr.Status == store.MROpen {
		out.CanMerge = a.canMergeOnto(user, proj, mr.TargetBranch)
		out.ChecksBlock = a.mrChecksBlock(proj, mr)
	}
	c.JSON(http.StatusOK, out)
}

func (a *App) handleCreateMergeRequest(c *gin.Context) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	if !a.canCreateMR(user, proj) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para abrir MR"})
		return
	}
	var req createMRRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" || utf8.RuneCountInString(title) > maxMRTitleRunes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "título inválido"})
		return
	}
	desc := strings.TrimSpace(req.Description)
	if utf8.RuneCountInString(desc) > maxMRDescRunes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "descrição longa demais"})
		return
	}
	source := strings.TrimSpace(req.SourceBranch)
	target := strings.TrimSpace(req.TargetBranch)
	if !forge.ValidBranchName(source) || !forge.ValidBranchName(target) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "branch inválida"})
		return
	}
	if source == target {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source e target iguais"})
		return
	}
	root := a.gitDir()
	if root == "" || !forge.Exists(root, proj.Slug) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "repositório git indisponível"})
		return
	}
	if !forge.BranchExists(root, proj.Slug, source) || !forge.BranchExists(root, proj.Slug, target) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "branch inexistente"})
		return
	}

	mr, err := a.insertMergeRequest(proj, user, title, desc, source, target)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "project.mr.open", fmt.Sprintf("%s!%d", proj.Slug, mr.Number))
	if sha, err := forge.RevParse(a.gitDir(), proj.Slug, source); err == nil {
		n := mr.Number
		st := store.CiAwaitingApproval
		if a.canApproveCi(user, proj) {
			st = store.CiPending
		}
		a.enqueueCiJobAs(proj, ciTriggerMR, "refs/heads/"+source, sha, &n, user.Username, st)
	}
	c.JSON(http.StatusCreated, a.mrJSON(mr))
}

func (a *App) handleMergeMergeRequest(c *gin.Context) {
	proj, user, mr, ok := a.loadMR(c)
	if !ok {
		return
	}
	if mr.Status != store.MROpen {
		c.JSON(http.StatusConflict, gin.H{"error": "MR não está aberto"})
		return
	}
	if !a.canMergeOnto(user, proj, mr.TargetBranch) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para merge nesta branch"})
		return
	}
	if reason := a.mrChecksBlock(proj, mr); reason != "" {
		c.JSON(http.StatusConflict, gin.H{"error": reason})
		return
	}
	msg := fmt.Sprintf("Merge !%d: %s into %s", mr.Number, mr.SourceBranch, mr.TargetBranch)
	if err := forge.MergeBranches(a.gitDir(), proj.Slug, mr.SourceBranch, mr.TargetBranch, msg); err != nil {
		switch {
		case errors.Is(err, forge.ErrMergeConflict):
			c.JSON(http.StatusConflict, gin.H{"error": "conflito de merge"})
		case errors.Is(err, forge.ErrBranchMissing), errors.Is(err, forge.ErrEmptyRepo):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, forge.ErrGitMissing):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "git indisponível"})
		default:
			c.JSON(http.StatusBadGateway, gin.H{"error": "merge falhou"})
		}
		return
	}
	now := time.Now()
	uid := user.ID
	mr.Status = store.MRMerged
	mr.MergedAt = &now
	mr.MergedByID = &uid
	if err := a.Store.DB.Save(&mr).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	note := store.Message{
		ThreadKind: store.ThreadKindDM,
		ThreadID:   mr.ThreadID,
		AuthorID:   user.ID,
		Kind:       "text",
		Body:       fmt.Sprintf("MR !%d mergeado em %s", mr.Number, mr.TargetBranch),
	}
	_ = a.Store.DB.Create(&note).Error
	_ = a.Store.LogAudit(callerUsername(c), "project.mr.merge", fmt.Sprintf("%s!%d", proj.Slug, mr.Number))
	if sha, err := forge.RevParse(a.gitDir(), proj.Slug, mr.TargetBranch); err == nil {
		n := mr.Number
		a.enqueueCiJobAs(proj, ciTriggerMR, "refs/heads/"+mr.TargetBranch, sha, &n, user.Username, store.CiPending)
	}
	c.JSON(http.StatusOK, a.mrJSON(mr))
}

func (a *App) handleCloseMergeRequest(c *gin.Context) {
	proj, user, mr, ok := a.loadMR(c)
	if !ok {
		return
	}
	if mr.Status != store.MROpen {
		c.JSON(http.StatusConflict, gin.H{"error": "MR não está aberto"})
		return
	}
	if !a.canCloseMR(user, proj, mr) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para fechar"})
		return
	}
	mr.Status = store.MRClosed
	if err := a.Store.DB.Save(&mr).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "project.mr.close", fmt.Sprintf("%s!%d", proj.Slug, mr.Number))
	c.JSON(http.StatusOK, a.mrJSON(mr))
}

func (a *App) ensureMRThreadMember(proj store.Project, user store.User, threadID uint) {
	if threadID == 0 {
		return
	}
	_, member := a.projectMemberRole(user, proj)
	if !member && !store.HasProduct(user.Role, user.Products, store.ProductForge) {
		return
	}
	var n int64
	_ = a.Store.DB.Model(&store.DirectThreadMember{}).Where("thread_id = ? AND user_id = ?", threadID, user.ID).Count(&n).Error
	if n > 0 {
		return
	}
	_ = a.Store.DB.Create(&store.DirectThreadMember{ThreadID: threadID, UserID: user.ID}).Error
}

func (a *App) mrChecksBlock(proj store.Project, mr store.MergeRequest) string {
	var jobs []store.CiJob
	if err := a.Store.DB.Where("project_id = ? AND merge_request_number = ?", proj.ID, mr.Number).Order("number desc").Find(&jobs).Error; err != nil {
		return ""
	}
	if len(jobs) == 0 {
		return ""
	}
	for _, j := range jobs {
		if j.Status == store.CiFailed {
			return "CI falhou — o merge fica bloqueado"
		}
	}
	return ""
}

func (a *App) handlePatchMergeRequest(c *gin.Context) {
	proj, user, mr, ok := a.loadMR(c)
	if !ok {
		return
	}
	if mr.Status != store.MROpen {
		c.JSON(http.StatusConflict, gin.H{"error": "MR não está aberto"})
		return
	}
	if user.ID != mr.AuthorID && !a.canMaintainerWrite(user, proj) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para editar"})
		return
	}
	var req struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" || utf8.RuneCountInString(title) > maxMRTitleRunes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "título inválido"})
			return
		}
		mr.Title = title
	}
	if req.Description != nil {
		desc := strings.TrimSpace(*req.Description)
		if utf8.RuneCountInString(desc) > maxMRDescRunes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "descrição longa demais"})
			return
		}
		mr.Description = desc
	}
	if err := a.Store.DB.Save(&mr).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.JSON(http.StatusOK, a.mrJSON(mr))
}

func (a *App) handleListMRCommits(c *gin.Context) {
	proj, _, mr, ok := a.loadMR(c)
	if !ok {
		return
	}
	items, err := forge.CompareCommits(a.gitDir(), proj.Slug, mr.TargetBranch, mr.SourceBranch, 80)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"items": []forge.CommitInfo{}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) handleGetMRDiff(c *gin.Context) {
	proj, _, mr, ok := a.loadMR(c)
	if !ok {
		return
	}
	diff, err := forge.DiffUnified(a.gitDir(), proj.Slug, mr.TargetBranch, mr.SourceBranch)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"diff": ""})
		return
	}
	c.JSON(http.StatusOK, gin.H{"diff": diff})
}

func (a *App) handleListMRReviews(c *gin.Context) {
	_, _, mr, ok := a.loadMR(c)
	if !ok {
		return
	}
	var rows []store.MergeRequestReview
	_ = a.Store.DB.Where("merge_request_id = ?", mr.ID).Order("id desc").Limit(50).Find(&rows).Error
	items := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		name := ""
		var u store.User
		if a.Store.DB.First(&u, r.AuthorID).Error == nil {
			name = u.Username
		}
		items = append(items, gin.H{
			"id": r.ID, "author": name, "state": r.State, "body": r.Body, "created_at": r.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) handleCreateMRReview(c *gin.Context) {
	proj, user, mr, ok := a.loadMR(c)
	if !ok {
		return
	}
	if mr.Status != store.MROpen {
		c.JSON(http.StatusConflict, gin.H{"error": "MR não está aberto"})
		return
	}
	if !a.canGitPush(user, proj) && !a.canReporterWrite(user, proj) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para review"})
		return
	}
	var req struct {
		State string `json:"state"`
		Body  string `json:"body"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	st := store.MRReviewState(strings.TrimSpace(req.State))
	if !st.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "state inválido"})
		return
	}
	if (st == store.MRReviewApprove || st == store.MRReviewRequestChanges) && !a.canMaintainerWrite(user, proj) && user.ID == mr.AuthorID {
		c.JSON(http.StatusForbidden, gin.H{"error": "autor não aprova o próprio PR"})
		return
	}
	if st == store.MRReviewApprove && !a.canMaintainerWrite(user, proj) && !a.canGitPush(user, proj) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para aprovar"})
		return
	}
	body := strings.TrimSpace(req.Body)
	if utf8.RuneCountInString(body) > 2000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "comentário longo demais"})
		return
	}
	row := store.MergeRequestReview{MergeRequestID: mr.ID, AuthorID: user.ID, State: st, Body: body}
	if err := a.Store.DB.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if body != "" || st != store.MRReviewComment {
		note := store.Message{
			ThreadKind: store.ThreadKindDM, ThreadID: mr.ThreadID, AuthorID: user.ID, Kind: "text",
			Body: reviewNote(st, body),
		}
		_ = a.Store.DB.Create(&note).Error
	}
	c.JSON(http.StatusCreated, gin.H{"id": row.ID, "author": user.Username, "state": row.State, "body": row.Body, "created_at": row.CreatedAt})
}

func reviewNote(st store.MRReviewState, body string) string {
	label := "comentou"
	switch st {
	case store.MRReviewApprove:
		label = "aprovou"
	case store.MRReviewRequestChanges:
		label = "pediu alterações"
	}
	if body == "" {
		return "Review: " + label
	}
	return "Review: " + label + " — " + body
}

func (a *App) insertMergeRequest(proj store.Project, user store.User, title, desc, source, target string) (store.MergeRequest, error) {
	var mr store.MergeRequest
	err := a.Store.DB.Transaction(func(tx *gorm.DB) error {
		var last store.MergeRequest
		number := uint(1)
		if err := tx.Where("project_id = ?", proj.ID).Order("number desc").First(&last).Error; err == nil {
			number = last.Number + 1
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		thID, postID, err := a.createProjectThread(tx, proj, user, store.ThreadKindMR,
			fmt.Sprintf("!%d %s", number, title),
			fmt.Sprintf("MR !%d aberto: %s → %s", number, source, target),
			fmt.Sprintf("MR !%d: %s (%s → %s)", number, title, source, target))
		if err != nil {
			return err
		}
		mr = store.MergeRequest{
			ProjectID:    proj.ID,
			Number:       number,
			Title:        title,
			Description:  desc,
			SourceBranch: source,
			TargetBranch: target,
			AuthorID:     user.ID,
			Status:       store.MROpen,
			ThreadID:     thID,
			SocialPostID: &postID,
		}
		return tx.Create(&mr).Error
	})
	return mr, err
}

func truncateRunes(s string, n int) string {
	if n <= 1 || utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}
