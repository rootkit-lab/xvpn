package sshkey

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsureIn_GeneratesPairWithRestrictivePermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".ssh")

	publicKey, err := ensureIn(dir)
	if err != nil {
		t.Fatalf("esperava sucesso, obtido erro: %v", err)
	}

	if !strings.HasPrefix(publicKey, "ssh-ed25519 AAAA") {
		t.Fatalf("esperava uma linha authorized_keys ed25519, obtido %q", publicKey)
	}
	if strings.Count(publicKey, "\n") != 0 {
		t.Fatalf("a chave pública tem que ser uma única linha, obtido %q", publicKey)
	}
	if fields := strings.Fields(publicKey); len(fields) != 3 || !strings.HasPrefix(fields[2], "xvpn-") {
		t.Fatalf("esperava comentário identificando a máquina, obtido %q", publicKey)
	}

	// 0600 na privada não é preferência: o próprio OpenSSH recusa uma
	// chave que outros usuários da máquina consigam ler.
	info, err := os.Stat(filepath.Join(dir, privateKeyName))
	if err != nil {
		t.Fatalf("erro lendo a chave privada: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("esperava a chave privada com modo 0600, obtido %o", perm)
	}

	publicInfo, err := os.Stat(filepath.Join(dir, publicKeyName))
	if err != nil {
		t.Fatalf("erro lendo a chave pública: %v", err)
	}
	if perm := publicInfo.Mode().Perm(); perm&0o022 != 0 {
		t.Fatalf("a chave pública não pode ser gravável por grupo/outros, obtido %o", perm)
	}

	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("erro lendo o diretório: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm&0o077 != 0 {
		t.Fatalf("esperava o diretório restrito ao dono, obtido %o", perm)
	}
}

// TestEnsureIn_IsIdempotent: gerar outra chave na segunda chamada
// invalidaria a que o servidor já colocou no authorized_keys deste
// dispositivo — o acesso a arquivos pararia sozinho no próximo start.
func TestEnsureIn_IsIdempotent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".ssh")

	first, err := ensureIn(dir)
	if err != nil {
		t.Fatalf("primeira chamada falhou: %v", err)
	}
	privateBefore, err := os.ReadFile(filepath.Join(dir, privateKeyName))
	if err != nil {
		t.Fatalf("erro lendo a chave privada: %v", err)
	}

	second, err := ensureIn(dir)
	if err != nil {
		t.Fatalf("segunda chamada falhou: %v", err)
	}
	if first != second {
		t.Fatalf("esperava a mesma chave pública nas duas chamadas:\n%q\n%q", first, second)
	}

	privateAfter, err := os.ReadFile(filepath.Join(dir, privateKeyName))
	if err != nil {
		t.Fatalf("erro lendo a chave privada: %v", err)
	}
	if string(privateBefore) != string(privateAfter) {
		t.Fatal("a chave privada foi reescrita numa chamada que deveria ser no-op")
	}
}

// TestEnsureIn_RestoresMissingPublicKey: com a privada intacta, derivar a
// pública de volta preserva o acesso já concedido; gerar um par novo o
// derrubaria.
func TestEnsureIn_RestoresMissingPublicKey(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".ssh")

	first, err := ensureIn(dir)
	if err != nil {
		t.Fatalf("primeira chamada falhou: %v", err)
	}
	if err := os.Remove(filepath.Join(dir, publicKeyName)); err != nil {
		t.Fatalf("erro removendo a chave pública: %v", err)
	}

	second, err := ensureIn(dir)
	if err != nil {
		t.Fatalf("segunda chamada falhou: %v", err)
	}
	if first != second {
		t.Fatalf("esperava a mesma chave pública derivada da privada:\n%q\n%q", first, second)
	}
}

func TestEnsureConfigEntryIn_DoesNotDuplicateBlock(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".ssh")
	if _, err := ensureIn(dir); err != nil {
		t.Fatalf("setup: erro gerando o par: %v", err)
	}

	for i := 0; i < 2; i++ {
		if err := ensureConfigEntryIn(dir, "rootkit"); err != nil {
			t.Fatalf("chamada %d falhou: %v", i+1, err)
		}
	}

	content := readConfig(t, dir)
	if got := strings.Count(content, blockBegin); got != 1 {
		t.Fatalf("esperava exatamente 1 bloco do XVPN, obtido %d:\n%s", got, content)
	}
	if got := strings.Count(content, "Host "+hostAlias); got != 1 {
		t.Fatalf("esperava exatamente 1 entrada %q, obtido %d:\n%s", hostAlias, got, content)
	}
	for _, expected := range []string{
		"HostName " + serverVPNAddress,
		"User rootkit",
		"IdentityFile " + filepath.Join(dir, privateKeyName),
		"IdentitiesOnly yes",
	} {
		if !strings.Contains(content, expected) {
			t.Fatalf("esperava %q no bloco, obtido:\n%s", expected, content)
		}
	}

	info, err := os.Stat(filepath.Join(dir, configName))
	if err != nil {
		t.Fatalf("erro lendo o config: %v", err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Fatalf("esperava o ~/.ssh/config restrito ao dono, obtido %o", perm)
	}
}

