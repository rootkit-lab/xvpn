package forge

import (
	"strings"
)

var privateKeyNeedles = []string{
	"BEGIN RSA PRIVATE KEY",
	"BEGIN OPENSSH PRIVATE KEY",
	"BEGIN EC PRIVATE KEY",
	"BEGIN DSA PRIVATE KEY",
	"BEGIN PRIVATE KEY",
}

// RevHasPrivateKey procura PEM de chave privada na tree do rev (descomprimida).
func RevHasPrivateKey(root, slug, rev string) bool {
	if !Exists(root, slug) || strings.TrimSpace(rev) == "" || IsZeroOID(rev) {
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
	args := []string{"--git-dir=" + dir, "grep", "-I", "-l"}
	for _, n := range privateKeyNeedles {
		args = append(args, "-e", n)
	}
	args = append(args, rev)
	out, err := gitCmd(bin, args...).Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

// ResetRef volta um ref ao SHA anterior (ou apaga se zero).
func ResetRef(root, slug, ref, sha string) error {
	dir, err := RepoPath(root, slug)
	if err != nil {
		return err
	}
	bin, err := LookGit()
	if err != nil {
		return err
	}
	if IsZeroOID(sha) || strings.TrimSpace(sha) == "" {
		return gitCmd(bin, "--git-dir="+dir, "update-ref", "-d", ref).Run()
	}
	return gitCmd(bin, "--git-dir="+dir, "update-ref", ref, sha).Run()
}
