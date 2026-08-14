// Package marketplace implementa o armazenamento em disco dos blobs de
// asset do catálogo de software (Fase 11 — ver PLAN.md §6.8). Endereçado
// por conteúdo (SHA-256): o caminho físico de um arquivo é sempre derivado
// do próprio hash, nunca do nome ou de qualquer entrada vinda do cliente —
// isso elimina path traversal por construção (não há string de usuário no
// caminho) e deduplica uploads idênticos automaticamente.
package marketplace

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// MaxAssetSize limita o tamanho de um único upload de asset — generoso o
// bastante para um instalador Windows ou AppImage grandes, curto o
// suficiente para não deixar um upload só encher o disco do VPS (ver
// PLAN.md §6.8, "documentar limites"). Reavalie se o catálogo passar a
// hospedar imagens de VM/ISO ou algo nessa faixa.
const MaxAssetSize = 2 << 30 // 2 GiB

// ErrAssetTooLarge é devolvido por Put quando o conteúdo lido excede
// MaxAssetSize.
var ErrAssetTooLarge = errors.New("asset excede o tamanho máximo permitido")

// Store é um armazenamento de blobs endereçado por conteúdo em disco.
// Seguro para uso concorrente: cada Put grava num arquivo temporário
// próprio e só o publica via os.Rename (atômico no mesmo filesystem) no
// caminho final derivado do hash.
type Store struct {
	root string
	// maxSize é sempre MaxAssetSize em produção; os testes deste pacote
	// reduzem o valor (via um Store construído manualmente, mesmo
	// arquivo/pacote) para exercitar o caminho de rejeição sem precisar
	// gravar gigabytes de verdade em disco.
	maxSize int64
}

// NewStore garante que a estrutura de diretórios em root existe e retorna
// um Store pronto para uso. Chamado uma vez na inicialização do servidor,
// igual ao padrão de wireguard.NewManager/EnsureInterface.
func NewStore(root string) (*Store, error) {
	if root == "" {
		return nil, errors.New("diretório de armazenamento do marketplace vazio")
	}
	for _, dir := range []string{filepath.Join(root, "blobs"), filepath.Join(root, "tmp")} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return nil, fmt.Errorf("criando diretório %q: %w", dir, err)
		}
	}
	return &Store{root: root, maxSize: MaxAssetSize}, nil
}

// PutResult descreve um blob já gravado com sucesso.
type PutResult struct {
	SHA256 string
	Size   int64
	// RelPath é relativo à raiz do Store — é este valor (nunca um
	// caminho absoluto) que fica salvo em store.AppAsset.StoragePath.
	RelPath string
}

// Put lê todo o conteúdo de r, calcula o SHA-256 e grava o blob content-
// addressed. Se outro AppAsset já tiver feito upload do mesmo conteúdo
// antes, o arquivo novo é descartado e o existente é reaproveitado
// (dedupe automático via os.Stat antes do rename).
func (s *Store) Put(r io.Reader) (PutResult, error) {
	tmp, err := os.CreateTemp(filepath.Join(s.root, "tmp"), "upload-*")
	if err != nil {
		return PutResult{}, fmt.Errorf("criando arquivo temporário de upload: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op após o rename bem-sucedido abaixo

	hasher := sha256.New()
	// Lê até um byte a mais que o limite: se vier esse byte extra,
	// sabemos que o conteúdo excede o limite sem confiar num
	// Content-Length potencialmente forjado pelo cliente.
	limited := io.LimitReader(r, s.maxSize+1)
	written, copyErr := io.Copy(io.MultiWriter(tmp, hasher), limited)
	closeErr := tmp.Close()
	if copyErr != nil {
		return PutResult{}, fmt.Errorf("gravando upload: %w", copyErr)
	}
	if closeErr != nil {
		return PutResult{}, fmt.Errorf("fechando arquivo temporário: %w", closeErr)
	}
	if written > s.maxSize {
		return PutResult{}, ErrAssetTooLarge
	}

	sum := hex.EncodeToString(hasher.Sum(nil))
	relPath := blobRelPath(sum)
	finalPath := filepath.Join(s.root, relPath)

	if _, err := os.Stat(finalPath); err == nil {
		// Já existe um blob idêntico — descarta o temporário (via defer
		// acima) e reaproveita o que já está em disco.
		return PutResult{SHA256: sum, Size: written, RelPath: relPath}, nil
	} else if !os.IsNotExist(err) {
		return PutResult{}, fmt.Errorf("verificando blob existente: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(finalPath), 0o750); err != nil {
		return PutResult{}, fmt.Errorf("criando diretório do blob: %w", err)
	}
	if err := os.Rename(tmpPath, finalPath); err != nil {
		return PutResult{}, fmt.Errorf("publicando blob: %w", err)
	}
	return PutResult{SHA256: sum, Size: written, RelPath: relPath}, nil
}

// AbsPath resolve o caminho absoluto de um blob a partir do RelPath salvo
// em store.AppAsset.StoragePath. relPath sempre vem do banco (nunca
// diretamente de input HTTP) — ainda assim, valida que o resultado não
// escapa de root como defesa em profundidade (ver ROADMAP.md Fase 12,
// "revisar path traversal").
func (s *Store) AbsPath(relPath string) (string, error) {
	abs := filepath.Join(s.root, relPath)
	rootWithSep := filepath.Clean(s.root) + string(filepath.Separator)
	if !strings.HasPrefix(abs, rootWithSep) {
		return "", fmt.Errorf("caminho de blob %q fora da raiz de armazenamento", relPath)
	}
	return abs, nil
}

// Remove apaga o blob físico em relPath. Idempotente: "arquivo não existe"
// não é erro (permite chamar de novo com segurança). O chamador (handler
// HTTP, que tem acesso ao banco) é responsável por só invocar Remove quando
// nenhum outro AppAsset ainda referencia o mesmo RelPath — ver
// marketplace_handler.go.
func (s *Store) Remove(relPath string) error {
	abs, err := s.AbsPath(relPath)
	if err != nil {
		return err
	}
	if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removendo blob: %w", err)
	}
	return nil
}

func blobRelPath(sum string) string {
	// Sharding pelos 2 primeiros hex chars (mesma ideia do .git/objects):
	// evita um único diretório com milhares de entradas conforme o
	// catálogo cresce.
	return filepath.Join("blobs", sum[:2], sum)
}
