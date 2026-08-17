package forge

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

const maxBlobBytes = 1 << 20

// TreeEntry é um arquivo ou pasta no ls-tree.
type TreeEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Type string `json:"type"`
	Mode string `json:"mode"`
	Size int64  `json:"size"`
	SHA  string `json:"sha"`
}

// CommitInfo é uma linha do git log.
type CommitInfo struct {
	SHA     string `json:"sha"`
	Subject string `json:"subject"`
	Author  string `json:"author"`
	Date    string `json:"date"`
}

func validTreePath(p string) bool {
	p = strings.Trim(strings.ReplaceAll(p, "\\", "/"), "/")
	if p == "" {
		return true
	}
	if strings.Contains(p, "..") {
		return false
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	return true
}

func resolveRev(root, slug, ref string) (string, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		ref = "HEAD"
	}
	if ref == "HEAD" {
		return RevParseHEAD(root, slug)
	}
	if ValidBranchName(ref) {
		return RevParse(root, slug, ref)
	}
	if len(ref) >= 7 && len(ref) <= 40 && isHex(ref) {
		return RevParseRaw(root, slug, ref)
	}
	return "", ErrInvalidBranch
}

func isHex(s string) bool {
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') {
			return false
		}
	}
	return true
}

// RevParseHEAD resolve HEAD do bare.
func RevParseHEAD(root, slug string) (string, error) {
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
	cmd := gitCmd(bin, "--git-dir="+dir, "rev-parse", "--verify", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "", ErrEmptyRepo
	}
	return strings.TrimSpace(string(out)), nil
}

// RevParseRaw aceita um OID parcial/completo.
func RevParseRaw(root, slug, rev string) (string, error) {
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
	cmd := gitCmd(bin, "--git-dir="+dir, "rev-parse", "--verify", rev+"^{commit}")
	out, err := cmd.Output()
	if err != nil {
		return "", ErrBranchMissing
	}
	return strings.TrimSpace(string(out)), nil
}

// ListTree lista o diretório em ref:path.
func ListTree(root, slug, ref, path string) ([]TreeEntry, error) {
	if !validTreePath(path) {
		return nil, ErrInvalidSlug
	}
	sha, err := resolveRev(root, slug, ref)
	if err != nil {
		return nil, err
	}
	dir, err := RepoPath(root, slug)
	if err != nil {
		return nil, err
	}
	bin, err := LookGit()
	if err != nil {
		return nil, err
	}
	spec := sha
	clean := strings.Trim(path, "/")
	if clean != "" {
		spec = sha + ":" + clean
	}
	cmd := gitCmd(bin, "--git-dir="+dir, "ls-tree", "-l", spec)
	out, err := cmd.Output()
	if err != nil {
		return nil, ErrEmptyRepo
	}
	lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
	items := make([]TreeEntry, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimRight(line, "\n")
		if line == "" {
			continue
		}
		ent, ok := parseLsTree(line, clean)
		if ok {
			items = append(items, ent)
		}
	}
	return items, nil
}

func parseLsTree(line, parent string) (TreeEntry, bool) {
	// 100644 blob <sha> <size>\tname
	tab := strings.IndexByte(line, '\t')
	if tab < 0 {
		return TreeEntry{}, false
	}
	meta, name := line[:tab], line[tab+1:]
	fields := strings.Fields(meta)
	if len(fields) < 3 {
		return TreeEntry{}, false
	}
	kind := fields[1]
	if kind != "blob" && kind != "tree" {
		return TreeEntry{}, false
	}
	var size int64
	if len(fields) >= 4 && fields[3] != "-" {
		size, _ = strconv.ParseInt(fields[3], 10, 64)
	}
	path := name
	if parent != "" {
		path = parent + "/" + name
	}
	return TreeEntry{
		Name: name, Path: path, Type: kind, Mode: fields[0],
		Size: size, SHA: fields[2],
	}, true
}

// ReadBlob devolve o conteúdo de um arquivo (texto; binário vira flag).
func ReadBlob(root, slug, ref, path string) (content string, binary bool, err error) {
	if !validTreePath(path) || strings.Trim(path, "/") == "" {
		return "", false, ErrInvalidSlug
	}
	sha, err := resolveRev(root, slug, ref)
	if err != nil {
		return "", false, err
	}
	dir, err := RepoPath(root, slug)
	if err != nil {
		return "", false, err
	}
	bin, err := LookGit()
	if err != nil {
		return "", false, err
	}
	cmd := gitCmd(bin, "--git-dir="+dir, "cat-file", "-s", sha+":"+strings.Trim(path, "/"))
	szOut, err := cmd.Output()
	if err != nil {
		return "", false, fmt.Errorf("arquivo não encontrado")
	}
	sz, _ := strconv.ParseInt(strings.TrimSpace(string(szOut)), 10, 64)
	if sz > maxBlobBytes {
		return "", true, nil
	}
	show := gitCmd(bin, "--git-dir="+dir, "show", sha+":"+strings.Trim(path, "/"))
	raw, err := show.Output()
	if err != nil {
		return "", false, fmt.Errorf("arquivo não encontrado")
	}
	if !utf8.Valid(raw) || strings.Contains(string(raw), "\x00") {
		return "", true, nil
	}
	return string(raw), false, nil
}

// ListCommits lista até n commits de ref (opcionalmente num path).
func ListCommits(root, slug, ref, path string, n int) ([]CommitInfo, error) {
	if n <= 0 || n > 50 {
		n = 20
	}
	if path != "" && !validTreePath(path) {
		return nil, ErrInvalidSlug
	}
	sha, err := resolveRev(root, slug, ref)
	if err != nil {
		return nil, err
	}
	dir, err := RepoPath(root, slug)
	if err != nil {
		return nil, err
	}
	bin, err := LookGit()
	if err != nil {
		return nil, err
	}
	args := []string{"--git-dir=" + dir, "log", "-n", strconv.Itoa(n), "--format=%H%x09%s%x09%an%x09%aI", sha}
	if strings.Trim(path, "/") != "" {
		args = append(args, "--", strings.Trim(path, "/"))
	}
	cmd := gitCmd(bin, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, ErrEmptyRepo
	}
	lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
	items := make([]CommitInfo, 0, len(lines))
	for _, line := range lines {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) < 4 {
			continue
		}
		items = append(items, CommitInfo{SHA: parts[0], Subject: parts[1], Author: parts[2], Date: parts[3]})
	}
	return items, nil
}
