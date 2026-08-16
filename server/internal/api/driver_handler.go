package api

import (
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
	if a.Config != nil {
		if a.Config.DriverSharedDir != "" {
			shared = a.Config.DriverSharedDir
		}
		if a.Config.DriverHomeRoot != "" {
			home = a.Config.DriverHomeRoot
		}
	}
	return driver.Roots{SharedDir: shared, HomeRoot: home}
}

func (a *App) driverResolve(c *gin.Context, user store.User, root, rel string) (string, bool) {
	if root == "home" && !user.SambaEnabled && !user.SFTPEnabled {
		c.JSON(http.StatusForbidden, gin.H{"error": "Meu Drive desligado nesta conta"})
		return "", false
	}
	full, err := a.driverRoots().Resolve(user.Username, root, rel)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "caminho inválido"})
		return "", false
	}
	return full, true
}

func (a *App) handleDriverList(c *gin.Context) {
	var user store.User
	if err := a.Store.DB.First(&user, callerUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	root := c.DefaultQuery("root", "shared")
	rel := c.Query("path")
	full, ok := a.driverResolve(c, user, root, rel)
	if !ok {
		return
	}
	ents, err := driver.List(full)
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
	parent, ok := a.driverResolve(c, user, req.Root, req.Path)
	if !ok {
		return
	}
	dest := filepath.Join(parent, req.Name)
	if err := os.MkdirAll(dest, 0o775); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao criar pasta"})
		return
	}
	_ = driver.ChownShare(dest, req.Root, user.Username)
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
	dir, ok := a.driverResolve(c, user, root, rel)
	if !ok {
		return
	}
	if err := os.MkdirAll(dir, 0o775); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao gravar"})
		return
	}
	src, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao gravar"})
		return
	}
	defer src.Close()
	dest := filepath.Join(dir, name)
	dst, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o664)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao gravar"})
		return
	}
	defer dst.Close()
	if _, err := io.Copy(dst, src); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao gravar"})
		return
	}
	_ = driver.ChownShare(dest, root, user.Username)
	c.JSON(http.StatusCreated, gin.H{"ok": true, "name": name})
}

func (a *App) handleDriverDownload(c *gin.Context) {
	var user store.User
	if err := a.Store.DB.First(&user, callerUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	full, ok := a.driverResolve(c, user, c.Query("root"), c.Query("path"))
	if !ok {
		return
	}
	st, err := os.Stat(full)
	if err != nil || st.IsDir() {
		c.JSON(http.StatusNotFound, gin.H{"error": "arquivo não encontrado"})
		return
	}
	c.FileAttachment(full, filepath.Base(full))
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
	full, ok := a.driverResolve(c, user, req.Root, req.Path)
	if !ok {
		return
	}
	if err := os.RemoveAll(full); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao apagar"})
		return
	}
	c.Status(http.StatusNoContent)
}
