// Command xvpn-user-provision é o binário privilegiado (rodado como root
// via sudoers.d restrito — ver PLAN.md §6.9 e SECURITY.md) que cria
// contas Unix e habilita/desabilita acesso SFTP/Samba por usuário do
// painel. O processo xvpn-server nunca tem privilégio pra isso; ele só
// chama este binário via os/exec (ver internal/userprovision).
//
// Subcomandos:
//
//	xvpn-user-provision create <username>
//	xvpn-user-provision enable-sftp <username>   # lê a chave pública SSH do stdin
//	xvpn-user-provision enable-samba <username>
//	xvpn-user-provision set-quota <username> <mb>  # 0 = sem limite (Fase 15)
//	xvpn-user-provision disable <username>
//	xvpn-user-provision dns-apply                  # JSON no stdin (zona corp)
//	xvpn-user-provision svc-apply                  # JSON no stdin (serviço gerenciado)
//
// O username é validado via regex (ver provision.ValidUsername) ANTES de
// qualquer chamada de sistema — defesa em profundidade contra injeção
// de argumento, mesmo que o sudoers só permita o caminho exato do
// binário. Tudo é idempotente: chamar de novo com o mesmo argumento é
// no-op (ou reaplica o dono/permissão esperado).
//
// Saídas: 0 = sucesso; 2 = uso/argumento inválido; 1 = erro de runtime
// (useradd falhou, sshd -t rejeitou a config, etc.). As mensagens de
// erro vão pra stderr em texto puro (sem stack trace, sem detalhe
// interno — o xvpn-server repassa o stderr pro handler, que mostra pro
// admin).
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/rootkit-lab/xvpn/server/internal/provision"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		switch {
		case errors.Is(err, errUsage):
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		case errors.Is(err, provision.ErrInvalidUsername):
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		default:
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

// errUsage sinaliza erro de linha de comando (subcomando desconhecido,
// argumento faltando) — mapeado pra exit code 2, distinto de erros de
// runtime (exit 1). Separado pra run() ser testável sem os.Exit.
var errUsage = errors.New("uso: xvpn-user-provision <create|enable-sftp|enable-samba|set-quota|disable|dns-apply|svc-apply|…> [username] [mb]")

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) < 1 {
		return errUsage
	}
	if args[0] == "dns-apply" {
		if len(args) != 1 {
			return errUsage
		}
		return provision.ApplyDNS(runnerFn(), stdin)
	}
	if args[0] == "svc-apply" {
		if len(args) != 1 {
			return errUsage
		}
		return provision.ApplyService(svcRunnerFn(), stdin)
	}
	if len(args) < 2 {
		return errUsage
	}
	cmd, username := args[0], args[1]
	r := runnerFn()
	switch cmd {
	case "create":
		if len(args) != 2 {
			return errUsage
		}
		return provision.Create(r, username)
	case "enable-sftp":
		if len(args) != 2 {
			return errUsage
		}
		key, err := io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("lendo chave pública do stdin: %w", err)
		}
		return provision.EnableSFTP(r, username, string(key))
	case "enable-samba":
		if len(args) != 2 {
			return errUsage
		}
		return provision.EnableSamba(r, username)
	case "disable-sftp":
		if len(args) != 2 {
			return errUsage
		}
		return provision.DisableSFTP(r, username)
	case "disable-samba":
		if len(args) != 2 {
			return errUsage
		}
		return provision.DisableSamba(r, username)
	case "disable":
		if len(args) != 2 {
			return errUsage
		}
		return provision.Disable(r, username)
	case "set-quota":
		if len(args) != 3 {
			return errUsage
		}
		mb, err := strconv.ParseUint(args[2], 10, 64)
		if err != nil {
			return fmt.Errorf("%w: mb inválido", errUsage)
		}
		return provision.SetDiskQuotaMB(r, username, mb)
	default:
		return errUsage
	}
}

// runnerFn devolve o Runner usado por run(). Em produção é
// provision.NewRunner (osRunner, chamadas reais); em teste, injetamos
// um fake pra validar a dispatch sem precisar de root. Padrão comum
// de "variável de pacote injetável" em CLIs Go testáveis.
var runnerFn = provision.NewRunner

// svcRunnerFn isola apt/systemctl do apply de serviços (Fase 43).
var svcRunnerFn = provision.NewSvcRunner
