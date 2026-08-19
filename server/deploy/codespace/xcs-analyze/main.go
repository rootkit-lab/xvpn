package main

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	maxModules = 4
	maxPkgs    = 24
	maxFiles   = 12
	maxSyms    = 16
	maxBytes   = 14 << 10
)

type report struct {
	Root    string   `json:"root"`
	Modules []module `json:"modules"`
	Docs    []string `json:"docs"`
	Hint    string   `json:"hint"`
}

type module struct {
	Path     string    `json:"path"`
	Dir      string    `json:"dir"`
	Packages []pkgInfo `json:"packages"`
}

type pkgInfo struct {
	Name    string   `json:"name"`
	Dir     string   `json:"dir"`
	Files   []string `json:"files,omitempty"`
	Symbols []string `json:"symbols,omitempty"`
}

var skipDir = map[string]struct{}{
	".git": {}, "node_modules": {}, "vendor": {}, "dist": {},
	".openvscode-server": {}, ".cache": {}, "testdata": {},
}

func main() {
	root, err := os.Getwd()
	if err != nil {
		fail(err)
	}
	if len(os.Args) > 1 && strings.TrimSpace(os.Args[1]) != "" {
		root = os.Args[1]
	}
	root, err = filepath.Abs(root)
	if err != nil {
		fail(err)
	}
	out, err := analyze(root)
	if err != nil {
		fail(err)
	}
	raw, err := json.Marshal(out)
	if err != nil {
		fail(err)
	}
	if len(raw) > maxBytes {
		out.Hint = "mapa truncado"
		out.Modules = out.Modules[:min(len(out.Modules), 2)]
		raw, _ = json.Marshal(out)
		if len(raw) > maxBytes {
			raw = raw[:maxBytes]
		}
	}
	os.Stdout.Write(raw)
	os.Stdout.Write([]byte("\n"))
}

func fail(err error) {
	os.Stderr.WriteString(err.Error() + "\n")
	os.Exit(1)
}

func analyze(root string) (report, error) {
	rep := report{
		Root: root,
		Hint: "Use os packages/symbols para grep e testes (go test ./...) no clone. Sem docker.sock.",
	}
	mods, err := findModules(root)
	if err != nil {
		return rep, err
	}
	for _, dir := range mods {
		mod, err := readModule(root, dir)
		if err != nil {
			continue
		}
		rep.Modules = append(rep.Modules, mod)
	}
	for _, name := range []string{"AGENTS.md", "README.md", "PLAN.md", "CONTRIBUTING.md", "go.mod"} {
		if _, err := os.Stat(filepath.Join(root, name)); err == nil {
			rep.Docs = append(rep.Docs, name)
		}
	}
	return rep, nil
}

func findModules(root string) ([]string, error) {
	var mods []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if _, skip := skipDir[d.Name()]; skip && path != root {
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() == "go.mod" && len(mods) < maxModules {
			mods = append(mods, filepath.Dir(path))
		}
		return nil
	})
	return mods, err
}

func readModule(root, dir string) (module, error) {
	raw, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return module{}, err
	}
	modPath := modulePath(string(raw))
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		rel = dir
	}
	if rel == "." {
		rel = "."
	}
	m := module{Path: modPath, Dir: filepath.ToSlash(rel)}
	pkgs, err := scanPackages(dir)
	if err != nil {
		return m, err
	}
	m.Packages = pkgs
	return m, nil
}

func modulePath(src string) string {
	for _, line := range strings.Split(src, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

func scanPackages(modDir string) ([]pkgInfo, error) {
	byDir := map[string]*pkgInfo{}
	fset := token.NewFileSet()
	err := filepath.WalkDir(modDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if _, skip := skipDir[d.Name()]; skip && path != modDir {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".go") || strings.HasSuffix(d.Name(), "_test.go") {
			return nil
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(modDir, filepath.Dir(path))
		if err != nil {
			rel = filepath.Dir(path)
		}
		key := filepath.ToSlash(rel)
		pkg := byDir[key]
		if pkg == nil {
			if len(byDir) >= maxPkgs {
				return nil
			}
			pkg = &pkgInfo{Name: file.Name.Name, Dir: key}
			byDir[key] = pkg
		}
		if len(pkg.Files) < maxFiles {
			pkg.Files = append(pkg.Files, d.Name())
		}
		for _, decl := range file.Decls {
			if len(pkg.Symbols) >= maxSyms {
				break
			}
			switch n := decl.(type) {
			case *ast.FuncDecl:
				if n.Name != nil && n.Name.IsExported() {
					pkg.Symbols = append(pkg.Symbols, n.Name.Name)
				}
			case *ast.GenDecl:
				if n.Tok != token.TYPE {
					continue
				}
				for _, spec := range n.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if ok && ts.Name != nil && ts.Name.IsExported() && len(pkg.Symbols) < maxSyms {
						pkg.Symbols = append(pkg.Symbols, ts.Name.Name)
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]pkgInfo, 0, len(byDir))
	for _, p := range byDir {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dir < out[j].Dir })
	if len(out) > maxPkgs {
		out = out[:maxPkgs]
	}
	return out, nil
}
