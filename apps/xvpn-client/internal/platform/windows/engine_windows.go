//go:build windows

// Package windows implementa o motor de túnel (tunnel.Engine) usando o
// motor WireGuard userspace de referência (wireguard-go: device+tun+conn)
// com o driver wintun como TUN — Windows não tem um WireGuard nativo de
// kernel como o Linux, então essa é a "engine embarcada" descrita em
// PLAN.md §3.2/§7.1.
//
// Atenção para quem for validar isso no Windows (ver ROADMAP.md Fase 4 e
// AGENTS.md — Windows só é buildado/testado pelo usuário, nunca aqui): este
// arquivo compila via cross-compilation (GOOS=windows) mas nunca foi
// executado de fato num Windows real. Configuração de IP/rota/DNS usa
// `netsh` por simplicidade e depurabilidade — se algo não aplicar
// corretamente, rode os mesmos comandos `netsh` manualmente para comparar.
package windows

import (
	"encoding/hex"
	"fmt"
	"net"
	"net/netip"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/rootkit-lab/xvpn/client/internal/tunnel"
)

// ifaceName é o nome do adaptador wintun criado/reaproveitado — precisa
// bater com o que os comandos `netsh interface ...` referenciam.
const ifaceName = "XVPN"

const defaultMTU = 1420

// Engine implementa tunnel.Engine sobre wireguard-go + wintun.
type Engine struct {
	mu sync.Mutex

	dev *device.Device

	connected      bool
	cfg            tunnel.Config
	connectedSince time.Time

	// killSwitch é nil quando o bloqueio fail-closed não está ativo — ver
	// killswitch_windows.go e ROADMAP.md Fase 6. Ao contrário do Linux
	// (engine_linux.go), aqui o kill switch é sempre desfeito e refeito a
	// cada teardown/Connect — limitação conhecida documentada no
	// ROADMAP.md: numa reconexão automática existe uma janela breve sem
	// bloqueio enquanto a sessão WFP antiga é fechada e a nova ainda não
	// subiu, diferente do Linux onde as regras nftables persistem entre
	// tentativas.
	killSwitch *killSwitch
}

func New() (*Engine, error) {
	return &Engine{}, nil
}

var _ tunnel.Engine = (*Engine)(nil)

func (e *Engine) Connect(cfg tunnel.Config) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.connected {
		if err := e.teardown(); err != nil {
			return fmt.Errorf("desfazendo túnel anterior antes de reconectar: %w", err)
		}
	}

	if err := ensureWintunDLL(); err != nil {
		return err
	}

	mtu := cfg.MTU
	if mtu <= 0 {
		mtu = defaultMTU
	}

	tunDevice, err := tun.CreateTUN(ifaceName, mtu)
	if err != nil {
		return fmt.Errorf("criando adaptador wintun %q: %w", ifaceName, err)
	}

	logger := device.NewLogger(device.LogLevelError, "xvpn-client: ")
	dev := device.NewDevice(tunDevice, conn.NewDefaultBind(), logger)

	uapiConfig, err := buildUAPIConfig(cfg)
	if err != nil {
		dev.Close()
		return err
	}
	if err := dev.IpcSet(uapiConfig); err != nil {
		dev.Close()
		return fmt.Errorf("configurando dispositivo WireGuard: %w", err)
	}
	if err := dev.Up(); err != nil {
		dev.Close()
		return fmt.Errorf("subindo dispositivo WireGuard: %w", err)
	}

	if err := configureInterface(cfg); err != nil {
		dev.Close()
		return err
	}

	if cfg.KillSwitch {
		ks, err := enableKillSwitchForConfig(tunDevice, cfg)
		if err != nil {
			dev.Close()
			return fmt.Errorf("ativando kill switch: %w", err)
		}
		e.killSwitch = ks
	}

	e.dev = dev
	e.connected = true
	e.cfg = cfg
	e.connectedSince = time.Now()
	return nil
}

// enableKillSwitchForConfig resolve o LUID da interface recém-criada e o
// IP do servidor a partir de cfg, e então instala as regras WFP — ver
// killswitch_windows.go.
func enableKillSwitchForConfig(tunDevice tun.Device, cfg tunnel.Config) (*killSwitch, error) {
	nativeTun, ok := tunDevice.(*tun.NativeTun)
	if !ok {
		return nil, fmt.Errorf("adaptador TUN não é wintun nativo (tipo %T) — não é possível obter o LUID para o kill switch", tunDevice)
	}
	serverAddr, err := net.ResolveUDPAddr("udp", cfg.ServerEndpoint)
	if err != nil {
		return nil, fmt.Errorf("resolvendo %q: %w", cfg.ServerEndpoint, err)
	}
	serverIP, ok := netip.AddrFromSlice(serverAddr.IP.To4())
	if !ok {
		serverIP, ok = netip.AddrFromSlice(serverAddr.IP.To16())
		if !ok {
			return nil, fmt.Errorf("IP do servidor %q inválido", serverAddr.IP)
		}
	}
	return enableKillSwitch(nativeTun.LUID(), serverIP)
}

func (e *Engine) Disconnect() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.connected {
		return nil
	}
	return e.teardown()
}

