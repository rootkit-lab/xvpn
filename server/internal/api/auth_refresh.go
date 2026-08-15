package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

// refreshCallerFromDB recarrega username/role do banco depois do JWT.
// Sem isso, uma promoção (ex.: admin → super_admin) deixa a UI mostrando
// o papel novo via GET /auth/me (lê o DB) enquanto CanManage ainda usa o
// claim velho do token — o sintoma concreto era 403 "seu papel não pode
// gerenciar acesso a arquivos deste usuário" para o próprio super_admin
// recém-promovido. Também cobre o inverso (rebaixamento): o JWT antigo
// não pode continuar concedendo privilégio acima do DB.
//
// Deve rodar SEMPRE depois de auth.RequireAuth no mesmo grupo.
func (a *App) refreshCallerFromDB() gin.HandlerFunc {
	return func(c *gin.Context) {
		uid := callerUserID(c)
		if uid == 0 {
			c.Next()
			return
		}
		var u store.User
		if err := a.Store.DB.Select("id", "username", "role").First(&u, uid).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "sessão inválida ou expirada"})
			return
		}
		c.Set(auth.ContextUsernameKey, u.Username)
		c.Set(auth.ContextRoleKey, u.Role)
		c.Next()
	}
}
