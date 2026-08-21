package api

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

const pagesHost = "pages.corp.ihuull.com"

var pagesExtOK = map[string]bool{
	".html": true, ".htm": true, ".css": true, ".js": true, ".json": true,
	".svg": true, ".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".webp": true, ".ico": true, ".txt": true, ".md": true, ".woff": true,
	".woff2": true, ".ttf": true, ".map": true, ".xml": true,
}

func pagesHostOK(host string) bool {
	h, _, err := net.SplitHostPort(host)
	if err != nil {
		h = host
	}
	h = strings.ToLower(h)
	return h == pagesHost || h == "pages.corp.localhost"
}

func (a *App) RequirePagesHost() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !pagesHostOK(c.Request.Host) || !packagesForwardedFromMesh(c) {
			c.AbortWithStatusJSON(http.StatusNotFound, gin.H{"error": "não encontrado"})
			return
		}
		c.Next()
	}
}

func (a *App) pagesRoot() string {
	if a.Config != nil && strings.TrimSpace(a.Config.PagesDir) != "" {
		return a.Config.PagesDir
	}
	return "/opt/xvpn/data/pages"
}

func pagesSiteDir(root, org, slug string) (string, error) {
	org, slug = forge.NormalizeSlug(org), forge.NormalizeSlug(slug)
	if !store.ValidOrgSlug(org) || !store.ValidProjectSlug(slug) {
		return "", forge.ErrInvalidSlug
	}
	dir := filepath.Join(filepath.Clean(root), org, slug)
	rel, err := filepath.Rel(filepath.Clean(root), dir)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", forge.ErrInvalidSlug
	}
	return dir, nil
}

func (a *App) handleGetPagesStatus(c *gin.Context) {
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	org := a.projectOrgSlug(proj)
	dir, err := pagesSiteDir(a.pagesRoot(), org, proj.Slug)
	published := false
	if err == nil {
		if _, statErr := os.Stat(filepath.Join(dir, "index.html")); statErr == nil {
			published = true
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"url":       "https://" + pagesHost + "/" + org + "/" + proj.Slug + "/",
		"published": published,
	})
}

func (a *App) handlePublishPages(c *gin.Context) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	if !a.canAccessProjectFiles(user, proj, true) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para publicar Pages"})
		return
	}
	org := a.projectOrgSlug(proj)
	dest, err := pagesSiteDir(a.pagesRoot(), org, proj.Slug)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "repo inválido"})
		return
	}
	source := strings.TrimSpace(c.PostForm("source"))
	if source == "" {
		source = strings.TrimSpace(c.Query("source"))
	}
	if fh, ferr := c.FormFile("file"); ferr == nil && fh != nil {
		src, err := fh.Open()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "arquivo inválido"})
			return
		}
		defer src.Close()
		body, err := io.ReadAll(io.LimitReader(src, 32<<20+1))
		if err != nil || len(body) > 32<<20 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "artifact grande demais"})
			return
		}
		if err := extractPagesArchive(dest, fh.Filename, body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "url": "https://" + pagesHost + "/" + org + "/" + proj.Slug + "/"})
		return
	}
	if source == "" {
		source = "docs"
	}
	if err := a.publishPagesFromTree(proj, dest, source); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "url": "https://" + pagesHost + "/" + org + "/" + proj.Slug + "/"})
}

func (a *App) publishPagesFromTree(proj store.Project, dest, source string) error {
	source = strings.Trim(source, "/")
	if source != "docs" && source != "public" {
		return errors.New("source deve ser docs ou public")
	}
	repo := a.projectRepo(proj)
	entries, err := forge.ListTreeFiles(a.gitDir(), repo, "HEAD", source)
	if err != nil || len(entries) == 0 {
		return errors.New("pasta " + source + " vazia ou ausente")
	}
	stage, err := pagesStageDir(dest)
	if err != nil {
		return err
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(stage)
		}
	}()
	wrote := 0
	for _, rel := range entries {
		body, binary, err := forge.ReadBlob(a.gitDir(), repo, "HEAD", source+"/"+rel)
		if err != nil {
			continue
		}
		if !pagesPathOK(rel) {
			continue
		}
		if binary && !pagesExtOK[strings.ToLower(filepath.Ext(rel))] {
			continue
		}
		if err := writePagesFile(stage, rel, []byte(body)); err != nil {
			return err
		}
		wrote++
	}
	if wrote == 0 {
		return errors.New("nenhum arquivo estático em " + source)
	}
	if err := replacePagesDir(dest, stage); err != nil {
		return err
	}
	keep = true
	return nil
}

func pagesStageDir(dest string) (string, error) {
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return "", err
	}
	return os.MkdirTemp(filepath.Dir(dest), "pages-stage-")
}

