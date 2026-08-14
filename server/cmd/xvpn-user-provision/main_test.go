package main

import (
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/rootkit-lab/xvpn/server/internal/provision"
)

// run() é testável sem root porque runnerFn é injetável: em produção é
// provision.NewRunner (osRunner, chamadas reais); em teste injetamos
// um noopRunner que satisfaz a interface sem tocar o sistema. A
// orquestração (sequência de chamadas, idempotência, rollback) já está
// coberta no pacote provision com fakeRunner; aqui só validamos o
// parsing de argumentos, a leitura do stdin e a dispatch pro Runner.

// noopRunner satisfaz provision.Runner sem efeitos colaterais. Só
// registra WriteFile (path, content) pra o teste poder confirmar que
// a dispatch chegou ao Runner e que o stdin da chave pública foi
// repassado pra o authorized_keys.
type noopRunner struct {
	writes     map[string]string
	userExists map[string]bool
}

func newNoopRunner() *noopRunner {
	return &noopRunner{
		writes:     map[string]string{},
		userExists: map[string]bool{},
	}
}

func (n *noopRunner) UserExists(u string) (bool, error) { return n.userExists[u], nil }
func (n *noopRunner) AddSystemUser(username, homeDir string) error {
	n.userExists[username] = true
	return nil
}
func (n *noopRunner) LookupUIDGID(string) (int, int, error) { return 1000, 1000, nil }
func (n *noopRunner) MkdirAll(string, os.FileMode) error    { return nil }
func (n *noopRunner) Chown(string, int, int) error          { return nil }
func (n *noopRunner) Chmod(string, os.FileMode) error       { return nil }
func (n *noopRunner) WriteFile(path, content string, _ os.FileMode) error {
	n.writes[path] = content
	return nil
}
func (n *noopRunner) FileExists(path string) (bool, error) {
	_, ok := n.writes[path]
	return ok, nil
}
func (n *noopRunner) ReadDir(dir string) ([]string, error) {
	prefix := dir
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	var names []string
	for path := range n.writes {
		if strings.HasPrefix(path, prefix) {
			name := strings.TrimPrefix(path, prefix)
			if !strings.Contains(name, "/") {
				names = append(names, name)
			}
		}
	}
	return names, nil
}
func (n *noopRunner) RemoveFile(path string) error { delete(n.writes, path); return nil }
func (n *noopRunner) ReloadSSH() error             { return nil }
func (n *noopRunner) ReloadSamba() error           { return nil }

func TestRun_InvalidArgCount(t *testing.T) {
	for _, args := range [][]string{{}, {"create"}, {"create", "alice", "extra"}} {
		if err := run(args, nil, nil, nil); !errors.Is(err, errUsage) {
			t.Fatalf("args=%v: esperava errUsage, obtido: %v", args, err)
		}
	}
}

func TestRun_UnknownSubcommand(t *testing.T) {
	if err := run([]string{"frobnicate", "alice"}, nil, nil, nil); !errors.Is(err, errUsage) {
		t.Fatalf("esperava errUsage com subcomando desconhecido, obtido: %v", err)
	}
}

func TestRun_InvalidUsername(t *testing.T) {
	// Username inválido é rejeitado ANTES de tocar o Runner — defesa
	// em profundidade. Não precisamos injetar fake aqui.
	if err := run([]string{"create", "1invalid"}, nil, nil, nil); !errors.Is(err, provision.ErrInvalidUsername) {
		t.Fatalf("esperava ErrInvalidUsername, obtido: %v", err)
	}
}

func TestRun_CreateDispatches(t *testing.T) {
	r := newNoopRunner()
	runnerFn = func() provision.Runner { return r }
	defer func() { runnerFn = provision.NewRunner }()
	if err := run([]string{"create", "alice"}, nil, nil, nil); err != nil {
		t.Fatalf("create falhou: %v", err)
	}
	// Create escreve o drop-in sshd? Não — Create só cria dirs e
	// conta. Confirmamos que o usuário foi "criado" (registrado no
	// userExists do fake) e que pelo menos um dir foi "feito"
	// (MkdirAll é no-op, mas AddSystemUser marca o usuário).
	if !r.userExists["alice"] {
		t.Fatal("Create não chamou AddSystemUser")
	}
}

