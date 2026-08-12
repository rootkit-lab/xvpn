// Package tunnel define o contrato cross-platform do motor WireGuard usado
// pelo helper privilegiado. A implementação real fica em internal/platform/
// (linux/ usa a interface WireGuard nativa do kernel via netlink+wgctrl;
// windows/ usa o motor userspace wireguard-go + wintun) — nunca use
// runtime.GOOS aqui, ver .cursor/rules/go-client.mdc.
package tunnel

import (
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// Config descreve os parâmetros necessários para estabelecer o túnel — é o
// resultado direto do enrollment (ver internal/apiclient) mais a chave
// privada, que nunca sai do processo do helper.
type Config struct {
	// PrivateKey nunca é logada, persistida em texto plano fora de um
	// arquivo com permissão restrita, nem enviada de volta a nenhuma API.
	PrivateKey wgtypes.Key

	// Address é o IP/máscara atribuído a este dispositivo pelo servidor
	// (ex.: "10.66.66.5/32").
	Address string

	// ServerPublicKey e ServerEndpoint identificam o único peer do túnel:
	// o próprio servidor XVPN.
	ServerPublicKey string
	ServerEndpoint  string

	// AllowedIPs replica o que o servidor devolveu no enrollment (full
	// tunnel: "0.0.0.0/0, ::/0").
	AllowedIPs []string

	// DNS é opcionalmente aplicado à interface/resolvedor do SO enquanto o
	// túnel estiver ativo (restaurado ao desconectar).
	DNS []string

	// PersistentKeepalive — sempre configurado; o cliente deve assumir que
	// pode estar atrás de NAT/CGNAT (ver .cursor/rules/go-client.mdc).
	PersistentKeepalive time.Duration

	// MTU é configurável para contornar o "black hole" de PMTU observado
	// na Fase 1 (ver ROADMAP.md) quando o dispositivo já está atrás de
	// outra VPN/rede restritiva. Zero usa o padrão da plataforma (1420).
	MTU int
}

// Status reporta o estado ao vivo do túnel para a UI.
type Status struct {
	Connected      bool
	AssignedIP     string
	ServerEndpoint string
	ConnectedSince *time.Time
	LastHandshake  *time.Time
	ReceiveBytes   int64
	TransmitBytes  int64
}

// Engine é implementado por internal/platform/{linux,windows} — cada
// plataforma cria/destrói a interface TUN e configura o peer único (o
// servidor) por trás desta interface comum.
type Engine interface {
	// Connect estabelece o túnel. Idempotente: chamar Connect enquanto já
	// conectado reconfigura o peer/interface com o Config novo em vez de
	// falhar.
	Connect(cfg Config) error

	// Disconnect desfaz o túnel e libera todos os recursos do SO (rotas,
	// DNS, interface). Idempotente: chamar sem estar conectado é um no-op.
	Disconnect() error

	// Status nunca deve falhar por não haver túnel ativo — nesse caso
	// devolve Status{Connected: false}.
	Status() (Status, error)
}
