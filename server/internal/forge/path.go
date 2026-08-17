// Package forge gerencia repos bare e o smart HTTP do xgit (PLAN.md §6.15).
package forge

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

var (
	ErrInvalidSlug = errors.New("slug inválido")
	ErrGitMissing  = errors.New("git não encontrado no PATH")
)

// RepoPath devolve <root>/<slug>.git. Recusa path traversal.
func RepoPath(root, slug string) (string, error) {
	slug = NormalizeSlug(slug)
	if root == "" || !store.ValidProjectSlug(slug) {
		return "", ErrInvalidSlug
	}
	dir := filepath.Join(filepath.Clean(root), slug+".git")
	rel, err := filepath.Rel(filepath.Clean(root), dir)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", ErrInvalidSlug
	}
	return dir, nil
}

func NormalizeSlug(slug string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(slug)), ".git")
}

func Exists(root, slug string) bool {
	dir, err := RepoPath(root, slug)
	if err != nil {
		return false
	}
	_, err = os.Stat(filepath.Join(dir, "HEAD"))
	return err == nil
}

func LookGit() (string, error) {
	bin, err := exec.LookPath("git")
	if err != nil {
		return "", ErrGitMissing
	}
	return bin, nil
}

// InitBare cria o repo se ainda não existir. http.receivepack fica ligado
// para o smart HTTP autenticado (git-http-backend).
func InitBare(root, slug string) error {
	dir, err := RepoPath(root, slug)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(dir, "HEAD")); err == nil {
		return nil
	}
	bin, err := LookGit()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Clean(root), 0o750); err != nil {
		return err
	}
	cmd := exec.Command(bin, "init", "--bare", "--initial-branch=main", dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.New(strings.TrimSpace(string(out)))
	}
	cfg := exec.Command(bin, "--git-dir="+dir, "config", "http.receivepack", "true")
	if out, err := cfg.CombinedOutput(); err != nil {
		return errors.New(strings.TrimSpace(string(out)))
	}
	return nil
}
