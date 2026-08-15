package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"runtime"
	"time"

	"github.com/rootkit-lab/xvpn/client/internal/apiclient"
	"github.com/rootkit-lab/xvpn/client/internal/autostart"
	"github.com/rootkit-lab/xvpn/client/internal/helper"
	"github.com/rootkit-lab/xvpn/client/internal/ipc"
	"github.com/rootkit-lab/xvpn/client/internal/marketplaceclient"
	"github.com/rootkit-lab/xvpn/client/internal/opener"
	"github.com/rootkit-lab/xvpn/client/internal/sshkey"
	"github.com/rootkit-lab/xvpn/client/internal/version"
)

// marketplace é o cliente do catálogo (Fase 12 — ROADMAP.md), com sessão
// de usuário mantida só em memória pelo processo GUI. Vive num var de
// pacote, não num campo de VPNService: várias partes do código (tray no
// main.go, cada chamada do frontend) instanciam `&VPNService{}` à vontade
// já que o struct historicamente não guarda estado — um campo aqui não
// sobreviveria a essas instâncias descartáveis, mas este var sim, porque
// é compartilhado por todas elas (mesmo processo, mesmo pacote).
var marketplace = marketplaceclient.New()

// serverVPNAddress é o IP fixo do servidor dentro do túnel WireGuard — não
// muda entre deployments (invariante do projeto, ver PLAN.md §5 e
// AGENTS.md). Samba/FileBrowser só respondem aqui, nunca no endpoint
// público do servidor.
const serverVPNAddress = "10.66.66.1"

// sharedSambaName é o nome do compartilhamento Samba criado na Fase 5 (ver
// server/deploy/samba/smb.conf).
const sharedSambaName = "shared"

// homeSambaPrefix compõe o nome do compartilhamento pessoal
// ("home-<username>"), que passa a existir quando o admin liga o Samba
// para aquele usuário — ver PLAN.md §6.9.
const homeSambaPrefix = "home-"

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
	// Username é a conta desta pessoa no painel, descoberta pelo helper a
	// cada conexão; SambaEnabled/SFTPEnabled espelham os toggles do
	// painel, para a UI desabilitar um botão com a razão visível em vez
	// de deixar o clique virar um erro de mount (ROADMAP.md Fase 14).
	Username     string `json:"username"`
	SambaEnabled bool   `json:"sambaEnabled"`
	SFTPEnabled  bool   `json:"sftpEnabled"`
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
	ClientVersion           string      `json:"clientVersion"`
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

// Version devolve a versão semântica embutida no binário (ldflags) ou "dev".
func (s *VPNService) Version() string {
	return version.String()
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
		Username:         resp.Username,
		SambaEnabled:     resp.SambaEnabled,
		SFTPEnabled:      resp.SFTPEnabled,
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
	if err := client.Call(ipc.MethodConnect, nil, nil); err != nil {
		return err
	}
	s.mountFileSharesBestEffort()
	return nil
}

