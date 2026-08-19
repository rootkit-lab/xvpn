package api

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/marketplace"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

const maxSocialMediaBytes = 32 << 20 // 32 MiB

var allowedSocialMIME = map[string]string{
	"image/jpeg":      "image",
	"image/png":       "image",
	"image/gif":       "image",
	"image/webp":      "image",
	"audio/webm":      "audio",
	"audio/ogg":       "audio",
	"audio/mp4":       "audio",
	"audio/mpeg":      "audio",
	"audio/wav":       "audio",
	"application/pdf": "file",
	"text/plain":      "file",
	"application/zip": "file",
}

func (a *App) messageResponse(m store.Message) socialMessageResponse {
	resp := socialMessageResponse{
		ID: m.ID, ThreadKind: m.ThreadKind, ThreadID: m.ThreadID, AuthorID: m.AuthorID,
		Kind: m.Kind, Body: m.Body, AttachmentID: m.AttachmentID, CreatedAt: m.CreatedAt,
	}
	if m.Kind == "" {
		resp.Kind = "text"
	}
	if m.AttachmentID != nil {
		var att store.SocialAttachment
		if a.Store.DB.First(&att, *m.AttachmentID).Error == nil {
			resp.Filename = att.Filename
			resp.Mime = att.Mime
			resp.SizeBytes = att.SizeBytes
		}
	}
	return resp
}

func (a *App) handleSocialUploadAttachment(c *gin.Context) {
	if a.SocialMedia == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "armazenamento de mídia indisponível"})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSocialMediaBytes+1<<20)
	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "arquivo é obrigatório"})
		return
	}
	if fh.Size > maxSocialMediaBytes {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "arquivo excede 32 MB"})
		return
	}
	src, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "não foi possível ler o arquivo"})
		return
	}
	defer src.Close()

	mime := normalizeSocialMIME(fh.Header.Get("Content-Type"))
	if mime == "" {
		mime = sniffSocialMIME(fh.Filename)
	}
	kind, ok := allowedSocialMIME[mime]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "tipo de arquivo não permitido"})
		return
	}

	result, err := a.SocialMedia.Put(io.LimitReader(src, maxSocialMediaBytes+1))
	if err != nil {
		if err == marketplace.ErrAssetTooLarge {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "arquivo excede 32 MB"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if result.Size > maxSocialMediaBytes {
		_ = a.SocialMedia.Remove(result.RelPath)
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "arquivo excede 32 MB"})
		return
	}

	me := callerUserID(c)
	att := store.SocialAttachment{
		UploaderID:  me,
		StoragePath: result.RelPath,
		Filename:    sanitizeFilename(fh.Filename),
		Mime:        mime,
		SizeBytes:   result.Size,
		SHA256:      result.SHA256,
	}
	if err := a.Store.DB.Create(&att).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{
		"id": att.ID, "filename": att.Filename, "mime": att.Mime,
		"size_bytes": att.SizeBytes, "kind": kind,
	})
}

func (a *App) handleSocialDownloadAttachment(c *gin.Context) {
	if a.SocialMedia == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "armazenamento de mídia indisponível"})
		return
	}
	var id uint
	if _, err := parseUintParam(c, "id", &id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var att store.SocialAttachment
	if err := a.Store.DB.First(&att, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "anexo não encontrado"})
		return
	}
	me := callerUserID(c)
	if !a.canAccessAttachment(att.ID, me) && att.UploaderID != me {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem acesso a este anexo"})
		return
	}
	abs, err := a.SocialMedia.AbsPath(att.StoragePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.Header("X-Content-Type-Options", "nosniff")
	if att.Mime != "" {
		c.Header("Content-Type", att.Mime)
	}
	c.Header("Content-Disposition", `inline; filename="`+sanitizeFilename(att.Filename)+`"`)
	c.File(abs)
}

func (a *App) canAccessAttachment(attachmentID, userID uint) bool {
	if a.isProfileMedia(attachmentID) {
		return true
	}
	var msgs []store.Message
	_ = a.Store.DB.Where("attachment_id = ?", attachmentID).Find(&msgs).Error
	for _, m := range msgs {
		if a.canAccessThread(m.ThreadKind, m.ThreadID, userID) {
			return true
		}
	}
	var stories []store.Story
	_ = a.Store.DB.Where("attachment_id = ?", attachmentID).Find(&stories).Error
	return len(stories) > 0
}

type createStoryRequest struct {
	Body         string `json:"body"`
	Kind         string `json:"kind"`
	AttachmentID *uint  `json:"attachment_id"`
}

