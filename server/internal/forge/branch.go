package forge

import (
	"strings"
)

// ValidBranchName aceita o nome de uma branch real — sem glob.
func ValidBranchName(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || strings.Contains(s, "*") {
		return false
	}
	return ValidBranchPattern(s)
}

// ListBranches lista refs/heads do bare. Repo inexistente → fatia vazia.
func ListBranches(root, slug string) ([]string, error) {
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
	cmd := gitCmd(bin, "--git-dir="+dir, "for-each-ref", "--format=%(refname:short)", "refs/heads/")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(string(out), "\r\n", "\n"), "\n")
	names := make([]string, 0, len(lines))
	for _, line := range lines {
		name := strings.TrimSpace(line)
		if name == "" || !ValidBranchName(name) {
			continue
		}
		names = append(names, name)
	}
	return names, nil
}

// BranchExists verifica refs/heads/<name> no bare.
func BranchExists(root, slug, name string) bool {
	if !ValidBranchName(name) || !Exists(root, slug) {
		return false
	}
	dir, err := RepoPath(root, slug)
	if err != nil {
		return false
	}
	bin, err := LookGit()
	if err != nil {
		return false
	}
	cmd := gitCmd(bin, "--git-dir="+dir, "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	return cmd.Run() == nil
}
