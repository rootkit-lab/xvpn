package driver

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"
)

func TestSafeArchiveRelRejectsTraversal(t *testing.T) {
	for _, name := range []string{"../etc/passwd", "/etc/passwd", "a/../../x", `..\win`} {
		if _, err := SafeArchiveRel(name); err == nil {
			t.Fatalf("deveria recusar %q", name)
		}
	}
	rel, err := SafeArchiveRel("docs/a.txt")
	if err != nil || rel != "docs/a.txt" {
		t.Fatalf("rel=%q err=%v", rel, err)
	}
}

func TestExtractZipWritesUnderDest(t *testing.T) {
	dir := t.TempDir()
	shared := filepath.Join(dir, "shared")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(shared, "pacote.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	w, err := zw.Create("ola.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("oi")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zf.Close(); err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(shared, "pacote")
	if err := os.Mkdir(dest, 0o775); err != nil {
		t.Fatal(err)
	}
	if err := ExtractArchive(shared, zipPath, dest, "shared", "nobody"); err != nil {
		// chown/setfacl pode falhar fora do VPS — ainda assim o arquivo deve existir
		if !os.IsNotExist(err) && !os.IsPermission(err) {
			t.Logf("extract: %v (ok se setfacl/chown falhou no sandbox)", err)
		}
	}
	got, err := os.ReadFile(filepath.Join(dest, "ola.txt"))
	if err != nil {
		t.Fatalf("arquivo extraído: %v", err)
	}
	if string(got) != "oi" {
		t.Fatalf("conteúdo=%q", got)
	}
}

func TestExtractZipRejectsSlip(t *testing.T) {
	dir := t.TempDir()
	shared := filepath.Join(dir, "shared")
	if err := os.MkdirAll(shared, 0o755); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(shared, "evil.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	w, err := zw.Create("../fora.txt")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write([]byte("no"))
	_ = zw.Close()
	_ = zf.Close()

	dest := filepath.Join(shared, "evil")
	if err := os.Mkdir(dest, 0o775); err != nil {
		t.Fatal(err)
	}
	if err := ExtractArchive(shared, zipPath, dest, "shared", "nobody"); err == nil {
		t.Fatal("zip slip deveria falhar")
	}
	if _, err := os.Stat(filepath.Join(dir, "fora.txt")); err == nil {
		t.Fatal("arquivo escapou da pasta de destino")
	}
}
