package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"strings"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

// maxAuthorizedKeyLines limita quantas chaves cabem num authorized_keys.
// validSSHPublicKey já limitava o tamanho de cada chave, mas não a
// quantidade — sem teto, o textarea do painel aceitaria um arquivo
// arbitrariamente grande e a união com as chaves dos dispositivos
// multiplicaria isso. 16 é folgado para o uso real (1-15 usuários, alguns
// dispositivos cada) e ainda mantém o arquivo legível a olho nu.
const maxAuthorizedKeyLines = 16

// renderAuthorizedKeys monta as linhas do authorized_keys de um usuário: a
// união das chaves auto-registradas pelos dispositivos dele (Fase 14) com
// manualKey, a chave colada à mão pelo admin no painel (Fase 13, que
// continua existindo como escape hatch para celular ou máquina sem o
// cliente XVPN instalado). A quebra de linha final quem acrescenta é
// provision.EnableSFTP, que normaliza isso ao gravar o arquivo.
//
// manualKey vem como parâmetro em vez de ser lida do User de propósito: o
// handler de acesso a arquivos precisa renderizar com a chave que o admin
// *acabou* de enviar, antes de persistir — passar o valor explicitamente
// evita depender de qual versão do struct está em memória naquele ponto.
//
// A saída é determinística (dispositivos por ID crescente, chave manual
// no fim) para que registrar a mesma chave duas vezes produza exatamente
// o mesmo arquivo — é o que torna a re-renderização segura de repetir.
func (a *App) renderAuthorizedKeys(userID uint, manualKey string) (string, error) {
	var devices []store.Device
	if err := a.Store.DB.
		Where("user_id = ? AND ssh_public_key <> ''", userID).
		Order("id").Find(&devices).Error; err != nil {
		return "", err
	}

	lines := make([]string, 0, len(devices)+1)
	// Deduplica pelo par (tipo, blob base64), ignorando o comentário: é o
	// que o sshd considera "a mesma chave". Duas máquinas podem ter
	// copiado o mesmo par de chaves, e duplicar a linha não quebraria o
	// login, mas polui o arquivo e faz um re-render parecer uma mudança.
	seen := make(map[string]bool, len(devices)+1)
	appendKeys := func(raw string) {
		for _, line := range strings.Split(raw, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if len(lines) >= maxAuthorizedKeyLines {
				return
			}
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			id := fields[0] + " " + fields[1]
			if seen[id] {
				continue
			}
			seen[id] = true
			lines = append(lines, line)
		}
	}

	for _, d := range devices {
		appendKeys(d.SSHPublicKey)
	}
	appendKeys(manualKey)

	return strings.Join(lines, "\n"), nil
}

// applyAuthorizedKeys re-renderiza no sistema o authorized_keys do
// usuário. É a função única exigida pelos três caminhos que podem mudar o
// conjunto de chaves sem passar pelo painel — registro automático de
// chave, revogação de dispositivo e reconcile no boot. Sem ela em um
// desses caminhos, sobra chave viva de dispositivo revogado (ver
// ROADMAP.md Fase 14.2).
//
// No-op deliberado quando o usuário está com SFTP desligado: reusar
// EnableSFTP aqui escreveria o drop-in do sshd junto (ver
// provision.EnableSFTP) e concederia acesso a quem o admin não liberou. O
// registro da chave nesse caso fica só no banco, e passa a valer no
// instante em que o admin ligar o toggle.
func (a *App) applyAuthorizedKeys(ctx context.Context, user store.User) error {
	if a.UserProvisioner == nil || !user.SFTPEnabled {
		return nil
	}
	content, err := a.renderAuthorizedKeys(user.ID, user.SSHPublicKey)
	if err != nil {
		return err
	}
	return a.UserProvisioner.EnableSFTP(ctx, user.Username, content)
}

// sshKeyFingerprint devolve o fingerprint SHA-256 no mesmo formato que
// `ssh-keygen -lf` imprime ("SHA256:<base64 sem padding>"), calculado
// sobre o blob da chave. Usado no audit log e na resposta ao cliente para
// identificar uma chave sem nunca registrar a chave inteira. Devolve ""
// se a linha não tiver um blob base64 decodificável.
func sshKeyFingerprint(key string) string {
	fields := strings.Fields(strings.TrimSpace(key))
	if len(fields) < 2 {
		return ""
	}
	blob, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(blob)
	return "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
}

// sameSSHKey compara duas linhas de authorized_keys ignorando espaços em
// volta e o comentário no fim — é o que decide se um POST
// /api/me/ssh-key é um no-op (mesma chave, cliente reiniciado) ou uma
// troca real. Sem isso, um comentário diferente ("xvpn-laptop" virando
// "xvpn-laptop-2") faria o servidor reescrever o authorized_keys e
// recarregar o sshd a cada conexão.
func sameSSHKey(a, b string) bool {
	fa, fb := strings.Fields(strings.TrimSpace(a)), strings.Fields(strings.TrimSpace(b))
	if len(fa) < 2 || len(fb) < 2 {
		return false
	}
	return fa[0] == fb[0] && fa[1] == fb[1]
}
