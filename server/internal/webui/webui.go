// Package webui embute o build de produção do painel React (server/web) no
// binário do servidor, via embed.FS — ver PLAN.md §6.3. O diretório dist/ é
// um artefato de build (não versionado, ver PLAN.md §11.1 e .gitignore),
// exceto um placeholder que garante que `go build`/`go test` funcionem
// mesmo num checkout limpo, antes de `npm run build` ter sido executado em
// server/web/.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed dist
var distFS embed.FS

// FS devolve a raiz do build do painel (equivalente a "dist/" do Vite).
func FS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}

// Built reporta se o painel foi de fato compilado (index.html presente),
// distinguindo isso do placeholder committado só para o embed não falhar.
func Built() bool {
	sub, err := FS()
	if err != nil {
		return false
	}
	f, err := sub.Open("index.html")
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}
