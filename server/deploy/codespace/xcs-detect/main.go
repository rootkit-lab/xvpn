package main

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxBytes = 12 << 10

type report struct {
	Root      string   `json:"root"`
	Languages []string `json:"languages"`
	Manifests []string `json:"manifests"`
	Recipes   []recipe `json:"recipes"`
	Hint      string   `json:"hint"`
}

type recipe struct {
	ID    string `json:"id"`
	Lang  string `json:"lang"`
	Cmd   string `json:"cmd"`
	Port  int    `json:"port,omitempty"`
	Cwd   string `json:"cwd"`
	Bind  string `json:"bind,omitempty"`
	Label string `json:"label"`
}

type found struct {
	rel string
	abs string
}

var skipDir = map[string]struct{}{
	".git": {}, "node_modules": {}, "vendor": {}, "dist": {},
	".openvscode-server": {}, ".cache": {}, "testdata": {},
	"target": {}, "__pycache__": {},
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
	out, err := detect(root)
	if err != nil {
		fail(err)
	}
	raw, err := json.Marshal(out)
	if err != nil {
		fail(err)
	}
	if len(raw) > maxBytes {
		out.Hint = "mapa truncado"
		if len(out.Recipes) > 6 {
			out.Recipes = out.Recipes[:6]
		}
		raw, _ = json.Marshal(out)
	}
	os.Stdout.Write(raw)
	os.Stdout.Write([]byte("\n"))
}

func fail(err error) {
	os.Stderr.WriteString(err.Error() + "\n")
	os.Exit(1)
}

func detect(root string) (report, error) {
	rep := report{
		Root: root,
		Hint: "Receitas no clone. Bind 0.0.0.0 para preview demo-*. Sem docker.sock.",
	}
	files, err := listFiles(root)
	if err != nil {
		return rep, err
	}
	seenLang := map[string]struct{}{}
	seenMan := map[string]struct{}{}
	addLang := func(name string) {
		if _, ok := seenLang[name]; ok {
			return
		}
		seenLang[name] = struct{}{}
		rep.Languages = append(rep.Languages, name)
	}
	addMan := func(rel string) {
		rel = filepath.ToSlash(rel)
		if _, ok := seenMan[rel]; ok {
			return
		}
		seenMan[rel] = struct{}{}
		rep.Manifests = append(rep.Manifests, rel)
	}
	lookup := func(name string) (found, bool) {
		for _, f := range files {
			if f.rel == name || strings.HasSuffix(f.rel, "/"+name) {
				return f, true
			}
		}
		return found{}, false
	}
	hasRel := func(name string) bool {
		for _, f := range files {
			if f.rel == name {
				return true
			}
		}
		return false
	}

	if f, ok := lookup("package.json"); ok {
		addLang("JavaScript")
		addMan(f.rel)
		cwd := dirOf(f.rel)
		body, _ := os.ReadFile(f.abs)
		s := string(body)
		if strings.Contains(s, `"test"`) {
			rep.Recipes = append(rep.Recipes, recipe{ID: "npm-test", Lang: "JavaScript", Cmd: "npm test", Cwd: cwd, Label: "npm test"})
		}
		if strings.Contains(s, `"start"`) {
			rep.Recipes = append(rep.Recipes, recipe{ID: "npm-start", Lang: "JavaScript", Cmd: "npm start", Cwd: cwd, Port: 3000, Bind: "0.0.0.0", Label: "npm start"})
		} else if hasRel(joinRel(cwd, "index.js")) {
			rep.Recipes = append(rep.Recipes, recipe{ID: "node-index", Lang: "JavaScript", Cmd: "node index.js", Cwd: cwd, Label: "node index.js"})
		}
	}
	if f, ok := lookup("pyproject.toml"); ok {
		addLang("Python")
		addMan(f.rel)
		rep.Recipes = append(rep.Recipes, recipe{
			ID: "py-import", Lang: "Python", Cmd: "python3 -c \"import hello_ihuull; print('ok')\"",
			Cwd: dirOf(f.rel), Label: "import hello_ihuull",
		})
	}
	if hasRel("web/flask/app.py") {
		addLang("Python")
		addMan("web/flask/app.py")
		rep.Recipes = append(rep.Recipes, recipe{
			ID: "flask", Lang: "Python", Cmd: "python3 web/flask/app.py",
			Cwd: ".", Port: 8080, Bind: "0.0.0.0", Label: "Flask",
		})
	} else if f, ok := lookup("app.py"); ok {
		addLang("Python")
		addMan(f.rel)
		rep.Recipes = append(rep.Recipes, recipe{
			ID: "flask", Lang: "Python", Cmd: "python3 " + f.rel,
			Cwd: ".", Port: 8080, Bind: "0.0.0.0", Label: "Flask",
		})
	}
	if f, ok := lookup("go.mod"); ok {
		addLang("Go")
		addMan(f.rel)
		cwd := dirOf(f.rel)
		rep.Recipes = append(rep.Recipes, recipe{ID: "go-test", Lang: "Go", Cmd: "go test ./...", Cwd: cwd, Label: "go test"})
		if hasRel(joinRel(cwd, "main.go")) || hasSuffix(files, "/main.go") || hasRel("hello.go") {
			rep.Recipes = append(rep.Recipes, recipe{ID: "go-run", Lang: "Go", Cmd: "go test ./...", Cwd: cwd, Label: "go test"})
		}
	}
	if f, ok := lookup("Cargo.toml"); ok {
		addLang("Rust")
		addMan(f.rel)
		rep.Recipes = append(rep.Recipes, recipe{ID: "cargo-test", Lang: "Rust", Cmd: "cargo test", Cwd: dirOf(f.rel), Label: "cargo test"})
	}
	if f, ok := lookup("pom.xml"); ok {
		addLang("Java")
		addMan(f.rel)
		rep.Recipes = append(rep.Recipes, recipe{ID: "mvn-package", Lang: "Java", Cmd: "mvn -q -DskipTests package", Cwd: dirOf(f.rel), Label: "mvn package"})
	}
	if f, ok := lookup(".xvpn-ci.sh"); ok {
		addMan(f.rel)
	}

	sort.Strings(rep.Languages)
	sort.Strings(rep.Manifests)
	return rep, nil
}

func listFiles(root string) ([]found, error) {
	var files []found
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
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		files = append(files, found{rel: filepath.ToSlash(rel), abs: path})
		return nil
	})
	return files, err
}

func dirOf(rel string) string {
	d := filepath.ToSlash(filepath.Dir(rel))
	if d == "" || d == "." {
		return "."
	}
	return d
}

func joinRel(cwd, name string) string {
	if cwd == "." || cwd == "" {
		return name
	}
	return cwd + "/" + name
}

func hasSuffix(files []found, suf string) bool {
	for _, f := range files {
		if strings.HasSuffix(f.rel, suf) {
			return true
		}
	}
	return false
}
