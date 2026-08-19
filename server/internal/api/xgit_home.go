package api

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type xgitOverviewResponse struct {
	Profile       socialProfileResponse `json:"profile"`
	RepoCount     int                   `json:"repo_count"`
	StarCount     int64                 `json:"star_count"`
	Popular       []projectResponse     `json:"popular"`
	Contributions xgitContributions     `json:"contributions"`
	Activity      []xgitActivityItem    `json:"activity"`
}

type xgitContributions struct {
	Total int            `json:"total"`
	Days  []xgitDayCount `json:"days"`
}

type xgitDayCount struct {
	Date  string `json:"date"`
	Count int    `json:"count"`
}

type xgitActivityItem struct {
	Kind        string    `json:"kind"`
	Month       string    `json:"month,omitempty"`
	Count       int       `json:"count,omitempty"`
	RepoCount   int       `json:"repo_count,omitempty"`
	Repos       []string  `json:"repos,omitempty"`
	Slug        string    `json:"slug,omitempty"`
	Number      uint      `json:"number,omitempty"`
	Title       string    `json:"title,omitempty"`
	Description string    `json:"description,omitempty"`
	Comments    int64     `json:"comments,omitempty"`
	ThreadID    uint      `json:"thread_id,omitempty"`
	Language    string    `json:"language,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

func (a *App) visibleProjects(user store.User) []store.Project {
	q := a.Store.DB.Where("archived_at IS NULL")
	if user.Role.Rank() < store.RoleViewer.Rank() {
		var ids []uint
		_ = a.Store.DB.Model(&store.ProjectMember{}).Where("user_id = ?", user.ID).Pluck("project_id", &ids).Error
		if len(ids) == 0 {
			return nil
		}
		q = q.Where("id IN ?", ids)
	}
	var rows []store.Project
	_ = q.Order("updated_at desc").Find(&rows).Error
	return rows
}

func (a *App) decorateProjectCard(user store.User, proj store.Project, cards bool) projectResponse {
	out := a.projectResponse(proj, false)
	var stars int64
	_ = a.Store.DB.Model(&store.ProjectStar{}).Where("project_id = ?", proj.ID).Count(&stars).Error
	out.StarCount = stars
	var mine int64
	_ = a.Store.DB.Model(&store.ProjectStar{}).Where("project_id = ? AND user_id = ?", proj.ID, user.ID).Count(&mine).Error
	out.Starred = mine > 0
	if !cards {
		return out
	}
	root := a.gitDir()
	out.Language = forge.PrimaryLanguage(root, proj.Slug)
	if last := forge.LastCommit(root, proj.Slug); last != nil {
		out.LastCommitAt = forge.ParseCommitTime(last.Date)
	}
	out.Spark = forge.WeeklyCounts(root, proj.Slug, 12)
	return out
}

func (a *App) handleXgitOverview(c *gin.Context) {
	var user store.User
	if err := a.Store.DB.First(&user, callerUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	prof, err := a.ensureProfile(user)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	rows := a.visibleProjects(user)
	var starred int64
	_ = a.Store.DB.Model(&store.ProjectStar{}).Where("user_id = ?", user.ID).Count(&starred).Error

	popular := make([]projectResponse, 0, 6)
	for i, p := range rows {
		if i >= 6 {
			break
		}
		popular = append(popular, a.decorateProjectCard(user, p, true))
	}

	since := time.Now().UTC().AddDate(-1, 0, 0)
	dayMap := map[string]int{}
	author := user.Username
	if prof.DisplayName != "" {
		author = prof.DisplayName
	}
	root := a.gitDir()
	for _, p := range rows {
		for day, n := range forge.AuthorDayCounts(root, p.Slug, user.Username, since) {
			dayMap[day] += n
		}
		if !strings.EqualFold(author, user.Username) {
			for day, n := range forge.AuthorDayCounts(root, p.Slug, author, since) {
				dayMap[day] += n
			}
		}
	}

	var mrs []store.MergeRequest
	if len(rows) > 0 {
		ids := make([]uint, 0, len(rows))
		for _, p := range rows {
			ids = append(ids, p.ID)
		}
		_ = a.Store.DB.Where("author_id = ? AND project_id IN ? AND created_at >= ?", user.ID, ids, since).
			Order("created_at desc").Find(&mrs).Error
	}
	for _, mr := range mrs {
		dayMap[mr.CreatedAt.UTC().Format("2006-01-02")]++
	}
	for _, p := range rows {
		if p.CreatedAt.Before(since) {
			continue
		}
		dayMap[p.CreatedAt.UTC().Format("2006-01-02")]++
	}

	days := make([]xgitDayCount, 0, 371)
	total := 0
	for d := 0; d < 371; d++ {
		day := since.AddDate(0, 0, d).Format("2006-01-02")
		n := dayMap[day]
		total += n
		days = append(days, xgitDayCount{Date: day, Count: n})
	}

	c.JSON(http.StatusOK, xgitOverviewResponse{
		Profile:       a.profileResponse(prof, user.Username, user.ID),
		RepoCount:     len(rows),
		StarCount:     starred,
		Popular:       popular,
		Contributions: xgitContributions{Total: total, Days: days},
		Activity:      a.xgitActivity(user, rows, mrs),
	})
}

func (a *App) xgitActivity(user store.User, rows []store.Project, mrs []store.MergeRequest) []xgitActivityItem {
	now := time.Now().UTC()
	month := now.Format("2006-01")
	bySlug := map[uint]store.Project{}
	for _, p := range rows {
		bySlug[p.ID] = p
	}

	commitRepos := map[string]int{}
	commitTotal := 0
	root := a.gitDir()
	sinceMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	for _, p := range rows {
		n := 0
		for day, c := range forge.AuthorDayCounts(root, p.Slug, user.Username, sinceMonth) {
			if strings.HasPrefix(day, month) {
				n += c
			}
		}
		if n > 0 {
			commitRepos[p.Slug] = n
			commitTotal += n
		}
	}

	out := make([]xgitActivityItem, 0, 8)
	if commitTotal > 0 {
		repos := make([]string, 0, len(commitRepos))
		for slug := range commitRepos {
			repos = append(repos, slug)
		}
		sort.Strings(repos)
		out = append(out, xgitActivityItem{
			Kind:      "commits",
			Month:     month,
			Count:     commitTotal,
			RepoCount: len(repos),
			Repos:     repos,
			CreatedAt: now,
		})
	}

	created := make([]xgitActivityItem, 0)
	for _, p := range rows {
		if p.CreatedAt.Format("2006-01") != month {
			continue
		}
		created = append(created, xgitActivityItem{
			Kind:      "repo_created",
			Slug:      p.Slug,
			Title:     p.Name,
			Language:  forge.PrimaryLanguage(root, p.Slug),
			CreatedAt: p.CreatedAt,
		})
	}
	if len(created) > 0 {
		sort.Slice(created, func(i, j int) bool { return created[i].CreatedAt.After(created[j].CreatedAt) })
		out = append(out, xgitActivityItem{
			Kind:      "repos_created",
			Month:     month,
			Count:     len(created),
			CreatedAt: created[0].CreatedAt,
		})
		out = append(out, created...)
	}

	for _, mr := range mrs {
		if mr.CreatedAt.Format("2006-01") != month {
			continue
		}
		proj := bySlug[mr.ProjectID]
		var comments int64
		_ = a.Store.DB.Model(&store.Message{}).
			Where("thread_kind = ? AND thread_id = ?", "dm", mr.ThreadID).Count(&comments).Error
		if comments == 0 {
			_ = a.Store.DB.Model(&store.Message{}).Where("thread_id = ?", mr.ThreadID).Count(&comments).Error
		}
		out = append(out, xgitActivityItem{
			Kind:        "merge_request",
			Slug:        proj.Slug,
			Number:      mr.Number,
			Title:       mr.Title,
			Description: mr.Description,
			Comments:    comments,
			ThreadID:    mr.ThreadID,
			CreatedAt:   mr.CreatedAt,
		})
	}
	return out
}

func (a *App) handleXgitStars(c *gin.Context) {
	var user store.User
	if err := a.Store.DB.First(&user, callerUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	var stars []store.ProjectStar
	_ = a.Store.DB.Where("user_id = ?", user.ID).Order("created_at desc").Find(&stars).Error
	items := make([]projectResponse, 0, len(stars))
	for _, s := range stars {
		var p store.Project
		if err := a.Store.DB.First(&p, s.ProjectID).Error; err != nil || p.ArchivedAt != nil {
			continue
		}
		if !a.canSeeProject(user, p) {
			continue
		}
		items = append(items, a.decorateProjectCard(user, p, true))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) handleToggleProjectStar(c *gin.Context) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	var row store.ProjectStar
	err := a.Store.DB.Where("project_id = ? AND user_id = ?", proj.ID, user.ID).First(&row).Error
	if err == nil {
		_ = a.Store.DB.Delete(&row).Error
		c.JSON(http.StatusOK, a.decorateProjectCard(user, proj, false))
		return
	}
	if err := a.Store.DB.Create(&store.ProjectStar{ProjectID: proj.ID, UserID: user.ID}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.JSON(http.StatusOK, a.decorateProjectCard(user, proj, false))
}
