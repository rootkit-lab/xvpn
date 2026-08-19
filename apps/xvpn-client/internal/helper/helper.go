// Package helper implementa o processo privilegiado do xvpn-client: é o
// único lugar que fala com internal/tunnel.Engine (TUN/rotas/DNS) e com
// internal/apiclient (enrollment). A GUI nunca importa este pacote — só
// conversa com ele via internal/ipc, ver .cursor/rules/go-client.mdc.
package helper

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"sync"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/rootkit-lab/xvpn/client/internal/apiclient"
	"github.com/rootkit-lab/xvpn/client/internal/config"
	"github.com/rootkit-lab/xvpn/client/internal/intranet"
	"github.com/rootkit-lab/xvpn/client/internal/ipc"
	"github.com/rootkit-lab/xvpn/client/internal/tunnel"
)

// splitTunnelCIDR é a única sub-rede roteada pelo túnel quando a
// preferência SplitTunnel está ligada — a sub-rede WireGuard é fixa no
// projeto (ver AGENTS.md), nunca vem do servidor.
const splitTunnelCIDR = "10.66.66.0/24"

// logBufferLines é quantas linhas recentes de log o helper guarda em
// memória para a página de diagnóstico (ver logbuffer.go).
const logBufferLines = 500

// mtuMin/mtuMax delimitam o override manual do MTU: abaixo de 1280 o IPv6
// deixa de funcionar (é o mínimo exigido pela RFC 8200) e acima de 1500 o
// pacote não passa numa Ethernet comum. Zero significa "automático" (o
// padrão da plataforma, 1420).
const (
	mtuMin = 1280
	mtuMax = 1500
)

// EnrollRequest são os parâmetros do método RPC "enroll" — mesmo tipo é
// usado pelo cliente IPC do lado da GUI (ver apps/xvpn-client/vpnservice.go), já que
// ambos vivem no mesmo módulo Go.
type EnrollRequest struct {
	ServerBaseURL string `json:"server_base_url"`
	InviteToken   string `json:"invite_token"`
	DeviceName    string `json:"device_name"`
	// MTU é opcional (0 = usa o padrão da plataforma) — ver
	// config.DeviceState.MTU.
	MTU int `json:"mtu,omitempty"`
}

type EnrollResponse struct {
	AssignedIP string `json:"assigned_ip"`
}

// StatusResponse é o resultado do método RPC "status".
type StatusResponse struct {
	Connected      bool   `json:"connected"`
	Enrolled       bool   `json:"enrolled"`
	AssignedIP     string `json:"assigned_ip,omitempty"`
	ServerEndpoint string `json:"server_endpoint,omitempty"`
	// ServerBaseURL é a URL do painel usada no enrollment (não é
	// segredo — o usuário digitou ela na tela de enrollment) — exposta
	// aqui pra página de diagnóstico da GUI poder testar conectividade
	// com o painel sem precisar guardar isso duplicado no lado da GUI.
	ServerBaseURL    string     `json:"server_base_url,omitempty"`
	ConnectedSince   *time.Time `json:"connected_since,omitempty"`
	LastHandshake    *time.Time `json:"last_handshake,omitempty"`
	ReceiveBytes     int64      `json:"receive_bytes"`
	TransmitBytes    int64      `json:"transmit_bytes"`
	KillSwitchActive bool       `json:"kill_switch_active"`
	// Reconnecting/ReconnectAttempt refletem o monitor de reconexão
	// automática (ver reconnect.go) — a GUI usa isso pra mostrar "tentando
	// reconectar..." em vez de simplesmente "desconectado".
	Reconnecting     bool `json:"reconnecting"`
	ReconnectAttempt int  `json:"reconnect_attempt,omitempty"`
	// Username/SambaEnabled/SFTPEnabled vêm do cache de identidade em
	// config.DeviceState (atualizado a cada conexão via GET /api/me) —
	// a GUI precisa deles para abrir o share pessoal certo e explicar um
	// botão desabilitado, ver ROADMAP.md Fase 14.
	Username     string `json:"username,omitempty"`
	SambaEnabled bool   `json:"samba_enabled"`
	SFTPEnabled  bool   `json:"sftp_enabled"`
}

