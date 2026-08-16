package api

import (
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

const maxPostRunes = 280

type socialPostResponse struct {
	ID          uint      `json:"id"`
	AuthorID    uint      `json:"author_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"created_at"`
}

type createPostRequest struct {
	Body string `json:"body"`
}

func (a *App) postResponse(p store.SocialPost, author store.User, prof store.SocialProfile) socialPostResponse {
	name := prof.DisplayName
	if name == "" {
		name = author.Username
	}
	return socialPostResponse{
		ID:          p.ID,
		AuthorID:    p.AuthorID,
		Username:    author.Username,
		DisplayName: name,
		AvatarURL:   prof.AvatarURL,
		Body:        p.Body,
		CreatedAt:   p.CreatedAt,
	}
}

func (a *App) handleSocialCreatePost(c *gin.Context) {
	var req createPostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" || utf8.RuneCountInString(body) > maxPostRunes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "texto vazio ou acima de 280 caracteres"})
		return
	}
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
	post := store.SocialPost{AuthorID: user.ID, Body: body}
	if err := a.Store.DB.Create(&post).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.JSON(http.StatusCreated, a.postResponse(post, user, prof))
}

func (a *App) handleSocialFeed(c *gin.Context) {
	p := parsePage(c)
	viewer := callerUserID(c)
	var followIDs []uint
	_ = a.Store.DB.Model(&store.Follow{}).Where("follower_id = ?", viewer).Pluck("following_id", &followIDs).Error
	ids := append(followIDs, viewer)

	q := a.Store.DB.Model(&store.SocialPost{}).Where("author_id IN ?", ids)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	// Rede vazia: mostra o mural geral para o xgroup não nascer mudo.
	if total == 0 {
		q = a.Store.DB.Model(&store.SocialPost{})
		if err := q.Count(&total).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
			return
		}
	}
	var posts []store.SocialPost
	if err := p.apply(q.Order("created_at DESC")).Find(&posts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	writePage(c, a.hydratePosts(posts), total, p)
}

func (a *App) handleSocialUserPosts(c *gin.Context) {
	u, err := a.findUserByUsername(c.Param("username"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	p := parsePage(c)
	q := a.Store.DB.Model(&store.SocialPost{}).Where("author_id = ?", u.ID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	var posts []store.SocialPost
	if err := p.apply(q.Order("created_at DESC")).Find(&posts).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	writePage(c, a.hydratePosts(posts), total, p)
}

func (a *App) handleSocialDeletePost(c *gin.Context) {
	var post store.SocialPost
	if err := a.Store.DB.First(&post, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "post não encontrado"})
		return
	}
	caller := callerUserID(c)
	var user store.User
	_ = a.Store.DB.First(&user, caller).Error
	if post.AuthorID != caller && user.Role != store.RoleAdmin && user.Role != store.RoleSuperAdmin {
		c.JSON(http.StatusForbidden, gin.H{"error": "só o autor pode apagar"})
		return
	}
	if err := a.Store.DB.Delete(&post).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *App) hydratePosts(posts []store.SocialPost) []socialPostResponse {
	out := make([]socialPostResponse, 0, len(posts))
	for _, post := range posts {
		var user store.User
		if err := a.Store.DB.First(&user, post.AuthorID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				continue
			}
			continue
		}
		prof, err := a.ensureProfile(user)
		if err != nil {
			continue
		}
		out = append(out, a.postResponse(post, user, prof))
	}
	return out
}
