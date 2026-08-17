package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/forge"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type putContentsRequest struct {
	Path        string `json:"path"`
	Ref         string `json:"ref"`
	Content     string `json:"content"`
	Message     string `json:"message"`
	Description string `json:"description"`
	NewBranch   string `json:"new_branch"`
	OpenPR      *bool  `json:"open_pr"`
}

type putContentsJSON struct {
	SHA                string `json:"sha"`
	Branch             string `json:"branch"`
	MergeRequestNumber *uint  `json:"merge_request_number,omitempty"`
}

func (a *App) handlePutContents(c *gin.Context) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	if !a.canGitPush(user, proj) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para commitar"})
		return
	}
	var req putContentsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	path := strings.Trim(strings.ReplaceAll(req.Path, "\\", "/"), "/")
	ref := strings.TrimSpace(req.Ref)
	if ref == "" {
		ref = "HEAD"
	}
	msg := strings.TrimSpace(req.Message)
	if msg == "" || utf8.RuneCountInString(msg) > 200 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "mensagem de commit inválida"})
		return
	}
	if utf8.RuneCountInString(req.Description) > 2000 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "descrição longa demais"})
		return
	}

	heads, _ := forge.ListBranches(a.gitDir(), proj.Slug)
	targetBranch := ref
	if ref == "HEAD" || !forge.ValidBranchName(ref) {
		targetBranch = defaultHead(heads)
		ref = targetBranch
	}

	newBranch := strings.TrimSpace(req.NewBranch)
	mustPR := !a.canPushBranch(user, proj, targetBranch)
	if mustPR && newBranch == "" {
		newBranch = fmt.Sprintf("%s-patch-%d", sanitizeBranchActor(user.Username), time.Now().Unix()%100000)
	}
	if mustPR && (newBranch == targetBranch || newBranch == "") {
		c.JSON(http.StatusForbidden, gin.H{"error": "branch protegida: crie outra branch e abra um PR"})
		return
	}

	res, err := forge.CommitFile(a.gitDir(), proj.Slug, forge.CommitFileOpts{
		Path:        path,
		Ref:         ref,
		Content:     req.Content,
		Message:     msg,
		Description: req.Description,
		NewBranch:   newBranch,
		AuthorName:  user.Username,
		AuthorEmail: user.Username + "@corp.ihuull.com",
	})
	if err != nil {
		switch {
		case errors.Is(err, forge.ErrUnchanged):
			c.JSON(http.StatusConflict, gin.H{"error": "sem alterações"})
		case errors.Is(err, forge.ErrEmptyMessage), errors.Is(err, forge.ErrContentHuge),
			errors.Is(err, forge.ErrBinaryEdit), errors.Is(err, forge.ErrInvalidSlug),
			errors.Is(err, forge.ErrInvalidBranch), errors.Is(err, forge.ErrEmptyRepo),
			errors.Is(err, forge.ErrBranchMissing):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case errors.Is(err, forge.ErrGitMissing):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "git indisponível"})
		default:
			c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		}
		return
	}

	out := putContentsJSON{SHA: res.SHA, Branch: res.Branch}
	openPR := mustPR
	if req.OpenPR != nil {
		openPR = *req.OpenPR || mustPR
	}
	if openPR && res.Branch != targetBranch {
		mr, err := a.insertMergeRequest(proj, user, msg, req.Description, res.Branch, targetBranch)
		if err == nil {
			out.MergeRequestNumber = &mr.Number
			if sha, err := forge.RevParse(a.gitDir(), proj.Slug, res.Branch); err == nil {
				n := mr.Number
				st := store.CiAwaitingApproval
				if a.canApproveCi(user, proj) {
					st = store.CiPending
				}
				a.enqueueCiJobAs(proj, ciTriggerMR, "refs/heads/"+res.Branch, sha, &n, user.Username, st)
			}
		}
	}
	_ = a.Store.LogAudit(callerUsername(c), "project.contents.commit", fmt.Sprintf("%s %s %s", proj.Slug, res.Branch, path))
	c.JSON(http.StatusOK, out)
}

func (a *App) handleGetArchive(c *gin.Context) {
	proj, _, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	ref := strings.TrimSpace(c.DefaultQuery("ref", "HEAD"))
	body, err := forge.ArchiveZIP(a.gitDir(), proj.Slug, ref)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "archive indisponível"})
		return
	}
	name := proj.Slug + ".zip"
	c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
	c.Data(http.StatusOK, "application/zip", body)
}

func defaultHead(heads []string) string {
	if containsString(heads, "main") {
		return "main"
	}
	if containsString(heads, "master") {
		return "master"
	}
	if len(heads) > 0 {
		return heads[0]
	}
	return "HEAD"
}

func sanitizeBranchActor(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte('-')
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = "user"
	}
	if len(out) > 20 {
		out = out[:20]
	}
	return out
}

func containsString(items []string, want string) bool {
	for _, s := range items {
		if s == want {
			return true
		}
	}
	return false
}