// MTUSetting atravessa os métodos RPC "get_mtu"/"set_mtu" nas duas
// direções (parâmetro e resultado têm a mesma forma). 0 = automático.
type MTUSetting struct {
	MTU int `json:"mtu"`
}

// Helper orquestra o motor de túnel, o cliente de API e o estado
// persistido do dispositivo.
type Helper struct {
	// mu protege os campos abaixo (state/desiredConnected/monitor/
	// reconnect*) — leitura/escrita rápida, nunca travado durante uma
	// chamada ao motor de túnel (netlink/rotas/DNS, potencialmente
	// lenta). Handlers de IPC que não tocam o motor (preferências, logs,
	// is_enrolled) por isso nunca ficam bloqueados esperando um
	// Connect/Disconnect em andamento (ver ROADMAP.md Fase 9).
	mu     sync.Mutex
	engine tunnel.Engine
	state  *config.DeviceState
	logs   *ringBuffer

	// engineMu serializa só as chamadas que mutam o motor de túnel
	// (Connect/Disconnect, inclusive as disparadas pela reconexão
	// automática em reconnect.go) — evita duas tentativas de
	// conectar/desconectar em paralelo sem prender mu o tempo todo.
	// Observação: Status() ainda pode ficar brevemente bloqueado pelo
	// mutex interno do próprio Engine (por plataforma) enquanto uma
	// mutação está em andamento — isso é intencional (nunca expor
	// estado parcialmente configurado), só não usa mais o Helper.mu
	// geral para isso.
	engineMu sync.Mutex

	// desiredConnected é o que o usuário pediu por último (Connect ou
	// Disconnect) — usado pelo monitor de reconexão pra saber se uma
	// queda do túnel é "o usuário desconectou" ou "caiu sem avisar" (ver
	// reconnect.go).
	desiredConnected bool
	monitorCancel    context.CancelFunc
	reconnecting     bool
	reconnectAttempt int

	// fetchIdentity e saveState existem para os testes conseguirem
	// exercitar os handlers sem depender de um túnel de verdade nem
	// escrever no arquivo de estado real (root-only, em /var/lib). Nulos
	// — como sempre são em produção — caem no comportamento real, ver
	// identity() e persistState().
	fetchIdentity func(context.Context) (*apiclient.MeResult, error)
	saveState     func(*config.DeviceState) error
}

// identity consulta o servidor por dentro do túnel (só ali a rota
// responde, ver apiclient.TunnelBaseURL).
func (h *Helper) identity(ctx context.Context) (*apiclient.MeResult, error) {
	if h.fetchIdentity != nil {
		return h.fetchIdentity(ctx)
	}
	return apiclient.New(apiclient.TunnelBaseURL).Me(ctx)
}

func (h *Helper) persistState(state *config.DeviceState) error {
	if h.saveState != nil {
		return h.saveState(state)
	}
	return config.Save(state)
}

// New cria o helper para a plataforma atual (motor selecionado via build
// tags — ver helper_linux.go/helper_windows.go) e carrega o estado de
// enrollment salvo, se houver.
func New() (*Helper, error) {
	engine, err := newEngine()
	if err != nil {
		return nil, fmt.Errorf("inicializando motor de túnel: %w", err)
	}
	state, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("carregando estado do dispositivo: %w", err)
	}
	return &Helper{engine: engine, state: state, logs: newRingBuffer(logBufferLines)}, nil
}

// Run inicia o socket/pipe IPC e bloqueia servindo requisições. Chamado só
// pelo entrypoint --helper (ver cmd principal em main.go).
func (h *Helper) Run() error {
	// A partir daqui, tudo que passar por log.Print* (deste processo)
	// também fica disponível via get_logs para a página de diagnóstico da
	// GUI — além de continuar indo pro stderr normal (journalctl no
	// systemd, ver apps/xvpn-client/deploy/systemd).
	log.SetOutput(io.MultiWriter(os.Stderr, h.logs))

	listener, err := ipc.Listen()
	if err != nil {
		return fmt.Errorf("iniciando canal IPC: %w", err)
	}
	defer listener.Close()

	server := ipc.NewServer()
	h.registerHandlers(server)

	slog.Info("helper ready", "enrolled", h.state != nil)
	return server.Serve(listener)
}

