package api

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type socialCommentResponse struct {
	ID          uint      `json:"id"`
	PostID      uint      `json:"post_id"`
	AuthorID    uint      `json:"author_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"created_at"`
}

type createCommentRequest struct {
	Body string `json:"body"`
}

func (a *App) loadPost(c *gin.Context) (store.SocialPost, bool) {
	var post store.SocialPost
	if err := a.Store.DB.First(&post, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "post não encontrado"})
		return post, false
	}
	return post, true
}

func (a *App) rootOriginal(post store.SocialPost) store.SocialPost {
	if post.Kind != "repost" || post.OriginalID == nil {
		return post
	}
	var orig store.SocialPost
	if err := a.Store.DB.First(&orig, *post.OriginalID).Error; err != nil {
		return post
	}
	return orig
}

func (a *App) handleSocialStarPost(c *gin.Context) {
	post, ok := a.loadPost(c)
	if !ok {
		return
	}
	target := a.rootOriginal(post)
	me := callerUserID(c)
	var existing store.SocialPostStar
	err := a.Store.DB.Where("post_id = ? AND user_id = ?", target.ID, me).First(&existing).Error
	starred := false
	if err == nil {
		_ = a.Store.DB.Delete(&existing).Error
	} else {
		if err := a.Store.DB.Create(&store.SocialPostStar{PostID: target.ID, UserID: me}).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
			return
		}
		starred = true
	}
	var n int64
	_ = a.Store.DB.Model(&store.SocialPostStar{}).Where("post_id = ?", target.ID).Count(&n).Error
	c.JSON(http.StatusOK, gin.H{"starred": starred, "stars": n, "post_id": target.ID})
}

func (a *App) handleSocialListComments(c *gin.Context) {
	post, ok := a.loadPost(c)
	if !ok {
		return
	}
	target := a.rootOriginal(post)
	p := parsePage(c)
	q := a.Store.DB.Model(&store.SocialPostComment{}).Where("post_id = ?", target.ID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	var rows []store.SocialPostComment
	if err := p.apply(q.Order("created_at ASC")).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]socialCommentResponse, 0, len(rows))
	for _, row := range rows {
		var user store.User
		if err := a.Store.DB.First(&user, row.AuthorID).Error; err != nil {
			continue
		}
		prof, err := a.ensureProfile(user)
		if err != nil {
			continue
		}
		name := prof.DisplayName
		if name == "" {
			name = user.Username
		}
		items = append(items, socialCommentResponse{
			ID: row.ID, PostID: row.PostID, AuthorID: row.AuthorID,
			Username: user.Username, DisplayName: name, AvatarURL: prof.AvatarURL,
			Body: row.Body, CreatedAt: row.CreatedAt,
		})
	}
	writePage(c, items, total, p)
}

func (a *App) handleSocialCreateComment(c *gin.Context) {
	post, ok := a.loadPost(c)
	if !ok {
		return
	}
	target := a.rootOriginal(post)
	var req createCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" || utf8.RuneCountInString(body) > maxPostRunes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "texto vazio ou acima de 280 caracteres"})
		return
	}
	me := callerUserID(c)
	row := store.SocialPostComment{PostID: target.ID, AuthorID: me, Body: body}
	if err := a.Store.DB.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	var user store.User
	if err := a.Store.DB.First(&user, me).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	prof, _ := a.ensureProfile(user)
	name := prof.DisplayName
	if name == "" {
		name = user.Username
	}
	c.JSON(http.StatusCreated, socialCommentResponse{
		ID: row.ID, PostID: row.PostID, AuthorID: row.AuthorID,
		Username: user.Username, DisplayName: name, AvatarURL: prof.AvatarURL,
		Body: row.Body, CreatedAt: row.CreatedAt,
	})
}

func (a *App) handleSocialRepost(c *gin.Context) {
	post, ok := a.loadPost(c)
	if !ok {
		return
	}
	target := a.rootOriginal(post)
	me := callerUserID(c)
	if target.AuthorID == me {
		c.JSON(http.StatusBadRequest, gin.H{"error": "não dá para repostar o próprio post"})
		return
	}
	var existing store.SocialPost
	err := a.Store.DB.Where("kind = ? AND original_id = ? AND author_id = ?", "repost", target.ID, me).First(&existing).Error
	reposted := false
	if err == nil {
		_ = a.Store.DB.Delete(&existing).Error
	} else {
		row := store.SocialPost{AuthorID: me, Body: "", Kind: "repost", OriginalID: &target.ID}
		if err := a.Store.DB.Create(&row).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
			return
		}
		reposted = true
	}
	var n int64
	_ = a.Store.DB.Model(&store.SocialPost{}).Where("kind = ? AND original_id = ?", "repost", target.ID).Count(&n).Error
	c.JSON(http.StatusOK, gin.H{"reposted": reposted, "reposts": n, "post_id": target.ID})
}
