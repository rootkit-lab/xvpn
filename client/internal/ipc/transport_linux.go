//go:build linux

package ipc

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// SocketPath é onde o helper escuta. Diretório com bit 0755 (root:root) e
// socket 0660 + grupo "xvpn": qualquer usuário local no grupo "xvpn" pode
// falar com o helper sem sudo, mas usuários fora do grupo não podem —
// mesmo modelo do grupo "docker". O instalador (Fase 7) cria o grupo e
// adiciona o usuário que instalou; em ambiente de desenvolvimento, rode
// `sudo usermod -aG xvpn $USER` manualmente (ver client/README.md).
const SocketPath = "/run/xvpn-client/helper.sock"

const socketGroup = "xvpn"

// Listen cria o socket Unix do helper. Chamado só pelo próprio helper
// (processo privilegiado).
func Listen() (net.Listener, error) {
	dir := filepath.Dir(SocketPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("criando %q: %w", dir, err)
	}
	// Socket órfão de uma execução anterior que crashou sem limpar.
	_ = os.Remove(SocketPath)

	l, err := net.Listen("unix", SocketPath)
	if err != nil {
		return nil, fmt.Errorf("escutando em %q: %w", SocketPath, err)
	}

	if err := os.Chmod(SocketPath, 0o660); err != nil {
		l.Close()
		return nil, fmt.Errorf("ajustando permissão de %q: %w", SocketPath, err)
	}
	// Best-effort: se o grupo "xvpn" não existir ainda (instalador não
	// rodou, ou execução manual em dev), o helper continua acessível só
	// via root/sudo — documentado, não é fatal.
	_ = exec.Command("chgrp", socketGroup, SocketPath).Run()

	return l, nil
}

// Dial conecta ao socket do helper. Chamado pelo processo GUI.
func Dial() (*Client, error) {
	conn, err := net.DialTimeout("unix", SocketPath, 3*time.Second)
	if err != nil {
		return nil, fmt.Errorf("conectando ao helper em %q (ele está rodando? ver systemctl status xvpn-client-helper): %w", SocketPath, err)
	}
	return newClient(conn), nil
}
