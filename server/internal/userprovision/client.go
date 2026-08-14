// Package userprovision é o cliente usado pelo processo xvpn-server (que
// roda como usuário de sistema xvpn, só com CAP_NET_ADMIN — sem root)
// para invocar o binário privilegiado xvpn-user-provision (Fase 13, ver
// PLAN.md §6.9 e internal/provision). O xvpn-server nunca ganha
// privilégio de root diretamente; ele chama este binário via sudo, e o
// sudoers.d restrito (caminho exato, sem wildcard de argumento) é o que
// autoriza essa única chamada.
//
// Segurança: nunca usa `sh -c` com concatenação de string (ver
// go-backend.mdc) — sempre exec.Command com args isolados. O username
// é validado pelo próprio binário (ver provision.ValidUsername) antes
// de qualquer chamada de sistema, mas o handler do painel também valida
// antes de chegar aqui (defesa em profundidade).
package userprovision

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ErrBinaryMissing é devolvido quando o caminho do binário configurado
// não existe no disco — caso típico de dev/teste sem o binário
// instalado, ou deploy da Fase 13 incompleto. Distinto de erro de
// execução (o binário existe mas falhou), pra o handler poder decidir
// se loga como warning (configuração faltando) ou erro (provisionamento
// quebrou de verdade).
var ErrBinaryMissing = errors.New("binário xvpn-user-provision não encontrado (instalação da Fase 13 incompleta?)")

// Client chama o binário privilegiado xvpn-user-provision via sudo.
// Seguro para uso concorrente (cada método monta um exec.Command
// novo; nenhum estado compartilhado entre chamadas).
type Client struct {
	binaryPath string
	executor   executor
}

// executor isola a chamada de exec.Command.Run — em produção usa-se
// osExecutor (que chama cmd.CombinedOutput de verdade); em teste
// injeta-se um fake que registra os args e devolve erro controlado.
// Isolar aqui (e não no handler) mantém o handler testável sem
// depender de sudo/binário instalado.
type executor func(ctx context.Context, name string, args []string, stdin string) ([]byte, error)

// osExecutor é a implementação de produção: roda o comando, captura
// stdout+stderr junto (o binário escreve erros em texto puro pro
// stderr, ver cmd/xvpn-user-provision/main.go), e respeita o ctx
// (cancelamento de request HTTP propaga pro subprocess).
func osExecutor(ctx context.Context, name string, args []string, stdin string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if stdin != "" {
		cmd.Stdin = bytes.NewBufferString(stdin)
	}
	out, err := cmd.CombinedOutput()
	return out, err
}

// New cria um Client que invoca o binário em binaryPath via sudo.
func New(binaryPath string) *Client {
	return &Client{binaryPath: binaryPath, executor: osExecutor}
}

// newWithExecutor é interno — usado só por testes pra injetar um
// executor fake. Não exportado pra não vazar a abstração de teste
// pra fora do pacote.
func newWithExecutor(binaryPath string, e executor) *Client {
	return &Client{binaryPath: binaryPath, executor: e}
}

// run é o núcleo comum: monta `sudo -n <binary> <subcmd> <username>`,
// injeta stdin (para enable-sftp), e traduz saídas. sudo -n = não
// interativo: se o sudoers exigisse senha, falha alto em vez de travar
// esperando input (não deveria acontecer — o sudoers.d é NOPASSWD).
func (c *Client) run(ctx context.Context, subcmd, username, stdin string) error {
	if c.binaryPath == "" {
		return ErrBinaryMissing
	}
	args := []string{"-n", c.binaryPath, subcmd, username}
	out, err := c.executor(ctx, "sudo", args, stdin)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			// Limpa o stderr do binário pra uma mensagem curta —
			// o handler repassa pro admin, então não queremos
			// stack trace ou detalhe interno (ver go-backend.mdc).
			msg := strings.TrimSpace(string(out))
			if msg == "" {
				msg = fmt.Sprintf("xvpn-user-provision %s %s falhou (exit %d)", subcmd, username, exitErr.ExitCode())
			}
			return fmt.Errorf("%s: %w", msg, err)
		}
		// Erro de execução (binário não existe, sudo não instalado,
		// ctx cancelado) — distingue "binário missing" do resto.
		if strings.Contains(err.Error(), "no such file") || strings.Contains(strings.ToLower(err.Error()), "not found") {
			return ErrBinaryMissing
		}
		return fmt.Errorf("executando sudo: %w", err)
	}
	return nil
}

// Create provisiona a conta Unix (sem habilitar SFTP/Samba). Idempotente.
func (c *Client) Create(ctx context.Context, username string) error {
	return c.run(ctx, "create", username, "")
}

// EnableSFTP habilita SFTP chrootado com a chave pública sshPublicKey
// (lida do painel, repassada via stdin pro binário). Idempotente.
func (c *Client) EnableSFTP(ctx context.Context, username, sshPublicKey string) error {
	return c.run(ctx, "enable-sftp", username, sshPublicKey)
}

// EnableSamba habilita o share [home-<username>] (guest + force user,
// auth via VPN — ver internal/provision.SambaShareConfig). Idempotente.
func (c *Client) EnableSamba(ctx context.Context, username string) error {
	return c.run(ctx, "enable-samba", username, "")
}

// Disable remove o acesso SFTP e Samba (preserva a conta e os dados).
// Idempotente.
func (c *Client) Disable(ctx context.Context, username string) error {
	return c.run(ctx, "disable", username, "")
}

// DisableSFTP remove só o acesso SFTP, mantendo o Samba. Idempotente.
// Necessário porque os toggles SFTP/Samba são independentes no painel.
func (c *Client) DisableSFTP(ctx context.Context, username string) error {
	return c.run(ctx, "disable-sftp", username, "")
}

// DisableSamba remove só o share Samba, mantendo o SFTP. Idempotente.
func (c *Client) DisableSamba(ctx context.Context, username string) error {
	return c.run(ctx, "disable-samba", username, "")
}
