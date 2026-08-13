//go:build linux

// Package linux implementa o motor de túnel (tunnel.Engine) usando a
// interface WireGuard nativa do kernel Linux — mesma dupla netlink+wgctrl
// que o control-plane do servidor usa (server/internal/wireguard), em vez
// do motor userspace wireguard-go usado no Windows (ver PLAN.md §3.2: aqui
// o kernel já resolve isso sem dependência externa nenhuma).
package linux

import (
	"fmt"
	"net"
	"os/exec"
	"sync"
	"time"

	"github.com/vishvananda/netlink"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/rootkit-lab/xvpn/client/internal/tunnel"
)

// ifaceName é distinto de "wg0" (usado pelo servidor) para nunca colidir
// caso cliente e servidor sejam testados na mesma máquina durante
// desenvolvimento — ver ROADMAP.md Fase 1.
const ifaceName = "xvpn0"

// defaultMTU replica o achado da Fase 1 (ROADMAP.md): 1420 é seguro mesmo
// atrás de outra VPN/rede com PMTU reduzido; cfg.MTU pode sobrescrever.
const defaultMTU = 1420

// Engine implementa tunnel.Engine usando o kernel WireGuard nativo.
type Engine struct {
	mu sync.Mutex

	client *wgctrl.Client

	connected      bool
	cfg            tunnel.Config
	connectedSince time.Time

	// hostRoute é a rota de exceção para o IP público do servidor, via o
	// gateway/interface original — sem ela, ao adicionar a rota padrão via
	// xvpn0 os próprios pacotes UDP do WireGuard para o servidor ficariam
	// presos num loop de roteamento.
	hostRoute *netlink.Route

	// originalDefaultRoute é a rota padrão pré-existente (via eth0/gateway
	// original), capturada antes de conectar. Em túnel completo (AllowedIPs
	// contendo 0.0.0.0/0), a rota 0.0.0.0/0 adicionada via xvpn0 em Connect
	// SUBSTITUI essa entrada na tabela principal (netlink.RouteReplace é um
	// NLM_F_REPLACE — não empilha) em vez de coexistir com ela. Por isso
	// precisa ser reaplicada explicitamente em teardown, ou a máquina fica
	// sem rota padrão alguma depois de Disconnect — achado do teste E2E em
	// Docker na Fase 4 (ver ROADMAP.md).
	originalDefaultRoute *netlink.Route

	dnsApplied       bool
	killSwitchActive bool
}

// New abre o cliente wgctrl. Requer CAP_NET_ADMIN (ou root) — ver
// client/deploy/systemd/xvpn-client-helper.service.
func New() (*Engine, error) {
	client, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("abrindo cliente wgctrl: %w", err)
	}
	return &Engine{client: client}, nil
}

var _ tunnel.Engine = (*Engine)(nil)

