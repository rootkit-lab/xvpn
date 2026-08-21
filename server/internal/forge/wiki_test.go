package forge

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWikiHomeRoundTrip(t *testing.T) {
	if _, err := LookGit(); err != nil {
		t.Skip("git ausente")
	}
	root := t.TempDir()
	if err := InitBare(root, "xcorp/lab"); err != nil {
		t.Fatal(err)
	}
	pages, err := ListWiki(root, "xcorp/lab")
	if err != nil || len(pages) != 0 {
		t.Fatalf("vazio: %v %v", pages, err)
	}
	if _, err := WriteWiki(root, "xcorp/lab", "Home", "# Olá\n", "wiki: Home", "alice", "alice@corp.ihuull.com"); err != nil {
		t.Fatal(err)
	}
	got, err := ReadWiki(root, "xcorp/lab", "1")
	if err != nil || got != "# Olá\n" {
		t.Fatalf("read: %q %v", got, err)
	}
	if _, err := WriteWiki(root, "xcorp/lab", "Guia", "passo\n", "wiki: Guia", "alice", "alice@corp.ihuull.com"); err != nil {
		t.Fatal(err)
	}
	list, err := ListWiki(root, "xcorp/lab")
	if err != nil || len(list) != 2 || list[0] != "Home" || list[1] != "Guia" {
		t.Fatalf("list: %v %v", list, err)
	}
	dir, err := RepoPath(root, "xcorp/lab")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "refs/xgit/wiki")); err != nil {
		t.Fatal(err)
	}
}

func TestWikiPageFileRejectsTraversal(t *testing.T) {
	if _, err := WikiPageFile("../etc"); err == nil {
		t.Fatal("esperava recusar")
	}
	file, err := WikiPageFile("#1")
	if err != nil || file != "Home.md" {
		t.Fatalf("home: %q %v", file, err)
	}
}
