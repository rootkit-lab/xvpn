package forge

import (
	"path"
	"strings"
	"unicode"
)

// ValidBranchPattern aceita main, release/*, refs/heads/main — sem `..`.
func ValidBranchPattern(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" || len(s) > 80 {
		return false
	}
	if strings.Contains(s, "..") {
		return false
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '/' || r == '*' || r == '.' {
			continue
		}
		return false
	}
	return true
}

// MatchProtected compara um ref (refs/heads/main) com padrões do projeto.
// Padrão sem prefixo casa só branches; refs/ casa o nome completo.
func MatchProtected(patterns []string, ref string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return false
	}
	branch := strings.TrimPrefix(ref, "refs/heads/")
	isBranch := strings.HasPrefix(ref, "refs/heads/") || !strings.HasPrefix(ref, "refs/")
	for _, raw := range patterns {
		p := strings.TrimSpace(raw)
		if p == "" || !ValidBranchPattern(p) {
			continue
		}
		if strings.HasPrefix(p, "refs/") {
			if globOK(p, ref) {
				return true
			}
			continue
		}
		if isBranch && globOK(p, branch) {
			return true
		}
	}
	return false
}

func globOK(pattern, name string) bool {
	if pattern == name {
		return true
	}
	ok, err := path.Match(pattern, name)
	return err == nil && ok
}
