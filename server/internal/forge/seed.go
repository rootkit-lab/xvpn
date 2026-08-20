package forge

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// FileContent é um arquivo de texto a commitar no bare.
type FileContent struct {
	Path    string
	Content string
}

// CommitFilesOpts descreve um commit com um ou mais arquivos.
type CommitFilesOpts struct {
	Files       []FileContent
	Ref         string
	Message     string
	Description string
	NewBranch   string
	AuthorName  string
	AuthorEmail string
}

// HasCommits é true quando o bare já tem HEAD resolvível.
func HasCommits(root, slug string) bool {
	_, err := resolveRev(root, slug, "HEAD")
	return err == nil
}

// CommitFiles grava arquivos e commita. Funciona em bare vazio (primeiro
// commit em main) ou com histórico (worktree, como CommitFile).
func CommitFiles(root, slug string, opts CommitFilesOpts) (*CommitFileResult, error) {
	files, err := normalizeFiles(opts.Files)
	if err != nil {
		return nil, err
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
	if !HasCommits(root, slug) {
		return commitIntoEmpty(root, slug, files, msg, opts.AuthorName, opts.AuthorEmail)
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

	tmp, err := os.MkdirTemp("", "xvpn-seed-")
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
		} else if base == "HEAD" {
			branch = "main"
			if !ValidBranchName(branch) {
				return nil, ErrInvalidBranch
			}
		} else {
			return nil, ErrInvalidBranch
		}
	}
	co := gitCmd(bin, "-C", wt, "checkout", "-B", branch)
	if out, err := co.CombinedOutput(); err != nil {
		return nil, errors.New(strings.TrimSpace(string(out)))
	}
	if err := writeFiles(wt, files); err != nil {
		return nil, err
	}
	if out, err := gitCmd(bin, "-C", wt, "add", "-A").CombinedOutput(); err != nil {
		return nil, errors.New(strings.TrimSpace(string(out)))
	}
	if err := gitCmd(bin, "-C", wt, "diff", "--cached", "--quiet").Run(); err == nil {
		return nil, ErrUnchanged
	}
	return runCommit(bin, wt, branch, msg, opts.AuthorName, opts.AuthorEmail)
}

func commitIntoEmpty(root, slug string, files []FileContent, msg, author, email string) (*CommitFileResult, error) {
	dir, err := RepoPath(root, slug)
	if err != nil {
		return nil, err
	}
	bin, err := LookGit()
	if err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp("", "xvpn-init-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	if err := writeFiles(tmp, files); err != nil {
		return nil, err
	}
	if out, err := gitCmd(bin, "-C", tmp, "init", "--initial-branch=main").CombinedOutput(); err != nil {
		return nil, errors.New(strings.TrimSpace(string(out)))
	}
	if out, err := gitCmd(bin, "-C", tmp, "add", "-A").CombinedOutput(); err != nil {
		return nil, errors.New(strings.TrimSpace(string(out)))
	}
	if err := gitCmd(bin, "-C", tmp, "diff", "--cached", "--quiet").Run(); err == nil {
		return nil, ErrUnchanged
	}
	res, err := runCommit(bin, tmp, "main", msg, author, email)
	if err != nil {
		return nil, err
	}
	fetch := gitCmd(bin, "--git-dir="+dir, "fetch", tmp, "+refs/heads/main:refs/heads/main")
	if out, err := fetch.CombinedOutput(); err != nil {
		return nil, errors.New(strings.TrimSpace(string(out)))
	}
	_ = gitCmd(bin, "--git-dir="+dir, "symbolic-ref", "HEAD", "refs/heads/main").Run()
	return res, nil
}

func runCommit(bin, work, branch, msg, author, email string) (*CommitFileResult, error) {
	name := strings.TrimSpace(author)
	mail := strings.TrimSpace(email)
	if name == "" {
		name = "xgit"
	}
	if mail == "" {
		mail = "xgit@corp.ihuull.com"
	}
	commit := gitCmd(bin, "-C", work,
		"-c", "user.name="+name,
		"-c", "user.email="+mail,
		"commit", "-m", msg)
	if out, err := commit.CombinedOutput(); err != nil {
		return nil, errors.New(strings.TrimSpace(string(out)))
	}
	shaOut, err := gitCmd(bin, "-C", work, "rev-parse", "HEAD").Output()
	if err != nil {
		return nil, err
	}
	if branch == "" {
		branch = "main"
	}
	return &CommitFileResult{SHA: strings.TrimSpace(string(shaOut)), Branch: branch}, nil
}

func writeFiles(root string, files []FileContent) error {
	for _, f := range files {
		abs := filepath.Join(root, filepath.FromSlash(f.Path))
		rel, err := filepath.Rel(root, abs)
		if err != nil || strings.HasPrefix(rel, "..") {
			return ErrInvalidSlug
		}
		if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(abs, []byte(f.Content), 0o644); err != nil {
			return err
		}
	}
	return nil
}

func normalizeFiles(in []FileContent) ([]FileContent, error) {
	if len(in) == 0 {
		return nil, ErrUnchanged
	}
	out := make([]FileContent, 0, len(in))
	seen := map[string]struct{}{}
	for _, f := range in {
		path := strings.Trim(strings.ReplaceAll(f.Path, "\\", "/"), "/")
		if !validTreePath(path) || path == "" {
			return nil, ErrInvalidSlug
		}
		if len(f.Content) > MaxCommitBytes {
			return nil, ErrContentHuge
		}
		if !utf8.ValidString(f.Content) || strings.Contains(f.Content, "\x00") {
			return nil, ErrBinaryEdit
		}
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, FileContent{Path: path, Content: f.Content})
	}
	return out, nil
}
