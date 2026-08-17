package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/driver"
	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

const (
	maxCodespacesPerUser = 5
	maxCodespaceBlob     = 2 << 20
)

type codespaceJSON struct {
	ID        string    `json:"id"`
	Slug      string    `json:"slug"`
	Branch    string    `json:"branch"`
	Author    string    `json:"author"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	CanWrite  bool      `json:"can_write,omitempty"`
	OpenURL   string    `json:"open_url"`
}

type createCodespaceRequest struct {
	Slug   string `json:"slug"`
	Branch string `json:"branch"`
}

type writeCodespaceRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type commitCodespaceRequest struct {
	Message     string `json:"message"`
	Description string `json:"description"`
}

func (a *App) codespacesDir() string {
	if a.Config != nil && a.Config.CodespacesDir != "" {
		return a.Config.CodespacesDir
	}
	return "/opt/xvpn/data/codespaces"
}

func codespaceOpenURL(id string) string {
	return "https://xcodespaces.corp.ihuull.com/" + id
}

func (a *App) codespaceJSON(user store.User, proj store.Project, cs store.CodeSpace) codespaceJSON {
	out := codespaceJSON{
		ID:        cs.PublicID,
		Slug:      proj.Slug,
		Branch:    cs.Branch,
		CreatedAt: cs.CreatedAt,
		UpdatedAt: cs.UpdatedAt,
		OpenURL:   codespaceOpenURL(cs.PublicID),
	}
	var author store.User
	if a.Store.DB.First(&author, cs.UserID).Error == nil {
		out.Author = author.Username
	}
	out.CanWrite = a.canGitPush(user, proj)
	return out
}

func (a *App) loadCodespace(c *gin.Context) (store.User, store.Project, store.CodeSpace, bool) {
	var user store.User
	if err := a.Store.DB.First(&user, callerUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return store.User{}, store.Project{}, store.CodeSpace{}, false
	}
	id := strings.TrimSpace(c.Param("id"))
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "codespace inválido"})
		return user, store.Project{}, store.CodeSpace{}, false
	}
	var cs store.CodeSpace
	if err := a.Store.DB.Where("public_id = ?", id).First(&cs).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "codespace não encontrado"})
		return user, store.Project{}, store.CodeSpace{}, false
	}
	if cs.UserID != user.ID && !store.HasProduct(user.Role, user.Products, store.ProductForge) {
		c.JSON(http.StatusNotFound, gin.H{"error": "codespace não encontrado"})
		return user, store.Project{}, store.CodeSpace{}, false
	}
	var proj store.Project
	if err := a.Store.DB.First(&proj, cs.ProjectID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "projeto não encontrado"})
		return user, store.Project{}, store.CodeSpace{}, false
	}
	if !a.canSeeProject(user, proj) {
		c.JSON(http.StatusNotFound, gin.H{"error": "codespace não encontrado"})
		return user, store.Project{}, store.CodeSpace{}, false
	}
	return user, proj, cs, true
}

func (a *App) handleListCodespaces(c *gin.Context) {
	var user store.User
	if err := a.Store.DB.First(&user, callerUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	q := a.Store.DB.Where("user_id = ?", user.ID)
	if slug := strings.TrimSpace(c.Query("slug")); slug != "" {
		var proj store.Project
		if err := a.Store.DB.Where("slug = ?", slug).First(&proj).Error; err != nil {
			c.JSON(http.StatusOK, gin.H{"items": []codespaceJSON{}})
			return
		}
		q = q.Where("project_id = ?", proj.ID)
	}
	var rows []store.CodeSpace
	if err := q.Order("updated_at desc").Limit(40).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]codespaceJSON, 0, len(rows))
	for _, cs := range rows {
		var proj store.Project
		if a.Store.DB.First(&proj, cs.ProjectID).Error != nil {
			continue
		}
		items = append(items, a.codespaceJSON(user, proj, cs))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) handleCreateCodespace(c *gin.Context) {
	var user store.User
	if err := a.Store.DB.First(&user, callerUserID(c)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	var req createCodespaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	var proj store.Project
	if err := a.Store.DB.Where("slug = ?", strings.TrimSpace(req.Slug)).First(&proj).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "projeto não encontrado"})
		return
	}
	if !a.canSeeProject(user, proj) {
		c.JSON(http.StatusNotFound, gin.H{"error": "projeto não encontrado"})
		return
	}
	branch := strings.TrimSpace(req.Branch)
	if branch == "" {
		branch = "main"
	}
	if !forge.ValidBranchName(branch) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "branch inválida"})
		return
	}
	var n int64
	_ = a.Store.DB.Model(&store.CodeSpace{}).Where("user_id = ?", user.ID).Count(&n).Error
	if n >= maxCodespacesPerUser {
		c.JSON(http.StatusConflict, gin.H{"error": "limite de codespaces atingido"})
		return
	}
	raw := make([]byte, 6)
	if _, err := rand.Read(raw); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	publicID := hex.EncodeToString(raw)
	rel := filepath.Join(sanitizePathPart(user.Username), proj.Slug, publicID)
	dest := filepath.Join(a.codespacesDir(), rel)
	if err := forge.AddWorktree(a.gitDir(), proj.Slug, dest, branch); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	cs := store.CodeSpace{
		PublicID:  publicID,
		UserID:    user.ID,
		ProjectID: proj.ID,
		Branch:    branch,
		RelPath:   filepath.ToSlash(rel),
	}
	if err := a.Store.DB.Create(&cs).Error; err != nil {
		_ = forge.RemoveWorktree(a.gitDir(), proj.Slug, dest)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "codespace.create", fmt.Sprintf("%s %s %s", proj.Slug, branch, publicID))
	c.JSON(http.StatusCreated, a.codespaceJSON(user, proj, cs))
}

func (a *App) handleGetCodespace(c *gin.Context) {
	user, proj, cs, ok := a.loadCodespace(c)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, a.codespaceJSON(user, proj, cs))
}

func (a *App) handleDeleteCodespace(c *gin.Context) {
	user, proj, cs, ok := a.loadCodespace(c)
	if !ok {
		return
	}
	if user.ID != cs.UserID && !store.HasProduct(user.Role, user.Products, store.ProductForge) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão"})
		return
	}
	dest := filepath.Join(a.codespacesDir(), filepath.FromSlash(cs.RelPath))
	_ = forge.RemoveWorktree(a.gitDir(), proj.Slug, dest)
	if err := a.Store.DB.Delete(&cs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "codespace.delete", cs.PublicID)
	c.Status(http.StatusNoContent)
}

func (a *App) handleCodespaceTree(c *gin.Context) {
	_, _, cs, ok := a.loadCodespace(c)
	if !ok {
		return
	}
	root := filepath.Join(a.codespacesDir(), filepath.FromSlash(cs.RelPath))
	rel := strings.Trim(strings.ReplaceAll(c.Query("path"), "\\", "/"), "/")
	dir := root
	if rel != "" {
		resolved, err := safeCodespaceFile(a.codespacesDir(), cs.RelPath, rel)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "path inválido"})
			return
		}
		dir = resolved
	} else if err := driver.RejectSymlinks(root, root); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path inválido"})
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "pasta não encontrada"})
		return
	}
	items := make([]gin.H, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if name == ".git" || e.Type()&os.ModeSymlink != 0 {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		kind := "blob"
		if e.IsDir() {
			kind = "tree"
		}
		p := name
		if rel != "" {
			p = rel + "/" + name
		}
		items = append(items, gin.H{"name": name, "path": p, "type": kind, "size": info.Size()})
	}
	c.JSON(http.StatusOK, gin.H{"path": rel, "items": items})
}

func (a *App) handleCodespaceBlob(c *gin.Context) {
	_, _, cs, ok := a.loadCodespace(c)
	if !ok {
		return
	}
	rel := strings.Trim(strings.ReplaceAll(c.Query("path"), "\\", "/"), "/")
	if rel == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path inválido"})
		return
	}
	root := filepath.Join(a.codespacesDir(), filepath.FromSlash(cs.RelPath))
	full, err := safeCodespaceFile(a.codespacesDir(), cs.RelPath, rel)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path inválido"})
		return
	}
	raw, info, err := readFileNoFollow(root, full)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "arquivo não encontrado"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "path inválido"})
		return
	}
	if info.Size() > maxCodespaceBlob || int64(len(raw)) > maxCodespaceBlob {
		c.JSON(http.StatusBadRequest, gin.H{"error": "arquivo grande demais para o editor"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": rel, "content": string(raw), "size": info.Size()})
}

func (a *App) handleCodespaceWrite(c *gin.Context) {
	user, proj, cs, ok := a.loadCodespace(c)
	if !ok {
		return
	}
	if !a.canGitPush(user, proj) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para editar"})
		return
	}
	var req writeCodespaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	rel := strings.Trim(strings.ReplaceAll(req.Path, "\\", "/"), "/")
	if len(req.Content) > maxCodespaceBlob {
		c.JSON(http.StatusBadRequest, gin.H{"error": "conteúdo grande demais"})
		return
	}
	root := filepath.Join(a.codespacesDir(), filepath.FromSlash(cs.RelPath))
	full, err := safeCodespaceFile(a.codespacesDir(), cs.RelPath, rel)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path inválido"})
		return
	}
	if err := writeFileNoFollow(root, full, []byte(req.Content)); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "path inválido"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"path": rel})
}

func (a *App) handleCodespaceCommit(c *gin.Context) {
	user, proj, cs, ok := a.loadCodespace(c)
	if !ok {
		return
	}
	if !a.canGitPush(user, proj) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para commitar"})
		return
	}
	var req commitCodespaceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	msg := strings.TrimSpace(req.Message)
	if msg == "" || utf8.RuneCountInString(msg) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mensagem de commit inválida"})
		return
	}
	dest := filepath.Join(a.codespacesDir(), filepath.FromSlash(cs.RelPath))
	base := cs.Branch
	mustPR := !a.canPushBranch(user, proj, base)
	if mustPR {
		branch := fmt.Sprintf("%s-cs-%d", sanitizeBranchActor(user.Username), time.Now().Unix()%100000)
		if err := forge.WorktreeCheckoutBranch(dest, branch); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		cs.Branch = branch
		_ = a.Store.DB.Save(&cs).Error
	}
	sha, err := forge.WorktreeCommit(dest, msg, user.Username, user.Username+"@corp.ihuull.com")
	if err != nil {
		switch {
		case errors.Is(err, forge.ErrUnchanged):
			c.JSON(http.StatusConflict, gin.H{"error": "sem alterações"})
		case errors.Is(err, forge.ErrEmptyMessage):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		}
		return
	}
	var mrNumber *uint
	if mustPR && cs.Branch != base {
		mr, err := a.insertMergeRequest(proj, user, msg, req.Description, cs.Branch, base)
		if err == nil {
			mrNumber = &mr.Number
		}
	}
	_ = a.Store.LogAudit(callerUsername(c), "codespace.commit", fmt.Sprintf("%s %s %s", proj.Slug, cs.Branch, sha))
	c.JSON(http.StatusOK, gin.H{"sha": sha, "branch": cs.Branch, "merge_request_number": mrNumber})
}

func sanitizePathPart(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		return "user"
	}
	return out
}

func rejectCodespaceRel(rel string) error {
	rel = strings.Trim(strings.ReplaceAll(rel, "\\", "/"), "/")
	if rel == "" || strings.Contains(rel, "\x00") {
		return fs.ErrInvalid
	}
	for _, part := range strings.Split(rel, "/") {
		if part == "" || part == "." || part == ".." {
			return fs.ErrInvalid
		}
		if strings.EqualFold(part, ".git") {
			return fs.ErrInvalid
		}
	}
	return nil
}

func safeCodespaceFile(root, rel, file string) (string, error) {
	if err := rejectCodespaceRel(file); err != nil {
		return "", err
	}
	base := filepath.Join(root, filepath.FromSlash(rel))
	full := filepath.Join(base, filepath.FromSlash(file))
	cleanBase, err := filepath.Abs(base)
	if err != nil {
		return "", err
	}
	cleanFull, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	if cleanFull == cleanBase || !strings.HasPrefix(cleanFull, cleanBase+string(os.PathSeparator)) {
		return "", fs.ErrInvalid
	}
	if err := driver.RejectSymlinks(cleanBase, cleanFull); err != nil {
		return "", err
	}
	return cleanFull, nil
}

func mkdirAllNoFollow(base, dir string) error {
	base = filepath.Clean(base)
	dir = filepath.Clean(dir)
	if dir == base {
		return driver.RejectSymlinks(base, base)
	}
	rel, err := filepath.Rel(base, dir)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return fs.ErrInvalid
	}
	cur := base
	for _, part := range strings.Split(rel, string(os.PathSeparator)) {
		if part == "" || part == "." {
			continue
		}
		next := filepath.Join(cur, part)
		fi, err := os.Lstat(next)
		if os.IsNotExist(err) {
			dirfd, err := driver.OpenDirNoFollow(base, cur)
			if err != nil {
				return err
			}
			mkErr := syscall.Mkdirat(dirfd, part, 0o750)
			_ = syscall.Close(dirfd)
			if mkErr != nil && !os.IsExist(mkErr) {
				return mkErr
			}
			cur = next
			continue
		}
		if err != nil {
			return err
		}
		if fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
			return fs.ErrInvalid
		}
		cur = next
	}
	return nil
}

func writeFileNoFollow(base, full string, data []byte) error {
	base = filepath.Clean(base)
	full = filepath.Clean(full)
	dir := filepath.Dir(full)
	name := filepath.Base(full)
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return fs.ErrInvalid
	}
	if err := mkdirAllNoFollow(base, dir); err != nil {
		return err
	}
	if err := driver.RejectSymlinks(base, dir); err != nil {
		return err
	}
	dirfd, err := driver.OpenDirNoFollow(base, dir)
	if err != nil {
		return err
	}
	defer syscall.Close(dirfd)
	fd, err := syscall.Openat(dirfd, name, syscall.O_CREAT|syscall.O_WRONLY|syscall.O_TRUNC|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0o640)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), name)
	defer f.Close()
	_, err = f.Write(data)
	return err
}

func readFileNoFollow(base, full string) ([]byte, os.FileInfo, error) {
	base = filepath.Clean(base)
	full = filepath.Clean(full)
	if err := driver.RejectSymlinks(base, full); err != nil {
		return nil, nil, err
	}
	info, err := os.Lstat(full)
	if err != nil {
		return nil, nil, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return nil, nil, fs.ErrInvalid
	}
	dirfd, err := driver.OpenDirNoFollow(base, filepath.Dir(full))
	if err != nil {
		return nil, nil, err
	}
	defer syscall.Close(dirfd)
	fd, err := syscall.Openat(dirfd, filepath.Base(full), syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	f := os.NewFile(uintptr(fd), full)
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, maxCodespaceBlob+1))
	if err != nil {
		return nil, nil, err
	}
	return raw, info, nil
}
