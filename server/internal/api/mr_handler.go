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
	c.JSON(http.StatusOK, a.mrJSON(mr))
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

	var mr store.MergeRequest
	err := a.Store.DB.Transaction(func(tx *gorm.DB) error {
		var last store.MergeRequest
		number := uint(1)
		if err := tx.Where("project_id = ?", proj.ID).Order("number desc").First(&last).Error; err == nil {
			number = last.Number + 1
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		th := store.DirectThread{Kind: store.ThreadKindMR, Title: fmt.Sprintf("!%d %s", number, title)}
		if err := tx.Create(&th).Error; err != nil {
			return err
		}
		seen := map[uint]struct{}{user.ID: {}}
		if err := tx.Create(&store.DirectThreadMember{ThreadID: th.ID, UserID: user.ID}).Error; err != nil {
			return err
		}
		var members []store.ProjectMember
		if err := tx.Where("project_id = ?", proj.ID).Find(&members).Error; err != nil {
			return err
		}
		for _, m := range members {
			if _, ok := seen[m.UserID]; ok {
				continue
			}
			seen[m.UserID] = struct{}{}
			if err := tx.Create(&store.DirectThreadMember{ThreadID: th.ID, UserID: m.UserID}).Error; err != nil {
				return err
			}
		}
		body := fmt.Sprintf("MR !%d aberto: %s → %s", number, source, target)
		msg := store.Message{ThreadKind: store.ThreadKindDM, ThreadID: th.ID, AuthorID: user.ID, Kind: "text", Body: body}
		if err := tx.Create(&msg).Error; err != nil {
			return err
		}
		slug := proj.Slug
		postBody := truncateRunes(fmt.Sprintf("MR !%d: %s (%s → %s)", number, title, source, target), maxPostRunes)
		post := store.SocialPost{AuthorID: user.ID, Body: postBody, Kind: "text", ProjectSlug: &slug}
		if err := tx.Create(&post).Error; err != nil {
			return err
		}
		postID := post.ID
		mr = store.MergeRequest{
			ProjectID:    proj.ID,
			Number:       number,
			Title:        title,
			Description:  desc,
			SourceBranch: source,
			TargetBranch: target,
			AuthorID:     user.ID,
			Status:       store.MROpen,
			ThreadID:     th.ID,
			SocialPostID: &postID,
		}
		return tx.Create(&mr).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "project.mr.open", fmt.Sprintf("%s!%d", proj.Slug, mr.Number))
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
		a.enqueueCiJob(proj, ciTriggerMR, "refs/heads/"+mr.TargetBranch, sha, &n)
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

func truncateRunes(s string, n int) string {
	if n <= 1 || utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n-1]) + "…"
}
