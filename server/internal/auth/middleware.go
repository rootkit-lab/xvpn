package auth

import (
	"net/http"
	"net/url"
	"strings"

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
	ContextAudienceKey = "xvpn_audience"
	ContextRepoKey     = "xvpn_repo"
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
			// Bearer velho no localStorage do painel não pode tapar o
			// cookie SSO válido — senão /admin fica no spinner e o
			// login nunca fecha.
			if ck := cookieToken(c); ck != "" && ck != token {
				claims, err = tm.Parse(ck)
			}
		}
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "sessão inválida ou expirada"})
			return
		}

		c.Set(ContextUserIDKey, claims.UserID)
		c.Set(ContextUsernameKey, claims.Username)
		c.Set(ContextRoleKey, claims.Role)
		if len(claims.Audience) > 0 {
			c.Set(ContextAudienceKey, string(claims.Audience[0]))
		}
		if claims.Repo != "" {
			c.Set(ContextRepoKey, claims.Repo)
		}
		c.Next()
	}
}

// AudienceFromContext é o aud do JWE (vazio = sessão antiga sem claim).
func AudienceFromContext(c *gin.Context) string {
	v, _ := c.Get(ContextAudienceKey)
	s, _ := v.(string)
	return s
}

// RejectPackagesScopedToken impede que o JWE curto do runner (aud=packages)
// chame APIs fora do registry.
func RejectPackagesScopedToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		if AudienceFromContext(c) != AudPackages {
			c.Next()
			return
		}
		if registryAPIPath(c.Request.URL.Path) {
			want, _ := c.Get(ContextRepoKey)
			repo, _ := want.(string)
			got := repoFromRegistryRequest(c)
			from := mountFromRegistryRequest(c)
			if repo == "" || (got != "" && got != repo) || (from != "" && from != repo) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "token só vale neste repositório"})
				return
			}
			c.Next()
			return
		}
		if !packagesAPIPath(c.Request.URL.Path) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "token só vale no registry de packages"})
			return
		}
		want, _ := c.Get(ContextRepoKey)
		repo, _ := want.(string)
		got := repoFromPackagesPath(c.Request.URL.Path)
		if repo == "" || got == "" || repo != got {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "token só vale neste repositório"})
			return
		}
		c.Next()
	}
}

func registryAPIPath(p string) bool {
	p = strings.TrimPrefix(p, "/api")
	return p == "/registry/token" || p == "/registry/auth" || strings.HasPrefix(p, "/registry/")
}

func repoFromRegistryRequest(c *gin.Context) string {
	raw := firstQuery(c, "scope", "scopes")
	if strings.HasPrefix(raw, "repository:") {
		rest := strings.TrimPrefix(raw, "repository:")
		name, _, ok := strings.Cut(rest, ":")
		if ok && strings.Count(name, "/") == 1 {
			return name
		}
	}
	uri := c.GetHeader("X-Original-URI")
	if uri == "" {
		uri = c.Query("uri")
	}
	path := uri
	if i := strings.Index(uri, "?"); i >= 0 {
		path = uri[:i]
	}
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) >= 3 && parts[0] == "v2" && parts[1] != "" && parts[2] != "" {
		return parts[1] + "/" + parts[2]
	}
	return ""
}

func mountFromRegistryRequest(c *gin.Context) string {
	uri := c.GetHeader("X-Original-URI")
	if uri == "" {
		uri = c.Query("uri")
	}
	i := strings.Index(uri, "?")
	if i < 0 {
		return ""
	}
	q, err := url.ParseQuery(uri[i+1:])
	if err != nil {
		return ""
	}
	from := strings.TrimSpace(q.Get("from"))
	if strings.Count(from, "/") != 1 {
		return ""
	}
	return from
}

func firstQuery(c *gin.Context, keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(c.Query(k)); v != "" {
			return v
		}
	}
	return ""
}

func packagesAPIPath(p string) bool {
	p = strings.TrimPrefix(p, "/api")
	return p == "/xgit/packages" || strings.HasSuffix(p, "/packages") || strings.Contains(p, "/packages/")
}

func repoFromPackagesPath(p string) string {
	p = strings.Trim(strings.TrimPrefix(p, "/api"), "/")
	parts := strings.Split(p, "/")
	if len(parts) >= 3 && parts[0] == "packages" {
		return parts[1] + "/" + parts[2]
	}
	if len(parts) >= 4 && parts[0] == "projects" && parts[3] == "packages" {
		return parts[1] + "/" + parts[2]
	}
	return ""
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
