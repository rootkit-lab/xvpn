package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectJavaScript(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "package.json", `{"name":"hello-js","scripts":{"test":"node --test"}}`)
	write(t, dir, "index.js", "console.log('ok')\n")
	write(t, dir, ".xvpn-ci.sh", "#!/bin/sh\nnpm test\n")
	rep, err := detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !has(rep.Languages, "JavaScript") {
		t.Fatalf("langs=%v", rep.Languages)
	}
	if !has(rep.Manifests, "package.json") || !has(rep.Manifests, ".xvpn-ci.sh") {
		t.Fatalf("manifests=%v", rep.Manifests)
	}
	if !hasRecipe(rep, "npm-test") || !hasRecipe(rep, "node-index") {
		t.Fatalf("recipes=%v", ids(rep))
	}
}

func TestDetectFlaskAndGo(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "web/flask/app.py", "print('flask')\n")
	write(t, dir, "go.mod", "module example.com/hello\n\ngo 1.25\n")
	write(t, dir, "hello.go", "package hello\n")
	rep, err := detect(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !has(rep.Languages, "Python") || !has(rep.Languages, "Go") {
		t.Fatalf("langs=%v", rep.Languages)
	}
	if !hasRecipe(rep, "flask") || !hasRecipe(rep, "go-test") {
		t.Fatalf("recipes=%v", ids(rep))
	}
	for _, r := range rep.Recipes {
		if r.ID == "flask" && (r.Port != 8080 || r.Bind != "0.0.0.0") {
			t.Fatalf("flask recipe: %+v", r)
		}
	}
}

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func has(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func hasRecipe(rep report, id string) bool {
	for _, r := range rep.Recipes {
		if r.ID == id {
			return true
		}
	}
	return false
}

func ids(rep report) []string {
	out := make([]string, 0, len(rep.Recipes))
	for _, r := range rep.Recipes {
		out = append(out, r.ID)
	}
	return out
}
