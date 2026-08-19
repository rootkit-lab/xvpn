package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnalyze_FindsModuleAndExported(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/toy\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hello.go"), []byte("package toy\n\nfunc Hello() string { return \"ok\" }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# toy\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Modules) != 1 || rep.Modules[0].Path != "example.com/toy" {
		t.Fatalf("mod: %+v", rep.Modules)
	}
	if len(rep.Modules[0].Packages) == 0 || !contains(rep.Modules[0].Packages[0].Symbols, "Hello") {
		t.Fatalf("pkg: %+v", rep.Modules[0].Packages)
	}
	if !contains(rep.Docs, "README.md") || !contains(rep.Docs, "go.mod") {
		t.Fatalf("docs: %v", rep.Docs)
	}
}

func TestAnalyze_SkipsGitAndVendor(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/skip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "vendor", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "vendor", "x", "go.mod"), []byte("module hidden\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rep, err := analyze(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Modules) != 1 || strings.Contains(rep.Modules[0].Path, "hidden") {
		t.Fatalf("não deve entrar em vendor: %+v", rep.Modules)
	}
}

func contains(in []string, want string) bool {
	for _, s := range in {
		if s == want {
			return true
		}
	}
	return false
}
