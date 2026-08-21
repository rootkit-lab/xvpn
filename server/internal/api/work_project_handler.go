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

const maxWorkTitleRunes = 120

type workProjectJSON struct {
	Number      uint                    `json:"number"`
	Title       string                  `json:"title"`
	Description string                  `json:"description"`
	Status      store.WorkProjectStatus `json:"status"`
	Layout      string                  `json:"layout"`
	Template    string                  `json:"template"`
	Columns     []string                `json:"columns"`
	ItemCount   int64                   `json:"item_count"`
	Author      string                  `json:"author"`
	ClosedAt    *time.Time              `json:"closed_at,omitempty"`
	CreatedAt   time.Time               `json:"created_at"`
	UpdatedAt   time.Time               `json:"updated_at"`
	CanUpdate   bool                    `json:"can_update,omitempty"`
	Items       []workItemJSON          `json:"items,omitempty"`
}

type workItemJSON struct {
	ID        uint      `json:"id"`
	Title     string    `json:"title"`
	Column    string    `json:"column"`
	Position  int       `json:"position"`
	Issue     *uint     `json:"issue,omitempty"`
	MR        *uint     `json:"mr,omitempty"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
}

type createWorkProjectRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Template    string `json:"template"`
}

type patchWorkProjectRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
}

type createWorkItemRequest struct {
	Title  string `json:"title"`
	Issue  *uint  `json:"issue"`
	MR     *uint  `json:"mr"`
	Column string `json:"column"`
}

type patchWorkItemRequest struct {
	Title    *string `json:"title"`
	Column   *string `json:"column"`
	Position *int    `json:"position"`
}

func workTemplate(name string) (layout string, columns []string, ok bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "board", "kanban":
		return "board", []string{"Todo", "In Progress", "Done"}, true
	case "table":
		return "table", []string{"Todo", "In Progress", "Done"}, true
	case "bug":
		return "board", []string{"Triage", "In Progress", "Done"}, true
	case "roadmap":
		return "table", []string{"Todo", "In Progress", "Done"}, true
	default:
		return "", nil, false
	}
}

func matchColumn(columns []string, raw string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" && len(columns) > 0 {
		return columns[0], true
	}
	for _, col := range columns {
		if strings.EqualFold(col, raw) {
			return col, true
		}
	}
	return "", false
}

func (a *App) workProjectJSON(user store.User, proj store.Project, wp store.WorkProject, withItems bool) workProjectJSON {
	out := workProjectJSON{
		Number:      wp.Number,
		Title:       wp.Title,
		Description: wp.Description,
		Status:      wp.Status,
		Layout:      wp.Layout,
		Template:    wp.Template,
		Columns:     wp.Columns,
		ClosedAt:    wp.ClosedAt,
		CreatedAt:   wp.CreatedAt,
		UpdatedAt:   wp.UpdatedAt,
	}
	if out.Columns == nil {
		out.Columns = []string{}
	}
	var author store.User
	if a.Store.DB.First(&author, wp.AuthorID).Error == nil {
		out.Author = author.Username
	}
	_ = a.Store.DB.Model(&store.WorkItem{}).Where("work_project_id = ?", wp.ID).Count(&out.ItemCount).Error
	if user.ID != 0 {
		out.CanUpdate = a.canMaintainerWrite(user, proj) || user.ID == wp.AuthorID
	}
	if withItems {
		var rows []store.WorkItem
		_ = a.Store.DB.Where("work_project_id = ?", wp.ID).Order("position asc, id asc").Find(&rows).Error
		out.Items = make([]workItemJSON, 0, len(rows))
		for _, it := range rows {
			out.Items = append(out.Items, a.workItemJSON(it))
		}
	}
	return out
}

func (a *App) workItemJSON(it store.WorkItem) workItemJSON {
	out := workItemJSON{
		ID:        it.ID,
		Title:     it.Title,
		Column:    it.Column,
		Position:  it.Position,
		Issue:     it.IssueNumber,
		MR:        it.MRNumber,
		CreatedAt: it.CreatedAt,
	}
	var author store.User
	if a.Store.DB.First(&author, it.AuthorID).Error == nil {
		out.Author = author.Username
	}
	return out
}

func (a *App) loadWorkProject(c *gin.Context) (store.Project, store.User, store.WorkProject, bool) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return store.Project{}, store.User{}, store.WorkProject{}, false
	}
	var n uint
	if _, err := parseUintParam(c, "n", &n); err != nil || n == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project inválido"})
		return proj, user, store.WorkProject{}, false
	}
	var wp store.WorkProject
	if err := a.Store.DB.Where("project_id = ? AND number = ?", proj.ID, n).First(&wp).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "project não encontrado"})
		return proj, user, store.WorkProject{}, false
	}
	return proj, user, wp, true
}

func (a *App) handleListWorkProjects(c *gin.Context) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	q := a.Store.DB.Where("project_id = ?", proj.ID)
	if st := strings.TrimSpace(c.Query("status")); st != "" {
		if !store.WorkProjectStatus(st).Valid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status inválido"})
			return
		}
		q = q.Where("status = ?", st)
	}
	if qstr := strings.TrimSpace(c.Query("q")); qstr != "" {
		q = q.Where("title LIKE ?", "%"+qstr+"%")
	}
	var rows []store.WorkProject
	if err := q.Order("number desc").Limit(80).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]workProjectJSON, 0, len(rows))
	for _, wp := range rows {
		items = append(items, a.workProjectJSON(user, proj, wp, false))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) handleCreateWorkProject(c *gin.Context) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	if !a.canReporterWrite(user, proj) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para criar project"})
		return
	}
	var req createWorkProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" || utf8.RuneCountInString(title) > maxWorkTitleRunes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "título inválido"})
		return
	}
	layout, columns, okTpl := workTemplate(req.Template)
	if !okTpl {
		c.JSON(http.StatusBadRequest, gin.H{"error": "template inválido"})
		return
	}
	tpl := strings.ToLower(strings.TrimSpace(req.Template))
	if tpl == "" {
		tpl = "kanban"
	}
	var wp store.WorkProject
	err := a.Store.DB.Transaction(func(tx *gorm.DB) error {
		var last store.WorkProject
		number := uint(1)
		if err := tx.Where("project_id = ?", proj.ID).Order("number desc").First(&last).Error; err == nil {
			number = last.Number + 1
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		wp = store.WorkProject{
			ProjectID:   proj.ID,
			Number:      number,
			Title:       title,
			Description: strings.TrimSpace(req.Description),
			Status:      store.WorkProjectOpen,
			Layout:      layout,
			Template:    tpl,
			Columns:     columns,
			AuthorID:    user.ID,
		}
		return tx.Create(&wp).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "project.work.open", fmt.Sprintf("%s/%d", proj.Slug, wp.Number))
	c.JSON(http.StatusCreated, a.workProjectJSON(user, proj, wp, true))
}

func (a *App) handleGetWorkProject(c *gin.Context) {
	proj, user, wp, ok := a.loadWorkProject(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, a.workProjectJSON(user, proj, wp, true))
}

func (a *App) handlePatchWorkProject(c *gin.Context) {
	proj, user, wp, ok := a.loadWorkProject(c)
	if !ok {
		return
	}
	if !a.canMaintainerWrite(user, proj) && user.ID != wp.AuthorID {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para editar o project"})
		return
	}
	var req patchWorkProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" || utf8.RuneCountInString(title) > maxWorkTitleRunes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "título inválido"})
			return
		}
		wp.Title = title
	}
	if req.Description != nil {
		wp.Description = strings.TrimSpace(*req.Description)
	}
	if req.Status != nil {
		st := store.WorkProjectStatus(strings.TrimSpace(*req.Status))
		if !st.Valid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status inválido"})
			return
		}
		if st == store.WorkProjectClosed && wp.Status == store.WorkProjectOpen {
			now := time.Now()
			wp.Status = store.WorkProjectClosed
			wp.ClosedAt = &now
		}
		if st == store.WorkProjectOpen && wp.Status == store.WorkProjectClosed {
			wp.Status = store.WorkProjectOpen
			wp.ClosedAt = nil
		}
	}
	if err := a.Store.DB.Save(&wp).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if wp.Status == store.WorkProjectOpen && wp.ClosedAt == nil {
		_ = a.Store.DB.Model(&wp).Update("closed_at", nil).Error
	}
	_ = a.Store.LogAudit(callerUsername(c), "project.work.patch", fmt.Sprintf("%s/%d", proj.Slug, wp.Number))
	c.JSON(http.StatusOK, a.workProjectJSON(user, proj, wp, true))
}

func (a *App) handleListWorkItems(c *gin.Context) {
	_, _, wp, ok := a.loadWorkProject(c)
	if !ok {
		return
	}
	var rows []store.WorkItem
	if err := a.Store.DB.Where("work_project_id = ?", wp.ID).Order("position asc, id asc").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]workItemJSON, 0, len(rows))
	for _, it := range rows {
		items = append(items, a.workItemJSON(it))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) handleCreateWorkItem(c *gin.Context) {
	proj, user, wp, ok := a.loadWorkProject(c)
	if !ok {
		return
	}
	if !a.canReporterWrite(user, proj) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para criar item"})
		return
	}
	if wp.Status != store.WorkProjectOpen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "project fechado"})
		return
	}
	var req createWorkItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	col, okCol := matchColumn(wp.Columns, req.Column)
	if !okCol {
		c.JSON(http.StatusBadRequest, gin.H{"error": "coluna inválida"})
		return
	}
	title := strings.TrimSpace(req.Title)
	var issueN, mrN *uint
	if req.Issue != nil && *req.Issue > 0 {
		var issue store.Issue
		if err := a.Store.DB.Where("project_id = ? AND number = ?", proj.ID, *req.Issue).First(&issue).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "issue não encontrada"})
			return
		}
		if !a.canSeeRestrictedIssue(user, proj, issue) {
			c.JSON(http.StatusNotFound, gin.H{"error": "issue não encontrada"})
			return
		}
		var dup int64
		_ = a.Store.DB.Model(&store.WorkItem{}).Where("work_project_id = ? AND issue_number = ?", wp.ID, issue.Number).Count(&dup).Error
		if dup > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "issue já está neste project"})
			return
		}
		n := issue.Number
		issueN = &n
		if title == "" {
			title = issue.Title
		}
	}
	if req.MR != nil && *req.MR > 0 {
		var mr store.MergeRequest
		if err := a.Store.DB.Where("project_id = ? AND number = ?", proj.ID, *req.MR).First(&mr).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "pull request não encontrado"})
			return
		}
		var dup int64
		_ = a.Store.DB.Model(&store.WorkItem{}).Where("work_project_id = ? AND mr_number = ?", wp.ID, mr.Number).Count(&dup).Error
		if dup > 0 {
			c.JSON(http.StatusConflict, gin.H{"error": "PR já está neste project"})
			return
		}
		n := mr.Number
		mrN = &n
		if title == "" {
			title = mr.Title
		}
	}
	if title == "" || utf8.RuneCountInString(title) > maxWorkTitleRunes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "título inválido"})
		return
	}
	var last store.WorkItem
	pos := 0
	if err := a.Store.DB.Where("work_project_id = ? AND column = ?", wp.ID, col).Order("position desc").First(&last).Error; err == nil {
		pos = last.Position + 1
	}
	item := store.WorkItem{
		WorkProjectID: wp.ID,
		IssueNumber:   issueN,
		MRNumber:      mrN,
		Title:         title,
		Column:        col,
		Position:      pos,
		AuthorID:      user.ID,
	}
	if err := a.Store.DB.Create(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.JSON(http.StatusCreated, a.workItemJSON(item))
}

func (a *App) handlePatchWorkItem(c *gin.Context) {
	proj, user, wp, ok := a.loadWorkProject(c)
	if !ok {
		return
	}
	if !a.canReporterWrite(user, proj) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para mover item"})
		return
	}
	var id uint
	if _, err := parseUintParam(c, "id", &id); err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "item inválido"})
		return
	}
	var item store.WorkItem
	if err := a.Store.DB.Where("id = ? AND work_project_id = ?", id, wp.ID).First(&item).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "item não encontrado"})
		return
	}
	var req patchWorkItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" || utf8.RuneCountInString(title) > maxWorkTitleRunes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "título inválido"})
			return
		}
		item.Title = title
	}
	if req.Column != nil {
		col, okCol := matchColumn(wp.Columns, *req.Column)
		if !okCol {
			c.JSON(http.StatusBadRequest, gin.H{"error": "coluna inválida"})
			return
		}
		item.Column = col
	}
	if req.Position != nil {
		item.Position = *req.Position
	}
	if err := a.Store.DB.Save(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.JSON(http.StatusOK, a.workItemJSON(item))
}

func (a *App) handleDeleteWorkItem(c *gin.Context) {
	proj, user, wp, ok := a.loadWorkProject(c)
	if !ok {
		return
	}
	var id uint
	if _, err := parseUintParam(c, "id", &id); err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "item inválido"})
		return
	}
	var item store.WorkItem
	if err := a.Store.DB.Where("id = ? AND work_project_id = ?", id, wp.ID).First(&item).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "item não encontrado"})
		return
	}
	if user.ID != item.AuthorID && !a.canMaintainerWrite(user, proj) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para remover item"})
		return
	}
	if err := a.Store.DB.Delete(&item).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.Status(http.StatusNoContent)
}
