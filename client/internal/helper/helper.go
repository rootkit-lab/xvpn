// Package helper implementa o processo privilegiado do xvpn-client: é o
// único lugar que fala com internal/tunnel.Engine (TUN/rotas/DNS) e com
// internal/apiclient (enrollment). A GUI nunca importa este pacote — só
// conversa com ele via internal/ipc, ver .cursor/rules/go-client.mdc.
package helper

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"

	"github.com/rootkit-lab/xvpn/client/internal/apiclient"
	"github.com/rootkit-lab/xvpn/client/internal/config"
	"github.com/rootkit-lab/xvpn/client/internal/ipc"
	"github.com/rootkit-lab/xvpn/client/internal/tunnel"
)

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
	Connected      bool       `json:"connected"`
	Enrolled       bool       `json:"enrolled"`
	AssignedIP     string     `json:"assigned_ip,omitempty"`
	ServerEndpoint string     `json:"server_endpoint,omitempty"`
	ConnectedSince *time.Time `json:"connected_since,omitempty"`
	LastHandshake  *time.Time `json:"last_handshake,omitempty"`
	ReceiveBytes   int64      `json:"receive_bytes"`
	TransmitBytes  int64      `json:"transmit_bytes"`
}

// Helper orquestra o motor de túnel, o cliente de API e o estado
// persistido do dispositivo.
type Helper struct {
	mu     sync.Mutex
	engine tunnel.Engine
	state  *config.DeviceState
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
	return &Helper{engine: engine, state: state}, nil
}

// Run inicia o socket/pipe IPC e bloqueia servindo requisições. Chamado só
// pelo entrypoint --helper (ver cmd principal em main.go).
func (h *Helper) Run() error {
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

func (h *Helper) handleConnect(_ json.RawMessage) (any, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.state == nil {
		return nil, fmt.Errorf("este dispositivo ainda não fez enrollment — insira um código de convite primeiro")
	}

	privateKey, err := wgtypes.ParseKey(h.state.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("estado do dispositivo corrompido (chave privada inválida): %w", err)
	}

	cfg := tunnel.Config{
		PrivateKey:          privateKey,
		Address:             h.state.AssignedIP,
		ServerPublicKey:     h.state.ServerPublicKey,
		ServerEndpoint:      h.state.ServerEndpoint,
		AllowedIPs:          h.state.AllowedIPs,
		DNS:                 h.state.DNS,
		PersistentKeepalive: time.Duration(h.state.PersistentKeepalive) * time.Second,
		MTU:                 h.state.MTU,
	}
	if err := h.engine.Connect(cfg); err != nil {
		slog.Error("connect failed", "err", err)
		return nil, fmt.Errorf("não foi possível conectar: %w", err)
	}
	slog.Info("connected", "assigned_ip", h.state.AssignedIP, "endpoint", h.state.ServerEndpoint)
	return nil, nil
}

func (h *Helper) handleDisconnect(_ json.RawMessage) (any, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
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
		Connected:     engineStatus.Connected,
		Enrolled:      h.state != nil,
		ReceiveBytes:  engineStatus.ReceiveBytes,
		TransmitBytes: engineStatus.TransmitBytes,
	}
	if engineStatus.Connected {
		resp.AssignedIP = engineStatus.AssignedIP
		resp.ServerEndpoint = engineStatus.ServerEndpoint
		resp.ConnectedSince = engineStatus.ConnectedSince
		resp.LastHandshake = engineStatus.LastHandshake
	}
	return resp, nil
}

func (h *Helper) handleIsEnrolled(_ json.RawMessage) (any, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return map[string]bool{"enrolled": h.state != nil}, nil
}
