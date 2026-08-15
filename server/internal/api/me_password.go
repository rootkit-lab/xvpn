package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type changeMyPasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

// handleChangeMyPassword é o autosserviço de senha (Fase 18): o próprio
// usuário troca a senha do painel sem passar por um admin. Exige a senha
// atual (prova de posse da sessão) e rejeita a nova se for igual à atual
// ou tiver menos de 8 caracteres — o mesmo mínimo de handleResetPassword.
// 400 (não 401) quando a senha atual está errada: o JWT já autenticou o
// chamador; 401 aqui faria o painel limpar a sessão como se o token
// tivesse expirado.
//
// PATCH /api/me/password  (authed)
func (a *App) handleChangeMyPassword(c *gin.Context) {
	var req changeMyPasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	if req.CurrentPassword == "" || req.NewPassword == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "senha atual e nova senha são obrigatórias"})
		return
	}
	if len(req.NewPassword) < 8 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nova senha deve ter ao menos 8 caracteres"})
		return
	}
	if req.NewPassword == req.CurrentPassword {
		c.JSON(http.StatusBadRequest, gin.H{"error": "a nova senha deve ser diferente da atual"})
		return
	}

	uid := callerUserID(c)
	var user store.User
	if err := a.Store.DB.First(&user, uid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}

	ok, err := auth.VerifyPassword(user.PasswordHash, req.CurrentPassword)
	if err != nil || !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "senha atual incorreta"})
		return
	}

	hash, err := auth.HashPassword(req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if err := a.Store.DB.Model(&store.User{}).Where("id = ?", user.ID).Update("password_hash", hash).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	_ = a.Store.LogAudit(user.Username, "me.password_change", "")
	c.Status(http.StatusNoContent)
}
