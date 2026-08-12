// Package apiclient é o único cliente HTTP para a API do xvpn-server,
// usado pelo helper privilegiado no fluxo de enrollment (ver PLAN.md §8).
// A chave privada gerada aqui nunca é enviada ao servidor — só a pública.
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
const SupportedAPIVersion = 1

// Client fala com um único xvpn-server, identificado por BaseURL (ex.:
// "https://vpn.officeempresa.com").
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
}

type enrollRequest struct {
	InviteToken string `json:"invite_token"`
	PublicKey   string `json:"public_key"`
	DeviceName  string `json:"device_name"`
}

type enrollResponse struct {
	AssignedIP          string `json:"assigned_ip"`
	ServerPublicKey     string `json:"server_public_key"`
	Endpoint            string `json:"endpoint"`
	AllowedIPs          string `json:"allowed_ips"`
	PersistentKeepalive int    `json:"persistent_keepalive"`
	APIVersion          int    `json:"api_version"`
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
	}, nil
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
