package marketplace

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("erro criando store: %v", err)
	}
	return s
}

func TestPut_ComputesHashAndPersists(t *testing.T) {
	s := newTestStore(t)
	content := []byte("conteudo de teste do asset")

	res, err := s.Put(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	if res.Size != int64(len(content)) {
		t.Fatalf("tamanho esperado %d, obtido %d", len(content), res.Size)
	}
	if res.SHA256 == "" || len(res.SHA256) != 64 {
		t.Fatalf("sha256 inesperado: %q", res.SHA256)
	}

	abs, err := s.AbsPath(res.RelPath)
	if err != nil {
		t.Fatalf("erro resolvendo caminho absoluto: %v", err)
	}
	got, err := os.ReadFile(abs)
	if err != nil {
		t.Fatalf("erro lendo blob gravado: %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("conteúdo gravado difere do original")
	}
}

func TestPut_DeduplicatesIdenticalContent(t *testing.T) {
	s := newTestStore(t)
	content := []byte("mesmo conteudo enviado duas vezes")

	first, err := s.Put(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("erro no primeiro upload: %v", err)
	}
	second, err := s.Put(bytes.NewReader(content))
	if err != nil {
		t.Fatalf("erro no segundo upload: %v", err)
	}
	if first.RelPath != second.RelPath || first.SHA256 != second.SHA256 {
		t.Fatalf("uploads idênticos deveriam produzir o mesmo blob: %+v vs %+v", first, second)
	}

	// Só deve existir um arquivo físico de blob (dedupe real, não só
	// hashes iguais coincidentemente).
	var blobCount int
	root := blobsRootForTest(t, s)
	_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			blobCount++
		}
		return nil
	})
	if blobCount != 1 {
		t.Fatalf("esperava exatamente 1 blob físico após dedupe, obtido %d", blobCount)
	}
}

func TestPut_DifferentContentProducesDifferentBlobs(t *testing.T) {
	s := newTestStore(t)

	a, err := s.Put(strings.NewReader("conteudo A"))
	if err != nil {
		t.Fatalf("erro no upload A: %v", err)
	}
	b, err := s.Put(strings.NewReader("conteudo B"))
	if err != nil {
		t.Fatalf("erro no upload B: %v", err)
	}
	if a.RelPath == b.RelPath {
		t.Fatalf("conteúdos diferentes não deveriam compartilhar blob")
	}
}

func TestPut_RejectsOversizedAsset(t *testing.T) {
	s := newTestStore(t)
	// Reduz o limite para este teste (mesmo pacote, campo não exportado)
	// em vez de gravar MaxAssetSize (2 GiB) de verdade em disco só para
	// testar o caminho de rejeição.
	s.maxSize = 10

	_, err := s.Put(strings.NewReader("conteudo maior que dez bytes"))
	if err == nil {
		t.Fatalf("esperava erro para asset maior que o limite")
	}
	if err != ErrAssetTooLarge {
		t.Fatalf("esperava ErrAssetTooLarge, obtido: %v", err)
	}
}

func TestAbsPath_RejectsEscapeFromRoot(t *testing.T) {
	s := newTestStore(t)
	if _, err := s.AbsPath("../../etc/passwd"); err == nil {
		t.Fatalf("esperava erro para caminho relativo que escapa da raiz")
	}
}

func TestRemove_IsIdempotent(t *testing.T) {
	s := newTestStore(t)
	res, err := s.Put(strings.NewReader("para remover"))
	if err != nil {
		t.Fatalf("erro no upload: %v", err)
	}
	if err := s.Remove(res.RelPath); err != nil {
		t.Fatalf("erro removendo blob: %v", err)
	}
	// Segunda remoção do mesmo blob não deve falhar.
	if err := s.Remove(res.RelPath); err != nil {
		t.Fatalf("segunda remoção deveria ser no-op, obtido erro: %v", err)
	}
}

func blobsRootForTest(t *testing.T, s *Store) string {
	t.Helper()
	return filepath.Join(s.root, "blobs")
}
