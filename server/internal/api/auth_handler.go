package api

import (
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Audience string `json:"aud"`
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

	if user.Role == store.RoleBot || user.Username == "xbot" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "credenciais inválidas"})
		return
	}

	ok, err := auth.VerifyPassword(user.PasswordHash, req.Password)
	if err != nil || !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "credenciais inválidas"})
		return
	}

	token, err := a.Tokens.IssueFor(user.ID, user.Username, user.Role, auth.NormalizeAudience(req.Audience))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	_ = a.Store.LogAudit(user.Username, "login", "")
	if a.Tokens != nil {
		auth.SetSessionCookie(c, token, a.Tokens.TTL())
	}
	c.JSON(http.StatusOK, loginResponse{Token: token, User: toUserResponse(user)})
}

// handleEstablishSession planta o cookie no host de destino (xvpn,
// marketplace, *.corp). O login no xauth sozinho não basta: o browser
// chega em /admin sem o JWE e o /auth/me 401 devolve ao xauth.
// POST /api/auth/session — form (handoff) ou JSON.
func (a *App) handleEstablishSession(c *gin.Context) {
	token := strings.TrimSpace(c.PostForm("token"))
	ret := c.PostForm("return")
	if token == "" && strings.Contains(c.ContentType(), "json") {
		var body struct {
			Token  string `json:"token"`
			Return string `json:"return"`
		}
		if err := c.ShouldBindJSON(&body); err == nil {
			token = strings.TrimSpace(body.Token)
			if ret == "" {
				ret = body.Return
			}
		}
	}
	if token == "" {
		token = auth.TokenFromRequest(c)
	}
	if !auth.HandoffAllowed(c.GetHeader("Origin"), c.GetHeader("Referer"), c.GetHeader("Sec-Fetch-Site"), c.Request.Host) {
		c.JSON(http.StatusForbidden, gin.H{"error": "origem não permitida"})
		return
	}
	if a.Tokens == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if _, err := a.Tokens.Parse(token); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "sessão inválida ou expirada"})
		return
	}
	auth.SetSessionCookieOnHost(c, token, a.Tokens.TTL())
	dest := auth.SafeReturnURL(ret)
	if dest == "" {
		dest = auth.PanelOrigin + "/"
	}
	if strings.Contains(c.ContentType(), "form") {
		c.Redirect(http.StatusSeeOther, dest)
		return
	}
	c.Status(http.StatusNoContent)
}

// handleHandoffContinue só no xauth, só navegação top-level: emite um
// ticket opaco e redireciona ao host de destino. O JWE não vai no corpo.
// GET /api/auth/handoff-continue
func (a *App) handleHandoffContinue(c *gin.Context) {
	if !auth.IsXAuthHost(c.Request.Host) {
		c.JSON(http.StatusForbidden, gin.H{"error": "só no xauth"})
		return
	}
	if !auth.IsDocumentNavigation(c.GetHeader("Sec-Fetch-Dest"), c.GetHeader("Sec-Fetch-Mode")) {
		c.JSON(http.StatusForbidden, gin.H{"error": "só navegação"})
		return
	}
	if a.Tokens == nil || a.Handoff == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	token := auth.TokenFromRequest(c)
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "não autenticado"})
		return
	}
	if _, err := a.Tokens.Parse(token); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "sessão inválida ou expirada"})
		return
	}
	dest := auth.SafeReturnURL(c.Query("return"))
	if dest == "" {
		dest = auth.PanelOrigin + "/"
	}
	u, err := url.Parse(dest)
	if err != nil || u.Host == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "return inválido"})
		return
	}
	id, err := a.Handoff.Issue(token, auth.HandoffTicketTTL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	redeem, err := url.Parse(u.Scheme + "://" + u.Host + "/api/auth/redeem")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	q := redeem.Query()
	q.Set("ticket", id)
	q.Set("return", dest)
	redeem.RawQuery = q.Encode()
	c.Header("Cache-Control", "no-store")
	c.Redirect(http.StatusSeeOther, redeem.String())
}

// handleRedeemHandoff troca o ticket por cookie no host de destino.
// GET /api/auth/redeem
func (a *App) handleRedeemHandoff(c *gin.Context) {
	if !auth.IsDocumentNavigation(c.GetHeader("Sec-Fetch-Dest"), c.GetHeader("Sec-Fetch-Mode")) {
		c.JSON(http.StatusForbidden, gin.H{"error": "só navegação"})
		return
	}
	if a.Tokens == nil || a.Handoff == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	token, ok := a.Handoff.Redeem(c.Query("ticket"))
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "ticket inválido ou expirado"})
		return
	}
	if _, err := a.Tokens.Parse(token); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "sessão inválida ou expirada"})
		return
	}
	auth.SetSessionCookieOnHost(c, token, a.Tokens.TTL())
	dest := auth.SafeReturnURL(c.Query("return"))
	if dest == "" {
		dest = auth.PanelOrigin + "/"
	}
	c.Redirect(http.StatusSeeOther, dest)
}

// handleLogout apaga o cookie de SSO. Público de propósito: quem tem o
// cookie pode descartá-lo; sem cookie é no-op. Desktop não usa cookie.
// POST /api/auth/logout
func (a *App) handleLogout(c *gin.Context) {
	auth.ClearSessionCookie(c)
	c.Status(http.StatusNoContent)
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
