package forge

import (
	"strings"
)

// IsZeroOID é o SHA de delete no receive-pack.
func IsZeroOID(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 40 {
		return false
	}
	for _, c := range s {
		if c != '0' {
			return false
		}
	}
	return true
}

// RevParse devolve o OID de refs/heads/<name> (ou o ref completo).
func RevParse(root, slug, ref string) (string, error) {
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
	name := strings.TrimSpace(ref)
	if !strings.HasPrefix(name, "refs/") {
		if !ValidBranchName(name) {
			return "", ErrInvalidBranch
		}
		name = "refs/heads/" + name
	}
	cmd := gitCmd(bin, "--git-dir="+dir, "rev-parse", "--verify", name)
	out, err := cmd.Output()
	if err != nil {
		return "", ErrBranchMissing
	}
	return strings.TrimSpace(string(out)), nil
}

// RevParseTag resolve refs/tags/<name> até o commit.
func RevParseTag(root, slug, name string) (string, error) {
	if !Exists(root, slug) || !ValidBranchName(name) {
		return "", ErrInvalidBranch
	}
	dir, err := RepoPath(root, slug)
	if err != nil {
		return "", err
	}
	bin, err := LookGit()
	if err != nil {
		return "", err
	}
	cmd := gitCmd(bin, "--git-dir="+dir, "rev-parse", "--verify", "refs/tags/"+name+"^{commit}")
	out, err := cmd.Output()
	if err != nil {
		return "", ErrBranchMissing
	}
	return strings.TrimSpace(string(out)), nil
}
