package api

import (
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/forge"
)

type wikiPageJSON struct {
	Page    string `json:"page"`
	Content string `json:"content,omitempty"`
}

type putWikiRequest struct {
	Content string `json:"content"`
	Message string `json:"message"`
}

func (a *App) handleListWiki(c *gin.Context) {
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	pages, err := forge.ListWiki(a.gitDir(), a.projectRepo(proj))
	if err != nil && !errors.Is(err, forge.ErrEmptyRepo) {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	items := make([]wikiPageJSON, 0, len(pages))
	for _, p := range pages {
		items = append(items, wikiPageJSON{Page: p})
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) handleGetWikiPage(c *gin.Context) {
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	page := c.Param("page")
	body, err := forge.ReadWiki(a.gitDir(), a.projectRepo(proj), page)
	if err != nil {
		if errors.Is(err, forge.ErrInvalidSlug) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "página inválida"})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "página não encontrada"})
		return
	}
	file, _ := forge.WikiPageFile(page)
	c.JSON(http.StatusOK, wikiPageJSON{Page: forge.WikiPageTitle(file), Content: body})
}

func (a *App) handlePutWikiPage(c *gin.Context) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	if !a.canGitPush(user, proj) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para editar a wiki"})
		return
	}
	var req putWikiRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	msg := strings.TrimSpace(req.Message)
	if msg == "" {
		msg = "wiki: " + c.Param("page")
	}
	if utf8.RuneCountInString(msg) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mensagem de commit inválida"})
		return
	}
	res, err := forge.WriteWiki(a.gitDir(), a.projectRepo(proj), c.Param("page"), req.Content, msg, user.Username, user.Username+"@corp.ihuull.com")
	if err != nil {
		switch {
		case errors.Is(err, forge.ErrUnchanged):
			c.JSON(http.StatusConflict, gin.H{"error": "sem alterações"})
		case errors.Is(err, forge.ErrEmptyMessage), errors.Is(err, forge.ErrContentHuge),
			errors.Is(err, forge.ErrBinaryEdit), errors.Is(err, forge.ErrInvalidSlug):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, forge.ErrEmptyRepo):
			c.JSON(http.StatusNotFound, gin.H{"error": "repositório sem git"})
		default:
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		}
		return
	}
	file, _ := forge.WikiPageFile(c.Param("page"))
	c.JSON(http.StatusOK, gin.H{"page": forge.WikiPageTitle(file), "sha": res.SHA})
}
