package main

import (
	"fmt"
	"time"

	"github.com/rootkit-lab/xvpn/client/internal/helper"
	"github.com/rootkit-lab/xvpn/client/internal/ipc"
)

// VPNService é o serviço Wails vinculado ao frontend (bindings TS gerados
// automaticamente por `wails3 generate bindings`). Roda no processo GUI,
// sem privilégio, e é só um cliente IPC do helper — nunca toca
// TUN/rotas/DNS diretamente, ver .cursor/rules/go-client.mdc.
type VPNService struct{}

// EnrollArgs são os dados inseridos na tela de enrollment (ver
// server/web para o painel que gera o InviteToken).
type EnrollArgs struct {
	ServerBaseURL string `json:"serverBaseURL"`
	InviteToken   string `json:"inviteToken"`
	DeviceName    string `json:"deviceName"`
	// MTU é um campo avançado opcional (0 = padrão) — útil para quem já
	// está atrás de outra VPN/rede restritiva, ver ROADMAP.md Fase 1.
	MTU int `json:"mtu,omitempty"`
}

// StatusView é o estado exibido na tela principal.
type StatusView struct {
	HelperReachable bool       `json:"helperReachable"`
	Connected       bool       `json:"connected"`
	Enrolled        bool       `json:"enrolled"`
	AssignedIP      string     `json:"assignedIP"`
	ServerEndpoint  string     `json:"serverEndpoint"`
	ConnectedSince  *time.Time `json:"connectedSince"`
	LastHandshake   *time.Time `json:"lastHandshake"`
	ReceiveBytes    int64      `json:"receiveBytes"`
	TransmitBytes   int64      `json:"transmitBytes"`
}

// Status consulta o helper. Se o helper não estiver acessível (serviço não
// instalado/parado), devolve HelperReachable=false em vez de erro — a UI
// mostra uma mensagem acionável ("instale/inicie o serviço") em vez de uma
// tela de erro genérica.
func (s *VPNService) Status() (StatusView, error) {
	client, err := ipc.Dial()
	if err != nil {
		return StatusView{HelperReachable: false}, nil
	}
	defer client.Close()

	var resp helper.StatusResponse
	if err := client.Call(ipc.MethodStatus, nil, &resp); err != nil {
		return StatusView{}, fmt.Errorf("consultando status: %w", err)
	}
	return StatusView{
		HelperReachable: true,
		Connected:       resp.Connected,
		Enrolled:        resp.Enrolled,
		AssignedIP:      resp.AssignedIP,
		ServerEndpoint:  resp.ServerEndpoint,
		ConnectedSince:  resp.ConnectedSince,
		LastHandshake:   resp.LastHandshake,
		ReceiveBytes:    resp.ReceiveBytes,
		TransmitBytes:   resp.TransmitBytes,
	}, nil
}

// Enroll registra este dispositivo usando um código de convite gerado no
// painel web (ver server/web/src/pages/users-page.tsx).
func (s *VPNService) Enroll(args EnrollArgs) error {
	client, err := ipc.Dial()
	if err != nil {
		return fmt.Errorf("serviço xvpn-client-helper indisponível — verifique se está instalado e rodando: %w", err)
	}
	defer client.Close()

	req := helper.EnrollRequest{
		ServerBaseURL: args.ServerBaseURL,
		InviteToken:   args.InviteToken,
		DeviceName:    args.DeviceName,
		MTU:           args.MTU,
	}
	return client.Call(ipc.MethodEnroll, req, nil)
}

// Connect estabelece o túnel usando as credenciais do último enrollment.
func (s *VPNService) Connect() error {
	client, err := ipc.Dial()
	if err != nil {
		return fmt.Errorf("serviço xvpn-client-helper indisponível: %w", err)
	}
	defer client.Close()
	return client.Call(ipc.MethodConnect, nil, nil)
}

// Disconnect desfaz o túnel.
func (s *VPNService) Disconnect() error {
	client, err := ipc.Dial()
	if err != nil {
		return fmt.Errorf("serviço xvpn-client-helper indisponível: %w", err)
	}
	defer client.Close()
	return client.Call(ipc.MethodDisconnect, nil, nil)
}
