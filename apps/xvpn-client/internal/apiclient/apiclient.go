// Package apiclient é o único cliente HTTP para a API do xvpn-server:
// usado pelo helper privilegiado no fluxo de enrollment (ver PLAN.md §8) e
// pelo processo GUI para as rotas que só respondem dentro do túnel
// (TunnelBaseURL). A chave privada gerada aqui nunca é enviada ao servidor
// — só a pública.
package apiclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// SupportedAPIVersion espelha api.APIVersion do servidor (server/internal/api/server.go).
// Ver PLAN.md §13.3 — bump aqui sempre que o servidor incrementar a versão
// de forma incompatível com este cliente.
const SupportedAPIVersion = 2

// TunnelBaseURL é a única base URL que alcança GET /api/me e
// POST /api/me/ssh-key: elas exigem que o IP de origem esteja em
// 10.66.66.0/24, e o HTTPS do painel nunca trafega dentro do túnel — o
// próprio helper instala uma rota /32 de exceção para o IP público do
// servidor antes de trocar a rota padrão (addHostRouteException em
// internal/platform/linux), senão o handshake WireGuard entraria em loop.
// Como o domínio do painel resolve para esse mesmo IP público, falar com
// ele sempre sai por fora do túnel. É consequência da topologia de rotas,
// não uma escolha de segurança — ver ROADMAP.md Fase 14.
const TunnelBaseURL = "http://10.66.66.1:8080"

// tunnelTimeout é curto de propósito: as rotas do túnel ou respondem de
// imediato (o servidor está a um salto de distância, dentro da VPN) ou não
// respondem nunca porque o túnel está fora do ar — esperar os 15s do
// timeout padrão só atrasaria o erro.
const tunnelTimeout = 5 * time.Second

