package provision

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const defaultGitSSHUser = "git"

const gitHomeDir = "/home/git"

// gitLoginShell não pode ser /usr/sbin/nologin: no OpenSSH 9.x do Ubuntu o
// PAM recusa a sessão antes de aplicar command= do authorized_keys.
const gitLoginShell = "/bin/bash"

const xvpnServiceGroup = "xvpn"

// EnsureGitUser cria o usuário Unix `git` se ainda não existir.
func EnsureGitUser(r Runner) error {
	exists, err := r.UserExists(defaultGitSSHUser)
	if err != nil {
		return err
	}
	if exists {
		return ensureGitUserAccess(r)
	}
	if err := r.AddSystemUser(defaultGitSSHUser, gitHomeDir); err != nil {
		return fmt.Errorf("criando usuário git: %w", err)
	}
	uid, gid, err := r.LookupUIDGID(defaultGitSSHUser)
	if err != nil {
		return err
	}
	if err := r.Chown(gitHomeDir, uid, gid); err != nil {
		return fmt.Errorf("chown %q: %w", gitHomeDir, err)
	}
	sshDir := filepath.Join(gitHomeDir, ".ssh")
	if err := r.MkdirAll(sshDir, 0o700); err != nil {
		return err
	}
	if err := r.Chown(sshDir, uid, gid); err != nil {
		return err
	}
	return ensureGitUserAccess(r)
}

func ensureGitUserAccess(r Runner) error {
	if err := r.SetUserShell(defaultGitSSHUser, gitLoginShell); err != nil {
		return fmt.Errorf("ajustando shell do usuário git: %w", err)
	}
	if err := r.AddUserToGroup(defaultGitSSHUser, xvpnServiceGroup); err != nil {
		return fmt.Errorf("adicionando git ao grupo %s: %w", xvpnServiceGroup, err)
	}
	return nil
}

// ApplyGitSSHKeys reescreve o authorized_keys do usuário Unix `git` usado
// por git@xgit.corp. O conteúdo já vem formatado (com command= por linha).
func ApplyGitSSHKeys(r Runner, content string) error {
	if err := EnsureGitUser(r); err != nil {
		return err
	}
	path := gitSSHAuthorizedKeysPath()
	dir := filepath.Dir(path)
	if err := r.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("criando %q: %w", dir, err)
	}
	uid, gid, err := r.LookupUIDGID(defaultGitSSHUser)
	if err != nil {
		return fmt.Errorf("usuário %q não encontrado: %w", defaultGitSSHUser, err)
	}
	normalized := strings.TrimRight(content, "\n")
	if normalized != "" {
		normalized += "\n"
	}
	if err := r.WriteFile(path, normalized, 0o600); err != nil {
		return fmt.Errorf("gravando %q: %w", path, err)
	}
	if err := r.Chown(path, uid, gid); err != nil {
		return fmt.Errorf("chown %q: %w", path, err)
	}
	if err := r.Chown(dir, uid, gid); err != nil {
		return fmt.Errorf("chown %q: %w", dir, err)
	}
	if err := r.Chown(gitHomeDir, uid, gid); err != nil {
		return fmt.Errorf("chown %q: %w", gitHomeDir, err)
	}
	if err := r.ReloadSSH(); err != nil {
		return fmt.Errorf("recarregando sshd após git authorized_keys: %w", err)
	}
	return nil
}

func gitSSHAuthorizedKeysPath() string {
	if p := strings.TrimSpace(os.Getenv("XVPN_GIT_SSH_AUTHORIZED_KEYS")); p != "" {
		return p
	}
	return "/home/git/.ssh/authorized_keys"
}
