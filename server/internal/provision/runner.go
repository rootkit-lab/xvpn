package provision

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// Runner isola as chamadas de sistema que o provisionador faz (useradd,
// mkdir, chown, write, systemctl reload), para que a orquestração em
// ops.go seja testável sem root — em produção usa-se osRunner (chamadas
// reais de os/exec e os); em teste, fakeRunner registra as chamadas em
// memória (ver provision_test.go).
type Runner interface {
	UserExists(username string) (bool, error)
	AddSystemUser(username, homeDir string) error
	LookupUIDGID(username string) (uid, gid int, err error)
	MkdirAll(path string, perm os.FileMode) error
	Chown(path string, uid, gid int) error
	Chmod(path string, perm os.FileMode) error
	WriteFile(path, content string, perm os.FileMode) error
	FileExists(path string) (bool, error)
	RemoveFile(path string) error
	ReloadSSH() error
	ReloadSamba() error
}

// osRunner é a implementação de produção de Runner: chama useradd via
// os/exec (nunca sh -c com concatenação de string — ver go-backend.mdc
// e PLAN.md §6.9), escreve arquivos com os.WriteFile, e recarrega
// serviços via systemctl. Roda só como root (o binário
// xvpn-user-provision é invocado via sudoers.d restrito).
type osRunner struct{}

// NewRunner devolve o Runner de produção.
func NewRunner() Runner { return osRunner{} }

func (osRunner) UserExists(username string) (bool, error) {
	// getent passwd <username> sai 0 se existe, 2 se não existe. Sem
	// shell-out pra sh -c: caminho direto do binário, argumento isolado.
	cmd := exec.Command("getent", "passwd", username)
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 2 {
			return false, nil
		}
		return false, fmt.Errorf("verificando usuário %q: %w", username, err)
	}
	return true, nil
}

func (osRunner) AddSystemUser(username, homeDir string) error {
	// --no-create-home: a raiz /home/<username> precisa ser root:root
	// 0755 (exigência do ChrootDirectory do sshd), então criamos e
	// ajustamos o dono/permissão nós mesmos em Create(), não o useradd.
	// --shell /usr/sbin/nologin: sem shell de login (o SFTP usa
	// ForceCommand internal-sftp, que ignora o shell; para qualquer
	// outro acesso SSH o nologin bloqueia).
	cmd := exec.Command("useradd",
		"--no-create-home",
		"--home-dir", homeDir,
		"--shell", "/usr/sbin/nologin",
		username,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("criando usuário %q: %w: %s", username, err, string(out))
	}
	return nil
}

func (osRunner) LookupUIDGID(username string) (int, int, error) {
	// getent passwd <user> devolve "user:x:uid:gid:...". Parse só os
	// campos que importam (uid=3, gid=4).
	out, err := exec.Command("getent", "passwd", username).Output()
	if err != nil {
		return 0, 0, fmt.Errorf("lendo uid/gid de %q: %w", username, err)
	}
	fields := splitFields(string(out))
	if len(fields) < 4 {
		return 0, 0, fmt.Errorf("saída inesperada de getent para %q: %q", username, string(out))
	}
	uid, err := strconv.Atoi(fields[2])
	if err != nil {
		return 0, 0, fmt.Errorf("parseando uid %q: %w", fields[2], err)
	}
	gid, err := strconv.Atoi(fields[3])
	if err != nil {
		return 0, 0, fmt.Errorf("parseando gid %q: %w", fields[3], err)
	}
	return uid, gid, nil
}

// splitFields quebra uma linha de getent passwd por ':' — separada de
// strings.Split pra evitar importar strings só por isso aqui (mantém
// o arquivo focado).
func splitFields(s string) []string {
	var fields []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ':' || s[i] == '\n' {
			fields = append(fields, s[start:i])
			start = i + 1
			if s[i] == '\n' {
				break
			}
		}
	}
	if start < len(s) {
		fields = append(fields, s[start:])
	}
	return fields
}

func (osRunner) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (osRunner) Chown(path string, uid, gid int) error {
	return os.Chown(path, uid, gid)
}

func (osRunner) Chmod(path string, perm os.FileMode) error {
	return os.Chmod(path, perm)
}

func (osRunner) WriteFile(path, content string, perm os.FileMode) error {
	return os.WriteFile(path, []byte(content), perm)
}

func (osRunner) FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (osRunner) RemoveFile(path string) error {
	return os.Remove(path)
}

func (osRunner) ReloadSSH() error {
	// sshd -t valida a config ANTES do reload — se o fragmento novo
	// tiver um erro de sintaxe, o reload falharia e derrubaria sessões
	// existentes; com -t, o erro é pego antes e o reload nem roda.
	if out, err := exec.Command("sshd", "-t").CombinedOutput(); err != nil {
		return fmt.Errorf("sshd -t rejeitou a config: %w: %s", err, string(out))
	}
	if err := exec.Command("systemctl", "reload", "ssh").Run(); err != nil {
		return fmt.Errorf("recarregando ssh: %w", err)
	}
	return nil
}

func (osRunner) ReloadSamba() error {
	// testparm valida o smb.conf inteiro (incluindo o include novo)
	// antes do reload — mesmo princípio do sshd -t.
	if out, err := exec.Command("testparm", "-s", "--suppress-prompt").CombinedOutput(); err != nil {
		return fmt.Errorf("testparm rejeitou a config: %w: %s", err, string(out))
	}
	if err := exec.Command("systemctl", "reload", "smbd").Run(); err != nil {
		// smbd reload pode não existir em todas as versões; fallback
		// para restart (mais caro, mas garante que a config pegue).
		if err2 := exec.Command("systemctl", "restart", "smbd").Run(); err2 != nil {
			return fmt.Errorf("recarregando smbd: %w (restart também falhou: %v)", err, err2)
		}
	}
	return nil
}
