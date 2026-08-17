package forge

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	ErrInvalidBranch = errors.New("branch inválida")
	ErrSameBranch    = errors.New("source e target iguais")
	ErrBranchMissing = errors.New("branch inexistente")
	ErrEmptyRepo     = errors.New("repositório sem commits")
	ErrMergeConflict = errors.New("conflito de merge")
)

func gitCmd(bin string, args ...string) *exec.Cmd {
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=xgit",
		"GIT_AUTHOR_EMAIL=xgit@corp.ihuull.com",
		"GIT_COMMITTER_NAME=xgit",
		"GIT_COMMITTER_EMAIL=xgit@corp.ihuull.com",
	)
	return cmd
}

// MergeBranches faz merge --no-ff de source em target no bare (worktree temporário).
func MergeBranches(root, slug, source, target, message string) error {
	source = strings.TrimSpace(source)
	target = strings.TrimSpace(target)
	if !ValidBranchName(source) || !ValidBranchName(target) {
		return ErrInvalidBranch
	}
	if source == target {
		return ErrSameBranch
	}
	if !Exists(root, slug) {
		return ErrEmptyRepo
	}
	if !BranchExists(root, slug, source) || !BranchExists(root, slug, target) {
		return ErrBranchMissing
	}
	dir, err := RepoPath(root, slug)
	if err != nil {
		return err
	}
	bin, err := LookGit()
	if err != nil {
		return err
	}
	_ = gitCmd(bin, "--git-dir="+dir, "worktree", "prune").Run()

	tmp, err := os.MkdirTemp("", "xvpn-mr-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	wt := filepath.Join(tmp, "wt")

	add := gitCmd(bin, "--git-dir="+dir, "worktree", "add", wt, target)
	if out, err := add.CombinedOutput(); err != nil {
		return errors.New(strings.TrimSpace(string(out)))
	}
	defer func() {
		_ = gitCmd(bin, "--git-dir="+dir, "worktree", "remove", "--force", wt).Run()
		_ = gitCmd(bin, "--git-dir="+dir, "worktree", "prune").Run()
	}()

	msg := strings.TrimSpace(message)
	if msg == "" {
		msg = "Merge " + source + " into " + target
	}
	merge := gitCmd(bin, "-C", wt, "-c", "user.name=xgit", "-c", "user.email=xgit@corp.ihuull.com",
		"merge", "--no-ff", source, "-m", msg)
	if out, err := merge.CombinedOutput(); err != nil {
		_ = gitCmd(bin, "-C", wt, "merge", "--abort").Run()
		low := bytes.ToLower(out)
		if bytes.Contains(low, []byte("conflict")) {
			return ErrMergeConflict
		}
		return errors.New(strings.TrimSpace(string(out)))
	}
	return nil
}
