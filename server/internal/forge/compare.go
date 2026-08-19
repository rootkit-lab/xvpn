package forge

import (
	"fmt"
	"strconv"
	"strings"
)

const maxDiffBytes = 1 << 20

// CompareCommits lista commits em head que não estão em base (base..head).
func CompareCommits(root, slug, base, head string, n int) ([]CommitInfo, error) {
	if n <= 0 || n > 80 {
		n = 40
	}
	baseSHA, err := resolveRev(root, slug, base)
	if err != nil {
		return nil, err
	}
	headSHA, err := resolveRev(root, slug, head)
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
	cmd := gitCmd(bin, "--git-dir="+dir, "log", "-n", strconv.Itoa(n),
		"--format=%H%x09%s%x09%an%x09%aI", baseSHA+".."+headSHA)
	out, err := cmd.Output()
	if err != nil {
		return []CommitInfo{}, nil
	}
	items := make([]CommitInfo, 0)
	for _, line := range strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n") {
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

// DiffUnified devolve o diff unificado base...head (três pontos = merge-base).
func DiffUnified(root, slug, base, head string) (string, error) {
	baseSHA, err := resolveRev(root, slug, base)
	if err != nil {
		return "", err
	}
	headSHA, err := resolveRev(root, slug, head)
	if err != nil {
		return "", err
	}
	dir, err := RepoPath(root, slug)
	if err != nil {
		return "", err
	}
	bin, err := LookGit()
	if err != nil {
		return "", err
	}
	cmd := gitCmd(bin, "--git-dir="+dir, "diff", "--no-color", "--find-renames", baseSHA+"..."+headSHA)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("diff indisponível")
	}
	if len(out) > maxDiffBytes {
		return string(out[:maxDiffBytes]) + "\n… (diff truncado)\n", nil
	}
	return string(out), nil
}

// ArchiveZIP empacota a árvore de ref em um zip.
func ArchiveZIP(root, slug, ref string) ([]byte, error) {
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
	cmd := gitCmd(bin, "--git-dir="+dir, "archive", "--format=zip", sha)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("archive falhou")
	}
	if len(out) > 32<<20 {
		return nil, fmt.Errorf("archive grande demais")
	}
	return out, nil
}
