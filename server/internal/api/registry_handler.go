package api

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

const registryHost = "registry.corp.ihuull.com"

func registryHostOK(host string) bool {
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host
	}
	h = strings.ToLower(h)
	return h == registryHost || h == "registry.corp.localhost"
}

func (a *App) RequireRegistryHost() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !registryHostOK(c.Request.Host) || !packagesForwardedFromMesh(c) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "não encontrado"})
			return
		}
		c.Next()
	}
}

func parseRegistryScope(raw string) (repo, action string) {
	raw = strings.TrimSpace(raw)
	// repository:<org>/<slug>:pull,push
	if !strings.HasPrefix(raw, "repository:") {
		return "", ""
	}
	rest := strings.TrimPrefix(raw, "repository:")
	name, acts, ok := strings.Cut(rest, ":")
	if !ok || strings.Count(name, "/") != 1 {
		return "", ""
	}
	return name, acts
}

func parseRegistryV2Repo(uri string) string {
	path := uri
	if i := strings.Index(uri, "?"); i >= 0 {
		path = uri[:i]
	}
	path = strings.Trim(path, "/")
	parts := strings.Split(path, "/")
	if len(parts) < 3 || parts[0] != "v2" {
		return ""
	}
	if parts[1] == "" || parts[2] == "" {
		return ""
	}
	return parts[1] + "/" + parts[2]
}

func (a *App) handleRegistryToken(c *gin.Context) {
	var user store.User
	if err := a.Store.DB.First(&user, callerUserID(c)).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "credenciais ausentes"})
		return
	}
	scope := firstNonEmpty(c.Query("scope"), c.Query("scopes"))
	repo, _ := parseRegistryScope(scope)
	if repo == "" {
		c.JSON(http.StatusOK, gin.H{"token": auth.TokenFromRequest(c), "expires_in": 7200})
		return
	}
	org, slug, ok := strings.Cut(repo, "/")
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "scope inválido"})
		return
	}
	proj, found := a.findProject(org, slug)
	if !found || !a.canSeeProject(user, proj) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem acesso a este repositório"})
		return
	}
	tok, err := a.Tokens.IssuePackages(user.ID, user.Username, user.Role, repo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"token": tok, "access_token": tok, "expires_in": 7200})
}

func (a *App) handleRegistryAuth(c *gin.Context) {
	var user store.User
	if err := a.Store.DB.First(&user, callerUserID(c)).Error; err != nil {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	uri := firstNonEmpty(c.GetHeader("X-Original-URI"), c.Query("uri"))
	method := strings.ToUpper(firstNonEmpty(c.GetHeader("X-Original-Method"), c.Request.Method, http.MethodGet))
	if uri == "/v2/" || uri == "/v2" || uri == "" {
		c.Status(http.StatusOK)
		return
	}
	repo := parseRegistryV2Repo(uri)
	if repo == "" {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	if audRepo, _ := c.Get(auth.ContextRepoKey); audRepo != nil {
		if s, ok := audRepo.(string); ok && s != "" && s != repo {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
	}
	org, slug, ok := strings.Cut(repo, "/")
	if !ok {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	proj, found := a.findProject(org, slug)
	if !found || !a.canSeeProject(user, proj) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	write := method != http.MethodGet && method != http.MethodHead
	if write && !a.canAccessProjectFiles(user, proj, true) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	c.Status(http.StatusOK)
}
