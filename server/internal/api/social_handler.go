package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type socialProfileResponse struct {
	UserID      uint   `json:"user_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Bio         string `json:"bio"`
	AvatarURL   string `json:"avatar_url"`
	Following   bool   `json:"following"`
	Followers   int64  `json:"followers"`
}

type socialGroupResponse struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	OwnerUserID uint   `json:"owner_user_id"`
	MemberCount int64  `json:"member_count"`
}

type socialThreadResponse struct {
	ID         uint       `json:"id"`
	Kind       string     `json:"kind"`
	Title      string     `json:"title"`
	PeerUserID uint       `json:"peer_user_id,omitempty"`
	LastBody   string     `json:"last_body,omitempty"`
	LastAt     *time.Time `json:"last_at,omitempty"`
}

type socialMessageResponse struct {
	ID           uint      `json:"id"`
	ThreadKind   string    `json:"thread_kind"`
	ThreadID     uint      `json:"thread_id"`
	AuthorID     uint      `json:"author_id"`
	Kind         string    `json:"kind"`
	Body         string    `json:"body"`
	AttachmentID *uint     `json:"attachment_id,omitempty"`
	Filename     string    `json:"filename,omitempty"`
	Mime         string    `json:"mime,omitempty"`
	SizeBytes    int64     `json:"size_bytes,omitempty"`
	Delivered    bool      `json:"delivered"`
	Read         bool      `json:"read"`
	CreatedAt    time.Time `json:"created_at"`
}

func (a *App) ensureProfile(user store.User) (store.SocialProfile, error) {
	var p store.SocialProfile
	err := a.Store.DB.Where("user_id = ?", user.ID).First(&p).Error
	if err == nil {
		return p, nil
	}
	if err != gorm.ErrRecordNotFound {
		return p, err
	}
	p = store.SocialProfile{UserID: user.ID, DisplayName: user.Username}
	if err := a.Store.DB.Create(&p).Error; err != nil {
		return p, err
	}
	return p, nil
}

func (a *App) profileResponse(p store.SocialProfile, username string, viewerID uint) socialProfileResponse {
	var followers int64
	_ = a.Store.DB.Model(&store.Follow{}).Where("following_id = ?", p.UserID).Count(&followers).Error
	var n int64
	_ = a.Store.DB.Model(&store.Follow{}).Where("follower_id = ? AND following_id = ?", viewerID, p.UserID).Count(&n).Error
	return socialProfileResponse{
		UserID:      p.UserID,
		Username:    username,
		DisplayName: p.DisplayName,
		Bio:         p.Bio,
		AvatarURL:   p.AvatarURL,
		Following:   n > 0 && viewerID != p.UserID,
		Followers:   followers,
	}
}

func (a *App) handleSocialPeople(c *gin.Context) {
	p := parsePage(c)
	viewer := callerUserID(c)
	q := a.Store.DB.Model(&store.User{})
	if p.Q != "" {
		q = q.Where("username LIKE ?", p.like())
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	var users []store.User
	if err := p.apply(q.Order("username")).Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]socialProfileResponse, 0, len(users))
	for _, u := range users {
		prof, err := a.ensureProfile(u)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
			return
		}
		items = append(items, a.profileResponse(prof, u.Username, viewer))
	}
	writePage(c, items, total, p)
}

func (a *App) handleSocialMeGet(c *gin.Context) {
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
	c.JSON(http.StatusOK, a.profileResponse(prof, user.Username, user.ID))
}

type patchSocialProfileRequest struct {
	DisplayName *string `json:"display_name"`
	Bio         *string `json:"bio"`
	AvatarURL   *string `json:"avatar_url"`
}

func (a *App) handleSocialMePatch(c *gin.Context) {
	var req patchSocialProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
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
	if req.DisplayName != nil {
		name := strings.TrimSpace(*req.DisplayName)
		if name == "" || len(name) > 80 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "display_name inválido"})
			return
		}
		prof.DisplayName = name
	}
	if req.Bio != nil {
		bio := *req.Bio
		if len(bio) > 500 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bio muito longa"})
			return
		}
		prof.Bio = bio
	}
	if req.AvatarURL != nil {
		prof.AvatarURL = strings.TrimSpace(*req.AvatarURL)
	}
	if err := a.Store.DB.Save(&prof).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.JSON(http.StatusOK, a.profileResponse(prof, user.Username, user.ID))
}

func (a *App) findUserByUsername(username string) (store.User, error) {
	var u store.User
	err := a.Store.DB.Where("username = ?", username).First(&u).Error
	return u, err
}

func (a *App) handleSocialProfileGet(c *gin.Context) {
	u, err := a.findUserByUsername(c.Param("username"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	prof, err := a.ensureProfile(u)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.JSON(http.StatusOK, a.profileResponse(prof, u.Username, callerUserID(c)))
}

func (a *App) handleSocialFollow(c *gin.Context) {
	target, err := a.findUserByUsername(c.Param("username"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	me := callerUserID(c)
	if target.ID == me {
		c.JSON(http.StatusBadRequest, gin.H{"error": "não dá para seguir a si mesmo"})
		return
	}
	f := store.Follow{FollowerID: me, FollowingID: target.ID}
	if err := a.Store.DB.Where("follower_id = ? AND following_id = ?", me, target.ID).FirstOrCreate(&f).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) handleSocialUnfollow(c *gin.Context) {
	target, err := a.findUserByUsername(c.Param("username"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	if err := a.Store.DB.Where("follower_id = ? AND following_id = ?", callerUserID(c), target.ID).Delete(&store.Follow{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type createGroupRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
}

func (a *App) handleSocialCreateGroup(c *gin.Context) {
	var req createGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nome é obrigatório"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || len(name) > 80 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nome inválido"})
		return
	}
	me := callerUserID(c)
	g := store.SocialGroup{Name: name, Description: strings.TrimSpace(req.Description), OwnerUserID: me}
	if err := a.Store.DB.Create(&g).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.DB.Create(&store.SocialGroupMember{GroupID: g.ID, UserID: me}).Error
	c.JSON(http.StatusCreated, socialGroupResponse{ID: g.ID, Name: g.Name, Description: g.Description, OwnerUserID: me, MemberCount: 1})
}

func (a *App) handleSocialListGroups(c *gin.Context) {
	p := parsePage(c)
	me := callerUserID(c)
	sub := a.Store.DB.Model(&store.SocialGroupMember{}).Select("group_id").Where("user_id = ?", me)
	q := a.Store.DB.Model(&store.SocialGroup{}).Where("id IN (?)", sub)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	var groups []store.SocialGroup
	if err := p.apply(q.Order("id desc")).Find(&groups).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]socialGroupResponse, 0, len(groups))
	for _, g := range groups {
		var n int64
		_ = a.Store.DB.Model(&store.SocialGroupMember{}).Where("group_id = ?", g.ID).Count(&n).Error
		items = append(items, socialGroupResponse{ID: g.ID, Name: g.Name, Description: g.Description, OwnerUserID: g.OwnerUserID, MemberCount: n})
	}
	writePage(c, items, total, p)
}

type joinGroupRequest struct {
	Username string `json:"username" binding:"required"`
}

func (a *App) handleSocialInviteGroup(c *gin.Context) {
	var req joinGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username é obrigatório"})
		return
	}
	var g store.SocialGroup
	if err := a.Store.DB.First(&g, c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "grupo não encontrado"})
		return
	}
	if !a.isGroupMember(g.ID, callerUserID(c)) {
		c.JSON(http.StatusForbidden, gin.H{"error": "só membros convidam"})
		return
	}
	u, err := a.findUserByUsername(req.Username)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	m := store.SocialGroupMember{GroupID: g.ID, UserID: u.ID}
	_ = a.Store.DB.Where("group_id = ? AND user_id = ?", g.ID, u.ID).FirstOrCreate(&m).Error
	if a.Hub != nil {
		a.Hub.sendToMany(a.threadMemberIDs("group", g.ID), wsEvent{Type: "group.updated", Payload: gin.H{"group_id": g.ID}})
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) isGroupMember(groupID, userID uint) bool {
	var n int64
	_ = a.Store.DB.Model(&store.SocialGroupMember{}).Where("group_id = ? AND user_id = ?", groupID, userID).Count(&n).Error
	return n > 0
}

type openThreadRequest struct {
	Username string `json:"username" binding:"required"`
}

func (a *App) handleSocialOpenThread(c *gin.Context) {
	var req openThreadRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username é obrigatório"})
		return
	}
	peer, err := a.findUserByUsername(req.Username)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	me := callerUserID(c)
	if peer.ID == me {
		c.JSON(http.StatusBadRequest, gin.H{"error": "não dá para abrir DM consigo mesmo"})
		return
	}
	threadID, err := a.findOrCreateDM(me, peer.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	th := socialThreadResponse{ID: threadID, Kind: "dm", Title: peer.Username, PeerUserID: peer.ID}
	a.attachLastMessage(&th)
	c.JSON(http.StatusOK, th)
}

func (a *App) findOrCreateDM(aID, bID uint) (uint, error) {
	var mine []store.DirectThreadMember
	if err := a.Store.DB.Where("user_id = ?", aID).Find(&mine).Error; err != nil {
		return 0, err
	}
	for _, m := range mine {
		var n int64
		_ = a.Store.DB.Model(&store.DirectThreadMember{}).Where("thread_id = ? AND user_id = ?", m.ThreadID, bID).Count(&n).Error
		if n > 0 {
			return m.ThreadID, nil
		}
	}
	th := store.DirectThread{}
	if err := a.Store.DB.Create(&th).Error; err != nil {
		return 0, err
	}
	if err := a.Store.DB.Create(&store.DirectThreadMember{ThreadID: th.ID, UserID: aID}).Error; err != nil {
		return 0, err
	}
	if err := a.Store.DB.Create(&store.DirectThreadMember{ThreadID: th.ID, UserID: bID}).Error; err != nil {
		return 0, err
	}
	return th.ID, nil
}

func (a *App) handleSocialListThreads(c *gin.Context) {
	me := callerUserID(c)
	p := parsePage(c)
	q := a.Store.DB.Model(&store.DirectThreadMember{}).Where("user_id = ?", me)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	var memberships []store.DirectThreadMember
	if err := p.apply(q.Order("id desc")).Find(&memberships).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]socialThreadResponse, 0, len(memberships))
	for _, m := range memberships {
		var others []store.DirectThreadMember
		_ = a.Store.DB.Where("thread_id = ? AND user_id <> ?", m.ThreadID, me).Find(&others).Error
		title := "DM"
		var peer uint
		if len(others) > 0 {
			peer = others[0].UserID
			var u store.User
			if a.Store.DB.First(&u, peer).Error == nil {
				title = u.Username
			}
		}
		items = append(items, socialThreadResponse{ID: m.ThreadID, Kind: "dm", Title: title, PeerUserID: peer})
		a.attachLastMessage(&items[len(items)-1])
	}
	writePage(c, items, total, p)
}

func (a *App) attachLastMessage(th *socialThreadResponse) {
	var msg store.Message
	err := a.Store.DB.Where("thread_kind = ? AND thread_id = ?", th.Kind, th.ID).Order("id desc").First(&msg).Error
	if err != nil {
		return
	}
	th.LastBody = previewMessageBody(msg)
	t := msg.CreatedAt
	th.LastAt = &t
}

func previewMessageBody(msg store.Message) string {
	switch msg.Kind {
	case "image":
		return "📷 Foto"
	case "audio":
		return "🎤 Áudio"
	case "file":
		return "📎 Arquivo"
	default:
		return msg.Body
	}
}

func (a *App) canAccessThread(kind string, threadID, userID uint) bool {
	if kind == "group" {
		return a.isGroupMember(threadID, userID)
	}
	var n int64
	_ = a.Store.DB.Model(&store.DirectThreadMember{}).Where("thread_id = ? AND user_id = ?", threadID, userID).Count(&n).Error
	return n > 0
}

func (a *App) handleSocialListMessages(c *gin.Context) {
	kind := c.Param("kind")
	if kind != "dm" && kind != "group" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kind inválido"})
		return
	}
	var threadID uint
	if _, err := parseUintParam(c, "id", &threadID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	me := callerUserID(c)
	if !a.canAccessThread(kind, threadID, me) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem acesso a este thread"})
		return
	}
	p := parsePage(c)
	q := a.Store.DB.Model(&store.Message{}).Where("thread_kind = ? AND thread_id = ?", kind, threadID)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	var msgs []store.Message
	if err := p.apply(q.Order("id desc")).Find(&msgs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]socialMessageResponse, 0, len(msgs))
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		items = append(items, a.messageResponse(m))
	}
	a.attachReceipts(items)
	writePage(c, items, total, p)
}

type postMessageRequest struct {
	Body         string `json:"body"`
	Kind         string `json:"kind"`
	AttachmentID *uint  `json:"attachment_id"`
}

func (a *App) handleSocialPostMessage(c *gin.Context) {
	kind := c.Param("kind")
	if kind != "dm" && kind != "group" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kind inválido"})
		return
	}
	var threadID uint
	if _, err := parseUintParam(c, "id", &threadID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var req postMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "JSON inválido"})
		return
	}
	body := strings.TrimSpace(req.Body)
	if len(body) > 4000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mensagem inválida"})
		return
	}
	msgKind := strings.TrimSpace(req.Kind)
	if msgKind == "" {
		msgKind = "text"
	}
	if msgKind != "text" && msgKind != "image" && msgKind != "file" && msgKind != "audio" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "kind de mensagem inválido"})
		return
	}
	if msgKind == "text" && body == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body é obrigatório"})
		return
	}
	if msgKind != "text" && (req.AttachmentID == nil || *req.AttachmentID == 0) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "anexo é obrigatório"})
		return
	}
	me := callerUserID(c)
	if !a.canAccessThread(kind, threadID, me) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem acesso a este thread"})
		return
	}
	if req.AttachmentID != nil {
		var att store.SocialAttachment
		if err := a.Store.DB.First(&att, *req.AttachmentID).Error; err != nil || att.UploaderID != me {
			c.JSON(http.StatusBadRequest, gin.H{"error": "anexo inválido"})
			return
		}
	}
	if a.msgLimiter != nil && !a.msgLimiter.allow("u:"+strconv.FormatUint(uint64(me), 10)) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "muitas mensagens"})
		return
	}
	msg := store.Message{ThreadKind: kind, ThreadID: threadID, AuthorID: me, Kind: msgKind, Body: body, AttachmentID: req.AttachmentID}
	if err := a.Store.DB.Create(&msg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "social.message", "kind="+kind+" thread="+strconv.FormatUint(uint64(threadID), 10))
	resp := a.messageResponse(msg)
	batch := []socialMessageResponse{resp}
	a.attachReceipts(batch)
	resp = batch[0]
	if a.Hub != nil {
		a.Hub.sendToMany(a.threadMemberIDs(kind, threadID), wsEvent{Type: "message.new", Payload: resp})
	}
	c.JSON(http.StatusCreated, resp)
}

func parseUintParam(c *gin.Context, name string, dest *uint) (uint64, error) {
	id, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil {
		return 0, err
	}
	*dest = uint(id)
	return id, nil
}
