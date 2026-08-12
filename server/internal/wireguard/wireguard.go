// Package wireguard é o único ponto do control-plane que fala diretamente
// com o kernel WireGuard, via wgctrl (nunca via exec.Command("wg", ...) —
// ver go-backend.mdc). Peers são adicionados/removidos dinamicamente em
// memória, sem jamais escrever/reler um arquivo wg0.conf.
package wireguard

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// PeerManager é o subconjunto de operações que os handlers HTTP precisam
// (adicionar/remover/listar peers). Extraído como interface para permitir
// testar a camada `api/` com um fake, sem precisar de CAP_NET_ADMIN/kernel
// real em ambiente de teste. `*Manager` implementa esta interface.
type PeerManager interface {
	AddPeer(spec PeerSpec) error
	RemovePeer(publicKey string) error
	ListPeers() ([]PeerStatus, error)
}

// Manager encapsula o cliente wgctrl e o nome da interface gerenciada.
type Manager struct {
	client    *wgctrl.Client
	ifaceName string
}

var _ PeerManager = (*Manager)(nil)

// PeerStatus é a visão de um peer exposta pela API (GET /api/devices),
// combinando dados estáticos (chave pública) com estatísticas ao vivo lidas
// direto do kernel.
type PeerStatus struct {
	PublicKey     string     `json:"public_key"`
	AllowedIPs    []string   `json:"allowed_ips"`
	Endpoint      string     `json:"endpoint,omitempty"`
	LastHandshake *time.Time `json:"last_handshake,omitempty"`
	ReceiveBytes  int64      `json:"receive_bytes"`
	TransmitBytes int64      `json:"transmit_bytes"`
}

// NewManager abre o socket de configuração do WireGuard (netlink genérico).
// Requer CAP_NET_ADMIN — ver server/deploy/systemd/xvpn-server.service.
func NewManager(ifaceName string) (*Manager, error) {
	client, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("abrindo cliente wgctrl: %w", err)
	}
	return &Manager{client: client, ifaceName: ifaceName}, nil
}

// Close libera o socket do wgctrl.
func (m *Manager) Close() error {
	return m.client.Close()
}

// ReadPrivateKey lê e decodifica a chave privada do servidor de um arquivo
// (gerada manualmente na Fase 1 — ver ROADMAP.md). O servidor nunca gera
// nem sobrescreve essa chave sozinho.
func ReadPrivateKey(path string) (wgtypes.Key, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return wgtypes.Key{}, fmt.Errorf("lendo chave privada em %q: %w", path, err)
	}
	key, err := wgtypes.ParseKey(strings.TrimSpace(string(raw)))
	if err != nil {
		return wgtypes.Key{}, fmt.Errorf("chave privada em %q inválida: %w", path, err)
	}
	return key, nil
}

// EnsureInterface garante que a interface WireGuard existe, tem o endereço
// IP correto, está "up", e está configurada com a chave privada/porta do
// servidor. É idempotente: seguro de chamar toda vez que o serviço sobe
// (inclusive quando a interface já foi criada manualmente, como na Fase 1,
// ou já existe de um boot anterior do próprio serviço).
func (m *Manager) EnsureInterface(privateKey wgtypes.Key, listenPort int, cidr string) error {
	link, err := netlink.LinkByName(m.ifaceName)
	if err != nil {
		if _, ok := err.(netlink.LinkNotFoundError); !ok {
			return fmt.Errorf("consultando interface %q: %w", m.ifaceName, err)
		}
		wgLink := &netlink.Wireguard{LinkAttrs: netlink.LinkAttrs{Name: m.ifaceName}}
		if err := netlink.LinkAdd(wgLink); err != nil {
			return fmt.Errorf("criando interface %q: %w", m.ifaceName, err)
		}
		link, err = netlink.LinkByName(m.ifaceName)
		if err != nil {
			return fmt.Errorf("interface %q criada mas não encontrada em seguida: %w", m.ifaceName, err)
		}
	}

	addr, err := netlink.ParseAddr(cidr)
	if err != nil {
		return fmt.Errorf("endereço %q inválido: %w", cidr, err)
	}
	existingAddrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("listando endereços de %q: %w", m.ifaceName, err)
	}
	hasAddr := false
	for _, a := range existingAddrs {
		if a.IPNet.String() == addr.IPNet.String() {
			hasAddr = true
			break
		}
	}
	if !hasAddr {
		if err := netlink.AddrAdd(link, addr); err != nil {
			return fmt.Errorf("atribuindo endereço %q a %q: %w", cidr, m.ifaceName, err)
		}
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("subindo interface %q: %w", m.ifaceName, err)
	}

	port := listenPort
	err = m.client.ConfigureDevice(m.ifaceName, wgtypes.Config{
		PrivateKey: &privateKey,
		ListenPort: &port,
	})
	if err != nil {
		return fmt.Errorf("configurando chave/porta de %q: %w", m.ifaceName, err)
	}

	return nil
}

