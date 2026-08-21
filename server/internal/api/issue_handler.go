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
	Number         uint              `json:"number"`
	Title          string            `json:"title"`
	Body           string            `json:"body"`
	Status         store.IssueStatus `json:"status"`
	Labels         []string          `json:"labels"`
	Assignees      []string          `json:"assignees"`
	Milestone      *uint             `json:"milestone,omitempty"`
	MilestoneTitle string            `json:"milestone_title,omitempty"`
	AuthorID       uint              `json:"author_id"`
	Author         string            `json:"author"`
	ThreadID       uint              `json:"thread_id"`
	ClosedAt       *time.Time        `json:"closed_at,omitempty"`
	ClosedBy       string            `json:"closed_by,omitempty"`
	Restricted     bool              `json:"restricted,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
	CanClose       bool              `json:"can_close,omitempty"`
	CanReopen      bool              `json:"can_reopen,omitempty"`
	CanUpdate      bool              `json:"can_update,omitempty"`
}

type createIssueRequest struct {
	Title     string   `json:"title"`
	Body      string   `json:"body"`
	Labels    []string `json:"labels"`
	Assignee  []string `json:"assignees"`
	Milestone *uint    `json:"milestone"`
}

type patchIssueRequest struct {
	Title     *string  `json:"title"`
	Body      *string  `json:"body"`
	Status    *string  `json:"status"`
	Labels    []string `json:"labels"`
	Assignees []string `json:"assignees"`
	Milestone *uint    `json:"milestone"`
}

func (a *App) canManageIssue(user store.User, proj store.Project, issue store.Issue) bool {
	if user.ID == issue.AuthorID {
		return true
	}
	return a.canMaintainerWrite(user, proj)
}

func (a *App) issueJSON(proj store.Project, user store.User, issue store.Issue) issueJSON {
	out := issueJSON{
		Number:     issue.Number,
		Title:      issue.Title,
		Body:       issue.Body,
		Status:     issue.Status,
		Labels:     issue.Labels,
		AuthorID:   issue.AuthorID,
		ThreadID:   issue.ThreadID,
		ClosedAt:   issue.ClosedAt,
		Restricted: issue.Restricted,
		CreatedAt:  issue.CreatedAt,
		UpdatedAt:  issue.UpdatedAt,
	}
	if out.Labels == nil {
		out.Labels = []string{}
	}
	var author store.User
	if a.Store.DB.First(&author, issue.AuthorID).Error == nil {
		out.Author = author.Username
	}
	out.Assignees = a.usernamesByIDs(issue.AssigneeIDs)
	if n, title := a.milestoneRef(issue.MilestoneID); n > 0 {
		out.Milestone = &n
		out.MilestoneTitle = title
	}
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
	if !a.canSeeRestrictedIssue(user, proj, issue) {
		c.JSON(http.StatusNotFound, gin.H{"error": "issue não encontrada"})
		return proj, user, store.Issue{}, false
	}
	return proj, user, issue, true
}

func (a *App) resolveUserQuery(c *gin.Context, me store.User, key string) (*store.User, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return nil, true
	}
	if raw == "me" {
		return &me, true
	}
	var u store.User
	if err := a.Store.DB.Where("username = ?", raw).First(&u).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": key + " inválido"})
		return nil, false
	}
	return &u, true
}

func hasLabel(labels []string, want string) bool {
	want = strings.ToLower(strings.TrimSpace(want))
	for _, lb := range labels {
		if strings.ToLower(lb) == want {
			return true
		}
	}
	return false
}

func mentionedIn(body, username string) bool {
	if username == "" {
		return false
	}
	return strings.Contains(strings.ToLower(body), "@"+strings.ToLower(username))
}

func (a *App) handleListIssues(c *gin.Context) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	author, ok := a.resolveUserQuery(c, user, "author")
	if !ok {
		return
	}
	assignee, ok := a.resolveUserQuery(c, user, "assignee")
	if !ok {
		return
	}
	mentioned, ok := a.resolveUserQuery(c, user, "mentioned")
	if !ok {
		return
	}
	q := a.Store.DB.Where("project_id = ?", proj.ID)
	if author != nil {
		q = q.Where("author_id = ?", author.ID)
	}
	if qstr := strings.TrimSpace(c.Query("q")); qstr != "" {
		q = q.Where("title LIKE ?", "%"+qstr+"%")
	}
	if ms := strings.TrimSpace(c.Query("milestone")); ms != "" {
		var n uint
		if _, err := fmt.Sscanf(ms, "%d", &n); err != nil || n == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "milestone inválido"})
			return
		}
		mid, err := a.resolveMilestoneID(proj, n)
		if err != nil || mid == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "milestone não encontrado"})
			return
		}
		q = q.Where("milestone_id = ?", *mid)
	}
	var rows []store.Issue
	if err := q.Order("number desc").Limit(500).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	label := strings.TrimSpace(c.Query("label"))
	filtered := make([]store.Issue, 0, len(rows))
	for _, it := range rows {
		if !a.canSeeRestrictedIssue(user, proj, it) {
			continue
		}
		if assignee != nil && !containsUint(it.AssigneeIDs, assignee.ID) {
			continue
		}
		if label != "" && !hasLabel(it.Labels, label) {
			continue
		}
		if mentioned != nil && !mentionedIn(it.Body, mentioned.Username) {
			continue
		}
		filtered = append(filtered, it)
	}
	var openCount, closedCount int
	for _, it := range filtered {
		if it.Status == store.IssueOpen {
			openCount++
		} else {
			closedCount++
		}
	}
	status := strings.TrimSpace(c.Query("status"))
	if status != "" {
		if !store.IssueStatus(status).Valid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status inválido"})
			return
		}
		kept := filtered[:0]
		for _, it := range filtered {
			if string(it.Status) == status {
				kept = append(kept, it)
			}
		}
		filtered = kept
	}
	sort := strings.TrimSpace(c.Query("sort"))
	switch sort {
	case "oldest":
		for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
			filtered[i], filtered[j] = filtered[j], filtered[i]
		}
	case "updated":
		for i := 0; i < len(filtered); i++ {
			for j := i + 1; j < len(filtered); j++ {
				if filtered[j].UpdatedAt.After(filtered[i].UpdatedAt) {
					filtered[i], filtered[j] = filtered[j], filtered[i]
				}
			}
		}
	}
	if len(filtered) > 80 {
		filtered = filtered[:80]
	}
	items := make([]issueJSON, 0, len(filtered))
	for _, it := range filtered {
		items = append(items, a.issueJSON(proj, user, it))
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "open_count": openCount, "closed_count": closedCount})
}

func (a *App) handleGetIssue(c *gin.Context) {
	proj, user, issue, ok := a.loadIssue(c)
	if !ok {
		return
	}
	a.ensureMRThreadMember(proj, user, issue.ThreadID)
	c.JSON(http.StatusOK, a.issueJSON(proj, user, issue))
}

func (a *App) insertIssue(proj store.Project, user store.User, title, body string, labels []string, assignees []uint, milestoneID *uint, restricted bool) (store.Issue, error) {
	var issue store.Issue
	err := a.Store.DB.Transaction(func(tx *gorm.DB) error {
		var last store.Issue
		number := uint(1)
		if err := tx.Where("project_id = ?", proj.ID).Order("number desc").First(&last).Error; err == nil {
			number = last.Number + 1
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var thID uint
		var postID *uint
		if restricted {
			id, err := a.createRestrictedIssueThread(tx, proj, user, fmt.Sprintf("#%d %s", number, title), fmt.Sprintf("Issue #%d aberta", number))
			if err != nil {
				return err
			}
			thID = id
		} else {
			tid, pid, err := a.createProjectThread(tx, proj, user, store.ThreadKindIssue,
				fmt.Sprintf("#%d %s", number, title),
				fmt.Sprintf("Issue #%d aberta", number),
				fmt.Sprintf("Issue #%d: %s", number, title))
			if err != nil {
				return err
			}
			thID = tid
			postID = &pid
		}
		issue = store.Issue{
			ProjectID:    proj.ID,
			Number:       number,
			Title:        title,
			Body:         body,
			Status:       store.IssueOpen,
			Labels:       labels,
			AssigneeIDs:  assignees,
			MilestoneID:  milestoneID,
			AuthorID:     user.ID,
			ThreadID:     thID,
			SocialPostID: postID,
			Restricted:   restricted,
		}
		return tx.Create(&issue).Error
	})
	return issue, err
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
	var milestoneID *uint
	if req.Milestone != nil {
		milestoneID, err = a.resolveMilestoneID(proj, *req.Milestone)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	issue, err := a.insertIssue(proj, user, title, body, labels, assignees, milestoneID, false)
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
	if req.Milestone != nil {
		mid, err := a.resolveMilestoneID(proj, *req.Milestone)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		issue.MilestoneID = mid
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

func (a *App) handleListIssueLabels(c *gin.Context) {
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	var rows []store.Issue
	if err := a.Store.DB.Where("project_id = ?", proj.ID).Limit(500).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	seen := map[string]struct{}{}
	items := make([]string, 0)
	for _, it := range rows {
		for _, lb := range it.Labels {
			key := strings.ToLower(lb)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			items = append(items, lb)
		}
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}
