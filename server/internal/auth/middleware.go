package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// ContextUserIDKey e ContextUsernameKey são as chaves usadas para expor a
// identidade do usuário autenticado aos handlers via gin.Context.
const (
	ContextUserIDKey   = "xvpn_user_id"
	ContextUsernameKey = "xvpn_username"
)

// RequireAuth é o middleware Gin que valida o header "Authorization: Bearer
// <jwt>" antes de deixar a requisição prosseguir. Todo endpoint autenticado
// deve usar este middleware (ver go-backend.mdc).
func RequireAuth(tm *TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "credenciais ausentes"})
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "formato de autorização inválido"})
			return
		}

		claims, err := tm.Parse(parts[1])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "sessão inválida ou expirada"})
			return
		}

		c.Set(ContextUserIDKey, claims.UserID)
		c.Set(ContextUsernameKey, claims.Username)
		c.Next()
	}
}
