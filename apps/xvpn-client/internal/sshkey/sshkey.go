// Package sshkey cuida do par de chaves SSH deste dispositivo, usado pelo
// acesso a arquivos por SFTP (ver PLAN.md §6.9, revisão da Fase 14).
//
// Diferente da chave WireGuard — que é gerada pelo helper privilegiado e
// vive num arquivo root-only (ver internal/config) —, esta é gerada pelo
// processo GUI sem privilégio e fica em ~/.ssh, porque quem precisa lê-la
// é o cliente SFTP do próprio usuário. O invariante 1 do AGENTS.md
// continua valendo igual: a chave privada nunca sai desta máquina, nunca
// é logada e nunca trafega pela rede — só a pública é registrada no
// servidor.
package sshkey

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
)

const (
	privateKeyName = "xvpn_ed25519"
	publicKeyName  = privateKeyName + ".pub"
	configName     = "config"

	// hostAlias é o nome do atalho no ~/.ssh/config: com ele, `sftp
	// xvpn-files` funciona sem o usuário precisar lembrar de IP, usuário
	// ou caminho de chave.
	hostAlias = "xvpn-files"

	// serverVPNAddress é o IP fixo do servidor dentro do túnel (ver
	// AGENTS.md) — o sshd só é alcançável por fora do túnel via o
	// endereço público, que não é o caminho que este atalho descreve.
	serverVPNAddress = "10.66.66.1"

	// Marcadores que delimitam o trecho do ~/.ssh/config que é nosso. O
	// arquivo é do usuário e pode ter dezenas de hosts dele: reescrever
	// só o que está entre os marcadores é o que permite atualizar o
	// bloco (o username pode mudar) sem duplicar nem estragar o resto.
	blockBegin = "# >>> xvpn"
	blockEnd   = "# <<< xvpn"
)

// Ensure devolve a chave pública deste dispositivo em formato
// authorized_keys (uma única linha), gerando o par ed25519 na primeira
// chamada. É idempotente por obrigação, não por elegância: gerar outra
// chave invalidaria a que o servidor já publicou no authorized_keys deste
// dispositivo.
func Ensure() (string, error) {
	dir, err := defaultSSHDir()
	if err != nil {
		return "", err
	}
	return ensureIn(dir)
}

// EnsureSSHConfigEntry escreve (ou atualiza) o bloco "Host xvpn-files" no
// ~/.ssh/config apontando para este par de chaves e para o usuário do
// painel.
func EnsureSSHConfigEntry(username string) error {
	dir, err := defaultSSHDir()
	if err != nil {
		return err
	}
	return ensureConfigEntryIn(dir, username)
}

func defaultSSHDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("não foi possível localizar a pasta pessoal do usuário: %w", err)
	}
	return filepath.Join(home, ".ssh"), nil
}

// ensureIn é a versão parametrizada pelo diretório, para os testes não
// escreverem no ~/.ssh de verdade de quem roda a suíte.
func ensureIn(sshDir string) (string, error) {
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return "", fmt.Errorf("criando o diretório %q: %w", sshDir, err)
	}
	privatePath := filepath.Join(sshDir, privateKeyName)
	publicPath := filepath.Join(sshDir, publicKeyName)

	privatePEM, err := os.ReadFile(privatePath)
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("lendo a chave privada em %q: %w", privatePath, err)
	}
	if err == nil {
		if public, readErr := os.ReadFile(publicPath); readErr == nil && strings.TrimSpace(string(public)) != "" {
			return strings.TrimSpace(string(public)), nil
		}
		// Privada intacta e pública sumida: derivar de volta é melhor
		// que gerar um par novo, que derrubaria o acesso já concedido.
		public, deriveErr := publicKeyFromPrivate(privatePEM)
		if deriveErr != nil {
			return "", deriveErr
		}
		if writeErr := os.WriteFile(publicPath, []byte(public+"\n"), 0o644); writeErr != nil {
			return "", fmt.Errorf("gravando a chave pública em %q: %w", publicPath, writeErr)
		}
		return public, nil
	}

	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", fmt.Errorf("gerando o par de chaves SSH: %w", err)
	}
	comment := keyComment()

	block, err := ssh.MarshalPrivateKey(privateKey, comment)
	if err != nil {
		return "", fmt.Errorf("codificando a chave privada SSH: %w", err)
	}
	// 0600 é requisito do próprio OpenSSH, que recusa usar uma chave que
	// outros usuários da máquina consigam ler.
	if err := os.WriteFile(privatePath, pem.EncodeToMemory(block), 0o600); err != nil {
		return "", fmt.Errorf("gravando a chave privada em %q: %w", privatePath, err)
	}

	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("codificando a chave pública SSH: %w", err)
	}
	authorized := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(sshPublicKey))) + " " + comment
	if err := os.WriteFile(publicPath, []byte(authorized+"\n"), 0o644); err != nil {
		return "", fmt.Errorf("gravando a chave pública em %q: %w", publicPath, err)
	}
	return authorized, nil
}

