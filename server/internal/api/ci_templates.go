package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/forge"
)

// WorkflowTemplate é um cartão da galeria "New workflow" (Fase 42.2).
// Aplicar grava `.xvpn-ci.sh` — um job `ci`, sem YAML multi-workflow.
type WorkflowTemplate struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Languages   []string `json:"languages"`
	Icon        string   `json:"icon"`
	Script      string   `json:"-"`
}

type workflowTemplateJSON struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Languages   []string `json:"languages"`
	Icon        string   `json:"icon"`
}

type workflowCategoryJSON struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

var workflowCategories = []workflowCategoryJSON{
	{ID: "continuous-integration", Label: "Continuous integration"},
	{ID: "deployment", Label: "Deployment"},
	{ID: "security", Label: "Security"},
	{ID: "automation", Label: "Automation"},
	{ID: "pages", Label: "Pages"},
	{ID: "publish", Label: "Publish a package"},
}

var workflowTemplates = []WorkflowTemplate{
	{
		ID: "go", Name: "Go", Category: "continuous-integration",
		Description: "Build and test a Go module with go test.",
		Languages:   []string{"Go"}, Icon: "go",
		Script: "#!/bin/sh\nset -eu\ngo test ./...\n",
	},
	{
		ID: "node", Name: "Node.js", Category: "continuous-integration",
		Description: "Install dependencies and run npm test.",
		Languages:   []string{"JavaScript", "TypeScript"}, Icon: "node",
		Script: "#!/bin/sh\nset -eu\nif [ -f package-lock.json ]; then npm ci; elif [ -f package.json ]; then npm install; fi\nnpm test\n",
	},
	{
		ID: "python", Name: "Python", Category: "continuous-integration",
		Description: "Run pytest or a compileall check.",
		Languages:   []string{"Python"}, Icon: "python",
		Script: "#!/bin/sh\nset -eu\nif [ -d src ]; then PYTHONPATH=src; export PYTHONPATH; fi\nif command -v pytest >/dev/null 2>&1; then pytest; else python3 -m compileall .\nfi\n",
	},
	{
		ID: "rust", Name: "Rust", Category: "continuous-integration",
		Description: "cargo test no módulo.",
		Languages:   []string{"Rust"}, Icon: "rust",
		Script: "#!/bin/sh\nset -eu\ncargo test\n",
	},
	{
		ID: "generic", Name: "Simple workflow", Category: "continuous-integration",
		Description: "Shell CI genérico — lista o clone e termina ok.",
		Languages:   []string{"Shell"}, Icon: "shell",
		Script: "#!/bin/sh\nset -eu\nls -la\necho ok\n",
	},
	{
		ID: "deploy-rsync", Name: "Deploy via rsync", Category: "deployment",
		Description: "Copia ci-artifacts para um destino da malha (placeholder).",
		Languages:   []string{"Shell"}, Icon: "server",
		Script: "#!/bin/sh\nset -eu\nmkdir -p ci-artifacts\necho \"$(date -Iseconds) deploy\" > ci-artifacts/deploy.txt\necho \"rsync placeholder — sem host público\"\n",
	},
	{
		ID: "deploy-docker", Name: "Docker image", Category: "deployment",
		Description: "Valida um Dockerfile no clone (sem docker.sock no runner).",
		Languages:   []string{"Dockerfile"}, Icon: "box",
		Script: "#!/bin/sh\nset -eu\nif [ -f Dockerfile ]; then echo \"Dockerfile ok\"; else echo \"sem Dockerfile — skip\"; fi\n",
	},
	{
		ID: "deploy-pages", Name: "Deploy Pages", Category: "deployment",
		Description: "Prepara o blob estático. Pages (Nginx) entra no backlog 45+.",
		Languages:   []string{"HTML"}, Icon: "globe",
		Script: "#!/bin/sh\nset -eu\nmkdir -p ci-artifacts/pages\nif [ -d public ]; then cp -a public/. ci-artifacts/pages/; elif [ -d docs ]; then cp -a docs/. ci-artifacts/pages/; else echo '<!doctype html><title>xgit</title><p>ok</p>' > ci-artifacts/pages/index.html; fi\n",
	},
	{
		ID: "govulncheck", Name: "Go vulnerability check", Category: "security",
		Description: "go test + govulncheck se o binário existir no runner.",
		Languages:   []string{"Go"}, Icon: "shield",
		Script: "#!/bin/sh\nset -eu\ngo test ./...\nif command -v govulncheck >/dev/null 2>&1; then govulncheck ./...; else echo \"govulncheck ausente — skip\"; fi\n",
	},
	{
		ID: "npm-audit", Name: "npm audit", Category: "security",
		Description: "npm audit --omit=dev (não falha o job se não houver lock).",
		Languages:   []string{"JavaScript"}, Icon: "shield",
		Script: "#!/bin/sh\nset -eu\nif [ -f package.json ]; then npm audit --omit=dev || true; else echo \"sem package.json\"; fi\n",
	},
	{
		ID: "pip-audit", Name: "pip-audit", Category: "security",
		Description: "pip-audit se instalado; senão compileall.",
		Languages:   []string{"Python"}, Icon: "shield",
		Script: "#!/bin/sh\nset -eu\nif command -v pip-audit >/dev/null 2>&1; then pip-audit || true; else python3 -m compileall .; fi\n",
	},
	{
		ID: "format-check", Name: "Format check", Category: "automation",
		Description: "gofmt / prettier --check conforme o que existir.",
		Languages:   []string{"Go", "JavaScript"}, Icon: "wand",
		Script: "#!/bin/sh\nset -eu\nif command -v gofmt >/dev/null 2>&1 && ls *.go >/dev/null 2>&1; then test -z \"$(gofmt -l .)\"; fi\nif [ -f package.json ] && command -v npx >/dev/null 2>&1; then npx --yes prettier --check . || true; fi\necho ok\n",
	},
	{
		ID: "echo-ok", Name: "Scheduled stub", Category: "automation",
		Description: "Job curto para validar o runner da malha.",
		Languages:   []string{"Shell"}, Icon: "clock",
		Script: "#!/bin/sh\nset -eu\ndate -u\necho ok\n",
	},
	{
		ID: "static-pages", Name: "Static HTML", Category: "pages",
		Description: "Empacota HTML/CSS em ci-artifacts/pages.",
		Languages:   []string{"HTML", "CSS"}, Icon: "globe",
		Script: "#!/bin/sh\nset -eu\nmkdir -p ci-artifacts/pages\nfind . -maxdepth 2 \\( -name '*.html' -o -name '*.css' \\) -exec cp {} ci-artifacts/pages/ \\; 2>/dev/null || true\nls ci-artifacts/pages || true\n",
	},
	{
		ID: "npm-xgit", Name: "Publish npm to XGIT", Category: "publish",
		Description: "Lembra o registry npm em xgit.corp (não interpola o JWE).",
		Languages:   []string{"JavaScript"}, Icon: "package",
		Script: "#!/bin/sh\nset -eu\necho \"npm publish --registry https://xgit.corp.ihuull.com/api/packages/{{REPO}}/npm/\"\necho \"Auth: Bearer JWE — não grave o token no script.\"\n",
	},
	{
		ID: "pypi-xgit", Name: "Publish PyPI to XGIT", Category: "publish",
		Description: "Lembra twine contra a Simple API do XGIT.",
		Languages:   []string{"Python"}, Icon: "package",
		Script: "#!/bin/sh\nset -eu\necho \"twine upload --repository-url https://xgit.corp.ihuull.com/api/packages/{{REPO}}/pypi\"\necho \"Auth: Basic user + JWE — não grave o token no script.\"\n",
	},
	{
		ID: "generic-xgit", Name: "Publish generic tarball", Category: "publish",
		Description: "Empacota o clone em ci-artifacts para upload generic.",
		Languages:   []string{"Shell"}, Icon: "package",
		Script: "#!/bin/sh\nset -eu\nmkdir -p ci-artifacts\ntar -czf ci-artifacts/src.tar.gz --exclude=.git --exclude=ci-artifacts .\nls -la ci-artifacts\necho \"POST multipart em /api/projects/{{REPO}}/packages — Auth Bearer JWE.\"\n",
	},
	{
		ID: "maven-xgit", Name: "Publish Maven to XGIT", Category: "publish",
		Description: "URL do registry Maven em xgit.corp (não interpola o JWE).",
		Languages:   []string{"Java"}, Icon: "package",
		Script: "#!/bin/sh\nset -eu\necho \"mvn deploy -DaltDeploymentRepository=xgit::default::https://xgit.corp.ihuull.com/api/packages/{{REPO}}/maven\"\necho \"Auth: Bearer JWE — não grave o token no script.\"\n",
	},
	{
		ID: "nuget-xgit", Name: "Publish NuGet to XGIT", Category: "publish",
		Description: "URL do feed NuGet em xgit.corp (não interpola o JWE).",
		Languages:   []string{"C#"}, Icon: "package",
		Script: "#!/bin/sh\nset -eu\necho \"dotnet nuget push *.nupkg --source https://xgit.corp.ihuull.com/api/packages/{{REPO}}/nuget/index.json\"\necho \"Auth: Bearer JWE — não grave o token no script.\"\n",
	},
	{
		ID: "gem-xgit", Name: "Publish RubyGems to XGIT", Category: "publish",
		Description: "URL do gem push em xgit.corp (não interpola o JWE).",
		Languages:   []string{"Ruby"}, Icon: "package",
		Script: "#!/bin/sh\nset -eu\necho \"gem push --host https://xgit.corp.ihuull.com/api/packages/{{REPO}}/rubygems\"\necho \"Auth: Bearer JWE — não grave o token no script.\"\n",
	},
}

