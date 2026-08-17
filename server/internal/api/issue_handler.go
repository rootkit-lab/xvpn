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

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

const (
	maxIssueTitleRunes = 120
	maxIssueBodyRunes  = 8000
	maxIssueLabels     = 20
	maxLabelRunes      = 32
	maxAssignees       = 10
)

type issueJSON struct {
	Number    uint              `json:"number"`
	Title     string            `json:"title"`
	Body      string            `json:"body"`
	Status    store.IssueStatus `json:"status"`
	Labels    []string          `json:"labels"`
	Assignees []string          `json:"assignees"`
	AuthorID  uint              `json:"author_id"`
	Author    string            `json:"author"`
	ThreadID  uint              `json:"thread_id"`
	ClosedAt  *time.Time        `json:"closed_at,omitempty"`
	ClosedBy  string            `json:"closed_by,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	CanClose  bool              `json:"can_close,omitempty"`
	CanReopen bool              `json:"can_reopen,omitempty"`
	CanUpdate bool              `json:"can_update,omitempty"`
}

type createIssueRequest struct {
	Title    string   `json:"title"`
	Body     string   `json:"body"`
	Labels   []string `json:"labels"`
	Assignee []string `json:"assignees"`
}

type patchIssueRequest struct {
	Title     *string  `json:"title"`
	Body      *string  `json:"body"`
	Status    *string  `json:"status"`
	Labels    []string `json:"labels"`
	Assignees []string `json:"assignees"`
}

func (a *App) canManageIssue(user store.User, proj store.Project, issue store.Issue) bool {
	if user.ID == issue.AuthorID {
		return true
	}
	return a.canMaintainerWrite(user, proj)
}

func (a *App) issueJSON(proj store.Project, user store.User, issue store.Issue) issueJSON {
	out := issueJSON{
		Number:    issue.Number,
		Title:     issue.Title,
		Body:      issue.Body,
		Status:    issue.Status,
		Labels:    issue.Labels,
		AuthorID:  issue.AuthorID,
		ThreadID:  issue.ThreadID,
		ClosedAt:  issue.ClosedAt,
		CreatedAt: issue.CreatedAt,
		UpdatedAt: issue.UpdatedAt,
	}
	if out.Labels == nil {
		out.Labels = []string{}
	}
	var author store.User
	if a.Store.DB.First(&author, issue.AuthorID).Error == nil {
		out.Author = author.Username
	}
	out.Assignees = a.usernamesByIDs(issue.AssigneeIDs)
	if issue.ClosedByID != nil {
		var closer store.User
		if a.Store.DB.First(&closer, *issue.ClosedByID).Error == nil {
			out.ClosedBy = closer.Username
		}
	}
	if user.ID != 0 {
		ok := a.canManageIssue(user, proj, issue)
		out.CanUpdate = ok
		out.CanClose = ok && issue.Status == store.IssueOpen
		out.CanReopen = ok && issue.Status == store.IssueClosed
	}
	return out
}

func (a *App) usernamesByIDs(ids []uint) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		var u store.User
		if a.Store.DB.First(&u, id).Error == nil {
			out = append(out, u.Username)
		}
	}
	return out
}

func (a *App) resolveAssigneeIDs(proj store.Project, names []string) ([]uint, error) {
	if len(names) > maxAssignees {
		return nil, errors.New("assignees demais")
	}
	ids := make([]uint, 0, len(names))
	seen := map[uint]struct{}{}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		var u store.User
		if err := a.Store.DB.Where("username = ?", name).First(&u).Error; err != nil {
			return nil, fmt.Errorf("usuário %s não encontrado", name)
		}
		if _, ok := a.projectMemberRole(u, proj); !ok && !store.HasProduct(u.Role, u.Products, store.ProductForge) {
			return nil, fmt.Errorf("%s não é membro do projeto", name)
		}
		if _, ok := seen[u.ID]; ok {
			continue
		}
		seen[u.ID] = struct{}{}
		ids = append(ids, u.ID)
	}
	return ids, nil
}

func sanitizeLabels(in []string) ([]string, error) {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, raw := range in {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		if utf8.RuneCountInString(s) > maxLabelRunes {
			return nil, errors.New("label longa demais")
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
		if len(out) > maxIssueLabels {
			return nil, errors.New("labels demais")
		}
	}
	return out, nil
}

func (a *App) loadIssue(c *gin.Context) (store.Project, store.User, store.Issue, bool) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return store.Project{}, store.User{}, store.Issue{}, false
	}
	var n uint
	if _, err := parseUintParam(c, "n", &n); err != nil || n == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "issue inválida"})
		return proj, user, store.Issue{}, false
	}
	var issue store.Issue
	if err := a.Store.DB.Where("project_id = ? AND number = ?", proj.ID, n).First(&issue).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "issue não encontrada"})
		return proj, user, store.Issue{}, false
	}
	return proj, user, issue, true
}

func (a *App) handleListIssues(c *gin.Context) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	q := a.Store.DB.Where("project_id = ?", proj.ID)
	if st := strings.TrimSpace(c.Query("status")); st != "" {
		if !store.IssueStatus(st).Valid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status inválido"})
			return
		}
		q = q.Where("status = ?", st)
	}
	if qstr := strings.TrimSpace(c.Query("q")); qstr != "" {
		q = q.Where("title LIKE ?", "%"+qstr+"%")
	}
	var rows []store.Issue
	if err := q.Order("number desc").Limit(80).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]issueJSON, 0, len(rows))
	for _, it := range rows {
		items = append(items, a.issueJSON(proj, user, it))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) handleGetIssue(c *gin.Context) {
	proj, user, issue, ok := a.loadIssue(c)
	if !ok {
		return
	}
	a.ensureMRThreadMember(proj, user, issue.ThreadID)
	c.JSON(http.StatusOK, a.issueJSON(proj, user, issue))
}

func (a *App) handleCreateIssue(c *gin.Context) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	if !a.canReporterWrite(user, proj) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para criar issue"})
		return
	}
	var req createIssueRequest
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
	labels, err := sanitizeLabels(req.Labels)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	assignees, err := a.resolveAssigneeIDs(proj, req.Assignee)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var issue store.Issue
	err = a.Store.DB.Transaction(func(tx *gorm.DB) error {
		var last store.Issue
		number := uint(1)
		if err := tx.Where("project_id = ?", proj.ID).Order("number desc").First(&last).Error; err == nil {
			number = last.Number + 1
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		thID, postID, err := a.createProjectThread(tx, proj, user, store.ThreadKindIssue,
			fmt.Sprintf("#%d %s", number, title),
			fmt.Sprintf("Issue #%d aberta", number),
			fmt.Sprintf("Issue #%d: %s", number, title))
		if err != nil {
			return err
		}
		issue = store.Issue{
			ProjectID:    proj.ID,
			Number:       number,
			Title:        title,
			Body:         body,
			Status:       store.IssueOpen,
			Labels:       labels,
			AssigneeIDs:  assignees,
			AuthorID:     user.ID,
			ThreadID:     thID,
			SocialPostID: &postID,
		}
		return tx.Create(&issue).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "project.issue.open", fmt.Sprintf("%s#%d", proj.Slug, issue.Number))
	c.JSON(http.StatusCreated, a.issueJSON(proj, user, issue))
}

func (a *App) handlePatchIssue(c *gin.Context) {
	proj, user, issue, ok := a.loadIssue(c)
	if !ok {
		return
	}
	if !a.canManageIssue(user, proj, issue) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para editar a issue"})
		return
	}
	var req patchIssueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" || utf8.RuneCountInString(title) > maxIssueTitleRunes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "título inválido"})
			return
		}
		issue.Title = title
	}
	if req.Body != nil {
		body := strings.TrimSpace(*req.Body)
		if utf8.RuneCountInString(body) > maxIssueBodyRunes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "corpo longo demais"})
			return
		}
		issue.Body = body
	}
	if req.Labels != nil {
		labels, err := sanitizeLabels(req.Labels)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		issue.Labels = labels
	}
	if req.Assignees != nil {
		ids, err := a.resolveAssigneeIDs(proj, req.Assignees)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		issue.AssigneeIDs = ids
	}
	if req.Status != nil {
		st := store.IssueStatus(strings.TrimSpace(*req.Status))
		if !st.Valid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status inválido"})
			return
		}
		if st == store.IssueClosed && issue.Status == store.IssueOpen {
			now := time.Now()
			uid := user.ID
			issue.Status = store.IssueClosed
			issue.ClosedAt = &now
			issue.ClosedByID = &uid
		}
		if st == store.IssueOpen && issue.Status == store.IssueClosed {
			issue.Status = store.IssueOpen
			issue.ClosedAt = nil
			issue.ClosedByID = nil
		}
	}
	if err := a.Store.DB.Save(&issue).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if issue.Status == store.IssueOpen && issue.ClosedAt == nil {
		_ = a.Store.DB.Model(&issue).Updates(map[string]any{"closed_at": nil, "closed_by_id": nil}).Error
	}
	_ = a.Store.LogAudit(callerUsername(c), "project.issue.patch", fmt.Sprintf("%s#%d", proj.Slug, issue.Number))
	c.JSON(http.StatusOK, a.issueJSON(proj, user, issue))
}
