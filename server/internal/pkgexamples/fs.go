// Package pkgexamples embute os exemplos de package por linguagem
// (Fase 45.3). Fonte canónica no monorepo; o boot do xvpn-server
// cria o projeto XGIT e publica o artefacto.
package pkgexamples

import (
	"embed"
	"io/fs"
	"strings"
)

//go:embed all:fs
var tree embed.FS

// Lang é um exemplo (pasta em fs/).
type Lang string

const (
	JavaScript Lang = "javascript"
	Python     Lang = "python"
	Go         Lang = "golang"
	Rust       Lang = "rust"
	Generic    Lang = "generic"
)

// Spec descreve o projeto XGIT e o artefacto publicado.
type Spec struct {
	Lang        Lang
	Slug        string
	Name        string
	Title       string
	Description string
	Kind        string // generic | npm | pypi
	PkgName     string
	Version     string
	Filename    string
}

// All é a ordem de seed (uma pasta por linguagem).
var All = []Lang{JavaScript, Python, Go, Rust, Generic}

// Specs são os cinco exemplos (slug 2–20 chars).
var Specs = []Spec{
	{
		Lang: JavaScript, Slug: "hello-js", Name: "hello-js", Title: "hello-js",
		Description: "Exemplo npm (@ihuull/hello-js) no registry XGIT.",
		Kind:        "npm", PkgName: "@ihuull/hello-js", Version: "0.1.0",
		Filename: "hello-js-0.1.0.tgz",
	},
	{
		Lang: Python, Slug: "hello-py", Name: "hello-py", Title: "hello-py",
		Description: "Exemplo PyPI (hello-ihuull) no registry XGIT.",
		Kind:        "pypi", PkgName: "hello-ihuull", Version: "0.1.0",
		Filename: "hello_ihuull-0.1.0.tar.gz",
	},
	{
		Lang: Go, Slug: "hello-go", Name: "hello-go", Title: "hello-go",
		Description: "Exemplo Go (tarball generic) no XGIT.",
		Kind:        "generic", PkgName: "hello-go", Version: "0.1.0",
		Filename: "hello-go-0.1.0.tar.gz",
	},
	{
		Lang: Rust, Slug: "hello-rs", Name: "hello-rs", Title: "hello-rs",
		Description: "Exemplo Rust (tarball generic) no XGIT.",
		Kind:        "generic", PkgName: "hello-rs", Version: "0.1.0",
		Filename: "hello-rs-0.1.0.tar.gz",
	},
	{
		Lang: Generic, Slug: "hello-bin", Name: "hello-bin", Title: "hello-bin",
		Description: "Exemplo generic (ficheiro solto) no registry XGIT.",
		Kind:        "generic", PkgName: "hello-bin", Version: "0.1.0",
		Filename: "hello-bin-0.1.0.tar.gz",
	},
}

// Files devolve path → conteúdo de texto da pasta do exemplo.
func Files(lang Lang) (map[string]string, error) {
	root := "fs/" + string(lang)
	out := map[string]string{}
	err := fs.WalkDir(tree, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := tree.ReadFile(path)
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(path, root+"/")
		rel = strings.TrimSuffix(rel, ".src")
		out[rel] = string(data)
		return nil
	})
	return out, err
}