// mountFileSharesBestEffort monta [shared] e [home-<user>] via GVFS guest
// depois que o túnel sobe. Falha silenciosa: o clique em Compartilhado
// ainda tenta montar de novo e devolve o erro ao usuário.
func (s *VPNService) mountFileSharesBestEffort() {
	status, err := s.Status()
	if err != nil || !status.Connected || !status.SambaEnabled {
		return
	}
	_ = opener.EnsureSMBMounted(serverVPNAddress, sharedSambaName)
	if status.Username != "" {
		_ = opener.EnsureSMBMounted(serverVPNAddress, homeSambaPrefix+status.Username)
	}
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

// OpenServerFiles abre o acesso a arquivos do servidor no aplicativo
// padrão do SO. kind é "smb-home" (o compartilhamento pessoal desta
// pessoa), "smb-shared" (o compartilhamento comum da Fase 5) ou
// "filebrowser" (interface web).
//
// Confere o estado real da conexão com o helper antes de abrir, em vez de
// confiar no que a UI tinha em mãos: um clique num status de 2 segundos
// atrás abriria o gerenciador de arquivos numa janela de rede vazia, que
// é exatamente o sintoma reportado na Fase 14 e não diz nada ao usuário
// sobre o que fazer.
func (s *VPNService) OpenServerFiles(kind string) error {
	status, err := s.Status()
	if err != nil {
		return fmt.Errorf("não foi possível confirmar o estado da conexão: %w", err)
	}
	if !status.HelperReachable {
		return fmt.Errorf("serviço xvpn-client-helper indisponível — verifique se está instalado e rodando")
	}
	if !status.Connected {
		return fmt.Errorf("conecte a VPN primeiro — os arquivos do servidor só são alcançáveis por dentro do túnel")
	}

	switch kind {
	case "smb-home":
		if err := requireSamba(status); err != nil {
			return err
		}
		if status.Username == "" {
			return fmt.Errorf("ainda não foi possível descobrir qual é a sua conta no painel — desconecte e conecte de novo para o servidor informar")
		}
		err := opener.OpenSMBShare(serverVPNAddress, homeSambaPrefix+status.Username)
		if err != nil {
			slog.Warn("open smb-home failed", "err", err, "user", status.Username)
		}
		return err
	case "smb-shared":
		if err := requireSamba(status); err != nil {
			return err
		}
		err := opener.OpenSMBShare(serverVPNAddress, sharedSambaName)
		if err != nil {
			slog.Warn("open smb-shared failed", "err", err)
		}
		return err
	case "filebrowser":
		return opener.OpenURL(fmt.Sprintf("http://%s:8081", serverVPNAddress))
	default:
		return fmt.Errorf("tipo de acesso a arquivos desconhecido: %q", kind)
	}
}

// requireSamba transforma "o admin não liberou" num erro que explica o
// que houve — sem isso o usuário só veria a falha de mount do SO, que não
// distingue permissão negada de compartilhamento inexistente.
func requireSamba(status StatusView) error {
	if !status.SambaEnabled {
		return fmt.Errorf("o administrador ainda não liberou o acesso a arquivos (Samba) para a sua conta — peça a liberação no painel")
	}
	return nil
}

// SSHKeyStatus é o que a página de Diagnóstico mostra sobre a chave SSH
// deste dispositivo. Só metadados: a chave privada nunca sai de ~/.ssh
// (ver internal/sshkey) e nem a pública precisa aparecer aqui — o
// fingerprint já basta para conferir com o painel.
type SSHKeyStatus struct {
	Fingerprint string `json:"fingerprint"`
	// SFTPEnabled=false não é erro: a chave fica guardada no servidor e
	// passa a valer no instante em que o admin ligar o acesso.
	SFTPEnabled bool `json:"sftpEnabled"`
	// Changed=false significa que esta mesma chave já estava registrada
	// (o servidor é idempotente).
	Changed bool `json:"changed"`
}

// RegisterSSHKey garante o par de chaves local, registra a pública no
// servidor e deixa o atalho `sftp xvpn-files` pronto no ~/.ssh/config.
// Roda no processo GUI sem privilégio de propósito — a chave precisa ser
// legível pelo cliente SFTP do próprio usuário, ver PLAN.md §6.9.
func (s *VPNService) RegisterSSHKey() (SSHKeyStatus, error) {
	status, err := s.Status()
	if err != nil {
		return SSHKeyStatus{}, fmt.Errorf("não foi possível confirmar o estado da conexão: %w", err)
	}
	if !status.Connected {
		return SSHKeyStatus{}, fmt.Errorf("conecte a VPN primeiro — a chave só pode ser registrada por dentro do túnel")
	}

	publicKey, err := sshkey.Ensure()
	if err != nil {
		return SSHKeyStatus{}, err
	}

	result, err := apiclient.New(apiclient.TunnelBaseURL).RegisterSSHKey(context.Background(), publicKey)
	if err != nil {
		return SSHKeyStatus{}, err
	}

	// O atalho do ~/.ssh/config é conveniência: se falhar, a chave já
	// está registrada e o acesso funciona informando usuário e host à
	// mão — não vale reverter nem alarmar por isso.
	if err := sshkey.EnsureSSHConfigEntry(status.Username); err != nil {
		slog.Warn("ssh config entry failed", "err", err)
	}

	return SSHKeyStatus{
		Fingerprint: result.Fingerprint,
		SFTPEnabled: result.SFTPEnabled,
		Changed:     result.Changed,
	}, nil
}

// Platform devolve o SO deste processo ("linux" ou "windows") — usado
// pelo frontend para filtrar o catálogo do marketplace (Fase 12,
// ROADMAP.md) pela plataforma do asset (store.Platform no servidor),
// mostrando só o que este dispositivo consegue de fato instalar.
func (s *VPNService) Platform() string {
	return runtime.GOOS
}

// MarketplaceLoginArgs são os dados inseridos na tela de login do
// marketplace (Fase 12) — separado do enrollment de dispositivo
// (EnrollArgs): aquele registra o dispositivo na VPN via código de
// convite, sem JWT; este autentica um USUÁRIO do painel para poder
// listar/baixar do catálogo (que exige JWT, ver PLAN.md §6.8).
type MarketplaceLoginArgs struct {
	ServerBaseURL string `json:"serverBaseURL"`
	Username      string `json:"username"`
	Password      string `json:"password"`
}

// MarketplaceSession é o estado de autenticação do marketplace exibido
// pelo frontend — nunca inclui o token em si (fica só em memória dentro
// de internal/marketplaceclient, nunca serializado pra fora do processo
// Go, nem para o próprio frontend).
type MarketplaceSession struct {
	LoggedIn bool   `json:"loggedIn"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

// MarketplaceAsset espelha marketplaceclient.Asset em camelCase (padrão
// JSON deste arquivo, ver StatusView/EnrollArgs) — o pacote interno usa
// snake_case por espelhar o servidor diretamente.
type MarketplaceAsset struct {
	ID            uint   `json:"id"`
	Platform      string `json:"platform"`
	Arch          string `json:"arch"`
	Filename      string `json:"filename"`
	SHA256        string `json:"sha256"`
	SizeBytes     int64  `json:"sizeBytes"`
	DownloadCount int64  `json:"downloadCount"`
}

type MarketplaceVersion struct {
	ID        uint               `json:"id"`
	Version   string             `json:"version"`
	Channel   string             `json:"channel"`
	Changelog string             `json:"changelog"`
	Assets    []MarketplaceAsset `json:"assets"`
}

type MarketplaceApp struct {
	ID          uint                 `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	IconURL     string               `json:"iconURL"`
	Versions    []MarketplaceVersion `json:"versions"`
}

// MarketplaceLogin autentica no painel (POST /api/auth/login) e guarda o
// JWT em memória (nunca em disco, ver comentário em
// internal/marketplaceclient) para as chamadas seguintes de
// ListMarketplaceApps/DownloadMarketplaceAsset.
func (s *VPNService) MarketplaceLogin(args MarketplaceLoginArgs) (MarketplaceSession, error) {
	session, err := marketplace.Login(context.Background(), args.ServerBaseURL, args.Username, args.Password)
	if err != nil {
		return MarketplaceSession{}, err
	}
	return MarketplaceSession{LoggedIn: session.LoggedIn, Username: session.Username, Role: session.Role}, nil
}

// MarketplaceLogout descarta a sessão em memória — a próxima
// ListMarketplaceApps volta a pedir login.
func (s *VPNService) MarketplaceLogout() {
	marketplace.Logout()
}

// MarketplaceSessionStatus consulta o estado de login atual sem chamar o
// servidor — usado pelo frontend ao abrir a aba "Apps" para decidir entre
// mostrar a tela de login ou já carregar o catálogo.
func (s *VPNService) MarketplaceSessionStatus() MarketplaceSession {
	session := marketplace.Session()
	return MarketplaceSession{LoggedIn: session.LoggedIn, Username: session.Username, Role: session.Role}
}

// ListMarketplaceApps busca o catálogo (já filtrado por ACL pelo
// servidor, ver handleListMarketplaceApps). Qualquer erro aqui — sessão
// expirada ou falha de rede — o frontend trata voltando pra tela de
// login com a mensagem de erro visível, em vez de tentar distinguir os
// dois casos por conteúdo da mensagem (ver ROADMAP.md Fase 12).
func (s *VPNService) ListMarketplaceApps() ([]MarketplaceApp, error) {
	apps, err := marketplace.ListApps(context.Background())
	if err != nil {
		return nil, err
	}
	result := make([]MarketplaceApp, 0, len(apps))
	for _, app := range apps {
		versions := make([]MarketplaceVersion, 0, len(app.Versions))
		for _, v := range app.Versions {
			assets := make([]MarketplaceAsset, 0, len(v.Assets))
			for _, a := range v.Assets {
				assets = append(assets, MarketplaceAsset{
					ID:            a.ID,
					Platform:      a.Platform,
					Arch:          a.Arch,
					Filename:      a.Filename,
					SHA256:        a.SHA256,
					SizeBytes:     a.SizeBytes,
					DownloadCount: a.DownloadCount,
				})
			}
			versions = append(versions, MarketplaceVersion{
				ID:        v.ID,
				Version:   v.Version,
				Channel:   v.Channel,
				Changelog: v.Changelog,
				Assets:    assets,
			})
		}
		result = append(result, MarketplaceApp{
			ID:          app.ID,
			Name:        app.Name,
			Description: app.Description,
			IconURL:     app.IconURL,
			Versions:    versions,
		})
	}
	return result, nil
}

// MarketplaceDownloadArgs identifica o asset e traz os metadados já
// conhecidos pelo frontend (vindos de ListMarketplaceApps) necessários
// pra conferir a integridade do download — ver
// marketplaceclient.DownloadAsset.
type MarketplaceDownloadArgs struct {
	AssetID  uint   `json:"assetId"`
	Filename string `json:"filename"`
	SHA256   string `json:"sha256"`
}

// MarketplaceDownloadResult é o resultado de um download concluído com
// sucesso — Path é absoluto, pronto para OpenLocalPath.
type MarketplaceDownloadResult struct {
	Path      string `json:"path"`
	SizeBytes int64  `json:"sizeBytes"`
}

// DownloadMarketplaceAsset baixa o asset para a pasta de Downloads do
// usuário, verificando o SHA-256 antes de devolver sucesso (ver
// marketplaceclient.DownloadAsset — nunca deixa um arquivo com hash
// incompatível na pasta do usuário). Chamada síncrona (sem progresso
// incremental): assets grandes (até 2 GiB, ver PLAN.md §6.8) podem levar
// algum tempo — o frontend mostra um spinner enquanto aguarda, mesmo
// padrão usado para Enroll/Connect.
func (s *VPNService) DownloadMarketplaceAsset(args MarketplaceDownloadArgs) (MarketplaceDownloadResult, error) {
	result, err := marketplace.DownloadAsset(context.Background(), args.AssetID, args.Filename, args.SHA256)
	if err != nil {
		return MarketplaceDownloadResult{}, err
	}
	return MarketplaceDownloadResult{Path: result.Path, SizeBytes: result.SizeBytes}, nil
}

// OpenLocalPath abre um arquivo ou pasta local (ex.: um instalador do
// marketplace já baixado) no aplicativo/gerenciador de arquivos padrão do
// SO. Diferente de OpenServerFiles (recursos do servidor, só acessíveis
// com o túnel ativo), aqui o caminho é sempre local — nunca depende da
// VPN.
func (s *VPNService) OpenLocalPath(path string) error {
	return opener.OpenPath(path)
}

// OpenDownloadsFolder abre a pasta de Downloads do usuário (mesmo destino
// usado por DownloadMarketplaceAsset) — o botão "abrir pasta" do
// ROADMAP.md Fase 12, para quando o usuário quer ver o arquivo baixado no
// gerenciador de arquivos em vez de abri-lo diretamente.
func (s *VPNService) OpenDownloadsFolder() error {
	return opener.OpenPath(marketplaceclient.DownloadsDir())
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

// GetMTU/SetMTU expõem o override manual do MTU do túnel (0 =
// automático), que até a Fase 14 só podia ser escolhido no enrollment.
// A faixa aceita e o motivo de existir estão em handleSetMTU (helper).
func (s *VPNService) GetMTU() (int, error) {
	client, err := ipc.Dial()
	if err != nil {
		return 0, fmt.Errorf("serviço xvpn-client-helper indisponível: %w", err)
	}
	defer client.Close()

	var result helper.MTUSetting
	if err := client.Call(ipc.MethodGetMTU, nil, &result); err != nil {
		return 0, fmt.Errorf("consultando MTU: %w", err)
	}
	return result.MTU, nil
}

func (s *VPNService) SetMTU(mtu int) error {
	client, err := ipc.Dial()
	if err != nil {
		return fmt.Errorf("serviço xvpn-client-helper indisponível: %w", err)
	}
	defer client.Close()
	return client.Call(ipc.MethodSetMTU, helper.MTUSetting{MTU: mtu}, nil)
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
	report := DiagnosticsReport{
		GeneratedAt:   time.Now(),
		ClientVersion: version.String(),
	}

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
