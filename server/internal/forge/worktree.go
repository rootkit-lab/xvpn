package forge

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// AddWorktree cria um worktree ligado ao bare, na branch pedida.
func AddWorktree(root, slug, dest, branch string) error {
	if !ValidBranchName(branch) {
		return ErrInvalidBranch
	}
	dir, err := RepoPath(root, slug)
	if err != nil {
		return err
	}
	if !Exists(root, slug) {
		return ErrEmptyRepo
	}
	if !BranchExists(root, slug, branch) {
		return ErrBranchMissing
	}
	bin, err := LookGit()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o750); err != nil {
		return err
	}
	_ = gitCmd(bin, "--git-dir="+dir, "worktree", "prune").Run()
	add := gitCmd(bin, "--git-dir="+dir, "worktree", "add", dest, branch)
	if out, err := add.CombinedOutput(); err != nil {
		return errors.New(strings.TrimSpace(string(out)))
	}
	return nil
}

// RemoveWorktree desliga o worktree e apaga o diretório.
func RemoveWorktree(root, slug, dest string) error {
	dir, err := RepoPath(root, slug)
	if err != nil {
		return err
	}
	bin, err := LookGit()
	if err != nil {
		return err
	}
	_ = gitCmd(bin, "--git-dir="+dir, "worktree", "remove", "--force", dest).Run()
	_ = gitCmd(bin, "--git-dir="+dir, "worktree", "prune").Run()
	_ = os.RemoveAll(dest)
	return nil
}

// WorktreeCommit grava o índice e cria um commit no worktree (o objeto
// vai para o bare compartilhado).
func WorktreeCommit(dest, message, name, email string) (string, error) {
	message = strings.TrimSpace(message)
	if message == "" {
		return "", ErrEmptyMessage
	}
	bin, err := LookGit()
	if err != nil {
		return "", err
	}
	if name == "" {
		name = "xgit"
	}
	if email == "" {
		email = "xgit@corp.ihuull.com"
	}
	if err := assertWorktreeGitSafe(dest); err != nil {
		return "", err
	}
	add := gitCmd(bin, "-C", dest, "-c", "core.hooksPath=/dev/null", "add", "-A")
	if out, err := add.CombinedOutput(); err != nil {
		return "", errors.New(strings.TrimSpace(string(out)))
	}
	commit := gitCmd(bin, "-C", dest, "-c", "user.name="+name, "-c", "user.email="+email, "-c", "core.hooksPath=/dev/null", "commit", "--no-verify", "-m", message)
	if out, err := commit.CombinedOutput(); err != nil {
		low := strings.ToLower(string(out))
		if strings.Contains(low, "nothing to commit") {
			return "", ErrUnchanged
		}
		return "", errors.New(strings.TrimSpace(string(out)))
	}
	sha := gitCmd(bin, "-C", dest, "rev-parse", "HEAD")
	out, err := sha.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// WorktreeCheckoutBranch cria/troca para uma branch no worktree.
func WorktreeCheckoutBranch(dest, branch string) error {
	if !ValidBranchName(branch) {
		return ErrInvalidBranch
	}
	if err := assertWorktreeGitSafe(dest); err != nil {
		return err
	}
	bin, err := LookGit()
	if err != nil {
		return err
	}
	cmd := gitCmd(bin, "-C", dest, "-c", "core.hooksPath=/dev/null", "checkout", "-B", branch)
	if out, err := cmd.CombinedOutput(); err != nil {
		return errors.New(strings.TrimSpace(string(out)))
	}
	return nil
}

func assertWorktreeGitSafe(dest string) error {
	gitPath := filepath.Join(dest, ".git")
	fi, err := os.Lstat(gitPath)
	if err != nil {
		return err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return errors.New("worktree .git inválido")
	}
	if !fi.Mode().IsRegular() {
		return nil
	}
	raw, err := os.ReadFile(gitPath)
	if err != nil {
		return err
	}
	line := strings.TrimSpace(string(raw))
	const prefix = "gitdir:"
	if !strings.HasPrefix(strings.ToLower(line), prefix) {
		return errors.New("worktree .git inválido")
	}
	gitdir := strings.TrimSpace(line[len(prefix):])
	if gitdir == "" {
		return errors.New("worktree .git inválido")
	}
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(dest, gitdir)
	}
	clean, err := filepath.Abs(gitdir)
	if err != nil {
		return err
	}
	cleanDest, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	if clean == cleanDest || strings.HasPrefix(clean, cleanDest+string(os.PathSeparator)) {
		return errors.New("worktree .git inválido")
	}
	return nil
}