// TestEnsureConfigEntryIn_UpdatesUsername: o dono do dispositivo pode
// mudar no painel, e um bloco desatualizado apontaria o sftp para uma
// conta que não é a da pessoa.
func TestEnsureConfigEntryIn_UpdatesUsername(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".ssh")

	if err := ensureConfigEntryIn(dir, "alice"); err != nil {
		t.Fatalf("primeira chamada falhou: %v", err)
	}
	if err := ensureConfigEntryIn(dir, "bob"); err != nil {
		t.Fatalf("segunda chamada falhou: %v", err)
	}

	content := readConfig(t, dir)
	if strings.Contains(content, "User alice") {
		t.Fatalf("esperava o username antigo substituído, obtido:\n%s", content)
	}
	if !strings.Contains(content, "User bob") {
		t.Fatalf("esperava o username novo no bloco, obtido:\n%s", content)
	}
	if got := strings.Count(content, blockBegin); got != 1 {
		t.Fatalf("esperava exatamente 1 bloco do XVPN, obtido %d:\n%s", got, content)
	}
}

// TestEnsureConfigEntryIn_PreservesUserContent é o ponto delicado deste
// pacote: o ~/.ssh/config é do usuário, não nosso — pode ter dezenas de
// hosts que ele depende, e nada fora dos marcadores pode ser tocado.
func TestEnsureConfigEntryIn_PreservesUserContent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".ssh")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("setup: %v", err)
	}
	userContent := "Host servidor-do-trabalho\n    HostName 192.0.2.10\n    User joao\n\nHost *\n    ServerAliveInterval 60\n"
	configPath := filepath.Join(dir, configName)
	if err := os.WriteFile(configPath, []byte(userContent), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	if err := ensureConfigEntryIn(dir, "rootkit"); err != nil {
		t.Fatalf("primeira chamada falhou: %v", err)
	}
	if err := ensureConfigEntryIn(dir, "rootkit2"); err != nil {
		t.Fatalf("segunda chamada falhou: %v", err)
	}

	content := readConfig(t, dir)
	if !strings.Contains(content, userContent) {
		t.Fatalf("esperava o conteúdo do usuário preservado na íntegra, obtido:\n%s", content)
	}
	if !strings.Contains(content, "User rootkit2") {
		t.Fatalf("esperava o bloco do XVPN atualizado, obtido:\n%s", content)
	}
}

func TestEnsureConfigEntryIn_RequiresUsername(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".ssh")

	err := ensureConfigEntryIn(dir, "  ")
	if err == nil {
		t.Fatal("esperava erro sem username conhecido")
	}
	if !strings.Contains(err.Error(), "conecte a VPN") {
		t.Fatalf("esperava erro acionável em português, obtido: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, configName)); statErr == nil {
		t.Fatal("não podia ter escrito no ~/.ssh/config sem username")
	}
}

// TestEnsureIn_ReadableByOpenSSH fecha o ciclo: de nada adianta o par
// estar bem formado para o x/crypto se o `sftp` do usuário não conseguir
// usá-lo. Pula onde não houver ssh-keygen (Windows, CI mínima) em vez de
// exigir a ferramenta.
func TestEnsureIn_ReadableByOpenSSH(t *testing.T) {
	sshKeygen, err := exec.LookPath("ssh-keygen")
	if err != nil {
		t.Skip("ssh-keygen não está instalado nesta máquina")
	}
	dir := filepath.Join(t.TempDir(), ".ssh")
	publicKey, err := ensureIn(dir)
	if err != nil {
		t.Fatalf("esperava sucesso, obtido erro: %v", err)
	}

	// -y deriva a pública a partir da privada: se o formato OpenSSH
	// estiver errado, falha aqui.
	out, err := exec.Command(sshKeygen, "-y", "-f", filepath.Join(dir, privateKeyName)).Output()
	if err != nil {
		t.Fatalf("ssh-keygen não conseguiu ler a chave privada: %v", err)
	}
	derived := strings.Fields(strings.TrimSpace(string(out)))
	generated := strings.Fields(publicKey)
	if len(derived) < 2 || derived[0] != generated[0] || derived[1] != generated[1] {
		t.Fatalf("chave pública derivada pelo ssh-keygen difere da gerada:\n%q\n%q", string(out), publicKey)
	}
}

func readConfig(t *testing.T, sshDir string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(sshDir, configName))
	if err != nil {
		t.Fatalf("erro lendo o ~/.ssh/config: %v", err)
	}
	return string(raw)
}
