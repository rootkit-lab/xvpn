package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

// ContextUserIDKey, ContextUsernameKey e ContextRoleKey são as chaves usadas
// para expor a identidade do usuário autenticado aos handlers via
// gin.Context.
const (
	ContextUserIDKey   = "xvpn_user_id"
	ContextUsernameKey = "xvpn_username"
	ContextRoleKey     = "xvpn_role"
	ContextProductsKey = "xvpn_products"
)

// RequireAuth é o middleware Gin que valida Authorization: Bearer ou o
// cookie de SSO (PLAN.md §6.13) antes de deixar a requisição prosseguir.
// Todo endpoint autenticado deve usar este middleware (ver go-backend.mdc).
// Só confirma identidade — não decide o que o papel pode fazer (ver
// RequireRole).
func RequireAuth(tm *TokenManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := TokenFromRequest(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "credenciais ausentes"})
			return
		}

		claims, err := tm.Parse(token)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "sessão inválida ou expirada"})
			return
		}

		c.Set(ContextUserIDKey, claims.UserID)
		c.Set(ContextUsernameKey, claims.Username)
		c.Set(ContextRoleKey, claims.Role)
		c.Next()
	}
}

// RequireRole é o middleware Gin de autorização (Fase 10 — ver PLAN.md
// §6.7): deve ser usado sempre depois de RequireAuth num mesmo grupo de
// rotas, e responde 403 se o papel do usuário autenticado não estiver entre
// os informados. Nunca usar sozinho — depende de ContextRoleKey já estar no
// gin.Context.
func RequireRole(allowed ...store.Role) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Value(ContextRoleKey).(store.Role)
		for _, r := range allowed {
			if role == r {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "seu papel não tem permissão para esta ação"})
	}
}

// RoleFromContext extrai o papel do usuário autenticado atual, definido por
// RequireAuth. Retorna ("", false) se chamado fora de uma rota autenticada.
func RoleFromContext(c *gin.Context) (store.Role, bool) {
	role, ok := c.Value(ContextRoleKey).(store.Role)
	return role, ok
}

// ProductsFromContext devolve o escopo de produto lido do banco por
// refreshCallerFromDB (não do JWE — o claim não carrega products).
func ProductsFromContext(c *gin.Context) []store.Product {
	v, _ := c.Get(ContextProductsKey)
	products, _ := v.([]store.Product)
	return products
}

// RequireProduct bloqueia escrita de um admin sem o produto no escopo
// (Fase 33). super_admin e admin sem lista passam. Deve rodar depois de
// RequireRole(AdminRoles) — viewer nunca chega aqui.
func RequireProduct(want store.Product) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := RoleFromContext(c)
		if store.HasProduct(role, ProductsFromContext(c), want) {
			c.Next()
			return
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "seu escopo não inclui este produto"})
	}
}
