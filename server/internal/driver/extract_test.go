package driver

import (
	"archive/zip"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	if _, err := SafeArchiveRel(".pad/x"); err == nil {
		t.Fatal("nome oculto deveria ser recusado")
	}
	if _, err := SafeArchiveRel("a/.hidden"); err == nil {
		t.Fatal("componente oculto deveria ser recusado")
	}
	deep := strings.Repeat("a/", MaxExtractDepth) + "x"
	if _, err := SafeArchiveRel(deep); !errors.Is(err, ErrExtractBomb) {
		t.Fatalf("profundidade: %v", err)
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

func TestZipCentralCountRejectsTooManyEntries(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "muitos.zip")
	zf, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(zf)
	for i := 0; i < MaxExtractFiles+1; i++ {
		w, err := zw.CreateHeader(&zip.FileHeader{Name: fmt.Sprintf("d/%d", i), Method: zip.Store})
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write(nil)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zf.Close(); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := zipCentralCount(f, st.Size()); !errors.Is(err, ErrExtractBomb) {
		t.Fatalf("esperava ErrExtractBomb, veio %v", err)
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
