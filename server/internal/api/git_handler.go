package api

import (
	"crypto/subtle"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

const gitCloneHost = "https://xgit.corp.ihuull.com"

func gitHostOK(host string) bool {
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host
	}
	h = strings.ToLower(h)
	return h == "xgit.corp.ihuull.com" || h == "xgit.corp.localhost"
}

func (a *App) RequireGitHost() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !gitHostOK(c.Request.Host) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "não encontrado"})
			return
		}
		c.Next()
	}
}

func (a *App) gitDir() string {
	if a.Config == nil {
		return ""
	}
	return strings.TrimSpace(a.Config.GitDir)
}

func (a *App) gitUnauthorized(c *gin.Context) (store.User, bool) {
	c.Header("WWW-Authenticate", `Basic realm="xgit"`)
	c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "credenciais inválidas"})
	return store.User{}, false
}

func (a *App) authenticateGit(c *gin.Context) (store.User, bool) {
	token := auth.TokenFromRequest(c)
	basicUser, basicPass, hasBasic := c.Request.BasicAuth()
	if token == "" && hasBasic {
		token = basicPass
	}
	if token == "" {
		return a.gitUnauthorized(c)
	}
	if claims, err := a.Tokens.Parse(token); err == nil {
		if basicUser != "" && !strings.EqualFold(basicUser, claims.Username) {
			return a.gitUnauthorized(c)
		}
		var user store.User
		if err := a.Store.DB.First(&user, claims.UserID).Error; err != nil {
			return a.gitUnauthorized(c)
		}
		return user, true
	}
	if hasBasic && strings.HasPrefix(basicUser, "codespace-") {
		if user, ok := a.authenticateCodespaceGit(basicUser, basicPass); ok {
			return user, true
		}
	}
	// Git CLI pede a senha da conta — JWE no prompt é inviável.
	if !hasBasic || basicUser == "" || basicPass == "" {
		return a.gitUnauthorized(c)
	}
	if a.loginLimiter != nil && !a.loginLimiter.allow(c.ClientIP()) {
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "muitas tentativas, tente novamente mais tarde"})
		return store.User{}, false
	}
	var user store.User
	if err := a.Store.DB.Where("username = ?", basicUser).First(&user).Error; err != nil {
		return a.gitUnauthorized(c)
	}
	if user.Role == store.RoleBot || user.Username == "xbot" {
		return a.gitUnauthorized(c)
	}
	ok, err := auth.VerifyPassword(user.PasswordHash, basicPass)
	if err != nil || !ok {
		return a.gitUnauthorized(c)
	}
	return user, true
}

func (a *App) authenticateCodespaceGit(basicUser, basicPass string) (store.User, bool) {
	id := strings.TrimPrefix(basicUser, "codespace-")
	if !codespaceHostRe.MatchString("cs-" + id + ".corp.ihuull.com") {
		return store.User{}, false
	}
	var cs store.CodeSpace
	if err := a.Store.DB.Where("public_id = ?", id).First(&cs).Error; err != nil {
		return store.User{}, false
	}
	if cs.GitTokenHash == "" || subtle.ConstantTimeCompare([]byte(cs.GitTokenHash), []byte(hashCodespaceToken(basicPass))) != 1 {
		return store.User{}, false
	}
	var user store.User
	if err := a.Store.DB.First(&user, cs.UserID).Error; err != nil {
		return store.User{}, false
	}
	return user, true
}

func (a *App) canGitRead(user store.User, proj store.Project) bool {
	return a.canSeeProject(user, proj)
}

func (a *App) canGitPush(user store.User, proj store.Project) bool {
	if store.HasProduct(user.Role, user.Products, store.ProductForge) {
		return true
	}
	role, ok := a.projectMemberRole(user, proj)
	if !ok {
		return false
	}
	return role.Rank() >= store.ProjectRoleDeveloper.Rank()
}

func (a *App) canGitPushProtected(user store.User, proj store.Project, min store.ProjectRole) bool {
	if store.HasProduct(user.Role, user.Products, store.ProductForge) {
		return true
	}
	role, ok := a.projectMemberRole(user, proj)
	if !ok {
		return false
	}
	if !min.Valid() {
		min = store.ProjectRoleMaintainer
	}
	return role.Rank() >= min.Rank()
}

