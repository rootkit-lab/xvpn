package api

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type loginResponse struct {
	Token string       `json:"token"`
	User  userResponse `json:"user"`
}

// handleLogin autentica um usuário do painel e emite um JWT de sessão com o
// papel (role) dele embutido — ver auth.Claims.
// POST /api/auth/login
func (a *App) handleLogin(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "usuário e senha são obrigatórios"})
		return
	}

	var user store.User
	err := a.Store.DB.Where("username = ?", req.Username).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Mesma mensagem genérica de "credenciais inválidas" seja o
			// usuário inexistente ou a senha errada — evita confirmar para
			// um atacante se um dado nome de usuário existe.
			c.JSON(http.StatusUnauthorized, gin.H{"error": "credenciais inválidas"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	ok, err := auth.VerifyPassword(user.PasswordHash, req.Password)
	if err != nil || !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "credenciais inválidas"})
		return
	}

	token, err := a.Tokens.Issue(user.ID, user.Username, user.Role)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	_ = a.Store.LogAudit(user.Username, "login", "")
	c.JSON(http.StatusOK, loginResponse{Token: token, User: toUserResponse(user)})
}

// handleMe devolve os dados do usuário autenticado atual — usado pelo
// painel para restaurar o papel/username após um refresh de página (o
// token em localStorage sozinho não é decodificado no cliente).
// GET /api/auth/me
func (a *App) handleMe(c *gin.Context) {
	userIDVal, _ := c.Get(auth.ContextUserIDKey)
	userID, _ := userIDVal.(uint)

	var user store.User
	if err := a.Store.DB.First(&user, userID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	c.JSON(http.StatusOK, toUserResponse(user))
}
