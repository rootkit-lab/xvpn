package api

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// UserProvisioner é a interface que o painel usa para pedir ao binário
// privilegiado xvpn-user-provision (Fase 13, PLAN.md §6.9) que crie
// contas Unix e habilite/desabilite SFTP/Samba por usuário. O
// *userprovision.Client satisfaz esta interface em produção; os testes
// injetam um fake (ver file_access_handler_test.go).
type UserProvisioner interface {
	Create(ctx context.Context, username string) error
	EnableSFTP(ctx context.Context, username, sshPublicKey string) error
	EnableSamba(ctx context.Context, username string) error
	DisableSFTP(ctx context.Context, username string) error
	DisableSamba(ctx context.Context, username string) error
	Disable(ctx context.Context, username string) error
	SetQuota(ctx context.Context, username string, quotaMB uint64) error
}

// fileAccessRequest é o corpo do PUT /api/users/:id/file-access. Os
// campos formam o estado DESEJADO pelo admin — o handler calcula
// o diff contra o estado atual e aplica só o que mudou.
type fileAccessRequest struct {
	SFTPEnabled  bool   `json:"sftp_enabled"`
	SambaEnabled bool   `json:"samba_enabled"`
	SSHPublicKey string `json:"ssh_public_key"`
	// DiskQuotaMB: 0 = sem limite. Aplicado quando SFTP ou Samba está on.
	DiskQuotaMB uint64 `json:"disk_quota_mb"`
}

// fileAccessResponse espelha o estado persistido no User depois de
// aplicar a requisição — devolvido pra o painel confirmar o que ficou.
type fileAccessResponse struct {
	SFTPEnabled  bool   `json:"sftp_enabled"`
	SambaEnabled bool   `json:"samba_enabled"`
	SSHPublicKey string `json:"ssh_public_key"`
	DiskQuotaMB  uint64 `json:"disk_quota_mb"`
}

// acceptedSSHKeyTypes são os tipos de chave que o painel aceita colar e
// que o cliente pode auto-registrar. Lista fechada de propósito: um tipo
// desconhecido aqui é mais provavelmente lixo colado por engano do que uma
// chave legítima que o sshd aceitaria.
var acceptedSSHKeyTypes = map[string]bool{
	"ssh-rsa": true, "ssh-ed25519": true,
	"ecdsa-sha2-nistp256": true, "ecdsa-sha2-nistp384": true, "ecdsa-sha2-nistp521": true,
	"sk-ssh-ed25519@openssh.com": true, "sk-ecdsa-sha2-nistp256@openssh.com": true,
}

// validSSHKeyLine faz a checagem superficial de uma única linha de
// authorized_keys — não é validação criptográfica (isso o sshd faz no
// login), só rejeita lixo óbvio.
func validSSHKeyLine(line string) bool {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) < 2 || !acceptedSSHKeyTypes[fields[0]] {
		return false
	}
	return len(fields[1]) >= 10 && len(fields[1]) <= 8192
}

// validSSHPublicKey valida o conteúdo do textarea do painel, que pode
// trazer várias chaves (uma por linha), comentários (#) e linhas em
// branco. Vazio é válido: desde a Fase 14 o SFTP não depende mais dessa
// chave (as dos dispositivos entram na união), então "nenhuma chave
// manual" é o estado normal, não um estado incompleto.
//
// Além do tamanho de cada chave, limita a QUANTIDADE de linhas: sem esse
// teto, o campo aceitaria um arquivo arbitrariamente grande, que depois
// ainda seria unido às chaves dos dispositivos.
func validSSHPublicKey(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return true
	}
	count := 0
	for _, line := range strings.Split(key, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !validSSHKeyLine(line) {
			return false
		}
		count++
		if count > maxAuthorizedKeyLines {
			return false
		}
	}
	return true
}

// errNotSingleSSHKey é devolvido quando POST /api/me/ssh-key recebe algo
// que não é exatamente uma chave.
var errNotSingleSSHKey = errors.New("envie exatamente uma chave pública SSH")

// normalizeSingleSSHPublicKey valida e normaliza a chave de um
// dispositivo: exatamente uma linha, sem comentários de arquivo, sem
// múltiplas chaves. Diferente do textarea do painel — ali o admin cola um
// pedaço de authorized_keys; aqui é uma máquina reportando a própria
// chave, e mais de uma linha significa que algo está errado no cliente,
// não que o usuário quis autorizar várias.
func normalizeSingleSSHPublicKey(raw string) (string, error) {
	var found string
	for _, line := range strings.Split(strings.TrimSpace(raw), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if found != "" {
			return "", errNotSingleSSHKey
		}
		found = line
	}
	if found == "" {
		return "", errNotSingleSSHKey
	}
	if !validSSHKeyLine(found) {
		return "", fmt.Errorf("chave pública SSH malformada")
	}
	return found, nil
}

// provisionerErrMsg extrai uma mensagem curta do erro do provisionador
// (o binário escreve erros em texto puro pro stderr, ver
// cmd/xvpn-user-provision/main.go). Evita vazar stack trace pro admin.
func provisionerErrMsg(err error) string {
	if err == nil {
		return "erro interno"
	}
	msg := strings.TrimSpace(err.Error())
	if msg == "" {
		return "erro no provisionamento de conta Unix"
	}
	return msg
}
