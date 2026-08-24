// Command xvpn-git-shell é o forced command do usuário Unix `git` para
// git@xgit.corp.ihuull.com (Fase 7). O sshd invoca:
//
//	xvpn-git-shell <panel-username>
//
// com SSH_ORIGINAL_COMMAND = git-upload-pack|receive-pack '<org>/<slug>.git'.
// Lê config de /opt/xvpn/xvpn-server.env se presente.
package main

import (
	"fmt"
	"os"

	"github.com/rootkit-lab/xvpn/server/internal/config"
	"github.com/rootkit-lab/xvpn/server/internal/gitssh"
	"github.com/rootkit-lab/xvpn/server/internal/provision"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

const envFile = "/opt/xvpn/xvpn-server.env"

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}

func run() error {
	_ = config.LoadEnvFile(envFile)

	if len(os.Args) != 2 {
		return fmt.Errorf("uso: xvpn-git-shell <username>")
	}
	panelUser := os.Args[1]
	if !provision.ValidUsername(panelUser) {
		return fmt.Errorf("usuário inválido")
	}
	original := os.Getenv("SSH_ORIGINAL_COMMAND")
	if original == "" {
		return fmt.Errorf("SSH_ORIGINAL_COMMAND ausente")
	}

	cfg, err := config.LoadGitShell()
	if err != nil {
		return err
	}
	st, err := store.OpenReadOnly(cfg.DBPath)
	if err != nil {
		return err
	}

	svc := &gitssh.Service{
		Access: &gitssh.Access{DB: st.DB, GitDir: cfg.GitDir},
	}
	err = svc.Run(panelUser, original, os.Stdin, os.Stdout, os.Stderr)
	switch err {
	case gitssh.ErrNotFound:
		return fmt.Errorf("repositório não encontrado")
	case gitssh.ErrAccessDenied:
		return fmt.Errorf("acesso negado")
	default:
		return err
	}
}
