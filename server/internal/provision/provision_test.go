package provision

import (
	"errors"
	"os"
	"strings"
	"testing"
)

// fakeRunner registra todas as chamadas feitas a ele em ordem, num
// slice, para que o teste possa afirmar a sequência exata de operações
// (ex.: "Create chamou MkdirAll(home) depois Chown(home,0,0) depois
// Chmod(home,0755)…"). Os "arquivos" escritos ficam num map path→content
// pra o teste poder conferir o que foi gravado.
type fakeRunner struct {
	calls     []string
	files     map[string]string
	filePerms map[string]os.FileMode
	users     map[string]bool // usernames que "existem" no sistema
	// failOn, se setado, faz a chamada cujo nome casa devolver errFake
	// — para testar caminhos de erro/rollback.
	failOn  string
	errFake error
}

func newFakeRunner() *fakeRunner {
	return &fakeRunner{
		files:     map[string]string{},
		filePerms: map[string]os.FileMode{},
		users:     map[string]bool{},
		errFake:   errors.New("fake runner error"),
	}
}

func (f *fakeRunner) record(call string) error {
	if f.failOn != "" && strings.Contains(call, f.failOn) {
		return f.errFake
	}
	f.calls = append(f.calls, call)
	return nil
}

func (f *fakeRunner) UserExists(username string) (bool, error) {
	return f.users[username], f.record("UserExists(" + username + ")")
}

func (f *fakeRunner) AddSystemUser(username, homeDir string) error {
	f.users[username] = true
	return f.record("AddSystemUser(" + username + "," + homeDir + ")")
}

func (f *fakeRunner) LookupUIDGID(username string) (int, int, error) {
	// UID/GID determinísticos por teste: 1000+hash do nome, só pra
	// distinguir usuários diferentes. Não precisa ser real.
	uid := 1000 + len(username)
	return uid, uid, f.record("LookupUIDGID(" + username + ")")
}

func (f *fakeRunner) MkdirAll(path string, perm os.FileMode) error {
	return f.record("MkdirAll(" + path + "," + perm.String() + ")")
}

func (f *fakeRunner) Chown(path string, uid, gid int) error {
	return f.record("Chown(" + path + "," + itoa(uid) + "," + itoa(gid) + ")")
}

func (f *fakeRunner) Chmod(path string, perm os.FileMode) error {
	return f.record("Chmod(" + path + "," + perm.String() + ")")
}

func (f *fakeRunner) WriteFile(path, content string, perm os.FileMode) error {
	if err := f.record("WriteFile(" + path + "," + perm.String() + ")"); err != nil {
		return err
	}
	f.files[path] = content
	f.filePerms[path] = perm
	return nil
}

func (f *fakeRunner) FileExists(path string) (bool, error) {
	_, ok := f.files[path]
	return ok, f.record("FileExists(" + path + ")")
}

// ReadDir devolve os nomes dos arquivos "presentes" no diretório
// (baseado no map f.files — extrai o basename das chaves cujo dir
// casa). Suficiente pra regenerateSambaAggregate nos testes.
func (f *fakeRunner) ReadDir(dir string) ([]string, error) {
	if err := f.record("ReadDir(" + dir + ")"); err != nil {
		return nil, err
	}
	prefix := dir
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	var names []string
	for path := range f.files {
		if strings.HasPrefix(path, prefix) {
			name := strings.TrimPrefix(path, prefix)
			if !strings.Contains(name, "/") {
				names = append(names, name)
			}
		}
	}
	return names, nil
}

func (f *fakeRunner) RemoveFile(path string) error {
	if err := f.record("RemoveFile(" + path + ")"); err != nil {
		return err
	}
	delete(f.files, path)
	delete(f.filePerms, path)
	return nil
}

func (f *fakeRunner) ReloadSSH() error   { return f.record("ReloadSSH()") }
func (f *fakeRunner) ReloadSamba() error { return f.record("ReloadSamba()") }

// itoa sem importar strconv só pra manter o fake enxuto.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

// --- ValidUsername ---

