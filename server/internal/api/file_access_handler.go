package api

import (
	"context"
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
}

// fileAccessRequest é o corpo do PUT /api/users/:id/file-access. Os
// três campos formam o estado DESEJADO pelo admin — o handler calcula
// o diff contra o estado atual e aplica só o que mudou.
type fileAccessRequest struct {
	SFTPEnabled  bool   `json:"sftp_enabled"`
	SambaEnabled bool   `json:"samba_enabled"`
	SSHPublicKey string `json:"ssh_public_key"`
}

// fileAccessResponse espelha o estado persistido no User depois de
// aplicar a requisição — devolvido pra o painel confirmar o que ficou.
type fileAccessResponse struct {
	SFTPEnabled  bool   `json:"sftp_enabled"`
	SambaEnabled bool   `json:"samba_enabled"`
	SSHPublicKey string `json:"ssh_public_key"`
}

// validSSHPublicKey faz checagem superficial de formato — não é
// validação criptográfica (isso o sshd faz no login), só rejeita lixo
// óbvio. Aceita múltiplas chaves (uma por linha), comentários (#) e
// linhas em branco. Vazio é válido (SFTP fica bloqueado até colar uma).
func validSSHPublicKey(key string) bool {
	key = strings.TrimSpace(key)
	if key == "" {
		return true
	}
	accepted := map[string]bool{
		"ssh-rsa": true, "ssh-ed25519": true,
		"ecdsa-sha2-nistp256": true, "ecdsa-sha2-nistp384": true, "ecdsa-sha2-nistp521": true,
		"sk-ssh-ed25519@openssh.com": true, "sk-ecdsa-sha2-nistp256@openssh.com": true,
	}
	for _, line := range strings.Split(key, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || !accepted[fields[0]] {
			return false
		}
		if len(fields[1]) < 10 || len(fields[1]) > 8192 {
			return false
		}
	}
	return true
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