func replacePagesDir(dest, stage string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return err
	}
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	if err := os.Rename(stage, dest); err != nil {
		return err
	}
	return nil
}

func (a *App) publishPagesBytes(org, slug, filename string, body []byte) error {
	dest, err := pagesSiteDir(a.pagesRoot(), org, slug)
	if err != nil {
		return err
	}
	return extractPagesArchive(dest, filename, body)
}

func extractPagesArchive(dest, name string, body []byte) error {
	name = strings.ToLower(filepath.Base(name))
	switch {
	case strings.HasSuffix(name, ".zip"):
		return extractPagesZip(dest, body)
	case strings.HasSuffix(name, ".tgz"), strings.HasSuffix(name, ".tar.gz"), strings.HasSuffix(name, ".tar"):
		return extractPagesTar(dest, body, strings.HasSuffix(name, ".gz") || strings.HasSuffix(name, ".tgz"))
	default:
		return errors.New("use tar.gz ou zip")
	}
}

func extractPagesTar(dest string, body []byte, gz bool) error {
	r := io.Reader(bytes.NewReader(body))
	if gz {
		gr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return err
		}
		defer gr.Close()
		r = gr
	}
	tr := tar.NewReader(r)
	stage, err := pagesStageDir(dest)
	if err != nil {
		return err
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(stage)
		}
	}()
	wrote := 0
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		rel := strings.TrimPrefix(filepath.ToSlash(hdr.Name), "./")
		if !pagesPathOK(rel) {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(tr, 2<<20+1))
		if err != nil || len(data) > 2<<20 {
			return errors.New("ficheiro grande demais")
		}
		if err := writePagesFile(stage, rel, data); err != nil {
			return err
		}
		wrote++
	}
	if wrote == 0 {
		return errors.New("arquivo sem páginas estáticas")
	}
	if err := replacePagesDir(dest, stage); err != nil {
		return err
	}
	keep = true
	return nil
}

func extractPagesZip(dest string, body []byte) error {
	zr, err := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if err != nil {
		return err
	}
	stage, err := pagesStageDir(dest)
	if err != nil {
		return err
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(stage)
		}
	}()
	wrote := 0
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		rel := strings.TrimPrefix(filepath.ToSlash(f.Name), "./")
		if !pagesPathOK(rel) {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(io.LimitReader(rc, 2<<20+1))
		_ = rc.Close()
		if err != nil || len(data) > 2<<20 {
			return errors.New("ficheiro grande demais")
		}
		if err := writePagesFile(stage, rel, data); err != nil {
			return err
		}
		wrote++
	}
	if wrote == 0 {
		return errors.New("arquivo sem páginas estáticas")
	}
	if err := replacePagesDir(dest, stage); err != nil {
		return err
	}
	keep = true
	return nil
}

func pagesPathOK(rel string) bool {
	rel = strings.Trim(strings.ReplaceAll(rel, "\\", "/"), "/")
	if rel == "" || strings.Contains(rel, "..") || strings.HasPrefix(rel, "/") {
		return false
	}
	ext := strings.ToLower(filepath.Ext(rel))
	return pagesExtOK[ext]
}

func writePagesFile(dest, rel string, body []byte) error {
	abs := filepath.Join(dest, filepath.FromSlash(rel))
	relOK, err := filepath.Rel(dest, abs)
	if err != nil || strings.HasPrefix(relOK, "..") {
		return forge.ErrInvalidSlug
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
		return err
	}
	return os.WriteFile(abs, body, 0o644)
}

func (a *App) maybeServePages() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !pagesHostOK(c.Request.Host) || !packagesForwardedFromMesh(c) {
			c.Next()
			return
		}
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.Next()
			return
		}
		a.handlePagesStatic(c)
		c.Abort()
	}
}

func (a *App) handlePagesStatic(c *gin.Context) {
	parts := strings.Split(strings.Trim(c.Request.URL.Path, "/"), "/")
	if len(parts) < 2 || parts[0] == "" {
		c.Status(http.StatusNotFound)
		return
	}
	org, slug := parts[0], parts[1]
	rest := strings.Join(parts[2:], "/")
	dir, err := pagesSiteDir(a.pagesRoot(), org, slug)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	if rest == "" || strings.HasSuffix(c.Request.URL.Path, "/") {
		rest = filepath.Join(rest, "index.html")
	}
	if !pagesPathOK(rest) && filepath.Base(rest) != "index.html" {
		c.Status(http.StatusNotFound)
		return
	}
	abs := filepath.Join(dir, filepath.FromSlash(rest))
	rel, err := filepath.Rel(dir, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		c.Status(http.StatusNotFound)
		return
	}
	body, err := os.ReadFile(abs)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	ctype := mime.TypeByExtension(filepath.Ext(abs))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	c.Data(http.StatusOK, ctype, body)
}
