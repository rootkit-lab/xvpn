package backup

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseResticSummary(t *testing.T) {
	out := []byte(`{"message_type":"status"}
{"message_type":"summary","snapshot_id":"abc123","total_bytes_processed":99}
`)
	snap, n := parseResticSummary(out)
	if snap != "abc123" || n != 99 {
		t.Fatalf("got %s %d", snap, n)
	}
}

func TestCollectPaths(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "git"), 0o750); err != nil {
		t.Fatal(err)
	}
	paths, err := collectPaths(Include{Git: true, GitDir: filepath.Join(dir, "git")})
	if err != nil || len(paths) != 1 {
		t.Fatalf("%v %v", paths, err)
	}
	if _, err := collectPaths(Include{}); err != ErrNoPaths {
		t.Fatalf("empty: %v", err)
	}
}

func TestLocalResticDryRun(t *testing.T) {
	if _, err := lookRestic(); err != nil {
		t.Skip("restic não está no PATH")
	}
	src := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "hello.txt"), []byte("oi"), 0o640); err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	r := &Runner{}
	res, err := r.Backup(context.Background(), Dest{
		Kind:   "local",
		Path:   repo,
		Secret: Secret{Password: "test-pass-not-a-secret"},
	}, Include{Git: true, GitDir: src}, t.TempDir(), true)
	if err != nil {
		t.Fatal(err)
	}
	if res.SnapshotID != "dry-run" {
		t.Fatalf("snap: %s log=%s", res.SnapshotID, res.Log)
	}
	if !strings.Contains(res.Log, "hello.txt") && res.Log == "" {
		t.Fatalf("log vazio: %+v", res)
	}
}

func lookRestic() (string, error) {
	return (&Runner{}).look("restic")
}

func TestRcloneConfigAllowlist(t *testing.T) {
	if _, err := rcloneConfig("drive", Secret{RcloneConf: "type = sftp\nssh = /bin/sh"}); err != ErrBadKind {
		t.Fatalf("raw conf: %v", err)
	}
	if _, err := rcloneConfig("drive", Secret{RcloneConf: `{"access_token":"x"}`}); err != nil {
		t.Fatal(err)
	}
	if _, err := rcloneConfig("webdav", Secret{WebDAVURL: "https://dav.example/"}); err != nil {
		t.Fatal(err)
	}
	if _, err := rcloneConfig("webdav", Secret{WebDAVURL: "https://dav.example/", WebDAVUser: "u\ntype = sftp"}); err != ErrBadKind {
		t.Fatalf("ini inject: %v", err)
	}
	if safeRclonePath("../etc") || safeRclonePath("a:b") {
		t.Fatal("path")
	}
	if safeSFTPHost("-oProxyCommand=x") || safeSFTPHost("host:22") || !safeSFTPHost("nas.example") {
		t.Fatal("sftp host")
	}
}
