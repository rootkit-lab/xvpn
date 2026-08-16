package driver

import (
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

func TestResolveRejectsTraversal(t *testing.T) {
	dir := t.TempDir()
	r := Roots{SharedDir: filepath.Join(dir, "shared"), HomeRoot: filepath.Join(dir, "home")}
	if err := os.MkdirAll(filepath.Join(r.HomeRoot, "alice", "files"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Resolve("alice", "home", "../etc/passwd"); err != ErrBadPath {
		t.Fatalf("esperava ErrBadPath, veio %v", err)
	}
	if _, err := r.Resolve("alice", "shared", ".."); err != ErrBadPath {
		t.Fatalf("esperava ErrBadPath no shared, veio %v", err)
	}
	got, err := r.Resolve("alice", "home", "docs/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(r.HomeRoot, "alice", "files", "docs", "a.txt")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestChownShare_RejectsBadUser(t *testing.T) {
	if err := ChownShare("/tmp", "home", "../root"); err != ErrBadUser {
		t.Fatalf("esperava ErrBadUser, veio %v", err)
	}
}

func TestChownShare_SameUser(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		t.Skip(err)
	}
	if !validUsername(u.Username) {
		t.Skip("username local fora do padrão Unix do Drive")
	}
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ChownShare(p, "home", u.Username); err != nil {
		t.Fatal(err)
	}
}

func TestResolveRejectsBadUser(t *testing.T) {
	r := Roots{SharedDir: "/tmp/s", HomeRoot: "/tmp/h"}
	if _, err := r.Resolve("../root", "home", ""); err != ErrBadUser {
		t.Fatalf("esperava ErrBadUser, veio %v", err)
	}
}