func (h *Helper) registerHandlers(server *ipc.Server) {
	server.Handle(ipc.MethodStatus, h.handleStatus)
	server.Handle(ipc.MethodIsEnrolled, h.handleIsEnrolled)
	server.Handle(ipc.MethodEnroll, h.handleEnroll)
	server.Handle(ipc.MethodConnect, h.handleConnect)
	server.Handle(ipc.MethodDisconnect, h.handleDisconnect)
	server.Handle(ipc.MethodGetPreferences, h.handleGetPreferences)
	server.Handle(ipc.MethodSetPreferences, h.handleSetPreferences)
	server.Handle(ipc.MethodGetMTU, h.handleGetMTU)
	server.Handle(ipc.MethodSetMTU, h.handleSetMTU)
	server.Handle(ipc.MethodGetLogs, h.handleGetLogs)
	server.HandlePeer(ipc.MethodMountSMB, h.handleMountSMB)
	server.HandlePeer(ipc.MethodUnmountSMB, h.handleUnmountSMB)
}

func (h *Helper) handleEnroll(raw json.RawMessage) (any, error) {
	var req EnrollRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("parâmetros de enrollment inválidos: %w", err)
	}
	if req.ServerBaseURL == "" || req.InviteToken == "" || req.DeviceName == "" {
		return nil, fmt.Errorf("servidor, código de convite e nome do dispositivo são obrigatórios")
	}

	client := apiclient.New(req.ServerBaseURL)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	serverStatus, err := client.CheckStatus(ctx)
	if err != nil {
		return nil, fmt.Errorf("não foi possível conectar a %s — confira o endereço e sua conexão com a internet: %w", req.ServerBaseURL, err)
	}
	if serverStatus.APIVersion != apiclient.SupportedAPIVersion {
		return nil, fmt.Errorf("servidor incompatível (versão de API %d, este cliente espera %d) — atualize o cliente ou o servidor", serverStatus.APIVersion, apiclient.SupportedAPIVersion)
	}

	result, err := client.Enroll(ctx, req.InviteToken, req.DeviceName)
	if err != nil {
		return nil, fmt.Errorf("enrollment falhou: %w", err)
	}

	state := &config.DeviceState{
		ServerBaseURL:       req.ServerBaseURL,
		DeviceName:          req.DeviceName,
		PrivateKey:          result.PrivateKey.String(),
		PublicKey:           result.PublicKey.String(),
		AssignedIP:          result.AssignedIP,
		ServerPublicKey:     result.ServerPublicKey,
		ServerEndpoint:      result.Endpoint,
		AllowedIPs:          result.AllowedIPs,
		PersistentKeepalive: int(result.PersistentKeepalive.Seconds()),
		MTU:                 req.MTU,
		EnrolledAt:          time.Now(),
		// Username chega já aqui (caminho rápido); os toggles de acesso a
		// arquivos só na primeira conexão, via GET /api/me — o enrollment
		// acontece fora do túnel, onde essa rota não responde.
		Username:      result.Username,
		DNS:           intranetDNS(result.DNS),
		IntranetHosts: toConfigHosts(result.IntranetHosts),
		// AutoReconnect começa ligado por padrão (ver ROADMAP.md Fase 6)
		// — é o comportamento que a maioria das VPNs pessoais espera.
		// KillSwitch e SplitTunnel começam desligados: são recursos
		// opt-in, com efeitos colaterais na rede que o usuário deve
		// escolher explicitamente na página de preferências.
		Preferences: config.Preferences{AutoReconnect: true},
	}
	if err := h.persistState(state); err != nil {
		return nil, fmt.Errorf("salvando estado do dispositivo: %w", err)
	}

	h.mu.Lock()
	h.state = state
	h.mu.Unlock()

	slog.Info("device enrolled",
		"device_name", state.DeviceName,
		"assigned_ip", state.AssignedIP,
		"server", state.ServerBaseURL,
	)
	return EnrollResponse{AssignedIP: state.AssignedIP}, nil
}

