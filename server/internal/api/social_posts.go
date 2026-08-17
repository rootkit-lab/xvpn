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

type socialPostOriginal struct {
	ID          uint      `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url"`
	Body        string    `json:"body"`
	CreatedAt   time.Time `json:"created_at"`
}

type socialPostResponse struct {
	ID          uint                `json:"id"`
	AuthorID    uint                `json:"author_id"`
	Username    string              `json:"username"`
	DisplayName string              `json:"display_name"`
	AvatarURL   string              `json:"avatar_url"`
	Body        string              `json:"body"`
	Kind        string              `json:"kind"`
	Presence    string              `json:"presence"`
	Starred     bool                `json:"starred"`
	Stars       int64               `json:"stars"`
	Comments    int64               `json:"comments"`
	Reposts     int64               `json:"reposts"`
	Reposted    bool                `json:"reposted"`
	Original    *socialPostOriginal `json:"original,omitempty"`
	CreatedAt   time.Time           `json:"created_at"`
}

type createPostRequest struct {
	Body string `json:"body"`
}

func postKind(p store.SocialPost) string {
	if p.Kind == "repost" {
		return "repost"
	}
	return "text"
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
		Kind:        postKind(p),
		Presence:    a.presenceOf(p.AuthorID),
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
	if _, err := a.ensureProfile(user); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	post := store.SocialPost{AuthorID: user.ID, Body: body, Kind: "text"}
	if err := a.Store.DB.Create(&post).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	out := a.hydratePosts([]store.SocialPost{post}, user.ID)
	if len(out) == 0 {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.JSON(http.StatusCreated, out[0])
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
	writePage(c, a.hydratePosts(posts, viewer), total, p)
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
	writePage(c, a.hydratePosts(posts, callerUserID(c)), total, p)
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
	_ = a.Store.DB.Where("post_id = ?", post.ID).Delete(&store.SocialPostStar{}).Error
	_ = a.Store.DB.Where("post_id = ?", post.ID).Delete(&store.SocialPostComment{}).Error
	_ = a.Store.DB.Where("original_id = ?", post.ID).Delete(&store.SocialPost{}).Error
	if err := a.Store.DB.Delete(&post).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.Status(http.StatusNoContent)
}

type countRow struct {
	PostID uint
	N      int64
}

func (a *App) hydratePosts(posts []store.SocialPost, viewer uint) []socialPostResponse {
	out := make([]socialPostResponse, 0, len(posts))
	if len(posts) == 0 {
		return out
	}
	ids := make([]uint, 0, len(posts))
	origIDs := make([]uint, 0)
	for _, post := range posts {
		ids = append(ids, post.ID)
		if post.OriginalID != nil && *post.OriginalID > 0 {
			origIDs = append(origIDs, *post.OriginalID)
		}
	}
	engageIDs := append(append([]uint{}, ids...), origIDs...)

	stars := a.countByPostID(&store.SocialPostStar{}, engageIDs)
	comments := a.countByPostID(&store.SocialPostComment{}, engageIDs)
	reposts := a.countReposts(engageIDs)
	starred := a.viewerPostSet(&store.SocialPostStar{}, engageIDs, viewer)
	reposted := a.viewerRepostSet(engageIDs, viewer)
	originals := a.loadOriginals(origIDs)

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
		item := a.postResponse(post, user, prof)
		engageID := post.ID
		if post.OriginalID != nil && *post.OriginalID > 0 {
			engageID = *post.OriginalID
		}
		item.Stars = stars[engageID]
		item.Comments = comments[engageID]
		item.Reposts = reposts[engageID]
		item.Starred = starred[engageID]
		item.Reposted = reposted[engageID]
		if post.OriginalID != nil {
			if orig, ok := originals[*post.OriginalID]; ok {
				item.Original = &orig
			}
		}
		out = append(out, item)
	}
	return out
}

func (a *App) countByPostID(model any, ids []uint) map[uint]int64 {
	out := map[uint]int64{}
	if len(ids) == 0 {
		return out
	}
	var rows []countRow
	_ = a.Store.DB.Model(model).Select("post_id, count(*) as n").
		Where("post_id IN ?", ids).Group("post_id").Scan(&rows).Error
	for _, r := range rows {
		out[r.PostID] = r.N
	}
	return out
}

func (a *App) countReposts(ids []uint) map[uint]int64 {
	out := map[uint]int64{}
	if len(ids) == 0 {
		return out
	}
	var rows []countRow
	_ = a.Store.DB.Model(&store.SocialPost{}).Select("original_id as post_id, count(*) as n").
		Where("kind = ? AND original_id IN ?", "repost", ids).Group("original_id").Scan(&rows).Error
	for _, r := range rows {
		out[r.PostID] = r.N
	}
	return out
}

func (a *App) viewerPostSet(model any, ids []uint, viewer uint) map[uint]bool {
	out := map[uint]bool{}
	if len(ids) == 0 || viewer == 0 {
		return out
	}
	var found []uint
	_ = a.Store.DB.Model(model).Where("post_id IN ? AND user_id = ?", ids, viewer).Pluck("post_id", &found).Error
	for _, id := range found {
		out[id] = true
	}
	return out
}

func (a *App) viewerRepostSet(ids []uint, viewer uint) map[uint]bool {
	out := map[uint]bool{}
	if len(ids) == 0 || viewer == 0 {
		return out
	}
	var found []uint
	_ = a.Store.DB.Model(&store.SocialPost{}).
		Where("kind = ? AND original_id IN ? AND author_id = ?", "repost", ids, viewer).
		Pluck("original_id", &found).Error
	for _, id := range found {
		out[id] = true
	}
	return out
}

func (a *App) loadOriginals(ids []uint) map[uint]socialPostOriginal {
	out := map[uint]socialPostOriginal{}
	if len(ids) == 0 {
		return out
	}
	var posts []store.SocialPost
	if err := a.Store.DB.Where("id IN ?", ids).Find(&posts).Error; err != nil {
		return out
	}
	for _, p := range posts {
		var user store.User
		if err := a.Store.DB.First(&user, p.AuthorID).Error; err != nil {
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
		out[p.ID] = socialPostOriginal{
			ID: p.ID, Username: user.Username, DisplayName: name,
			AvatarURL: prof.AvatarURL, Body: p.Body, CreatedAt: p.CreatedAt,
		}
	}
	return out
}