// PeerSpec descreve um peer esperado, na forma que a camada de persistência
// (store.Device) fornece.
type PeerSpec struct {
	PublicKey string
	AllowedIP string // ex.: "10.66.66.5/32"
}

// ReconcilePeers substitui o conjunto de peers da interface pelo conjunto
// exato fornecido. Chamado na inicialização do servidor para garantir que o
// estado do kernel reflita o banco de dados mesmo após um restart do
// serviço (ex.: o serviço reiniciou mas um DELETE /api/devices ficou
// pendente de aplicar — nunca deveria acontecer, mas fica resiliente).
func (m *Manager) ReconcilePeers(specs []PeerSpec) error {
	peers := make([]wgtypes.PeerConfig, 0, len(specs))
	for _, spec := range specs {
		peer, err := buildPeerConfig(spec)
		if err != nil {
			return err
		}
		peers = append(peers, peer)
	}

	return m.client.ConfigureDevice(m.ifaceName, wgtypes.Config{
		ReplacePeers: true,
		Peers:        peers,
	})
}

// AddPeer registra um novo peer na interface, sem afetar os demais.
func (m *Manager) AddPeer(spec PeerSpec) error {
	peer, err := buildPeerConfig(spec)
	if err != nil {
		return err
	}
	return m.client.ConfigureDevice(m.ifaceName, wgtypes.Config{
		Peers: []wgtypes.PeerConfig{peer},
	})
}

// RemovePeer revoga um peer imediatamente (o dispositivo para de conseguir
// estabelecer handshake a partir desta chamada).
func (m *Manager) RemovePeer(publicKey string) error {
	key, err := wgtypes.ParseKey(publicKey)
	if err != nil {
		return fmt.Errorf("chave pública %q inválida: %w", publicKey, err)
	}
	return m.client.ConfigureDevice(m.ifaceName, wgtypes.Config{
		Peers: []wgtypes.PeerConfig{
			{PublicKey: key, Remove: true},
		},
	})
}

// ListPeers retorna o estado ao vivo (handshake, bytes, endpoint) de todos
// os peers configurados na interface, direto do kernel.
func (m *Manager) ListPeers() ([]PeerStatus, error) {
	device, err := m.client.Device(m.ifaceName)
	if err != nil {
		return nil, fmt.Errorf("consultando dispositivo %q: %w", m.ifaceName, err)
	}

	statuses := make([]PeerStatus, 0, len(device.Peers))
	for _, p := range device.Peers {
		allowedIPs := make([]string, 0, len(p.AllowedIPs))
		for _, ip := range p.AllowedIPs {
			allowedIPs = append(allowedIPs, ip.String())
		}

		status := PeerStatus{
			PublicKey:     p.PublicKey.String(),
			AllowedIPs:    allowedIPs,
			ReceiveBytes:  p.ReceiveBytes,
			TransmitBytes: p.TransmitBytes,
		}
		if !p.LastHandshakeTime.IsZero() {
			t := p.LastHandshakeTime
			status.LastHandshake = &t
		}
		if p.Endpoint != nil {
			status.Endpoint = p.Endpoint.String()
		}
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func buildPeerConfig(spec PeerSpec) (wgtypes.PeerConfig, error) {
	key, err := wgtypes.ParseKey(spec.PublicKey)
	if err != nil {
		return wgtypes.PeerConfig{}, fmt.Errorf("chave pública %q inválida: %w", spec.PublicKey, err)
	}
	ip, ipNet, err := net.ParseCIDR(spec.AllowedIP)
	if err != nil {
		return wgtypes.PeerConfig{}, fmt.Errorf("allowed-ip %q inválido: %w", spec.AllowedIP, err)
	}
	ipNet.IP = ip
	return wgtypes.PeerConfig{
		PublicKey:         key,
		ReplaceAllowedIPs: true,
		AllowedIPs:        []net.IPNet{*ipNet},
	}, nil
}