// buildTunnelConfig monta a config do motor de túnel a partir do estado
// persistido e das preferências atuais — chamado tanto por handleConnect
// quanto pela reconexão automática (reconnect.go) e por
// handleSetPreferences (para reaplicar imediatamente com o túnel já
// conectado). Assume h.mu já travado e h.state != nil.
func (h *Helper) buildTunnelConfig() (tunnel.Config, error) {
	privateKey, err := wgtypes.ParseKey(h.state.PrivateKey)
	if err != nil {
		return tunnel.Config{}, fmt.Errorf("estado do dispositivo corrompido (chave privada inválida): %w", err)
	}

	allowedIPs := h.state.AllowedIPs
	if h.state.Preferences.SplitTunnel {
		// Só a sub-rede da VPN passa pelo túnel — o resto do tráfego do
		// dispositivo sai direto pela rede local, sem passar pelo VPS.
		allowedIPs = []string{splitTunnelCIDR}
	}

	return tunnel.Config{
		PrivateKey:          privateKey,
		Address:             h.state.AssignedIP,
		ServerPublicKey:     h.state.ServerPublicKey,
		ServerEndpoint:      h.state.ServerEndpoint,
		AllowedIPs:          allowedIPs,
		DNS:                 intranetDNS(h.state.DNS),
		PersistentKeepalive: time.Duration(h.state.PersistentKeepalive) * time.Second,
		MTU:                 h.state.MTU,
		KillSwitch:          h.state.Preferences.KillSwitch,
	}, nil
}

func (h *Helper) handleConnect(_ json.RawMessage) (any, error) {
	h.mu.Lock()
	if h.state == nil {
		h.mu.Unlock()
		return nil, fmt.Errorf("este dispositivo ainda não fez enrollment — insira um código de convite primeiro")
	}
	cfg, err := h.buildTunnelConfig()
	assignedIP, endpoint := h.state.AssignedIP, h.state.ServerEndpoint
	cachedHosts := h.state.IntranetHosts
	h.mu.Unlock()
	if err != nil {
		return nil, err
	}

	h.engineMu.Lock()
	err = h.engine.Connect(cfg)
	h.engineMu.Unlock()
	if err != nil {
		slog.Error("connect failed", "err", err)
		return nil, fmt.Errorf("não foi possível conectar: %w", err)
	}
	applyIntranetHosts(cachedHosts)
	slog.Info("connected", "assigned_ip", assignedIP, "endpoint", endpoint)

	h.mu.Lock()
	h.desiredConnected = true
	h.startMonitor()
	h.mu.Unlock()

	h.refreshIdentity()
	return nil, nil
}

// refreshIdentity pergunta ao servidor, por dentro do túnel, quem é o dono
// deste dispositivo e se os acessos a arquivos estão liberados, guardando
// o resultado no estado persistido. Roda depois de um Connect
// bem-sucedido, quando a rota já existe — e é best-effort de propósito: o
// túnel está de pé, então uma falha aqui não pode virar um erro de conexão
// na cara do usuário. Também é o que resolve dispositivos enrolled antes
// da Fase 14, que nunca receberam o username.
func (h *Helper) refreshIdentity() {
	// A chamada HTTP fica fora de h.mu: o mutex nunca é segurado durante
	// I/O de rede, mesmo padrão do motor de túnel (ver engineMu).
	me, err := h.identity(context.Background())
	if err != nil {
		slog.Warn("identity refresh failed", "err", err)
		return
	}

	h.mu.Lock()
	if h.state == nil {
		h.mu.Unlock()
		return
	}
	hosts := toConfigHosts(me.IntranetHosts)
	unchanged := h.state.Username == me.Username &&
		h.state.SambaEnabled == me.SambaEnabled &&
		h.state.SFTPEnabled == me.SFTPEnabled &&
		sameHosts(h.state.IntranetHosts, hosts)
	h.state.Username = me.Username
	h.state.SambaEnabled = me.SambaEnabled
	h.state.SFTPEnabled = me.SFTPEnabled
	if len(hosts) > 0 {
		h.state.IntranetHosts = hosts
	}
	var saveErr error
	if !unchanged {
		saveErr = h.persistState(h.state)
	}
	cached := h.state.IntranetHosts
	h.mu.Unlock()
	applyIntranetHosts(cached)

	if saveErr != nil {
		slog.Warn("persisting identity failed", "err", saveErr)
		return
	}
	slog.Info("identity refreshed",
		"username", me.Username,
		"samba_enabled", me.SambaEnabled,
		"sftp_enabled", me.SFTPEnabled,
	)
}

