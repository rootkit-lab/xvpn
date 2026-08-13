package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/rootkit-lab/xvpn/client/internal/autostart"
	"github.com/rootkit-lab/xvpn/client/internal/helper"
	"github.com/rootkit-lab/xvpn/client/internal/ipc"
	"github.com/rootkit-lab/xvpn/client/internal/opener"
)

// serverVPNAddress é o IP fixo do servidor dentro do túnel WireGuard — não
// muda entre deployments (invariante do projeto, ver PLAN.md §5 e
// AGENTS.md). Samba/FileBrowser só respondem aqui, nunca no endpoint
// público do servidor.
const serverVPNAddress = "10.66.66.1"

// sharedSambaName é o nome do compartilhamento Samba criado na Fase 5 (ver
// server/deploy/samba/smb.conf).
const sharedSambaName = "shared"

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
	HelperReachable  bool       `json:"helperReachable"`
	Connected        bool       `json:"connected"`
	Enrolled         bool       `json:"enrolled"`
	AssignedIP       string     `json:"assignedIP"`
	ServerEndpoint   string     `json:"serverEndpoint"`
	ServerBaseURL    string     `json:"serverBaseURL"`
	ConnectedSince   *time.Time `json:"connectedSince"`
	LastHandshake    *time.Time `json:"lastHandshake"`
	ReceiveBytes     int64      `json:"receiveBytes"`
	TransmitBytes    int64      `json:"transmitBytes"`
	KillSwitchActive bool       `json:"killSwitchActive"`
	Reconnecting     bool       `json:"reconnecting"`
	ReconnectAttempt int        `json:"reconnectAttempt"`
}

// Preferences são os recursos opcionais da Fase 6 (ver ROADMAP.md) —
// mesmos campos/tags JSON de config.Preferences no helper, mas definidos
// aqui de novo (não importamos internal/config na GUI) para manter a
// separação de privilégio clara: a GUI só conhece a forma dos dados que
// trafegam por IPC, nunca o pacote que persiste o arquivo de estado com a
// chave privada — ver .cursor/rules/go-client.mdc.
type Preferences struct {
	KillSwitch    bool `json:"kill_switch"`
	SplitTunnel   bool `json:"split_tunnel"`
	AutoReconnect bool `json:"auto_reconnect"`
}