func (e *Engine) teardown() error {
	if e.killSwitch != nil {
		_ = e.killSwitch.disable()
		e.killSwitch = nil
	}
	if e.dev != nil {
		// Close() do wireguard-go também fecha o tun.Device subjacente,
		// removendo o adaptador wintun e, com ele, IP/rotas associados.
		e.dev.Close()
		e.dev = nil
	}
	e.connected = false
	e.cfg = tunnel.Config{}
	return nil
}

func (e *Engine) Status() (tunnel.Status, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.connected {
		// Ver comentário equivalente em engine_linux.go: reporta o estado
		// real do kill switch mesmo desconectado (ex.: uma tentativa de
		// reconexão automática falhou e o kill switch permanece ativo de
		// propósito — fail-closed, ver Connect/teardown).
		return tunnel.Status{Connected: false, KillSwitchActive: e.killSwitch != nil}, nil
	}

	status := tunnel.Status{
		Connected:        true,
		AssignedIP:       e.cfg.Address,
		ServerEndpoint:   e.cfg.ServerEndpoint,
		ConnectedSince:   &e.connectedSince,
		KillSwitchActive: e.killSwitch != nil,
	}

	if e.dev != nil {
		if raw, err := e.dev.IpcGet(); err == nil {
			applyUAPIStats(raw, &status)
		}
	}
	return status, nil
}

// buildUAPIConfig monta a configuração no protocolo texto UAPI que
// device.Device.IpcSet espera — chaves em hexadecimal (não base64, ao
// contrário da representação usual do wg/wgctrl).
func buildUAPIConfig(cfg tunnel.Config) (string, error) {
	serverAddr, err := net.ResolveUDPAddr("udp", cfg.ServerEndpoint)
	if err != nil {
		return "", fmt.Errorf("não foi possível resolver %q: %w", cfg.ServerEndpoint, err)
	}
	peerPublicKey, err := wgtypes.ParseKey(cfg.ServerPublicKey)
	if err != nil {
		return "", fmt.Errorf("chave pública do servidor inválida: %w", err)
	}

	keepalive := cfg.PersistentKeepalive
	if keepalive <= 0 {
		keepalive = 25 * time.Second
	}

	var b strings.Builder
	fmt.Fprintf(&b, "private_key=%s\n", hex.EncodeToString(cfg.PrivateKey[:]))
	fmt.Fprintf(&b, "listen_port=0\n")
	fmt.Fprintf(&b, "public_key=%s\n", hex.EncodeToString(peerPublicKey[:]))
	fmt.Fprintf(&b, "endpoint=%s\n", serverAddr.String())
	fmt.Fprintf(&b, "persistent_keepalive_interval=%d\n", int(keepalive.Seconds()))
	for _, cidr := range cfg.AllowedIPs {
		fmt.Fprintf(&b, "allowed_ip=%s\n", cidr)
	}
	return b.String(), nil
}

// configureInterface atribui IP, rotas e DNS ao adaptador via `netsh` — ver
// aviso de plataforma não testada no topo do arquivo.
func configureInterface(cfg tunnel.Config) error {
	ip, ipNet, err := net.ParseCIDR(cfg.Address)
	if err != nil {
		return fmt.Errorf("endereço %q inválido: %w", cfg.Address, err)
	}
	mask := net.IP(ipNet.Mask).String()

	if err := runNetsh("interface", "ip", "set", "address", "name="+ifaceName, "static", ip.String(), mask); err != nil {
		return fmt.Errorf("configurando IP da interface %q: %w", ifaceName, err)
	}

	for _, cidr := range cfg.AllowedIPs {
		_, dst, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		// Erros de rota individuais são só logados (best-effort) — o
		// túnel já está de pé nesse ponto; falhar tudo por causa de uma
		// rota (ex.: IPv6 indisponível no host) seria pior para o usuário.
		_ = runNetsh("interface", "ipv4", "add", "route", dst.String(), "interface="+ifaceName, "metric=1")
	}

	if len(cfg.DNS) > 0 {
		_ = runNetsh("interface", "ip", "set", "dns", "name="+ifaceName, "static", cfg.DNS[0])
		for _, extra := range cfg.DNS[1:] {
			_ = runNetsh("interface", "ip", "add", "dns", "name="+ifaceName, extra)
		}
	}
	return nil
}

func runNetsh(args ...string) error {
	out, err := exec.Command("netsh", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("netsh %v: %w (%s)", args, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// applyUAPIStats extrai rx_bytes/tx_bytes/last_handshake_time_sec do texto
// UAPI devolvido por IpcGet (mesmo protocolo usado por `wg show`).
func applyUAPIStats(raw string, status *tunnel.Status) {
	for _, line := range strings.Split(raw, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		switch key {
		case "rx_bytes":
			if v, err := strconv.ParseInt(value, 10, 64); err == nil {
				status.ReceiveBytes = v
			}
		case "tx_bytes":
			if v, err := strconv.ParseInt(value, 10, 64); err == nil {
				status.TransmitBytes = v
			}
		case "last_handshake_time_sec":
			if v, err := strconv.ParseInt(value, 10, 64); err == nil && v > 0 {
				t := time.Unix(v, 0)
				status.LastHandshake = &t
			}
		}
	}
}
