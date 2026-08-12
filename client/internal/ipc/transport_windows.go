//go:build windows

package ipc

import (
	"fmt"
	"net"
	"time"

	"github.com/Microsoft/go-winio"
)

// PipePath é o Named Pipe onde o helper (rodando como Windows Service /
// LocalSystem) escuta.
const PipePath = `\\.\pipe\xvpn-client-helper`

// pipeSDDL restringe o pipe a "Authenticated Users" (AU) com leitura e
// escrita genéricas — equivalente em espírito ao grupo "xvpn" + modo 0660
// do Linux (ver transport_linux.go): qualquer usuário autenticado da
// máquina pode falar com o helper local, mas não um processo anônimo/rede.
const pipeSDDL = "D:P(A;;GRGW;;;AU)"

// Listen cria o Named Pipe do helper.
func Listen() (net.Listener, error) {
	l, err := winio.ListenPipe(PipePath, &winio.PipeConfig{SecurityDescriptor: pipeSDDL})
	if err != nil {
		return nil, fmt.Errorf("escutando em %q: %w", PipePath, err)
	}
	return l, nil
}

// Dial conecta ao Named Pipe do helper.
func Dial() (*Client, error) {
	timeout := 3 * time.Second
	conn, err := winio.DialPipe(PipePath, &timeout)
	if err != nil {
		return nil, fmt.Errorf("conectando ao helper em %q (o serviço XVPN Client Helper está rodando?): %w", PipePath, err)
	}
	return newClient(conn), nil
}
