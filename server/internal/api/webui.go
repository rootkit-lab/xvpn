package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/webui"
)

// notBuiltPlaceholderHTML é exibido no lugar do painel quando o binário foi
// compilado sem `npm run build` ter rodado antes em server/web/ (ex.:
// desenvolvimento local do backend isolado) — nunca deve aparecer em
// produção, ver server/README.md.
const notBuiltPlaceholderHTML = `<!DOCTYPE html>
<html lang="pt-BR"><head><meta charset="utf-8"><title>XVPN</title></head>
<body style="font-family: sans-serif; padding: 2rem; color: #333">
<h1>Painel web ainda não foi compilado</h1>
<p>Rode <code>cd server/web && npm run build</code> e reinicie o servidor.</p>
</body></html>`

// registerWebUI monta a UI (painel React embutido) para todas as rotas que
// não são /api/*. Se o painel não foi compilado, devolve uma página de
// aviso em vez de falhar.
func registerWebUI(r *gin.Engine) {
	if !webui.Built() {
		r.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.JSON(http.StatusNotFound, gin.H{"error": "não encontrado"})
				return
			}
			c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(notBuiltPlaceholderHTML))
		})
		return
	}

	sub, err := webui.FS()
	if err != nil {
		r.NoRoute(func(c *gin.Context) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "painel web indisponível"})
		})
		return
	}
	fileServer := http.FileServer(http.FS(sub))

	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"error": "não encontrado"})
			return
		}

		// Roteamento client-side (react-router): se o caminho não bate com
		// um arquivo real do build (JS/CSS/imagens), cai para index.html.
		requestPath := strings.TrimPrefix(c.Request.URL.Path, "/")
		if requestPath != "" {
			if f, err := sub.Open(requestPath); err == nil {
				_ = f.Close()
			} else {
				c.Request.URL.Path = "/"
			}
		}
		fileServer.ServeHTTP(c.Writer, c.Request)
	})
}