func TestRun_EnableSFTPReadsKeyFromStdin(t *testing.T) {
	r := newNoopRunner()
	runnerFn = func() provision.Runner { return r }
	defer func() { runnerFn = provision.NewRunner }()
	stdin := bytes.NewBufferString("ssh-ed25519 AAAA alice@host")
	if err := run([]string{"enable-sftp", "alice"}, stdin, nil, nil); err != nil {
		t.Fatalf("enable-sftp falhou: %v", err)
	}
	// A chave lida do stdin deve ter sido escrita no authorized_keys.
	ak := "/home/alice/.ssh/authorized_keys"
	content, ok := r.writes[ak]
	if !ok {
		t.Fatalf("authorized_keys não foi escrito. writes=%v", r.writes)
	}
	if !strings.Contains(content, "ssh-ed25519 AAAA alice@host") {
		t.Fatalf("authorized_keys não contém a chave do stdin: %q", content)
	}
	// E o drop-in sshd também foi escrito.
	dropIn := "/etc/ssh/sshd_config.d/xvpn-sftp-alice.conf"
	if _, ok := r.writes[dropIn]; !ok {
		t.Fatalf("drop-in sshd não foi escrito. writes=%v", r.writes)
	}
}

func TestRun_EnableSambaDispatches(t *testing.T) {
	r := newNoopRunner()
	runnerFn = func() provision.Runner { return r }
	defer func() { runnerFn = provision.NewRunner }()
	if err := run([]string{"enable-samba", "alice"}, strings.NewReader(""), nil, nil); err != nil {
		t.Fatalf("enable-samba falhou: %v", err)
	}
	share := "/etc/samba/smb.conf.d/xvpn-home-alice.conf"
	if _, ok := r.writes[share]; !ok {
		t.Fatalf("include samba não foi escrito. writes=%v", r.writes)
	}
}

func TestRun_DisableDispatches(t *testing.T) {
	r := newNoopRunner()
	// Pré-popula como se SFTP e Samba estivessem habilitados.
	r.writes["/etc/ssh/sshd_config.d/xvpn-sftp-alice.conf"] = "..."
	r.writes["/etc/samba/smb.conf.d/xvpn-home-alice.conf"] = "..."
	runnerFn = func() provision.Runner { return r }
	defer func() { runnerFn = provision.NewRunner }()
	if err := run([]string{"disable", "alice"}, strings.NewReader(""), nil, nil); err != nil {
		t.Fatalf("disable falhou: %v", err)
	}
	if _, ok := r.writes["/etc/ssh/sshd_config.d/xvpn-sftp-alice.conf"]; ok {
		t.Error("disable não removeu o drop-in sshd")
	}
	if _, ok := r.writes["/etc/samba/smb.conf.d/xvpn-home-alice.conf"]; ok {
		t.Error("disable não removeu o include samba")
	}
}

func TestRun_DisableSFTPAndDisableSambaDispatch(t *testing.T) {
	for _, cmd := range []string{"disable-sftp", "disable-samba"} {
		r := newNoopRunner()
		r.writes["/etc/ssh/sshd_config.d/xvpn-sftp-alice.conf"] = "..."
		r.writes["/etc/samba/smb.conf.d/xvpn-home-alice.conf"] = "..."
		runnerFn = func() provision.Runner { return r }
		defer func() { runnerFn = provision.NewRunner }()
		if err := run([]string{cmd, "alice"}, strings.NewReader(""), nil, nil); err != nil {
			t.Fatalf("%s falhou: %v", cmd, err)
		}
		// Ambos os subcomandos são dispatchados sem erro; a granularidade
		// (qual arquivo cada um remove) já é coberta no pacote provision.
	}
}
