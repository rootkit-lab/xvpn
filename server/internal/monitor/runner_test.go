package monitor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckGitMount_missingDir(t *testing.T) {
	r := &Runner{GitDir: filepath.Join(t.TempDir(), "missing")}
	res := r.checkGitMount()
	if res.Status != StatusCritical {
		t.Fatalf("status = %q, want critical", res.Status)
	}
}

func TestCheckGitMount_okWithXcorp(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "xcorp"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := &Runner{GitDir: dir}
	res := r.checkGitMount()
	if res.Status != StatusOK {
		t.Fatalf("status = %q, want ok", res.Status)
	}
}

func TestCheckGitMount_warnWithoutXcorp(t *testing.T) {
	dir := t.TempDir()
	r := &Runner{GitDir: dir}
	res := r.checkGitMount()
	if res.Status != StatusWarn {
		t.Fatalf("status = %q, want warn", res.Status)
	}
}

func TestCheckMongoData_skippedWhenDown(t *testing.T) {
	r := &Runner{}
	res := r.checkMongoData()
	if res.Status != StatusSkipped {
		t.Fatalf("status = %q, want skipped", res.Status)
	}
}
