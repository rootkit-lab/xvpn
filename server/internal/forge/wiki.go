package forge

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"
)

// WikiRef é a tree markdown first-class do repo (Fase 61). Não é branch.
const WikiRef = "refs/xgit/wiki"

// WikiPageFile normaliza Home.md (página #1) e recusa path traversal.
func WikiPageFile(page string) (string, error) {
	page = strings.TrimSpace(page)
	page = strings.TrimSuffix(page, ".md")
	switch page {
	case "", "1", "#1", "home", "Home":
		page = "Home"
	}
	if len(page) > 80 || strings.Contains(page, "..") || strings.Contains(page, "/") {
		return "", ErrInvalidSlug
	}
	for _, r := range page {
		ok := (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.'
		if !ok {
			return "", ErrInvalidSlug
		}
	}
	return page + ".md", nil
}

// WikiPageTitle é o nome sem .md.
func WikiPageTitle(file string) string {
	return strings.TrimSuffix(file, ".md")
}

func wikiRev(root, slug string) (string, error) {
	dir, err := RepoPath(root, slug)
	if err != nil {
		return "", err
	}
	bin, err := LookGit()
	if err != nil {
		return "", err
	}
	out, err := gitCmd(bin, "--git-dir="+dir, "rev-parse", "--verify", WikiRef).Output()
	if err != nil {
		return "", ErrEmptyRepo
	}
	return strings.TrimSpace(string(out)), nil
}

// ListWiki devolve os títulos (Home primeiro).
func ListWiki(root, slug string) ([]string, error) {
	if !Exists(root, slug) {
		return nil, ErrEmptyRepo
	}
	if _, err := wikiRev(root, slug); err != nil {
		return []string{}, nil
	}
	dir, err := RepoPath(root, slug)
	if err != nil {
		return nil, err
	}
	bin, err := LookGit()
	if err != nil {
		return nil, err
	}
	out, err := gitCmd(bin, "--git-dir="+dir, "ls-tree", "--name-only", WikiRef).Output()
	if err != nil {
		return []string{}, nil
	}
	var pages []string
	hasHome := false
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasSuffix(line, ".md") {
			continue
		}
		title := WikiPageTitle(line)
		if title == "Home" {
			hasHome = true
			continue
		}
		pages = append(pages, title)
	}
	sort.Strings(pages)
	if hasHome {
		pages = append([]string{"Home"}, pages...)
	}
	return pages, nil
}

// ReadWiki lê uma página da ref wiki.
func ReadWiki(root, slug, page string) (string, error) {
	file, err := WikiPageFile(page)
	if err != nil {
		return "", err
	}
	if !Exists(root, slug) {
		return "", ErrEmptyRepo
	}
	dir, err := RepoPath(root, slug)
	if err != nil {
		return "", err
	}
	bin, err := LookGit()
	if err != nil {
		return "", err
	}
	out, err := gitCmd(bin, "--git-dir="+dir, "show", WikiRef+":"+file).Output()
	if err != nil {
		return "", ErrEmptyRepo
	}
	return string(out), nil
}

// WriteWiki grava markdown em refs/xgit/wiki (plumbing; sem branch).
func WriteWiki(root, slug, page, content, msg, author, email string) (*CommitFileResult, error) {
	file, err := WikiPageFile(page)
	if err != nil {
		return nil, err
	}
	if len(content) > MaxCommitBytes {
		return nil, ErrContentHuge
	}
	if !utf8.ValidString(content) || strings.Contains(content, "\x00") {
		return nil, ErrBinaryEdit
	}
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return nil, ErrEmptyMessage
	}
	if !Exists(root, slug) {
		return nil, ErrEmptyRepo
	}
	dir, err := RepoPath(root, slug)
	if err != nil {
		return nil, err
	}
	bin, err := LookGit()
	if err != nil {
		return nil, err
	}

	hash := gitCmd(bin, "--git-dir="+dir, "hash-object", "-w", "--stdin")
	hash.Stdin = strings.NewReader(content)
	blobOut, err := hash.Output()
	if err != nil {
		return nil, err
	}
	blob := strings.TrimSpace(string(blobOut))

	entries := map[string]string{}
	if _, err := wikiRev(root, slug); err == nil {
		ls, err := gitCmd(bin, "--git-dir="+dir, "ls-tree", WikiRef).Output()
		if err != nil {
			return nil, err
		}
		for _, line := range strings.Split(string(ls), "\n") {
			tab := strings.IndexByte(line, '\t')
			if tab < 0 {
				continue
			}
			meta, name := line[:tab], line[tab+1:]
			fields := strings.Fields(meta)
			if len(fields) >= 3 && fields[1] == "blob" {
				entries[name] = fields[2]
			}
		}
	}
	if old, ok := entries[file]; ok && old == blob {
		return nil, ErrUnchanged
	}
	entries[file] = blob

	names := make([]string, 0, len(entries))
	for n := range entries {
		names = append(names, n)
	}
	sort.Strings(names)
	var treeBuf bytes.Buffer
	for _, n := range names {
		fmt.Fprintf(&treeBuf, "100644 blob %s\t%s\n", entries[n], n)
	}
	mktree := gitCmd(bin, "--git-dir="+dir, "mktree")
	mktree.Stdin = &treeBuf
	treeOut, err := mktree.Output()
	if err != nil {
		return nil, err
	}
	tree := strings.TrimSpace(string(treeOut))

	name := strings.TrimSpace(author)
	mail := strings.TrimSpace(email)
	if name == "" {
		name = "xgit"
	}
	if mail == "" {
		mail = "xgit@corp.ihuull.com"
	}
	args := []string{"--git-dir=" + dir, "commit-tree", tree, "-m", msg}
	if parent, err := wikiRev(root, slug); err == nil {
		args = append(args, "-p", parent)
	}
	commit := gitCmd(bin, args...)
	commit.Env = append(commit.Env,
		"GIT_AUTHOR_NAME="+name,
		"GIT_AUTHOR_EMAIL="+mail,
		"GIT_COMMITTER_NAME="+name,
		"GIT_COMMITTER_EMAIL="+mail,
	)
	shaOut, err := commit.Output()
	if err != nil {
		return nil, err
	}
	sha := strings.TrimSpace(string(shaOut))
	if out, err := gitCmd(bin, "--git-dir="+dir, "update-ref", WikiRef, sha).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return &CommitFileResult{SHA: sha, Branch: "wiki"}, nil
}
