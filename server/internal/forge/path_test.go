package forge

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestRepoPathRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	if _, err := RepoPath(root, "../etc"); err == nil {
		t.Fatal("traversal deveria falhar")
	}
	if _, err := RepoPath(root, "xchat"); err == nil {
		t.Fatal("path plano deveria falhar")
	}
	dir, err := RepoPath(root, "xcorp/xchat")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(dir, filepath.Clean(root)) || !strings.HasSuffix(dir, filepath.Join("xcorp", "xchat.git")) {
		t.Fatalf("path inesperado: %s", dir)
	}
}