func (e *Engine) Connect(cfg tunnel.Config) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.connected {
		// keepKillSwitch=true: isto é uma reconfiguração (reconectar com
		// config nova, inclusive um reconnect automático depois de uma
		// queda — ver internal/helper), não um Disconnect() pedido pelo
		// usuário. Se o kill switch estava ativo, ele tem que continuar
		// bloqueando tráfego durante a troca de interface — inclusive se
		// esta tentativa de Connect falhar adiante — senão o intervalo
		// entre desfazer o túnel antigo e o novo ficar de pé seria uma
		// janela real de vazamento, exatamente o que o kill switch existe
		// pra evitar.
		if err := e.teardown(true); err != nil {
			return fmt.Errorf("desfazendo túnel anterior antes de reconectar: %w", err)
		}
	}

	serverAddr, err := net.ResolveUDPAddr("udp", cfg.ServerEndpoint)
	if err != nil {
		return fmt.Errorf("não foi possível resolver %q: %w", cfg.ServerEndpoint, err)
	}

	// A rota de exceção precisa existir ANTES de mexer na rota padrão,
	// senão o próprio handshake WireGuard para o servidor fica sem rota
	// válida no meio da troca.
	hostRoute, originalDefault, err := addHostRouteException(serverAddr.IP)
	if err != nil {
		return fmt.Errorf("adicionando rota de exceção para %s: %w", serverAddr.IP, err)
	}

	// success só vira true no fim da função, depois de todo o setup dar
	// certo. Qualquer "return" antes disso aciona o rollback completo no
	// defer — inclusive removendo a interface xvpn0, que senão ficaria
	// "up" e half-configured segurando a rota padrão (0.0.0.0/0 via
	// xvpn0), quebrando a rede do host mesmo com Connect() tendo
	// retornado erro. Achado do teste E2E em Docker na Fase 4.
	success := false
	defer func() {
		if success {
			return
		}
		if link, err := netlink.LinkByName(ifaceName); err == nil {
			_ = netlink.LinkDel(link)
		}
		_ = removeRoute(hostRoute)
		if originalDefault != nil {
			_ = netlink.RouteReplace(originalDefault)
		}
		// Kill switch propositalmente NÃO é desativado aqui: se esta
		// tentativa de Connect falhou e o kill switch já estava ativo
		// (de uma conexão anterior), o comportamento fail-closed correto
		// é continuar bloqueando tráfego fora do túnel até uma
		// reconexão bem-sucedida ou um Disconnect() explícito — nunca
		// voltar a liberar a internet silenciosamente só porque uma
		// tentativa de reconectar deu erro.
	}()

	link, err := ensureLink()
	if err != nil {
		return err
	}

	mtu := cfg.MTU
	if mtu <= 0 {
		mtu = defaultMTU
	}
	if err := netlink.LinkSetMTU(link, mtu); err != nil {
		return fmt.Errorf("configurando MTU de %q: %w", ifaceName, err)
	}

	addr, err := netlink.ParseAddr(cfg.Address)
	if err != nil {
		return fmt.Errorf("endereço %q inválido: %w", cfg.Address, err)
	}
	if err := ensureAddr(link, addr); err != nil {
		return err
	}

	if err := netlink.LinkSetUp(link); err != nil {
		return fmt.Errorf("subindo interface %q: %w", ifaceName, err)
	}

	peerKey, err := wgtypes.ParseKey(cfg.ServerPublicKey)
	if err != nil {
		return fmt.Errorf("chave pública do servidor inválida: %w", err)
	}
	allowedIPs, err := parseCIDRs(cfg.AllowedIPs)
	if err != nil {
		return err
	}
	keepalive := cfg.PersistentKeepalive
	if keepalive <= 0 {
		keepalive = 25 * time.Second
	}

	privateKey := cfg.PrivateKey
	err = e.client.ConfigureDevice(ifaceName, wgtypes.Config{
		PrivateKey:   &privateKey,
		ReplacePeers: true,
		Peers: []wgtypes.PeerConfig{
			{
				PublicKey:                   peerKey,
				Endpoint:                    serverAddr,
				ReplaceAllowedIPs:           true,
				AllowedIPs:                  allowedIPs,
				PersistentKeepaliveInterval: &keepalive,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("configurando peer do servidor em %q: %w", ifaceName, err)
	}

	for _, ipNet := range allowedIPs {
		ipNet := ipNet
		route := &netlink.Route{LinkIndex: link.Attrs().Index, Dst: &ipNet, Scope: netlink.SCOPE_LINK}
		if err := netlink.RouteReplace(route); err != nil {
			// A rota ::/0 é só um blackhole anti-vazamento de IPv6 (o túnel
			// só tem endereço/subnet IPv4 — ver PLAN.md), não conectividade
			// real. Em hosts/containers sem stack IPv6 na interface (ex.:
			// kernel com ipv6.disable=1, ou o netns de teste em Docker desta
			// Fase 4), falhar aqui não pode derrubar o túnel IPv4 inteiro:
			// sem IPv6 utilizável já não há vazamento possível de qualquer
			// forma. Rotas IPv4 continuam sendo erro fatal.
			if ipNet.IP.To4() == nil {
				continue
			}
			return fmt.Errorf("adicionando rota %s via %q: %w", ipNet.String(), ifaceName, err)
		}
	}

	if len(cfg.DNS) > 0 {
		if err := applyDNS(cfg.DNS); err != nil {
			// DNS é best-effort: sem systemd-resolved, seguimos conectados
			// mas sem resolução automática — documentado como limitação
			// conhecida (ver ROADMAP.md Fase 4).
			e.dnsApplied = false
		} else {
			e.dnsApplied = true
		}
	}

	// Kill switch é o último passo: se falhar, o defer acima desfaz tudo
	// e Connect retorna erro em vez de deixar o usuário "conectado, mas
	// sem a proteção que pediu" — fail-closed também na ativação, não só
	// depois (ver .cursor/rules/go-client.mdc). Só reaplica se ainda não
	// estava ativo (ver keepKillSwitch acima) — evita uma janela sem
	// bloqueio ao simplesmente recriar a mesma tabela nftables de novo.
	switch {
	case cfg.KillSwitch && !e.killSwitchActive:
		if err := enableKillSwitch(serverAddr.IP); err != nil {
			return fmt.Errorf("ativando kill switch: %w", err)
		}
		e.killSwitchActive = true
	case !cfg.KillSwitch && e.killSwitchActive:
		// Preferência foi desligada enquanto conectado (ver
		// handleSetPreferences) — remove de fato, senão o usuário fica
		// bloqueado sem saber por quê.
		if err := disableKillSwitch(); err != nil {
			return fmt.Errorf("desativando kill switch: %w", err)
		}
		e.killSwitchActive = false
	}

	e.hostRoute = hostRoute
	e.originalDefaultRoute = originalDefault
	e.connected = true
	e.cfg = cfg
	e.connectedSince = time.Now()
	success = true
	return nil
}

func (e *Engine) Disconnect() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.connected {
		return nil
	}
	// keepKillSwitch=false: Disconnect() é sempre uma ação explícita do
	// usuário (ou do helper desistindo de reconectar) — aqui sim a
	// internet normal deve voltar a funcionar.
	return e.teardown(false)
}

// teardown desfaz tudo que Connect fez. Assume e.mu já travado.
// keepKillSwitch preserva as regras de bloqueio ativas mesmo depois do
// teardown — usado pelo caminho de reconexão em Connect (ver acima), nunca
// por um Disconnect() de verdade.
func (e *Engine) teardown(keepKillSwitch bool) error {
	if e.killSwitchActive && !keepKillSwitch {
		_ = disableKillSwitch()
		e.killSwitchActive = false
	}

	if e.dnsApplied {
		revertDNS()
		e.dnsApplied = false
	}

	if e.hostRoute != nil {
		_ = removeRoute(e.hostRoute)
		e.hostRoute = nil
	}

	// Deletar o link remove automaticamente endereço e rotas anexadas a
	// ele (as rotas AllowedIPs adicionadas em Connect) — incluindo, em
	// túnel completo, a rota 0.0.0.0/0 que havia substituído a rota padrão
	// original na tabela principal.
	if link, err := netlink.LinkByName(ifaceName); err == nil {
		_ = netlink.LinkDel(link)
	}

	// Restaura a rota padrão original (ver comentário no campo
	// originalDefaultRoute) — RouteReplace é idempotente mesmo se ela já
	// estiver presente (túnel dividido, sem 0.0.0.0/0 nas AllowedIPs).
	if e.originalDefaultRoute != nil {
		_ = netlink.RouteReplace(e.originalDefaultRoute)
		e.originalDefaultRoute = nil
	}

	e.connected = false
	e.cfg = tunnel.Config{}
	return nil
}

func (e *Engine) Status() (tunnel.Status, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.connected {
		return tunnel.Status{Connected: false, KillSwitchActive: e.killSwitchActive}, nil
	}

	status := tunnel.Status{
		Connected:        true,
		AssignedIP:       e.cfg.Address,
		ServerEndpoint:   e.cfg.ServerEndpoint,
		ConnectedSince:   &e.connectedSince,
		KillSwitchActive: e.killSwitchActive,
	}

	device, err := e.client.Device(ifaceName)
	if err != nil {
		// A interface pode ter sido removida por fora (ex.: NetworkManager
		// hostil) — reporta conectado=false em vez de erro, já que do
		// ponto de vista do usuário o túnel simplesmente caiu. Mas o kill
		// switch (tabela nftables separada, não depende da interface WG
		// existir) continua bloqueando de fato nesse meio-tempo — reportar
		// KillSwitchActive=false aqui seria uma mentira na UI/diagnóstico
		// bem no momento em que ele mais importa (ver teste E2E da Fase 6).
		return tunnel.Status{Connected: false, KillSwitchActive: e.killSwitchActive}, nil
	}
	for _, p := range device.Peers {
		if p.PublicKey.String() == e.cfg.ServerPublicKey {
			status.ReceiveBytes = p.ReceiveBytes
			status.TransmitBytes = p.TransmitBytes
			if !p.LastHandshakeTime.IsZero() {
				t := p.LastHandshakeTime
				status.LastHandshake = &t
			}
			break
		}
	}
	return status, nil
}

func ensureLink() (netlink.Link, error) {
	link, err := netlink.LinkByName(ifaceName)
	if err == nil {
		return link, nil
	}
	if _, ok := err.(netlink.LinkNotFoundError); !ok {
		return nil, fmt.Errorf("consultando interface %q: %w", ifaceName, err)
	}
	wgLink := &netlink.Wireguard{LinkAttrs: netlink.LinkAttrs{Name: ifaceName}}
	if err := netlink.LinkAdd(wgLink); err != nil {
		return nil, fmt.Errorf("criando interface %q: %w", ifaceName, err)
	}
	return netlink.LinkByName(ifaceName)
}

func ensureAddr(link netlink.Link, addr *netlink.Addr) error {
	existing, err := netlink.AddrList(link, netlink.FAMILY_V4)
	if err != nil {
		return fmt.Errorf("listando endereços de %q: %w", ifaceName, err)
	}
	for _, a := range existing {
		if a.IPNet.String() == addr.IPNet.String() {
			return nil
		}
		// Endereço antigo de uma conexão anterior — remove antes de somar
		// o novo, para não deixar endereços órfãos acumulando.
		_ = netlink.AddrDel(link, &a)
	}
	if err := netlink.AddrAdd(link, addr); err != nil {
		return fmt.Errorf("atribuindo endereço %q a %q: %w", addr.String(), ifaceName, err)
	}
	return nil
}

func parseCIDRs(cidrs []string) ([]net.IPNet, error) {
	nets := make([]net.IPNet, 0, len(cidrs))
	for _, c := range cidrs {
		ip, ipNet, err := net.ParseCIDR(c)
		if err != nil {
			return nil, fmt.Errorf("allowed-ip %q inválido: %w", c, err)
		}
		ipNet.IP = ip
		nets = append(nets, *ipNet)
	}
	return nets, nil
}

// addHostRouteException garante que o tráfego para serverIP continue
// saindo pelo gateway/interface padrão original, mesmo depois da rota
// padrão da máquina passar a apontar para a xvpn0 — ver comentário em
// Connect.
func addHostRouteException(serverIP net.IP) (hostRoute *netlink.Route, originalDefault *netlink.Route, err error) {
	routes, err := netlink.RouteList(nil, netlink.FAMILY_V4)
	if err != nil {
		return nil, nil, fmt.Errorf("listando tabela de rotas: %w", err)
	}
	var defaultRoute *netlink.Route
	for i := range routes {
		// A rota padrão pode vir com Dst == nil OU com um IPNet 0.0.0.0/0
		// explícito, dependendo da versão do netlink/kernel — ver
		// vishvananda/netlink route_linux.go ("Same logic ... as iproute2").
		if routes[i].Dst == nil {
			defaultRoute = &routes[i]
			break
		}
		if ones, bits := routes[i].Dst.Mask.Size(); ones == 0 && bits > 0 {
			defaultRoute = &routes[i]
			break
		}
	}
	if defaultRoute == nil {
		return nil, nil, fmt.Errorf("nenhuma rota padrão encontrada (necessária para preservar acesso ao servidor)")
	}
	// Cópia por valor: defaultRoute aponta para dentro do slice `routes`,
	// que sai de escopo ao retornar — sem a cópia, o ponteiro salvo em
	// e.originalDefaultRoute ficaria sujeito a ser sobrescrito/reciclado.
	savedDefault := *defaultRoute

	route := &netlink.Route{
		LinkIndex: defaultRoute.LinkIndex,
		Gw:        defaultRoute.Gw,
		Dst:       &net.IPNet{IP: serverIP, Mask: net.CIDRMask(32, 32)},
	}
	if err := netlink.RouteReplace(route); err != nil {
		return nil, nil, err
	}
	return route, &savedDefault, nil
}

func removeRoute(route *netlink.Route) error {
	if route == nil {
		return nil
	}
	return netlink.RouteDel(route)
}

// applyDNS usa systemd-resolved (presente na maioria das distros
// modernas, incluindo a imagem-base do Ubuntu) para rotear todas as
// consultas DNS pela xvpn0 enquanto o túnel estiver ativo — o mesmo
// mecanismo que o `wg-quick` usa. Se resolvectl não existir, é um no-op:
// o túnel funciona, só sem DNS automático (limitação conhecida da Fase 4;
// revisitar num hardening futuro, ex. fallback para reescrever
// /etc/resolv.conf).
func applyDNS(dns []string) error {
	if _, err := exec.LookPath("resolvectl"); err != nil {
		return fmt.Errorf("resolvectl não encontrado")
	}
	args := append([]string{"dns", ifaceName}, dns...)
	if err := exec.Command("resolvectl", args...).Run(); err != nil {
		return fmt.Errorf("resolvectl dns: %w", err)
	}
	if err := exec.Command("resolvectl", "domain", ifaceName, "~.").Run(); err != nil {
		return fmt.Errorf("resolvectl domain: %w", err)
	}
	return nil
}

func revertDNS() {
	if _, err := exec.LookPath("resolvectl"); err == nil {
		_ = exec.Command("resolvectl", "revert", ifaceName).Run()
	}
}
