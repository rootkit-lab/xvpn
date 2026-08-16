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

func TestRejectSymlinks(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "files")
	if err := os.MkdirAll(filepath.Join(base, "ok"), 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(dir, "outside")
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "evil")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if err := RejectSymlinks(base, filepath.Join(base, "ok")); err != nil {
		t.Fatal(err)
	}
	if err := RejectSymlinks(base, link); err != ErrBadPath {
		t.Fatalf("symlink na raiz: %v", err)
	}
	if err := RejectSymlinks(base, filepath.Join(link, "x")); err != ErrBadPath {
		t.Fatalf("symlink no meio: %v", err)
	}
}

func TestOpenDirNoFollow_RejectsSymlinkParent(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "files")
	outside := filepath.Join(dir, "outside")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "evil")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenDirNoFollow(base, link); err == nil {
		t.Fatal("esperava erro ao abrir symlink")
	}
	if err := MkdirShare(base, link, "novo", "home", "alice"); err == nil {
		t.Fatal("esperava erro ao mkdir via symlink")
	}
}

func TestChownShare_RejectsSymlink(t *testing.T) {
	u, err := user.Current()
	if err != nil || !validUsername(u.Username) {
		t.Skip("username local fora do padrão Unix do Drive")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := ChownShare(link, "home", u.Username); err == nil {
		t.Fatal("esperava erro em symlink")
	}
}

func TestResolveRejectsBadUser(t *testing.T) {
	r := Roots{SharedDir: "/tmp/s", HomeRoot: "/tmp/h"}
	if _, err := r.Resolve("../root", "home", ""); err != ErrBadUser {
		t.Fatalf("esperava ErrBadUser, veio %v", err)
	}
}
