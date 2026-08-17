package forge

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const MaxCommitBytes = 2 << 20

var (
	ErrEmptyMessage = errors.New("mensagem de commit obrigatória")
	ErrUnchanged    = errors.New("sem alterações")
	ErrContentHuge  = errors.New("arquivo grande demais")
	ErrBinaryEdit   = errors.New("arquivo binário não pode ser editado")
)

// CommitFileOpts descreve um commit de um arquivo no bare (worktree).
type CommitFileOpts struct {
	Path        string
	Ref         string
	Content     string
	Message     string
	Description string
	NewBranch   string
	AuthorName  string
	AuthorEmail string
}

// CommitFileResult é o commit criado.
type CommitFileResult struct {
	SHA    string
	Branch string
}

// CommitFile grava um arquivo e commita no bare via worktree temporário.
func CommitFile(root, slug string, opts CommitFileOpts) (*CommitFileResult, error) {
	path := strings.Trim(strings.ReplaceAll(opts.Path, "\\", "/"), "/")
	if !validTreePath(path) || path == "" {
		return nil, ErrInvalidSlug
	}
	if len(opts.Content) > MaxCommitBytes {
		return nil, ErrContentHuge
	}
	if !utf8.ValidString(opts.Content) || strings.Contains(opts.Content, "\x00") {
		return nil, ErrBinaryEdit
	}
	msg := strings.TrimSpace(opts.Message)
	if msg == "" {
		return nil, ErrEmptyMessage
	}
	if desc := strings.TrimSpace(opts.Description); desc != "" {
		msg = msg + "\n\n" + desc
	}
	if !Exists(root, slug) {
		return nil, ErrEmptyRepo
	}
	base := strings.TrimSpace(opts.Ref)
	if base == "" {
		base = "HEAD"
	}
	if _, err := resolveRev(root, slug, base); err != nil {
		return nil, err
	}
	newBranch := strings.TrimSpace(opts.NewBranch)
	if newBranch != "" && !ValidBranchName(newBranch) {
		return nil, ErrInvalidBranch
	}
	dir, err := RepoPath(root, slug)
	if err != nil {
		return nil, err
	}
	bin, err := LookGit()
	if err != nil {
		return nil, err
	}
	_ = gitCmd(bin, "--git-dir="+dir, "worktree", "prune").Run()

	tmp, err := os.MkdirTemp("", "xvpn-edit-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	wt := filepath.Join(tmp, "wt")

	add := gitCmd(bin, "--git-dir="+dir, "worktree", "add", "--detach", wt, base)
	if out, err := add.CombinedOutput(); err != nil {
		return nil, errors.New(strings.TrimSpace(string(out)))
	}
	defer func() {
		_ = gitCmd(bin, "--git-dir="+dir, "worktree", "remove", "--force", wt).Run()
		_ = gitCmd(bin, "--git-dir="+dir, "worktree", "prune").Run()
	}()

	branch := newBranch
	if branch == "" {
		if ValidBranchName(base) && BranchExists(root, slug, base) {
			branch = base
		} else {
			return nil, ErrInvalidBranch
		}
	}
	co := gitCmd(bin, "-C", wt, "checkout", "-B", branch)
	if out, err := co.CombinedOutput(); err != nil {
		return nil, errors.New(strings.TrimSpace(string(out)))
	}

	abs := filepath.Join(wt, filepath.FromSlash(path))
	rel, err := filepath.Rel(wt, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return nil, ErrInvalidSlug
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
		return nil, err
	}
	if err := os.WriteFile(abs, []byte(opts.Content), 0o644); err != nil {
		return nil, err
	}
	if out, err := gitCmd(bin, "-C", wt, "add", "--", path).CombinedOutput(); err != nil {
		return nil, errors.New(strings.TrimSpace(string(out)))
	}
	if err := gitCmd(bin, "-C", wt, "diff", "--cached", "--quiet").Run(); err == nil {
		return nil, ErrUnchanged
	}

	name := strings.TrimSpace(opts.AuthorName)
	email := strings.TrimSpace(opts.AuthorEmail)
	if name == "" {
		name = "xgit"
	}
	if email == "" {
		email = "xgit@corp.ihuull.com"
	}
	commit := gitCmd(bin, "-C", wt,
		"-c", "user.name="+name,
		"-c", "user.email="+email,
		"commit", "-m", msg)
	if out, err := commit.CombinedOutput(); err != nil {
		return nil, errors.New(strings.TrimSpace(string(out)))
	}
	shaOut, err := gitCmd(bin, "-C", wt, "rev-parse", "HEAD").Output()
	if err != nil {
		return nil, err
	}
	return &CommitFileResult{SHA: strings.TrimSpace(string(shaOut)), Branch: branch}, nil
}