func TestValidUsername(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"alice", true},
		{"bob", true},
		{"a_b", true},
		{"a-b", true},
		{"a1", false},                          // muito curto (2 chars)
		{"ab", false},                          // 2 chars, mínimo é 3
		{"abc", true},                          // mínimo (3 chars)
		{"a" + strings.Repeat("b", 31), true},  // máximo (32 chars)
		{"a" + strings.Repeat("b", 32), false}, // 33 chars
		{"1abc", false},                        // começa com dígito
		{"_abc", false},                        // começa com underscore
		{"-abc", false},                        // começa com hífen
		{"Alice", false},                       // maiúscula
		{"ab c", false},                        // espaço
		{"ab.c", false},                        // ponto
		{"", false},
		{"root", true}, // válido pelo regex (defesa em profundidade contra colisão fica em EnsureUnused)
	}
	for _, c := range cases {
		if got := ValidUsername(c.name); got != c.want {
			t.Errorf("ValidUsername(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

// --- SSHDMatchConfig ---

func TestSSHDMatchConfig_Content(t *testing.T) {
	got := SSHDMatchConfig("alice")
	mustContain := []string{
		"Match User alice",
		"ForceCommand internal-sftp",
		"ChrootDirectory /home/alice",
		"PubkeyAuthentication yes",
		"PasswordAuthentication no",
		"AuthorizedKeysFile /home/alice/.ssh/authorized_keys",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("SSHDMatchConfig(alice) faltando %q\nobtido:\n%s", s, got)
		}
	}
}

// --- SambaShareConfig ---

func TestSambaShareConfig_Content(t *testing.T) {
	got := SambaShareConfig("alice")
	mustContain := []string{
		"[home-alice]",
		"path = /home/alice/files",
		"guest ok = yes",
		"force user = alice",
		"writable = yes",
	}
	for _, s := range mustContain {
		if !strings.Contains(got, s) {
			t.Errorf("SambaShareConfig(alice) faltando %q\nobtido:\n%s", s, got)
		}
	}
}

// --- Create ---

func TestCreate_NewUserCreatesAccountAndDirs(t *testing.T) {
	r := newFakeRunner()
	if err := Create(r, "alice"); err != nil {
		t.Fatalf("Create falhou: %v", err)
	}
	if !r.users["alice"] {
		t.Fatal("usuário alice deveria ter sido criado")
	}
	// Confirma a sequência de dono/permissão da raiz do chroot —
	// exigência do sshd, se errar aqui o SFTP recusaria o login.
	// (os.FileMode.String() usa '-' no prefixo, não 'd' — o bit
	// ModeDir não é setado em 0o755 puro.)
	wantCalls := []string{
		"UserExists(alice)",
		"AddSystemUser(alice,/home/alice)",
		"MkdirAll(/home/alice,-rwxr-xr-x)",
		"Chown(/home/alice,0,0)",
		"Chmod(/home/alice,-rwxr-xr-x)",
		"LookupUIDGID(alice)",
		"MkdirAll(/home/alice/files,-rwx------)",
		"Chown(/home/alice/files,1005,1005)",
		"Chmod(/home/alice/files,-rwx------)",
		"MkdirAll(/home/alice/.ssh,-rwxr-xr-x)",
		"Chown(/home/alice/.ssh,0,0)",
		"Chmod(/home/alice/.ssh,-rwxr-xr-x)",
	}
	if len(r.calls) != len(wantCalls) {
		t.Fatalf("esperava %d calls, obtido %d: %v", len(wantCalls), len(r.calls), r.calls)
	}
	for i, want := range wantCalls {
		if r.calls[i] != want {
			t.Errorf("call[%d] = %q, want %q\nfull: %v", i, r.calls[i], want, r.calls)
		}
	}
}

func TestCreate_ExistingUserSkipsUseradd(t *testing.T) {
	r := newFakeRunner()
	r.users["alice"] = true
	if err := Create(r, "alice"); err != nil {
		t.Fatalf("Create falhou: %v", err)
	}
	for _, c := range r.calls {
		if strings.HasPrefix(c, "AddSystemUser") {
			t.Fatalf("AddSystemUser não deveria ter sido chamado para usuário existente: %v", r.calls)
		}
	}
}

func TestCreate_InvalidUsername(t *testing.T) {
	r := newFakeRunner()
	if err := Create(r, "1invalid"); !errors.Is(err, ErrInvalidUsername) {
		t.Fatalf("esperava ErrInvalidUsername, obtido: %v", err)
	}
	if len(r.calls) != 0 {
		t.Fatalf("nenhuma call deveria ter sido feita com username inválido: %v", r.calls)
	}
}

// --- EnableSFTP ---

func TestEnableSFTP_WritesDropInAndAuthorizedKeysAndReloads(t *testing.T) {
	r := newFakeRunner()
	key := "ssh-ed25519 AAAA... alice@host"
	if err := EnableSFTP(r, "alice", key); err != nil {
		t.Fatalf("EnableSFTP falhou: %v", err)
	}
	dropIn := "/etc/ssh/sshd_config.d/xvpn-sftp-alice.conf"
	ak := "/home/alice/.ssh/authorized_keys"
	if content, ok := r.files[dropIn]; !ok {
		t.Errorf("drop-in sshd não foi escrito: %v", r.files)
	} else if !strings.Contains(content, "Match User alice") {
		t.Errorf("drop-in sshd sem Match User alice:\n%s", content)
	}
	if content, ok := r.files[ak]; !ok {
		t.Errorf("authorized_keys não foi escrito: %v", r.files)
	} else if !strings.Contains(content, key) {
		t.Errorf("authorized_keys sem a chave:\n%s", content)
	}
	// authorized_keys termina com newline (exigência do sshd pra
	// parsear a última linha corretamente).
	if !strings.HasSuffix(r.files[ak], "\n") {
		t.Errorf("authorized_keys deveria terminar com newline: %q", r.files[ak])
	}
	// ReloadSSH deve ter sido chamado (depois de escrever os arquivos).
	found := false
	for _, c := range r.calls {
		if c == "ReloadSSH()" {
			found = true
		}
	}
	if !found {
		t.Errorf("ReloadSSH não foi chamado: %v", r.calls)
	}
}

func TestEnableSFTP_EmptyKeyStillWritesDropIn(t *testing.T) {
	// O handler do painel deve exigir a chave antes de chamar, mas o
	// provisionador não quebra com chave vazia — escreve o drop-in e
	// um authorized_keys vazio (login fica efetivamente bloqueado até
	// o admin colar uma chave, mas a config sshd está consistente).
	r := newFakeRunner()
	if err := EnableSFTP(r, "alice", ""); err != nil {
		t.Fatalf("EnableSFTP falhou: %v", err)
	}
	if _, ok := r.files["/etc/ssh/sshd_config.d/xvpn-sftp-alice.conf"]; !ok {
		t.Errorf("drop-in deveria ter sido escrito mesmo com chave vazia: %v", r.files)
	}
}

func TestEnableSFTP_ReloadFailureReturnsError(t *testing.T) {
	r := newFakeRunner()
	r.failOn = "ReloadSSH"
	err := EnableSFTP(r, "alice", "ssh-ed25519 AAAA alice")
	if err == nil {
		t.Fatal("esperava erro no reload falho")
	}
}

// --- EnableSamba ---

func TestEnableSamba_WritesShareIncludeAndReloads(t *testing.T) {
	r := newFakeRunner()
	if err := EnableSamba(r, "alice"); err != nil {
		t.Fatalf("EnableSamba falhou: %v", err)
	}
	share := "/etc/samba/smb.conf.d/xvpn-home-alice.conf"
	if content, ok := r.files[share]; !ok {
		t.Errorf("include samba não foi escrito: %v", r.files)
	} else if !strings.Contains(content, "[home-alice]") {
		t.Errorf("include samba sem [home-alice]:\n%s", content)
	}
	// O agregado xvpn-shares.conf precisa ser regenerado (o `include`
	// do Samba não suporta glob — ver samba.go).
	agg, ok := r.files["/etc/samba/smb.conf.d/xvpn-shares.conf"]
	if !ok {
		t.Fatalf("agregado xvpn-shares.conf não foi gerado: %v", r.files)
	}
	if !strings.Contains(agg, "include = /etc/samba/smb.conf.d/xvpn-home-alice.conf") {
		t.Errorf("agregado não lista o share da alice:\n%s", agg)
	}
	found := false
	for _, c := range r.calls {
		if c == "ReloadSamba()" {
			found = true
		}
	}
	if !found {
		t.Errorf("ReloadSamba não foi chamado: %v", r.calls)
	}
}

func TestRegenerateSambaAggregate_ListsAllPerUserShares(t *testing.T) {
	r := newFakeRunner()
	// Habilita dois usuários — o agregado final precisa listar ambos,
	// em ordem alfabética, e excluir a si mesmo.
	if err := EnableSamba(r, "bob"); err != nil {
		t.Fatal(err)
	}
	if err := EnableSamba(r, "alice"); err != nil {
		t.Fatal(err)
	}
	agg := r.files["/etc/samba/smb.conf.d/xvpn-shares.conf"]
	// Ordem alfabética: alice antes de bob.
	aIdx := strings.Index(agg, "xvpn-home-alice.conf")
	bIdx := strings.Index(agg, "xvpn-home-bob.conf")
	if aIdx < 0 || bIdx < 0 {
		t.Fatalf("agregado não lista ambos os shares:\n%s", agg)
	}
	if aIdx > bIdx {
		t.Errorf("agregado não está em ordem alfabética:\n%s", agg)
	}
	// Não deve listar a si mesmo (xvpn-shares.conf).
	if strings.Contains(agg, "xvpn-shares.conf") {
		t.Errorf("agregado se auto-referencia:\n%s", agg)
	}
}

func TestDisableSamba_RegeneratesAggregateWithoutRemovedUser(t *testing.T) {
	r := newFakeRunner()
	if err := EnableSamba(r, "alice"); err != nil {
		t.Fatal(err)
	}
	if err := EnableSamba(r, "bob"); err != nil {
		t.Fatal(err)
	}
	// Desliga alice — agregado deve listar só bob.
	if err := DisableSamba(r, "alice"); err != nil {
		t.Fatal(err)
	}
	agg := r.files["/etc/samba/smb.conf.d/xvpn-shares.conf"]
	if strings.Contains(agg, "xvpn-home-alice") {
		t.Errorf("agregado ainda lista alice após disable:\n%s", agg)
	}
	if !strings.Contains(agg, "xvpn-home-bob.conf") {
		t.Errorf("agregado não lista bob após disable da alice:\n%s", agg)
	}
}

// --- Disable ---

func TestDisable_RemovesBothDropInsAndReloads(t *testing.T) {
	r := newFakeRunner()
	// Pré-popula como se SFTP e Samba estivessem habilitados.
	r.files["/etc/ssh/sshd_config.d/xvpn-sftp-alice.conf"] = SSHDMatchConfig("alice")
	r.files["/etc/samba/smb.conf.d/xvpn-home-alice.conf"] = SambaShareConfig("alice")
	if err := Disable(r, "alice"); err != nil {
		t.Fatalf("Disable falhou: %v", err)
	}
	if _, ok := r.files["/etc/ssh/sshd_config.d/xvpn-sftp-alice.conf"]; ok {
		t.Error("drop-in sshd deveria ter sido removido")
	}
	if _, ok := r.files["/etc/samba/smb.conf.d/xvpn-home-alice.conf"]; ok {
		t.Error("include samba deveria ter sido removido")
	}
	// Ambos os reloads devem ter sido chamados (algo foi removido).
	if !contains(r.calls, "ReloadSSH()") || !contains(r.calls, "ReloadSamba()") {
		t.Errorf("ambos reloads deveriam ter sido chamados: %v", r.calls)
	}
}

func TestDisable_NoopWhenNothingEnabled(t *testing.T) {
	r := newFakeRunner()
	if err := Disable(r, "alice"); err != nil {
		t.Fatalf("Disable falhou: %v", err)
	}
	// Nada pra remover → nenhum reload (evita recarregar serviços à toa).
	if contains(r.calls, "ReloadSSH()") || contains(r.calls, "ReloadSamba()") {
		t.Errorf("nenhum reload deveria ter sido chamado sem nada pra remover: %v", r.calls)
	}
}

func TestDisable_PreservesAccountAndData(t *testing.T) {
	r := newFakeRunner()
	r.users["alice"] = true
	r.files["/etc/ssh/sshd_config.d/xvpn-sftp-alice.conf"] = SSHDMatchConfig("alice")
	// Simula o arquivo de dados do usuário dentro de files/.
	r.files["/home/alice/files/documento.txt"] = "conteudo importante"
	if err := Disable(r, "alice"); err != nil {
		t.Fatalf("Disable falhou: %v", err)
	}
	// Conta e arquivo de dados continuam lá — disable só remove acesso.
	if !r.users["alice"] {
		t.Error("conta Unix não deveria ter sido removida no disable")
	}
	if _, ok := r.files["/home/alice/files/documento.txt"]; !ok {
		t.Error("arquivo de dados do usuário não deveria ter sido removido no disable")
	}
}

func TestDisable_InvalidUsername(t *testing.T) {
	r := newFakeRunner()
	if err := Disable(r, "BAD"); !errors.Is(err, ErrInvalidUsername) {
		t.Fatalf("esperava ErrInvalidUsername, obtido: %v", err)
	}
}

func TestDisableSFTP_RemovesOnlySSHDDropIn(t *testing.T) {
	r := newFakeRunner()
	r.files["/etc/ssh/sshd_config.d/xvpn-sftp-alice.conf"] = SSHDMatchConfig("alice")
	r.files["/etc/samba/smb.conf.d/xvpn-home-alice.conf"] = SambaShareConfig("alice")
	if err := DisableSFTP(r, "alice"); err != nil {
		t.Fatalf("DisableSFTP falhou: %v", err)
	}
	if _, ok := r.files["/etc/ssh/sshd_config.d/xvpn-sftp-alice.conf"]; ok {
		t.Error("drop-in sshd deveria ter sido removido")
	}
	if _, ok := r.files["/etc/samba/smb.conf.d/xvpn-home-alice.conf"]; !ok {
		t.Error("include samba NÃO deveria ter sido removido por DisableSFTP")
	}
	if !contains(r.calls, "ReloadSSH()") {
		t.Error("ReloadSSH deveria ter sido chamado")
	}
	if contains(r.calls, "ReloadSamba()") {
		t.Error("ReloadSamba NÃO deveria ter sido chamado por DisableSFTP")
	}
}

func TestDisableSFTP_NoopWhenAlreadyOff(t *testing.T) {
	r := newFakeRunner()
	// Samba habilitado, SFTP não — DisableSFTP não deve tocar Samba nem
	// recarregar nada (já estava sem drop-in sshd).
	r.files["/etc/samba/smb.conf.d/xvpn-home-alice.conf"] = SambaShareConfig("alice")
	if err := DisableSFTP(r, "alice"); err != nil {
		t.Fatalf("DisableSFTP falhou: %v", err)
	}
	if _, ok := r.files["/etc/samba/smb.conf.d/xvpn-home-alice.conf"]; !ok {
		t.Error("include samba não deveria ter sido tocado")
	}
	if contains(r.calls, "ReloadSSH()") {
		t.Error("ReloadSSH não deveria ter sido chamado (nada pra remover)")
	}
}

func TestDisableSamba_RemovesOnlyShareInclude(t *testing.T) {
	r := newFakeRunner()
	r.files["/etc/ssh/sshd_config.d/xvpn-sftp-alice.conf"] = SSHDMatchConfig("alice")
	r.files["/etc/samba/smb.conf.d/xvpn-home-alice.conf"] = SambaShareConfig("alice")
	if err := DisableSamba(r, "alice"); err != nil {
		t.Fatalf("DisableSamba falhou: %v", err)
	}
	if _, ok := r.files["/etc/samba/smb.conf.d/xvpn-home-alice.conf"]; ok {
		t.Error("include samba deveria ter sido removido")
	}
	if _, ok := r.files["/etc/ssh/sshd_config.d/xvpn-sftp-alice.conf"]; !ok {
		t.Error("drop-in sshd NÃO deveria ter sido removido por DisableSamba")
	}
	if !contains(r.calls, "ReloadSamba()") {
		t.Error("ReloadSamba deveria ter sido chamado")
	}
	if contains(r.calls, "ReloadSSH()") {
		t.Error("ReloadSSH não deveria ter sido chamado por DisableSamba")
	}
}

// --- EnsureUnused ---

func TestEnsureUnused_RejectsExistingSystemUser(t *testing.T) {
	r := newFakeRunner()
	r.users["root"] = true // root sempre existe
	if err := EnsureUnused(r, "root"); err == nil {
		t.Fatal("EnsureUnused deveria rejeitar usuário que já existe")
	}
}

func TestEnsureUnused_AllowsNewName(t *testing.T) {
	r := newFakeRunner()
	if err := EnsureUnused(r, "alice"); err != nil {
		t.Fatalf("EnsureUnused rejeitou nome novo: %v", err)
	}
}

func contains(s []string, want string) bool {
	for _, v := range s {
		if v == want {
			return true
		}
	}
	return false
}