func publicKeyFromPrivate(privatePEM []byte) (string, error) {
	signer, err := ssh.ParsePrivateKey(privatePEM)
	if err != nil {
		return "", fmt.Errorf("chave privada SSH do XVPN ilegível — apague %s e reabra o aplicativo para gerar outra: %w", privateKeyName, err)
	}
	return strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))) + " " + keyComment(), nil
}

// keyComment identifica de qual máquina a chave veio, já que o mesmo
// usuário do painel pode ter vários dispositivos e o servidor lista as
// chaves por dispositivo.
func keyComment() string {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		hostname = "desconhecido"
	}
	return "xvpn-" + sanitizeComment(hostname)
}

// sanitizeComment mantém o comentário numa única palavra: espaço ou
// quebra de linha ali quebraria o formato de uma linha do authorized_keys.
func sanitizeComment(value string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		case r == '-', r == '_', r == '.':
			return r
		default:
			return '-'
		}
	}, value)
}

func ensureConfigEntryIn(sshDir, username string) error {
	if strings.TrimSpace(username) == "" {
		return fmt.Errorf("nome de usuário desconhecido — conecte a VPN para o servidor informar quem é o dono deste dispositivo")
	}
	if err := os.MkdirAll(sshDir, 0o700); err != nil {
		return fmt.Errorf("criando o diretório %q: %w", sshDir, err)
	}

	configPath := filepath.Join(sshDir, configName)
	existing, err := os.ReadFile(configPath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("lendo %q: %w", configPath, err)
	}

	updated := replaceBlock(string(existing), renderBlock(sshDir, username))
	if updated == string(existing) {
		return nil
	}
	if err := os.WriteFile(configPath, []byte(updated), 0o600); err != nil {
		return fmt.Errorf("gravando %q: %w", configPath, err)
	}
	return nil
}

func renderBlock(sshDir, username string) string {
	var b strings.Builder
	b.WriteString(blockBegin + "\n")
	b.WriteString("# Bloco mantido pelo XVPN — alterações aqui dentro são sobrescritas.\n")
	b.WriteString("Host " + hostAlias + "\n")
	b.WriteString("    HostName " + serverVPNAddress + "\n")
	b.WriteString("    User " + username + "\n")
	b.WriteString("    IdentityFile " + filepath.Join(sshDir, privateKeyName) + "\n")
	// Sem IdentitiesOnly, o ssh oferece antes todas as chaves do agente e
	// pode estourar o limite de tentativas do servidor antes de chegar na
	// nossa.
	b.WriteString("    IdentitiesOnly yes\n")
	b.WriteString(blockEnd + "\n")
	return b.String()
}

// replaceBlock devolve o conteúdo do ~/.ssh/config com o bloco do XVPN
// atualizado, preservando tudo que está fora dos marcadores. Um bloco
// novo entra no topo do arquivo porque o ssh resolve cada opção pela
// primeira ocorrência que casa — depois de um "Host *" do usuário, o
// nosso IdentityFile poderia nunca ser escolhido.
func replaceBlock(existing, block string) string {
	lines := strings.Split(existing, "\n")
	begin, end := -1, -1
	for i, line := range lines {
		switch strings.TrimSpace(line) {
		case blockBegin:
			if begin == -1 {
				begin = i
			}
		case blockEnd:
			if begin != -1 && end == -1 {
				end = i
			}
		}
	}

	// Marcador de abertura sem fechamento é arquivo editado à mão de
	// forma inconsistente: acrescentar um bloco novo deixa um resto
	// inofensivo, enquanto "reescrever até o fim do arquivo" apagaria
	// hosts do usuário.
	if begin == -1 || end == -1 {
		if strings.TrimSpace(existing) == "" {
			return block
		}
		return block + "\n" + existing
	}

	merged := make([]string, 0, len(lines))
	merged = append(merged, lines[:begin]...)
	merged = append(merged, strings.Split(strings.TrimSuffix(block, "\n"), "\n")...)
	merged = append(merged, lines[end+1:]...)
	return strings.Join(merged, "\n")
}