// DiagnosticsReport é o resultado de RunDiagnostics, pensado para ser
// exportado (copiado/salvo) pelo usuário ao pedir ajuda — nunca inclui a
// chave privada ou qualquer segredo, só metadados de conectividade.
type DiagnosticsReport struct {
	GeneratedAt             time.Time   `json:"generatedAt"`
	HelperReachable         bool        `json:"helperReachable"`
	Enrolled                bool        `json:"enrolled"`
	Connected               bool        `json:"connected"`
	KillSwitchActive        bool        `json:"killSwitchActive"`
	Preferences             Preferences `json:"preferences"`
	AssignedIP              string      `json:"assignedIP,omitempty"`
	ServerEndpoint          string      `json:"serverEndpoint,omitempty"`
	ServerBaseURL           string      `json:"serverBaseURL,omitempty"`
	LastHandshakeAgoSeconds *int        `json:"lastHandshakeAgoSeconds,omitempty"`
	ReceiveBytes            int64       `json:"receiveBytes"`
	TransmitBytes           int64       `json:"transmitBytes"`
	// PanelReachable/PanelLatencyMs testam se o painel web (fora do
	// túnel, pela internet normal) responde — útil pra diferenciar "sem
	// internet" de "internet ok, mas VPN não conecta".
	PanelReachable bool   `json:"panelReachable"`
	PanelLatencyMs *int64 `json:"panelLatencyMs,omitempty"`
	PanelError     string `json:"panelError,omitempty"`
	// VPNGatewayReachable/VPNGatewayLatencyMs só fazem sentido com o
	// túnel conectado — testam se o próprio servidor (10.66.66.1) está
	// respondendo dentro da VPN, ver server/deploy/filebrowser.
	VPNGatewayReachable bool   `json:"vpnGatewayReachable"`
	VPNGatewayLatencyMs *int64 `json:"vpnGatewayLatencyMs,omitempty"`
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
		HelperReachable:  true,
		Connected:        resp.Connected,
		Enrolled:         resp.Enrolled,
		AssignedIP:       resp.AssignedIP,
		ServerEndpoint:   resp.ServerEndpoint,
		ServerBaseURL:    resp.ServerBaseURL,
		ConnectedSince:   resp.ConnectedSince,
		LastHandshake:    resp.LastHandshake,
		ReceiveBytes:     resp.ReceiveBytes,
		TransmitBytes:    resp.TransmitBytes,
		KillSwitchActive: resp.KillSwitchActive,
		Reconnecting:     resp.Reconnecting,
		ReconnectAttempt: resp.ReconnectAttempt,
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

// OpenServerFiles abre o acesso a arquivos do servidor (Fase 5) no
// aplicativo padrão do SO. kind é "smb" (unidade de rede) ou "filebrowser"
// (interface web). Não passa pelo helper: é uma ação local da GUI que só
// funciona de fato com o túnel ativo (Samba/FileBrowser só escutam em
// wg0 — ver PLAN.md §3.4).
func (s *VPNService) OpenServerFiles(kind string) error {
	switch kind {
	case "smb":
		return opener.OpenSMBShare(serverVPNAddress, sharedSambaName)
	case "filebrowser":
		return opener.OpenURL(fmt.Sprintf("http://%s:8081", serverVPNAddress))
	default:
		return fmt.Errorf("tipo de acesso a arquivos desconhecido: %q", kind)
	}
}

// GetPreferences consulta as preferências atuais (kill switch,
// split-tunnel, reconexão automática) — ver ROADMAP.md Fase 6.
func (s *VPNService) GetPreferences() (Preferences, error) {
	client, err := ipc.Dial()
	if err != nil {
		return Preferences{}, fmt.Errorf("serviço xvpn-client-helper indisponível: %w", err)
	}
	defer client.Close()

	var prefs Preferences
	if err := client.Call(ipc.MethodGetPreferences, nil, &prefs); err != nil {
		return Preferences{}, fmt.Errorf("consultando preferências: %w", err)
	}
	return prefs, nil
}

// SetPreferences salva as preferências e, se o túnel já estiver
// conectado, reaplica imediatamente (ver handleSetPreferences no helper).
func (s *VPNService) SetPreferences(prefs Preferences) (Preferences, error) {
	client, err := ipc.Dial()
	if err != nil {
		return Preferences{}, fmt.Errorf("serviço xvpn-client-helper indisponível: %w", err)
	}
	defer client.Close()

	var result Preferences
	if err := client.Call(ipc.MethodSetPreferences, prefs, &result); err != nil {
		return Preferences{}, fmt.Errorf("salvando preferências: %w", err)
	}
	return result, nil
}

// GetAutostart/SetAutostart não passam pelo helper — inicialização
// automática é uma preferência do processo GUI sem privilégio (atalho no
// espaço do usuário/chave de registro HKCU), ver internal/autostart.
func (s *VPNService) GetAutostart() (bool, error) {
	return autostart.IsEnabled()
}

func (s *VPNService) SetAutostart(enabled bool) error {
	return autostart.SetEnabled(enabled)
}

// GetLogs devolve as últimas linhas do log do helper, para a página de
// diagnóstico.
func (s *VPNService) GetLogs() ([]string, error) {
	client, err := ipc.Dial()
	if err != nil {
		return nil, fmt.Errorf("serviço xvpn-client-helper indisponível: %w", err)
	}
	defer client.Close()

	var resp struct {
		Lines []string `json:"lines"`
	}
	if err := client.Call(ipc.MethodGetLogs, nil, &resp); err != nil {
		return nil, fmt.Errorf("consultando logs: %w", err)
	}
	return resp.Lines, nil
}

// RunDiagnostics junta o status atual do túnel com dois testes de
// conectividade simples (painel web pela internet normal, e o servidor
// dentro da VPN se o túnel estiver de pé) — pensado pra ajudar a
// diagnosticar "não conecta"/"conecta mas não navega" sem precisar abrir
// um terminal. Nunca inclui a chave privada ou qualquer segredo.
func (s *VPNService) RunDiagnostics() (DiagnosticsReport, error) {
	report := DiagnosticsReport{GeneratedAt: time.Now()}

	client, err := ipc.Dial()
	if err != nil {
		return report, nil
	}
	defer client.Close()
	report.HelperReachable = true

	var status helper.StatusResponse
	if err := client.Call(ipc.MethodStatus, nil, &status); err != nil {
		return report, fmt.Errorf("consultando status: %w", err)
	}
	report.Enrolled = status.Enrolled
	report.Connected = status.Connected
	report.KillSwitchActive = status.KillSwitchActive
	report.AssignedIP = status.AssignedIP
	report.ServerEndpoint = status.ServerEndpoint
	report.ServerBaseURL = status.ServerBaseURL
	report.ReceiveBytes = status.ReceiveBytes
	report.TransmitBytes = status.TransmitBytes
	if status.LastHandshake != nil {
		secs := int(time.Since(*status.LastHandshake).Seconds())
		report.LastHandshakeAgoSeconds = &secs
	}

	var prefs Preferences
	if err := client.Call(ipc.MethodGetPreferences, nil, &prefs); err == nil {
		report.Preferences = prefs
	}

	if report.ServerBaseURL != "" {
		reachable, latency, testErr := checkHTTPReachable(report.ServerBaseURL + "/api/status")
		report.PanelReachable = reachable
		report.PanelLatencyMs = latency
		if testErr != nil {
			report.PanelError = testErr.Error()
		}
	}

	if report.Connected {
		reachable, latency := checkTCPReachable(net.JoinHostPort(serverVPNAddress, "8081"), 2*time.Second)
		report.VPNGatewayReachable = reachable
		report.VPNGatewayLatencyMs = latency
	}

	return report, nil
}

// checkHTTPReachable faz um GET simples com timeout curto — usado só para
// medir se o painel responde e o tempo de ida-e-volta, não pra validar o
// conteúdo da resposta.
func checkHTTPReachable(rawURL string) (bool, *int64, error) {
	if _, err := url.Parse(rawURL); err != nil {
		return false, nil, fmt.Errorf("URL do painel inválida: %w", err)
	}
	httpClient := &http.Client{Timeout: 4 * time.Second}
	start := time.Now()
	resp, err := httpClient.Get(rawURL)
	if err != nil {
		return false, nil, err
	}
	defer resp.Body.Close()
	elapsed := time.Since(start).Milliseconds()
	if resp.StatusCode >= 500 {
		return false, &elapsed, fmt.Errorf("painel respondeu com status %s", resp.Status)
	}
	return true, &elapsed, nil
}

// checkTCPReachable tenta abrir (e imediatamente fecha) uma conexão TCP —
// serve como um "ping" aproximado sem precisar de socket bruto/ICMP, que
// exigiria privilégio extra no processo GUI.
func checkTCPReachable(addr string, timeout time.Duration) (bool, *int64) {
	start := time.Now()
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false, nil
	}
	defer conn.Close()
	elapsed := time.Since(start).Milliseconds()
	return true, &elapsed
}
