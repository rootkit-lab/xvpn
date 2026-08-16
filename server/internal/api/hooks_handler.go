package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type hookBroadcastRequest struct {
	Body  string `json:"body" binding:"required"`
	Group string `json:"group"`
}

// handleHooksChatBroadcast envia uma mensagem do xbot para o grupo Sistema
// (ou o nome pedido). Autentica com XVPN_XBOT_TOKEN — nunca JWT de humano.
// POST /api/hooks/chat/broadcast
func (a *App) handleHooksChatBroadcast(c *gin.Context) {
	want := ""
	if a.Config != nil {
		want = a.Config.XbotToken
	}
	if want == "" || !xbotTokenOK(c.GetHeader("Authorization"), want) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "não autorizado"})
		return
	}
	var req hookBroadcastRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "body é obrigatório"})
		return
	}
	body := strings.TrimSpace(req.Body)
	if body == "" || len(body) > 4000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mensagem inválida"})
		return
	}
	groupName := strings.TrimSpace(req.Group)
	if groupName == "" {
		groupName = "Sistema"
	}

	bot, err := a.ensureXbotUser()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "xbot indisponível"})
		return
	}
	g, err := a.ensureSystemGroup(bot.ID, groupName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "grupo sistema indisponível"})
		return
	}
	if err := a.ensureAllMembersInGroup(g.ID, bot.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao sincronizar membros"})
		return
	}

	msg := store.Message{ThreadKind: "group", ThreadID: g.ID, AuthorID: bot.ID, Kind: "text", Body: body}
	if err := a.Store.DB.Create(&msg).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit("xbot", "social.broadcast", groupName)
	resp := a.messageResponse(msg)
	if a.Hub != nil {
		a.Hub.sendToMany(a.threadMemberIDs("group", g.ID), wsEvent{Type: "message.new", Payload: resp})
	}
	c.JSON(http.StatusCreated, gin.H{"ok": true, "group_id": g.ID, "message_id": msg.ID})
}

func xbotTokenOK(header, want string) bool {
	parts := strings.SplitN(header, " ", 2)
	got := header
	if len(parts) == 2 && strings.EqualFold(parts[0], "Bearer") {
		got = parts[1]
	}
	if got == "" || want == "" {
		return false
	}
	if len(got) != len(want) {
		return subtle.ConstantTimeCompare([]byte(want), []byte(want)) == 1 && false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func (a *App) ensureXbotUser() (store.User, error) {
	var u store.User
	err := a.Store.DB.Where("username = ?", "xbot").First(&u).Error
	if err == nil {
		return u, nil
	}
	u = store.User{
		Username:     "xbot",
		PasswordHash: "!", // senha inválida de propósito — sem login
		Role:         store.RoleBot,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := a.Store.DB.Create(&u).Error; err != nil {
		return store.User{}, err
	}
	return u, nil
}

func (a *App) ensureSystemGroup(ownerID uint, name string) (store.SocialGroup, error) {
	var g store.SocialGroup
	err := a.Store.DB.Where("name = ?", name).First(&g).Error
	if err == nil {
		return g, nil
	}
	g = store.SocialGroup{Name: name, Description: "Notificações do sistema", OwnerUserID: ownerID}
	if err := a.Store.DB.Create(&g).Error; err != nil {
		return store.SocialGroup{}, err
	}
	_ = a.Store.DB.Create(&store.SocialGroupMember{GroupID: g.ID, UserID: ownerID}).Error
	return g, nil
}

func (a *App) ensureAllMembersInGroup(groupID, botID uint) error {
	var users []store.User
	if err := a.Store.DB.Where("role <> ?", store.RoleBot).Find(&users).Error; err != nil {
		return err
	}
	for _, u := range users {
		m := store.SocialGroupMember{GroupID: groupID, UserID: u.ID}
		_ = a.Store.DB.Where("group_id = ? AND user_id = ?", groupID, u.ID).FirstOrCreate(&m)
	}
	m := store.SocialGroupMember{GroupID: groupID, UserID: botID}
	_ = a.Store.DB.Where("group_id = ? AND user_id = ?", groupID, botID).FirstOrCreate(&m)
	return nil
}
