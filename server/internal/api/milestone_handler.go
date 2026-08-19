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

const maxMilestoneTitleRunes = 120

type milestoneJSON struct {
	Number       uint                  `json:"number"`
	Title        string                `json:"title"`
	Description  string                `json:"description"`
	Status       store.MilestoneStatus `json:"status"`
	DueOn        *time.Time            `json:"due_on,omitempty"`
	OpenIssues   int64                 `json:"open_issues"`
	ClosedIssues int64                 `json:"closed_issues"`
	Author       string                `json:"author"`
	ClosedAt     *time.Time            `json:"closed_at,omitempty"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
	CanUpdate    bool                  `json:"can_update,omitempty"`
}

type createMilestoneRequest struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	DueOn       string `json:"due_on"`
}

type patchMilestoneRequest struct {
	Title       *string `json:"title"`
	Description *string `json:"description"`
	Status      *string `json:"status"`
	DueOn       *string `json:"due_on"`
}

func (a *App) milestoneRef(id *uint) (uint, string) {
	if id == nil {
		return 0, ""
	}
	var m store.Milestone
	if a.Store.DB.First(&m, *id).Error != nil {
		return 0, ""
	}
	return m.Number, m.Title
}

func (a *App) resolveMilestoneID(proj store.Project, n uint) (*uint, error) {
	if n == 0 {
		return nil, nil
	}
	var m store.Milestone
	if err := a.Store.DB.Where("project_id = ? AND number = ?", proj.ID, n).First(&m).Error; err != nil {
		return nil, errors.New("milestone não encontrado")
	}
	return &m.ID, nil
}

func parseDueOn(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", raw)
	if err != nil {
		return nil, errors.New("due_on inválido")
	}
	return &t, nil
}

func (a *App) milestoneJSON(proj store.Project, user store.User, m store.Milestone) milestoneJSON {
	out := milestoneJSON{
		Number:      m.Number,
		Title:       m.Title,
		Description: m.Description,
		Status:      m.Status,
		DueOn:       m.DueOn,
		ClosedAt:    m.ClosedAt,
		CreatedAt:   m.CreatedAt,
		UpdatedAt:   m.UpdatedAt,
	}
	var author store.User
	if a.Store.DB.First(&author, m.AuthorID).Error == nil {
		out.Author = author.Username
	}
	_ = a.Store.DB.Model(&store.Issue{}).Where("project_id = ? AND milestone_id = ? AND status = ?", proj.ID, m.ID, store.IssueOpen).Count(&out.OpenIssues).Error
	_ = a.Store.DB.Model(&store.Issue{}).Where("project_id = ? AND milestone_id = ? AND status = ?", proj.ID, m.ID, store.IssueClosed).Count(&out.ClosedIssues).Error
	if user.ID != 0 {
		out.CanUpdate = a.canMaintainerWrite(user, proj) || user.ID == m.AuthorID
	}
	return out
}

func (a *App) loadMilestone(c *gin.Context) (store.Project, store.User, store.Milestone, bool) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return store.Project{}, store.User{}, store.Milestone{}, false
	}
	var n uint
	if _, err := parseUintParam(c, "n", &n); err != nil || n == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "milestone inválido"})
		return proj, user, store.Milestone{}, false
	}
	var m store.Milestone
	if err := a.Store.DB.Where("project_id = ? AND number = ?", proj.ID, n).First(&m).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "milestone não encontrado"})
		return proj, user, store.Milestone{}, false
	}
	return proj, user, m, true
}

func (a *App) handleListMilestones(c *gin.Context) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	q := a.Store.DB.Where("project_id = ?", proj.ID)
	if st := strings.TrimSpace(c.Query("status")); st != "" {
		if !store.MilestoneStatus(st).Valid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status inválido"})
			return
		}
		q = q.Where("status = ?", st)
	}
	var rows []store.Milestone
	if err := q.Order("number desc").Limit(80).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]milestoneJSON, 0, len(rows))
	for _, m := range rows {
		items = append(items, a.milestoneJSON(proj, user, m))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) handleCreateMilestone(c *gin.Context) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	if !a.canReporterWrite(user, proj) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para criar milestone"})
		return
	}
	var req createMilestoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" || utf8.RuneCountInString(title) > maxMilestoneTitleRunes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "título inválido"})
		return
	}
	due, err := parseDueOn(req.DueOn)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var m store.Milestone
	err = a.Store.DB.Transaction(func(tx *gorm.DB) error {
		var last store.Milestone
		number := uint(1)
		if err := tx.Where("project_id = ?", proj.ID).Order("number desc").First(&last).Error; err == nil {
			number = last.Number + 1
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		m = store.Milestone{
			ProjectID:   proj.ID,
			Number:      number,
			Title:       title,
			Description: strings.TrimSpace(req.Description),
			DueOn:       due,
			Status:      store.MilestoneOpen,
			AuthorID:    user.ID,
		}
		return tx.Create(&m).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "project.milestone.open", fmt.Sprintf("%s#%d", proj.Slug, m.Number))
	c.JSON(http.StatusCreated, a.milestoneJSON(proj, user, m))
}

func (a *App) handlePatchMilestone(c *gin.Context) {
	proj, user, m, ok := a.loadMilestone(c)
	if !ok {
		return
	}
	if !a.canMaintainerWrite(user, proj) && user.ID != m.AuthorID {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para editar o milestone"})
		return
	}
	var req patchMilestoneRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" || utf8.RuneCountInString(title) > maxMilestoneTitleRunes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "título inválido"})
			return
		}
		m.Title = title
	}
	if req.Description != nil {
		m.Description = strings.TrimSpace(*req.Description)
	}
	if req.DueOn != nil {
		due, err := parseDueOn(*req.DueOn)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		m.DueOn = due
	}
	if req.Status != nil {
		st := store.MilestoneStatus(strings.TrimSpace(*req.Status))
		if !st.Valid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "status inválido"})
			return
		}
		if st == store.MilestoneClosed && m.Status == store.MilestoneOpen {
			now := time.Now()
			m.Status = store.MilestoneClosed
			m.ClosedAt = &now
		}
		if st == store.MilestoneOpen && m.Status == store.MilestoneClosed {
			m.Status = store.MilestoneOpen
			m.ClosedAt = nil
		}
	}
	if err := a.Store.DB.Save(&m).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if m.Status == store.MilestoneOpen && m.ClosedAt == nil {
		_ = a.Store.DB.Model(&m).Update("closed_at", nil).Error
	}
	_ = a.Store.LogAudit(callerUsername(c), "project.milestone.patch", fmt.Sprintf("%s#%d", proj.Slug, m.Number))
	c.JSON(http.StatusOK, a.milestoneJSON(proj, user, m))
}