// Client fala com um único xvpn-server, identificado por BaseURL (ex.:
// "https://xvpn.ihuull.com").
type Client struct {
	BaseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimSuffix(baseURL, "/"),
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// StatusResult é o subconjunto de GET /api/status relevante ao cliente.
type StatusResult struct {
	APIVersion int `json:"api_version"`
}

// CheckStatus valida que o servidor está de pé e que a versão da API é
// compatível — checado antes do enrollment e antes de tentar conectar, para
// dar um erro acionável ("servidor incompatível") em vez de uma falha
// obscura de handshake WireGuard.
func (c *Client) CheckStatus(ctx context.Context) (*StatusResult, error) {
	var result StatusResult
	if err := c.doJSON(ctx, http.MethodGet, "/api/status", nil, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// EnrollResult é o resultado de um enrollment bem-sucedido: tudo que o
// helper precisa para configurar o túnel (ver internal/tunnel.Config),
// mais o par de chaves gerado localmente.
type EnrollResult struct {
	PrivateKey          wgtypes.Key
	PublicKey           wgtypes.Key
	AssignedIP          string
	ServerPublicKey     string
	Endpoint            string
	AllowedIPs          []string
	PersistentKeepalive time.Duration
	APIVersion          int
	// Username é o dono do dispositivo no painel — caminho rápido para o
	// cliente saber quem ele é já no enrollment, sem esperar a primeira
	// conexão para perguntar em GET /api/me.
	Username      string
	DNS           []string
	IntranetHosts []HostEntry
}

type HostEntry struct {
	Hostname string `json:"hostname"`
	IPv4     string `json:"ipv4"`
}

type enrollRequest struct {
	InviteToken string `json:"invite_token"`
	PublicKey   string `json:"public_key"`
	DeviceName  string `json:"device_name"`
}

type enrollResponse struct {
	AssignedIP          string      `json:"assigned_ip"`
	ServerPublicKey     string      `json:"server_public_key"`
	Endpoint            string      `json:"endpoint"`
	AllowedIPs          string      `json:"allowed_ips"`
	PersistentKeepalive int         `json:"persistent_keepalive"`
	APIVersion          int         `json:"api_version"`
	Username            string      `json:"username"`
	DNS                 []string    `json:"dns"`
	IntranetHosts       []HostEntry `json:"intranet_hosts"`
}

// Enroll gera um par de chaves WireGuard localmente (a privada nunca deixa
// este processo) e registra o dispositivo no servidor usando o código de
// convite fornecido pelo usuário.
// POST /api/devices/enroll
func (c *Client) Enroll(ctx context.Context, inviteToken, deviceName string) (*EnrollResult, error) {
	privateKey, err := wgtypes.GeneratePrivateKey()
	if err != nil {
		return nil, fmt.Errorf("gerando par de chaves: %w", err)
	}
	publicKey := privateKey.PublicKey()

	req := enrollRequest{
		InviteToken: inviteToken,
		PublicKey:   publicKey.String(),
		DeviceName:  deviceName,
	}

	var resp enrollResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/devices/enroll", req, &resp); err != nil {
		return nil, err
	}

	allowedIPs := make([]string, 0, 2)
	for _, ip := range strings.Split(resp.AllowedIPs, ",") {
		ip = strings.TrimSpace(ip)
		if ip != "" {
			allowedIPs = append(allowedIPs, ip)
		}
	}

	return &EnrollResult{
		PrivateKey:          privateKey,
		PublicKey:           publicKey,
		AssignedIP:          resp.AssignedIP,
		ServerPublicKey:     resp.ServerPublicKey,
		Endpoint:            resp.Endpoint,
		AllowedIPs:          allowedIPs,
		PersistentKeepalive: time.Duration(resp.PersistentKeepalive) * time.Second,
		APIVersion:          resp.APIVersion,
		Username:            resp.Username,
		DNS:                 resp.DNS,
		IntranetHosts:       resp.IntranetHosts,
	}, nil
}

// MeResult é a identidade deste dispositivo resolvida pelo servidor a
// partir do IP de origem dentro do túnel (não falsificável: o
// allowed-ips do WireGuard amarra o IP ao peer).
type MeResult struct {
	Username      string      `json:"username"`
	SFTPEnabled   bool        `json:"sftp_enabled"`
	SambaEnabled  bool        `json:"samba_enabled"`
	IntranetHosts []HostEntry `json:"intranet_hosts"`
}

// Me descobre quem é o dono deste dispositivo no painel e se os acessos a
// arquivos estão liberados para ele. Só responde em TunnelBaseURL, com o
// túnel conectado.
// GET /api/me
func (c *Client) Me(ctx context.Context) (*MeResult, error) {
	ctx, cancel := context.WithTimeout(ctx, tunnelTimeout)
	defer cancel()

	var result MeResult
	if err := c.doJSON(ctx, http.MethodGet, "/api/me", nil, &result); err != nil {
		return nil, fmt.Errorf("não foi possível identificar este dispositivo na VPN — conecte o túnel e tente de novo: %w", err)
	}
	return &result, nil
}

// SSHKeyResult é a confirmação do registro da chave pública. Changed é
// false quando a mesma chave já estava registrada (o servidor é
// idempotente); SFTPEnabled reflete o toggle do painel — a chave é aceita
// e guardada mesmo desligado, e passa a valer no instante em que o admin
// liga o acesso.
type SSHKeyResult struct {
	Fingerprint string `json:"fingerprint"`
	SFTPEnabled bool   `json:"sftp_enabled"`
	Changed     bool   `json:"changed"`
}

type sshKeyRequest struct {
	PublicKey string `json:"public_key"`
}

// RegisterSSHKey registra a chave pública SSH deste dispositivo para o
// usuário dono dele (ver PLAN.md §6.9, revisão da Fase 14). Recebe só a
// pública, uma única linha no formato authorized_keys — a privada fica em
// ~/.ssh e nunca sai da máquina (ver internal/sshkey).
// POST /api/me/ssh-key
func (c *Client) RegisterSSHKey(ctx context.Context, publicKey string) (*SSHKeyResult, error) {
	ctx, cancel := context.WithTimeout(ctx, tunnelTimeout)
	defer cancel()

	var result SSHKeyResult
	req := sshKeyRequest{PublicKey: strings.TrimSpace(publicKey)}
	if err := c.doJSON(ctx, http.MethodPost, "/api/me/ssh-key", req, &result); err != nil {
		return nil, fmt.Errorf("não foi possível registrar a chave SSH deste dispositivo — conecte o túnel e tente de novo: %w", err)
	}
	return &result, nil
}

type apiError struct {
	Error string `json:"error"`
}

func (c *Client) doJSON(ctx context.Context, method, path string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("codificando requisição: %w", err)
		}
		reqBody = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("montando requisição: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("não foi possível conectar a %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("lendo resposta do servidor: %w", err)
	}

	if resp.StatusCode >= 400 {
		var apiErr apiError
		if json.Unmarshal(respBody, &apiErr) == nil && apiErr.Error != "" {
			return fmt.Errorf("servidor respondeu: %s", apiErr.Error)
		}
		return fmt.Errorf("servidor respondeu %d", resp.StatusCode)
	}

	if out != nil {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decodificando resposta do servidor: %w", err)
		}
	}
	return nil
}
