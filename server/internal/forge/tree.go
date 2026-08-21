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
	Name       string      `json:"name"`
	Path       string      `json:"path"`
	Type       string      `json:"type"`
	Mode       string      `json:"mode"`
	Size       int64       `json:"size"`
	SHA        string      `json:"sha"`
	LastCommit *CommitInfo `json:"last_commit,omitempty"`
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
		if sha, err := RevParse(root, slug, ref); err == nil {
			return sha, nil
		}
		if sha, err := RevParseTag(root, slug, ref); err == nil {
			return sha, nil
		}
		return "", ErrBranchMissing
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
	if err := annotateLastCommits(bin, dir, sha, items); err != nil {
		return items, nil
	}
	return items, nil
}

func annotateLastCommits(bin, dir, sha string, items []TreeEntry) error {
	if len(items) == 0 {
		return nil
	}
	pending := make(map[int]struct{}, len(items))
	for i := range items {
		pending[i] = struct{}{}
	}
	cmd := gitCmd(bin, "--git-dir="+dir, "log", "-n", "200", "--name-only", "--pretty=format:%H%x09%s%x09%an%x09%aI", sha)
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	var cur *CommitInfo
	for _, line := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
		if line == "" {
			cur = nil
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) == 4 && len(parts[0]) == 40 && isHex(parts[0]) {
			cur = &CommitInfo{SHA: parts[0], Subject: parts[1], Author: parts[2], Date: parts[3]}
			continue
		}
		if cur == nil {
			continue
		}
		file := strings.TrimSpace(line)
		for i := range pending {
			ent := items[i]
			if ent.Type == "tree" {
				if file == ent.Path || strings.HasPrefix(file, ent.Path+"/") {
					cp := *cur
					items[i].LastCommit = &cp
					delete(pending, i)
				}
				continue
			}
			if file == ent.Path {
				cp := *cur
				items[i].LastCommit = &cp
				delete(pending, i)
			}
		}
		if len(pending) == 0 {
			break
		}
	}
	return nil
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

// CountCommits conta commits alcançáveis a partir de ref.
func CountCommits(root, slug, ref string) (int, error) {
	sha, err := resolveRev(root, slug, ref)
	if err != nil {
		return 0, err
	}
	dir, err := RepoPath(root, slug)
	if err != nil {
		return 0, err
	}
	bin, err := LookGit()
	if err != nil {
		return 0, err
	}
	cmd := gitCmd(bin, "--git-dir="+dir, "rev-list", "--count", sha)
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return 0, err
	}
	return n, nil
}

// ListTags lista refs/tags do bare.
func ListTags(root, slug string) ([]string, error) {
	if !Exists(root, slug) {
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
	cmd := gitCmd(bin, "--git-dir="+dir, "for-each-ref", "--format=%(refname:short)", "refs/tags/")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
	names := make([]string, 0, len(lines))
	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name == "" || strings.Contains(name, "..") {
			continue
		}
		names = append(names, name)
	}
	return names, nil
}

// LangStat é a fatia de uma linguagem no tree.
type LangStat struct {
	Name  string  `json:"name"`
	Bytes int64   `json:"bytes"`
	Pct   float64 `json:"pct"`
}

var langByExt = map[string]string{
	".go": "Go", ".ts": "TypeScript", ".tsx": "TypeScript", ".js": "JavaScript",
	".jsx": "JavaScript", ".mjs": "JavaScript", ".scss": "SCSS", ".css": "CSS",
	".sh": "Shell", ".bash": "Shell", ".md": "Markdown", ".yml": "YAML",
	".yaml": "YAML", ".json": "JSON", ".py": "Python", ".rs": "Rust",
	".html": "HTML", ".nsi": "NSIS", ".svelte": "Svelte",
}

// LanguageStats soma bytes por extensão no tree recursivo.
func LanguageStats(root, slug, ref string) ([]LangStat, error) {
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
	cmd := gitCmd(bin, "--git-dir="+dir, "ls-tree", "-r", "-l", sha)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	totals := map[string]int64{}
	var sum int64
	for _, line := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
		ent, ok := parseLsTree(line, "")
		if !ok || ent.Type != "blob" || ent.Size <= 0 {
			continue
		}
		dot := strings.LastIndex(ent.Name, ".")
		if dot < 0 {
			continue
		}
		lang, ok := langByExt[strings.ToLower(ent.Name[dot:])]
		if !ok {
			continue
		}
		totals[lang] += ent.Size
		sum += ent.Size
	}
	if sum == 0 {
		return []LangStat{}, nil
	}
	items := make([]LangStat, 0, len(totals))
	for name, n := range totals {
		items = append(items, LangStat{Name: name, Bytes: n, Pct: float64(n) * 100 / float64(sum)})
	}
	for i := 0; i < len(items); i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Bytes > items[i].Bytes {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
	if len(items) > 6 {
		items = items[:6]
	}
	return items, nil
}

// ListTreeFiles lista blobs recursivos em ref:dir.
func ListTreeFiles(root, slug, ref, dir string) ([]string, error) {
	if !validTreePath(dir) {
		return nil, ErrInvalidSlug
	}
	sha, err := resolveRev(root, slug, ref)
	if err != nil {
		return nil, err
	}
	repoDir, err := RepoPath(root, slug)
	if err != nil {
		return nil, err
	}
	bin, err := LookGit()
	if err != nil {
		return nil, err
	}
	spec := sha
	clean := strings.Trim(dir, "/")
	if clean != "" {
		spec = sha + ":" + clean
	}
	out, err := gitCmd(bin, "--git-dir="+repoDir, "ls-tree", "-r", "--name-only", spec).Output()
	if err != nil {
		return nil, ErrEmptyRepo
	}
	var files []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}