func (a *App) loadGitProject(c *gin.Context, user store.User) (store.Project, string, bool) {
	slug := forge.NormalizeSlug(c.Param("slug"))
	if !store.ValidProjectSlug(slug) {
		c.JSON(http.StatusNotFound, gin.H{"error": "repositório não encontrado"})
		return store.Project{}, "", false
	}
	var proj store.Project
	if err := a.Store.DB.Where("slug = ?", slug).First(&proj).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repositório não encontrado"})
		return store.Project{}, "", false
	}
	if !a.canGitRead(user, proj) {
		c.JSON(http.StatusNotFound, gin.H{"error": "repositório não encontrado"})
		return store.Project{}, "", false
	}
	return proj, slug, true
}

func (a *App) handleGitSmartHTTP(c *gin.Context) {
	if runner, ok := a.authenticateGitRunner(c); ok {
		a.serveGitForRunner(c, runner)
		return
	}
	user, ok := a.authenticateGit(c)
	if !ok {
		return
	}
	proj, slug, ok := a.loadGitProject(c, user)
	if !ok {
		return
	}
	pathInfo := forge.PathInfo(c.Request.URL.Path, slug)
	service := forge.ServiceName(c.Request.URL.RawQuery, pathInfo)
	push := service == "git-receive-pack"
	if push && !a.canGitPush(user, proj) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para push"})
		return
	}
	if push && c.Request.Method == http.MethodPost {
		if !a.enforceProtectedPush(c, user, proj) {
			return
		}
	}
	root := a.gitDir()
	if root == "" || !forge.Exists(root, slug) {
		c.JSON(http.StatusNotFound, gin.H{"error": "repositório não encontrado"})
		return
	}
	if err := forge.Serve(c.Writer, c.Request, root, slug, user.Username, pathInfo); err != nil {
		if !c.Writer.Written() {
			c.JSON(http.StatusBadGateway, gin.H{"error": "git indisponível"})
		}
		return
	}
	if push && c.Request.Method == http.MethodPost && c.Writer.Status() < 400 {
		if raw, ok := c.Get(contextPushUpdates); ok {
			if updates, ok := raw.([]forge.RefUpdate); ok {
				a.enqueuePushJobs(proj, updates, user.Username)
			}
		}
	}
}

func (a *App) serveGitForRunner(c *gin.Context, runner store.MeshServer) {
	slug := forge.NormalizeSlug(c.Param("slug"))
	if !store.ValidProjectSlug(slug) {
		c.JSON(http.StatusNotFound, gin.H{"error": "repositório não encontrado"})
		return
	}
	var proj store.Project
	if err := a.Store.DB.Where("slug = ?", slug).First(&proj).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "repositório não encontrado"})
		return
	}
	if !a.runnerMatchesProject(runner, proj) {
		c.JSON(http.StatusNotFound, gin.H{"error": "repositório não encontrado"})
		return
	}
	pathInfo := forge.PathInfo(c.Request.URL.Path, slug)
	if forge.ServiceName(c.Request.URL.RawQuery, pathInfo) == "git-receive-pack" {
		c.JSON(http.StatusForbidden, gin.H{"error": "runner só clona"})
		return
	}
	root := a.gitDir()
	if root == "" || !forge.Exists(root, slug) {
		c.JSON(http.StatusNotFound, gin.H{"error": "repositório não encontrado"})
		return
	}
	if err := forge.Serve(c.Writer, c.Request, root, slug, "runner", pathInfo); err != nil {
		if !c.Writer.Written() {
			c.JSON(http.StatusBadGateway, gin.H{"error": "git indisponível"})
		}
	}
}

const contextPushUpdates = "git_push_updates"

func (a *App) enforceProtectedPush(c *gin.Context, user store.User, proj store.Project) bool {
	updates, rest, err := forge.ParseReceivePack(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "pack inválido"})
		return false
	}
	c.Request.Body = io.NopCloser(rest)
	c.Set(contextPushUpdates, updates)
	rules := a.protectedBranchRules(proj.ID)
	for _, u := range updates {
		for _, rule := range rules {
			if !forge.MatchProtected([]string{rule.Pattern}, u.Ref) {
				continue
			}
			if !a.canGitPushProtected(user, proj, rule.MinPushRole) {
				c.JSON(http.StatusForbidden, gin.H{"error": "branch protegida: " + rule.Pattern})
				return false
			}
		}
	}
	return true
}

