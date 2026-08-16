//go:build linux

package helper

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestMkdirOpenat_ReplacesSymlink(t *testing.T) {
	root := t.TempDir()
	if err := os.Symlink("/tmp", filepath.Join(root, "XVPN")); err != nil {
		t.Fatal(err)
	}
	parent, err := unix.Open(root, unix.O_DIRECTORY|unix.O_RDONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(parent)
	fd, err := mkdirOpenat(parent, "XVPN", os.Getuid(), os.Getgid())
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(fd)
	fi, err := os.Lstat(filepath.Join(root, "XVPN"))
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		t.Fatal("ainda é symlink")
	}
	if !fi.IsDir() {
		t.Fatal("não é diretório")
	}
}

func TestMkdirOpenat_RejectsTraversal(t *testing.T) {
	root := t.TempDir()
	parent, err := unix.Open(root, unix.O_DIRECTORY|unix.O_RDONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer unix.Close(parent)
	if _, err := mkdirOpenat(parent, "..", os.Getuid(), os.Getgid()); err == nil {
		t.Fatal(".. deveria falhar")
	}
	if _, err := mkdirOpenat(parent, "a/b", os.Getuid(), os.Getgid()); err == nil {
		t.Fatal("slash deveria falhar")
	}
}