type storyItemResponse struct {
	ID           uint      `json:"id"`
	AuthorID     uint      `json:"author_id"`
	Username     string    `json:"username"`
	Kind         string    `json:"kind"`
	Body         string    `json:"body"`
	AttachmentID *uint     `json:"attachment_id,omitempty"`
	Mime         string    `json:"mime,omitempty"`
	Viewed       bool      `json:"viewed"`
	ExpiresAt    time.Time `json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
}

type storyAuthorResponse struct {
	AuthorID  uint                `json:"author_id"`
	Username  string              `json:"username"`
	AvatarURL string              `json:"avatar_url,omitempty"`
	Unseen    bool                `json:"unseen"`
	Items     []storyItemResponse `json:"items"`
}

func (a *App) handleSocialCreateStory(c *gin.Context) {
	var req createStoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "JSON inválido"})
		return
	}
	kind := strings.TrimSpace(req.Kind)
	if kind == "" {
		kind = "text"
	}
	body := strings.TrimSpace(req.Body)
	if kind == "text" && body == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "texto é obrigatório"})
		return
	}
	if kind == "image" && (req.AttachmentID == nil || *req.AttachmentID == 0) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "imagem é obrigatória"})
		return
	}
	if kind != "text" && kind != "image" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kind inválido"})
		return
	}
	me := callerUserID(c)
	if req.AttachmentID != nil {
		var att store.SocialAttachment
		if err := a.Store.DB.First(&att, *req.AttachmentID).Error; err != nil || att.UploaderID != me {
			c.JSON(http.StatusBadRequest, gin.H{"error": "anexo inválido"})
			return
		}
	}
	st := store.Story{
		AuthorID: me, Kind: kind, Body: body, AttachmentID: req.AttachmentID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if err := a.Store.DB.Create(&st).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.JSON(http.StatusCreated, a.storyItem(st, me, callerUsername(c)))
}

func (a *App) handleSocialListStories(c *gin.Context) {
	me := callerUserID(c)
	now := time.Now()
	var stories []store.Story
	if err := a.Store.DB.Where("expires_at > ?", now).Order("created_at asc").Find(&stories).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	byAuthor := map[uint]*storyAuthorResponse{}
	order := make([]uint, 0)
	for _, st := range stories {
		item := a.storyItem(st, me, "")
		g, ok := byAuthor[st.AuthorID]
		if !ok {
			g = &storyAuthorResponse{AuthorID: st.AuthorID, Username: item.Username, Items: nil}
			byAuthor[st.AuthorID] = g
			order = append(order, st.AuthorID)
		}
		g.Items = append(g.Items, item)
		if !item.Viewed {
			g.Unseen = true
		}
	}
	avatars := map[uint]string{}
	if len(order) > 0 {
		var profs []store.SocialProfile
		_ = a.Store.DB.Where("user_id IN ?", order).Find(&profs).Error
		for _, p := range profs {
			avatars[p.UserID] = p.AvatarURL
		}
	}
	out := make([]storyAuthorResponse, 0, len(order))
	for _, id := range order {
		g := *byAuthor[id]
		g.AvatarURL = avatars[id]
		out = append(out, g)
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

func (a *App) handleSocialViewStory(c *gin.Context) {
	var id uint
	if _, err := parseUintParam(c, "id", &id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var st store.Story
	if err := a.Store.DB.First(&st, id).Error; err != nil || st.ExpiresAt.Before(time.Now()) {
		c.JSON(http.StatusNotFound, gin.H{"error": "story não encontrada"})
		return
	}
	me := callerUserID(c)
	v := store.StoryView{StoryID: st.ID, ViewerID: me}
	_ = a.Store.DB.Where("story_id = ? AND viewer_id = ?", st.ID, me).FirstOrCreate(&v).Error
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) storyItem(st store.Story, viewerID uint, usernameHint string) storyItemResponse {
	username := usernameHint
	if username == "" {
		var u store.User
		if a.Store.DB.First(&u, st.AuthorID).Error == nil {
			username = u.Username
		}
	}
	var n int64
	_ = a.Store.DB.Model(&store.StoryView{}).Where("story_id = ? AND viewer_id = ?", st.ID, viewerID).Count(&n).Error
	item := storyItemResponse{
		ID: st.ID, AuthorID: st.AuthorID, Username: username, Kind: st.Kind,
		Body: st.Body, AttachmentID: st.AttachmentID, Viewed: n > 0 || st.AuthorID == viewerID,
		ExpiresAt: st.ExpiresAt, CreatedAt: st.CreatedAt,
	}
	if st.AttachmentID != nil {
		var att store.SocialAttachment
		if a.Store.DB.First(&att, *st.AttachmentID).Error == nil {
			item.Mime = att.Mime
		}
	}
	return item
}

func normalizeSocialMIME(mime string) string {
	base := strings.ToLower(strings.TrimSpace(strings.Split(mime, ";")[0]))
	if base == "" || base == "application/octet-stream" {
		return ""
	}
	if base == "audio/x-m4a" {
		return "audio/mp4"
	}
	return base
}

func sniffSocialMIME(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".webm":
		return "audio/webm"
	case ".ogg":
		return "audio/ogg"
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".pdf":
		return "application/pdf"
	case ".txt":
		return "text/plain"
	case ".zip":
		return "application/zip"
	default:
		return ""
	}
}

func sanitizeFilename(name string) string {
	base := filepath.Base(name)
	var b strings.Builder
	for _, r := range base {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" || out == "." {
		return "arquivo"
	}
	if len(out) > 120 {
		return out[:120]
	}
	return out
}
