package api

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type userResponse struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	CreatedAt time.Time `json:"created_at"`
}

func toUserResponse(u store.User) userResponse {
	return userResponse{ID: u.ID, Username: u.Username, CreatedAt: u.CreatedAt}
}

// handleListUsers lista os usuários cadastrados (sem hash de senha).
// GET /api/users
func (a *App) handleListUsers(c *gin.Context) {
	var users []store.User
	if err := a.Store.DB.Order("id").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	resp := make([]userResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, toUserResponse(u))
	}
	c.JSON(http.StatusOK, resp)
}

type createUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required,min=8"`
}

// handleCreateUser cria um novo usuário admin do painel.
// POST /api/users
func (a *App) handleCreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username e password (mín. 8 caracteres) são obrigatórios"})
		return
	}

	hash, err := auth.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	user := store.User{Username: req.Username, PasswordHash: hash}
	if err := a.Store.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": "usuário já existe ou dado inválido"})
		return
	}

	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "user.create", "username="+user.Username)

	c.JSON(http.StatusCreated, toUserResponse(user))
}

// handleDeleteUser remove um usuário e revoga todos os dispositivos dele
// (removendo os peers correspondentes da interface WireGuard).
// DELETE /api/users/:id
func (a *App) handleDeleteUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}

	var devices []store.Device
	if err := a.Store.DB.Where("user_id = ?", id).Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	for _, d := range devices {
		if err := a.WG.RemovePeer(d.PublicKey); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao revogar dispositivo do usuário na interface WireGuard"})
			return
		}
	}

	err = a.Store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("user_id = ?", id).Delete(&store.Device{}).Error; err != nil {
			return err
		}
		if err := tx.Where("user_id = ?", id).Delete(&store.InviteToken{}).Error; err != nil {
			return err
		}
		res := tx.Delete(&store.User{}, id)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "user.delete", "user_id="+c.Param("id"))

	c.Status(http.StatusNoContent)
}

type inviteResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
}

// handleCreateInvite gera um código de convite de curta duração para o
// usuário indicado, usado pelo cliente desktop no fluxo de enrollment.
// POST /api/users/:id/invite
func (a *App) handleCreateInvite(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}

	var user store.User
	if err := a.Store.DB.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}

	token, err := generateInviteToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	invite := store.InviteToken{
		UserID:    user.ID,
		Token:     token,
		ExpiresAt: time.Now().Add(time.Duration(a.Config.InviteTokenTTLMinutes) * time.Minute),
	}
	if err := a.Store.DB.Create(&invite).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	actor, _ := c.Get(auth.ContextUsernameKey)
	// Nunca logar o token completo (ver go-backend.mdc) — só o fato de que
	// um convite foi gerado e para quem.
	_ = a.Store.LogAudit(actorString(actor), "invite.create", "user_id="+c.Param("id"))

	c.JSON(http.StatusCreated, inviteResponse{Token: invite.Token, ExpiresAt: invite.ExpiresAt})
}

// generateInviteToken gera um código legível no formato XVPN-XXXX-XXXX,
// usando Base32 (sem caracteres ambíguos como 0/O, 1/I) para ser fácil de
// digitar manualmente se necessário.
func generateInviteToken() (string, error) {
	buf := make([]byte, 5)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)
	if len(encoded) < 8 {
		return "", errors.New("falha ao gerar token")
	}
	return "XVPN-" + encoded[:4] + "-" + encoded[4:8], nil
}

func actorString(actor any) string {
	if s, ok := actor.(string); ok && s != "" {
		return s
	}
	return "desconhecido"
}