func (h *Helper) handleDisconnect(_ json.RawMessage) (any, error) {
	h.mu.Lock()
	h.desiredConnected = false
	h.stopMonitorLocked()
	h.mu.Unlock()

	h.engineMu.Lock()
	err := h.engine.Disconnect()
	h.engineMu.Unlock()
	if err != nil {
		slog.Error("disconnect failed", "err", err)
		return nil, fmt.Errorf("não foi possível desconectar: %w", err)
	}
	if err := intranet.Revert(intranet.HostsPath()); err != nil {
		slog.Warn("intranet hosts revert failed", "err", err)
	}
	slog.Info("disconnected")
	return nil, nil
}

func (h *Helper) handleStatus(_ json.RawMessage) (any, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	engineStatus, err := h.engine.Status()
	if err != nil {
		return nil, fmt.Errorf("consultando status do túnel: %w", err)
	}

	resp := StatusResponse{
		Connected:        engineStatus.Connected,
		Enrolled:         h.state != nil,
		ReceiveBytes:     engineStatus.ReceiveBytes,
		TransmitBytes:    engineStatus.TransmitBytes,
		KillSwitchActive: engineStatus.KillSwitchActive,
		Reconnecting:     h.reconnecting,
		ReconnectAttempt: h.reconnectAttempt,
	}
	if engineStatus.Connected {
		resp.AssignedIP = engineStatus.AssignedIP
		resp.ServerEndpoint = engineStatus.ServerEndpoint
		resp.ConnectedSince = engineStatus.ConnectedSince
		resp.LastHandshake = engineStatus.LastHandshake
	}
	if h.state != nil {
		resp.ServerBaseURL = h.state.ServerBaseURL
		resp.Username = h.state.Username
		resp.SambaEnabled = h.state.SambaEnabled
		resp.SFTPEnabled = h.state.SFTPEnabled
	}
	return resp, nil
}

func (h *Helper) handleIsEnrolled(_ json.RawMessage) (any, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return map[string]bool{"enrolled": h.state != nil}, nil
}

func (h *Helper) handleGetPreferences(_ json.RawMessage) (any, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.state == nil {
		return config.Preferences{}, nil
	}
	return h.state.Preferences, nil
}

func (h *Helper) handleSetPreferences(raw json.RawMessage) (any, error) {
	var prefs config.Preferences
	if err := json.Unmarshal(raw, &prefs); err != nil {
		return nil, fmt.Errorf("preferências inválidas: %w", err)
	}

	h.mu.Lock()
	if h.state == nil {
		h.mu.Unlock()
		return nil, fmt.Errorf("este dispositivo ainda não fez enrollment")
	}
	h.state.Preferences = prefs
	saveErr := h.persistState(h.state)
	cfg, cfgErr := h.buildTunnelConfig()
	h.mu.Unlock()
	if saveErr != nil {
		return nil, fmt.Errorf("salvando preferências: %w", saveErr)
	}

	// Split-tunnel e kill switch mudam o comportamento do túnel em si —
	// se já estiver conectado, reaplica na hora em vez de só valer na
	// próxima conexão. engineMu (não h.mu) serializa essa chamada, para
	// não bloquear status/get-logs/etc. durante a reconfiguração (ver
	// ROADMAP.md Fase 9).
	h.engineMu.Lock()
	defer h.engineMu.Unlock()
	status, err := h.engine.Status()
	if err == nil && status.Connected {
		if cfgErr != nil {
			return nil, cfgErr
		}
		if err := h.engine.Connect(cfg); err != nil {
			return nil, fmt.Errorf("preferências salvas, mas falha ao reaplicar com o túnel já conectado: %w", err)
		}
	}
	return prefs, nil
}