func (a *App) protectedBranchRules(projectID uint) []store.ProtectedBranch {
	var rows []store.ProtectedBranch
	_ = a.Store.DB.Where("project_id = ?", projectID).Order("id").Find(&rows).Error
	return rows
}

type protectedBranchJSON struct {
	Pattern     string            `json:"pattern"`
	MinPushRole store.ProjectRole `json:"min_push_role"`
}

type projectGitResponse struct {
	CloneURL          string                `json:"clone_url"`
	Exists            bool                  `json:"exists"`
	ProtectedBranches []protectedBranchJSON `json:"protected_branches"`
}

type setProtectedBranchesRequest struct {
	Branches []protectedBranchJSON `json:"branches"`
}

func (a *App) projectGitJSON(proj store.Project) projectGitResponse {
	rules := a.protectedBranchRules(proj.ID)
	out := make([]protectedBranchJSON, 0, len(rules))
	for _, r := range rules {
		out = append(out, protectedBranchJSON{Pattern: r.Pattern, MinPushRole: r.MinPushRole})
	}
	return projectGitResponse{
		CloneURL:          gitCloneHost + "/" + proj.Slug,
		Exists:            forge.Exists(a.gitDir(), proj.Slug),
		ProtectedBranches: out,
	}
}

func (a *App) handleGetProjectGit(c *gin.Context) {
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, a.projectGitJSON(proj))
}

func (a *App) handleInitProjectGit(c *gin.Context) {
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	if err := a.ensureGitRepo(proj.Slug); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "não foi possível criar o repositório"})
		return
	}
	a.ensureDefaultProtected(proj.ID)
	_ = a.Store.LogAudit(callerUsername(c), "project.git.init", proj.Slug)
	c.JSON(http.StatusOK, a.projectGitJSON(proj))
}

func (a *App) handleSetProtectedBranches(c *gin.Context) {
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	var req setProtectedBranchesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	seen := map[string]struct{}{}
	rows := make([]store.ProtectedBranch, 0, len(req.Branches))
	for _, b := range req.Branches {
		pattern := strings.TrimSpace(b.Pattern)
		if !forge.ValidBranchPattern(pattern) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "padrão de branch inválido"})
			return
		}
		if _, dup := seen[pattern]; dup {
			c.JSON(http.StatusBadRequest, gin.H{"error": "padrão duplicado"})
			return
		}
		seen[pattern] = struct{}{}
		role := b.MinPushRole
		if role == "" {
			role = store.ProjectRoleMaintainer
		}
		if !role.Valid() || role.Rank() < store.ProjectRoleDeveloper.Rank() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "min_push_role inválido"})
			return
		}
		rows = append(rows, store.ProtectedBranch{ProjectID: proj.ID, Pattern: pattern, MinPushRole: role})
	}
	if err := a.Store.DB.Where("project_id = ?", proj.ID).Delete(&store.ProtectedBranch{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	for i := range rows {
		if err := a.Store.DB.Create(&rows[i]).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
			return
		}
	}
	_ = a.Store.LogAudit(callerUsername(c), "project.git.protect", proj.Slug)
	c.JSON(http.StatusOK, a.projectGitJSON(proj))
}

func (a *App) ensureGitRepo(slug string) error {
	root := a.gitDir()
	if root == "" {
		return nil
	}
	return forge.InitBare(root, slug)
}

func (a *App) ensureDefaultProtected(projectID uint) {
	var n int64
	_ = a.Store.DB.Model(&store.ProtectedBranch{}).Where("project_id = ?", projectID).Count(&n).Error
	if n > 0 {
		return
	}
	for _, def := range store.DefaultProtectedBranches {
		row := store.ProtectedBranch{ProjectID: projectID, Pattern: def.Pattern, MinPushRole: def.MinPushRole}
		_ = a.Store.DB.Create(&row).Error
	}
}
