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
//	xvpn-user-provision disable <username>
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
var errUsage = errors.New("uso: xvpn-user-provision <create|enable-sftp|enable-samba|disable> <username>")

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) != 2 {
		return errUsage
	}
	cmd, username := args[0], args[1]
	r := runnerFn()
	switch cmd {
	case "create":
		return provision.Create(r, username)
	case "enable-sftp":
		key, err := io.ReadAll(stdin)
		if err != nil {
			return fmt.Errorf("lendo chave pública do stdin: %w", err)
		}
		return provision.EnableSFTP(r, username, string(key))
	case "enable-samba":
		return provision.EnableSamba(r, username)
	case "disable-sftp":
		return provision.DisableSFTP(r, username)
	case "disable-samba":
		return provision.DisableSamba(r, username)
	case "disable":
		return provision.Disable(r, username)
	default:
		return errUsage
	}
}

// runnerFn devolve o Runner usado por run(). Em produção é
// provision.NewRunner (osRunner, chamadas reais); em teste, injetamos
// um fake pra validar a dispatch sem precisar de root. Padrão comum
// de "variável de pacote injetável" em CLIs Go testáveis.
var runnerFn = provision.NewRunner
