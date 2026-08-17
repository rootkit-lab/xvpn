package api

import (
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/driver"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

const maxDriverUpload = 2 << 30 // 2 GiB

func driverHostOK(host string) bool {
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host
	}
	h = strings.ToLower(h)
	return h == "xdriver.corp.ihuull.com" || h == "xdriver.corp.localhost"
}

func (a *App) RequireDriverHost() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !driverHostOK(c.Request.Host) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "não encontrado"})
			return
		}
		c.Next()
	}
}

func (a *App) driverRoots() driver.Roots {
	shared := "/srv/xvpn/shared"
	home := "/home"
	projects := ""
	if a.Config != nil {
		if a.Config.DriverSharedDir != "" {
			shared = a.Config.DriverSharedDir
		}
		if a.Config.DriverHomeRoot != "" {
			home = a.Config.DriverHomeRoot
		}
		projects = a.Config.DriverProjectsDir
	}
	return driver.Roots{SharedDir: shared, HomeRoot: home, ProjectsDir: projects}
}

func parseDriverProjectRoot(root, slugHint string) (ok bool, slug string) {
	root = strings.TrimSpace(root)
	if strings.HasPrefix(root, "project:") {
		return true, strings.TrimPrefix(root, "project:")
	}
	if root == "project" {
		return true, strings.TrimSpace(slugHint)
	}
	return false, ""
}

func (a *App) driverResolve(c *gin.Context, user store.User, root, rel string, write bool) (full, base string, ok bool) {
	return a.driverResolveSlug(c, user, root, rel, c.Query("slug"), write)
}

func (a *App) driverResolveSlug(c *gin.Context, user store.User, root, rel, slugHint string, write bool) (full, base string, ok bool) {
	if isProject, slug := parseDriverProjectRoot(root, slugHint); isProject {
		return a.driverResolveProject(c, user, slug, rel, write)
	}
	if root == "home" && !user.SambaEnabled && !user.SFTPEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "Meu Drive desligado nesta conta"})
		return "", "", false
	}
	roots := a.driverRoots()
	full, err := roots.Resolve(user.Username, root, rel)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "caminho inválido"})
		return "", "", false
	}
	base, err = roots.Resolve(user.Username, root, "")
	if err != nil || driver.RejectSymlinks(base, full) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "caminho inválido"})
		return "", "", false
	}
	return full, base, true
}

func (a *App) driverResolveProject(c *gin.Context, user store.User, slug, rel string, write bool) (full, base string, ok bool) {
	if !store.ValidProjectSlug(slug) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "caminho inválido"})
		return "", "", false
	}
	var proj store.Project
	if err := a.Store.DB.Where("slug = ?", slug).First(&proj).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "projeto não encontrado"})
		return "", "", false
	}
	if !a.canAccessProjectFiles(user, proj, false) {
		c.JSON(http.StatusNotFound, gin.H{"error": "projeto não encontrado"})
		return "", "", false
	}
	if !proj.FilesEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "arquivos deste projeto estão desligados"})
		return "", "", false
	}
	if write && !a.canAccessProjectFiles(user, proj, true) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para gravar neste projeto"})
		return "", "", false
	}
	roots := a.driverRoots()
	full, err := roots.ResolveProject(slug, rel)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "caminho inválido"})
		return "", "", false
	}
	base, err = roots.ResolveProject(slug, "")
	if err != nil || driver.RejectSymlinks(base, full) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "caminho inválido"})
		return "", "", false
	}
	return full, base, true
}

func (a *App) handleDriverList(c *gin.Context) {
	var user store.User
	if err := a.Store.DB.First(&user, callerUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	root := c.DefaultQuery("root", "shared")
	rel := c.Query("path")
	full, base, ok := a.driverResolve(c, user, root, rel, false)
	if !ok {
		return
	}
	ents, err := driver.ListNoFollow(base, full)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao listar"})
		return
	}
	for i := range ents {
		ents[i].Path = strings.Trim(strings.Trim(rel, "/")+"/"+ents[i].Name, "/")
	}
	c.JSON(http.StatusOK, gin.H{"root": root, "path": rel, "items": ents})
}

func (a *App) handleDriverMkdir(c *gin.Context) {
	var req struct {
		Root string `json:"root"`
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nome inválido"})
		return
	}
	if strings.ContainsAny(req.Name, `/\`) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nome inválido"})
		return
	}
	var user store.User
	if err := a.Store.DB.First(&user, callerUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	parent, base, ok := a.driverResolve(c, user, req.Root, req.Path, true)
	if !ok {
		return
	}
	if err := driver.MkdirShare(base, parent, req.Name, req.Root, user.Username); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao criar pasta"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ok": true})
}

func (a *App) handleDriverUpload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxDriverUpload)
	root := c.PostForm("root")
	rel := c.PostForm("path")
	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "arquivo ausente"})
		return
	}
	name := filepath.Base(fh.Filename)
	if name == "." || name == "" || strings.Contains(name, "..") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nome inválido"})
		return
	}
	var user store.User
	if err := a.Store.DB.First(&user, callerUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	dir, base, ok := a.driverResolve(c, user, root, rel, true)
	if !ok {
		return
	}
	src, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao gravar"})
		return
	}
	defer src.Close()
	dst, err := driver.CreateFileShare(base, dir, name, root, user.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao gravar"})
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao gravar"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"ok": true, "name": name})
}

func (a *App) handleDriverDownload(c *gin.Context) {
	var user store.User
	if err := a.Store.DB.First(&user, callerUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	full, base, ok := a.driverResolve(c, user, c.Query("root"), c.Query("path"), false)
	if !ok {
		return
	}
	f, err := driver.OpenFileNoFollow(base, full)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "arquivo não encontrado"})
		return
	}
	defer f.Close()
	name := filepath.Base(full)
	c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
	http.ServeContent(c.Writer, c.Request, name, time.Time{}, f)
}

func (a *App) handleDriverDelete(c *gin.Context) {
	var req struct {
		Root string `json:"root"`
		Path string `json:"path"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Path) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "caminho inválido"})
		return
	}
	var user store.User
	if err := a.Store.DB.First(&user, callerUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	full, base, ok := a.driverResolve(c, user, req.Root, req.Path, true)
	if !ok {
		return
	}
	if err := driver.RemoveNoFollow(base, full); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao apagar"})
		return
	}
	c.Status(http.StatusNoContent)
}
