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

// EnrollRequest são os parâmetros do método RPC "enroll" — mesmo tipo é
// usado pelo cliente IPC do lado da GUI (ver client/vpnservice.go), já que
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
}

// Helper orquestra o motor de túnel, o cliente de API e o estado
// persistido do dispositivo.
type Helper struct {
	mu     sync.Mutex
	engine tunnel.Engine
	state  *config.DeviceState
	logs   *ringBuffer

	// desiredConnected é o que o usuário pediu por último (Connect ou
	// Disconnect) — usado pelo monitor de reconexão pra saber se uma
	// queda do túnel é "o usuário desconectou" ou "caiu sem avisar" (ver
	// reconnect.go).
	desiredConnected bool
	monitorCancel    context.CancelFunc
	reconnecting     bool
	reconnectAttempt int
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
	// systemd, ver client/deploy/systemd).
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
	server.Handle(ipc.MethodGetLogs, h.handleGetLogs)
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
		// AutoReconnect começa ligado por padrão (ver ROADMAP.md Fase 6)
		// — é o comportamento que a maioria das VPNs pessoais espera.
		// KillSwitch e SplitTunnel começam desligados: são recursos
		// opt-in, com efeitos colaterais na rede que o usuário deve
		// escolher explicitamente na página de preferências.
		Preferences: config.Preferences{AutoReconnect: true},
	}
	if err := config.Save(state); err != nil {
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
		DNS:                 h.state.DNS,
		PersistentKeepalive: time.Duration(h.state.PersistentKeepalive) * time.Second,
		MTU:                 h.state.MTU,
		KillSwitch:          h.state.Preferences.KillSwitch,
	}, nil
}

func (h *Helper) handleConnect(_ json.RawMessage) (any, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.state == nil {
		return nil, fmt.Errorf("este dispositivo ainda não fez enrollment — insira um código de convite primeiro")
	}

	cfg, err := h.buildTunnelConfig()
	if err != nil {
		return nil, err
	}
	if err := h.engine.Connect(cfg); err != nil {
		slog.Error("connect failed", "err", err)
		return nil, fmt.Errorf("não foi possível conectar: %w", err)
	}
	slog.Info("connected", "assigned_ip", h.state.AssignedIP, "endpoint", h.state.ServerEndpoint)
	h.desiredConnected = true
	h.startMonitor()
	return nil, nil
}

func (h *Helper) handleDisconnect(_ json.RawMessage) (any, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.desiredConnected = false
	h.stopMonitorLocked()
	if err := h.engine.Disconnect(); err != nil {
		slog.Error("disconnect failed", "err", err)
		return nil, fmt.Errorf("não foi possível desconectar: %w", err)
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
	defer h.mu.Unlock()
	if h.state == nil {
		return nil, fmt.Errorf("este dispositivo ainda não fez enrollment")
	}
	h.state.Preferences = prefs
	if err := config.Save(h.state); err != nil {
		return nil, fmt.Errorf("salvando preferências: %w", err)
	}

	// Split-tunnel e kill switch mudam o comportamento do túnel em si —
	// se já estiver conectado, reaplica na hora em vez de só valer na
	// próxima conexão.
	if status, err := h.engine.Status(); err == nil && status.Connected {
		cfg, err := h.buildTunnelConfig()
		if err != nil {
			return nil, err
		}
		if err := h.engine.Connect(cfg); err != nil {
			return nil, fmt.Errorf("preferências salvas, mas falha ao reaplicar com o túnel já conectado: %w", err)
		}
	}
	return prefs, nil
}

func (h *Helper) handleGetLogs(_ json.RawMessage) (any, error) {
	return map[string][]string{"lines": h.logs.Lines()}, nil
}