func (h *Helper) handleGetMTU(_ json.RawMessage) (any, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.state == nil {
		return MTUSetting{}, nil
	}
	return MTUSetting{MTU: h.state.MTU}, nil
}

// handleSetMTU segue o mesmo desenho de handleSetPreferences: persiste e,
// se o túnel já estiver conectado, reaplica na hora — quem mexe no MTU
// normalmente está justamente tentando destravar uma conexão em curso
// ("black hole" de PMTU: handshake e ping passam, HTTP/TLS trava), então
// exigir reconectar à mão derrotaria o propósito.
func (h *Helper) handleSetMTU(raw json.RawMessage) (any, error) {
	var req MTUSetting
	if err := json.Unmarshal(raw, &req); err != nil {
		return nil, fmt.Errorf("MTU inválido: %w", err)
	}
	if req.MTU != 0 && (req.MTU < mtuMin || req.MTU > mtuMax) {
		return nil, fmt.Errorf("MTU deve ser 0 (automático) ou um valor entre %d e %d — abaixo de %d o IPv6 para de funcionar, acima de %d o pacote não passa numa rede Ethernet comum", mtuMin, mtuMax, mtuMin, mtuMax)
	}

	h.mu.Lock()
	if h.state == nil {
		h.mu.Unlock()
		return nil, fmt.Errorf("este dispositivo ainda não fez enrollment")
	}
	h.state.MTU = req.MTU
	saveErr := h.persistState(h.state)
	cfg, cfgErr := h.buildTunnelConfig()
	h.mu.Unlock()
	if saveErr != nil {
		return nil, fmt.Errorf("salvando MTU: %w", saveErr)
	}

	h.engineMu.Lock()
	defer h.engineMu.Unlock()
	status, err := h.engine.Status()
	if err == nil && status.Connected {
		if cfgErr != nil {
			return nil, cfgErr
		}
		if err := h.engine.Connect(cfg); err != nil {
			return nil, fmt.Errorf("MTU salvo, mas falha ao reaplicar com o túnel já conectado: %w", err)
		}
	}
	return MTUSetting{MTU: req.MTU}, nil
}

func (h *Helper) handleGetLogs(_ json.RawMessage) (any, error) {
	return map[string][]string{"lines": h.logs.Lines()}, nil
}

// intranetDNS garante o resolvedor da wg0 mesmo em devices enrolled
// antes da Fase 23 (estado sem o campo DNS).
func intranetDNS(fromServer []string) []string {
	if len(fromServer) > 0 {
		return fromServer
	}
	return []string{"10.66.66.1"}
}

func applyIntranetHosts(entries []config.HostEntry) {
	mapped := make([]intranet.HostEntry, 0, len(entries))
	for _, e := range entries {
		mapped = append(mapped, intranet.HostEntry{Hostname: e.Hostname, IPv4: e.IPv4})
	}
	if err := intranet.ApplyEntries(intranet.HostsPath(), mapped); err != nil {
		slog.Warn("intranet hosts apply failed", "err", err)
	}
}

func toConfigHosts(in []apiclient.HostEntry) []config.HostEntry {
	out := make([]config.HostEntry, 0, len(in))
	for _, e := range in {
		if e.Hostname == "" || e.IPv4 == "" {
			continue
		}
		out = append(out, config.HostEntry{Hostname: e.Hostname, IPv4: e.IPv4})
	}
	return out
}

func sameHosts(a, b []config.HostEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Hostname != b[i].Hostname || a[i].IPv4 != b[i].IPv4 {
			return false
		}
	}
	return true
}