func workflowTemplateByID(id string) (WorkflowTemplate, bool) {
	id = strings.TrimSpace(id)
	for _, t := range workflowTemplates {
		if t.ID == id {
			return t, true
		}
	}
	return WorkflowTemplate{}, false
}

func (a *App) handleListWorkflowTemplates(c *gin.Context) {
	cat := strings.TrimSpace(c.Query("category"))
	q := strings.ToLower(strings.TrimSpace(c.Query("q")))
	items := make([]workflowTemplateJSON, 0, len(workflowTemplates))
	for _, t := range workflowTemplates {
		if cat != "" && t.Category != cat {
			continue
		}
		if q != "" {
			blob := strings.ToLower(t.Name + " " + t.Description + " " + strings.Join(t.Languages, " "))
			if !strings.Contains(blob, q) {
				continue
			}
		}
		items = append(items, workflowTemplateJSON{
			ID: t.ID, Name: t.Name, Description: t.Description,
			Category: t.Category, Languages: t.Languages, Icon: t.Icon,
		})
	}
	c.JSON(http.StatusOK, gin.H{"categories": workflowCategories, "items": items})
}

type applyWorkflowRequest struct {
	TemplateID string `json:"template_id"`
}

func (a *App) handleApplyWorkflowTemplate(c *gin.Context) {
	proj, user, ok := a.loadProjectBySlug(c)
	if !ok {
		return
	}
	if !a.canGitPush(user, proj) {
		c.JSON(http.StatusForbidden, gin.H{"error": "sem permissão para commitar"})
		return
	}
	var req applyWorkflowRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.TemplateID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "template_id obrigatório"})
		return
	}
	tpl, found := workflowTemplateByID(req.TemplateID)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "template desconhecido"})
		return
	}
	if err := a.ensureGitRepo(proj); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "git indisponível"})
		return
	}
	repo := a.projectRepo(proj)
	if repo == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "org em falta"})
		return
	}
	script := strings.ReplaceAll(tpl.Script, "{{REPO}}", repo)

	ref := "main"
	heads, _ := forge.ListBranches(a.gitDir(), a.projectRepo(proj))
	if h := defaultHead(heads); h != "" {
		ref = h
	}
	newBranch := ""
	if forge.HasCommits(a.gitDir(), a.projectRepo(proj)) && !a.canPushBranch(user, proj, ref) {
		newBranch = sanitizeBranchActor(user.Username) + "-ci"
	}

	res, err := forge.CommitFiles(a.gitDir(), a.projectRepo(proj), forge.CommitFilesOpts{
		Files: []forge.FileContent{{
			Path:    ciWorkflowPath,
			Content: script,
		}},
		Ref:         ref,
		Message:     "ci: add " + tpl.Name + " workflow",
		NewBranch:   newBranch,
		AuthorName:  user.Username,
		AuthorEmail: user.Username + "@corp.ihuull.com",
	})
	if err != nil {
		switch {
		case errors.Is(err, forge.ErrUnchanged):
			c.JSON(http.StatusOK, gin.H{
				"path": ciWorkflowPath, "unchanged": true, "template_id": tpl.ID,
			})
		case errors.Is(err, forge.ErrGitMissing):
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "git indisponível"})
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		}
		return
	}
	_ = a.Store.LogAudit(user.Username, "ci.workflow_apply",
		"slug="+proj.Slug+" template="+tpl.ID)
	c.JSON(http.StatusCreated, gin.H{
		"path":        ciWorkflowPath,
		"sha":         res.SHA,
		"branch":      res.Branch,
		"template_id": tpl.ID,
	})
}
