// Package marketplaceclient é o cliente HTTP do catálogo do marketplace
// (Fase 12, ROADMAP.md) usado pelo processo GUI sem privilégio — não pelo
// helper. Diferente de internal/apiclient (usado pelo helper só no fluxo
// de enrollment de dispositivo, sem JWT), este pacote autentica como um
// USUÁRIO do painel (POST /api/auth/login) para poder listar/baixar do
// marketplace (server/internal/api/marketplace_handler.go), que exige JWT.
//
// Decisão deliberada (ver ROADMAP.md Fase 12): o token de sessão fica só
// em memória neste processo, nunca gravado em disco — diferente de
// internal/config, que persiste o device.json (chave privada WireGuard)
// para sempre. Sessão de usuário aqui é efêmera de propósito: expira
// sozinha (XVPN_JWT_TTL_MINUTES no servidor, padrão 12h) e some ao fechar
// o app, evitando mais um segredo de sessão sentado no disco do cliente
// além do que já é estritamente necessário (o par de chaves WireGuard).
package marketplaceclient

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/adrg/xdg"
)

// ErrNotLoggedIn é devolvido por ListApps/DownloadAsset quando não há
// sessão válida (nunca logou, fez logout, ou o JWT expirou no servidor —
// nesse caso o próprio 401 da API é traduzido para este erro). O chamador
// (vpnservice.go) usa isso para decidir quando mostrar a tela de login de
// novo, em vez de vazar "erro 401" cru pra UI.
var ErrNotLoggedIn = errors.New("sessão do marketplace expirada ou inexistente — faça login novamente")

// Client fala com o catálogo do marketplace de um único xvpn-server.
// Seguro para uso concorrente (Wails pode despachar chamadas do frontend
// em goroutines distintas).
type Client struct {
	httpClient *http.Client

	mu       sync.Mutex
	baseURL  string
	token    string
	username string
	role     string
}

// New cria um Client pronto para uso, sem sessão ativa.
func New() *Client {
	return &Client{httpClient: &http.Client{Timeout: 30 * time.Second}}
}

// Session é o estado de autenticação atual, exposto ao frontend via
// vpnservice.go (nunca inclui o token em si).
type Session struct {
	LoggedIn bool
	Username string
	Role     string
}

// Session devolve o estado de login atual sem fazer nenhuma chamada de
// rede — não confirma que o token ainda é válido no servidor (só uma
// chamada real, ex. ListApps, descobre isso via 401).
func (c *Client) Session() Session {
	c.mu.Lock()
	defer c.mu.Unlock()
	return Session{LoggedIn: c.token != "", Username: c.username, Role: c.role}
}

// Logout descarta a sessão em memória. Não chama o servidor — o JWT
// simplesmente para de ser enviado; ele mesmo expira sozinho no TTL
// configurado no servidor (não há endpoint de revogação de token nesta
// API, ver auth.TokenManager).
func (c *Client) Logout() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.baseURL = ""
	c.token = ""
	c.username = ""
	c.role = ""
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type loginResponse struct {
	Token string `json:"token"`
	User  struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	} `json:"user"`
}

// Login autentica no painel (POST /api/auth/login) e guarda o JWT em
// memória para as chamadas seguintes deste Client.
func (c *Client) Login(ctx context.Context, baseURL, username, password string) (Session, error) {
	baseURL = strings.TrimSuffix(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return Session{}, errors.New("endereço do servidor vazio")
	}
	if username == "" || password == "" {
		return Session{}, errors.New("usuário e senha são obrigatórios")
	}

	var resp loginResponse
	req := loginRequest{Username: username, Password: password}
	if err := doJSON(ctx, c.httpClient, http.MethodPost, baseURL, "/api/auth/login", "", req, &resp); err != nil {
		return Session{}, err
	}
	if resp.Token == "" {
		return Session{}, errors.New("servidor não devolveu um token de sessão")
	}

	c.mu.Lock()
	c.baseURL = baseURL
	c.token = resp.Token
	c.username = resp.User.Username
	c.role = resp.User.Role
	c.mu.Unlock()

	return Session{LoggedIn: true, Username: resp.User.Username, Role: resp.User.Role}, nil
}

// Asset espelha marketplaceAssetResponse do servidor (ver
// server/internal/api/marketplace_handler.go) — mantido manualmente em
// sincronia, como o resto do cliente (não importa tipos do módulo server).
type Asset struct {
	ID            uint   `json:"id"`
	VersionID     uint   `json:"version_id"`
	Platform      string `json:"platform"`
	Arch          string `json:"arch"`
	Filename      string `json:"filename"`
	SHA256        string `json:"sha256"`
	SizeBytes     int64  `json:"size_bytes"`
	DownloadCount int64  `json:"download_count"`
}

// Version espelha marketplaceVersionResponse.
type Version struct {
	ID        uint    `json:"id"`
	Version   string  `json:"version"`
	Channel   string  `json:"channel"`
	Changelog string  `json:"changelog"`
	Assets    []Asset `json:"assets"`
}

// App espelha marketplaceAppResponse (sem AccessUserIDs — é um detalhe
// admin-only que não faz sentido para a lista de consumo do cliente).
type App struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	IconURL     string    `json:"icon_url"`
	Versions    []Version `json:"versions"`
}

// ListApps devolve o catálogo já filtrado por ACL para o usuário logado
// (o servidor decide o que aparece, ver handleListMarketplaceApps — o
// cliente nunca precisa saber a regra de visibilidade).
// GET /api/marketplace/apps
func (c *Client) ListApps(ctx context.Context) ([]App, error) {
	token, baseURL, err := c.authSnapshot()
	if err != nil {
		return nil, err
	}
	var apps []App
	if err := doJSON(ctx, c.httpClient, http.MethodGet, baseURL, "/api/marketplace/apps", token, nil, &apps); err != nil {
		if errors.Is(err, ErrNotLoggedIn) {
			c.Logout()
		}
		return nil, err
	}
	return apps, nil
}

// DownloadResult descreve um download concluído com sucesso (o checksum
// já foi conferido — DownloadAsset nunca devolve sucesso com hash
// incompatível, ver comentário abaixo).
type DownloadResult struct {
	Path      string
	SizeBytes int64
}

// DownloadAsset baixa o asset id para a pasta de Downloads do usuário
// (xdg.UserDirs.Download — resolve corretamente tanto em Linux quanto em
// Windows, ver github.com/adrg/xdg), calculando o SHA-256 do conteúdo
// recebido em paralelo à gravação em disco (streaming, sem carregar o
// arquivo inteiro em memória — mesmo padrão do servidor em
// internal/marketplace/storage.go). expectedSHA256 vem da própria
// listagem (ListApps) — nunca confiamos apenas no que o servidor alega no
// momento do download; se o hash não bater, o arquivo é apagado e um erro
// é devolvido: nunca deixamos um download corrompido/adulterado na pasta
// do usuário se fazendo passar por íntegro.
func (c *Client) DownloadAsset(ctx context.Context, assetID uint, filename, expectedSHA256 string) (DownloadResult, error) {
	return c.downloadAssetTo(ctx, DownloadsDir(), assetID, filename, expectedSHA256)
}

// downloadAssetTo é o corpo real de DownloadAsset, parametrizado pela
// pasta de destino — separado só para os testes poderem apontar para um
// diretório temporário sem depender de xdg.UserDirs (resolvido uma única
// vez no processo, no import do pacote, então nenhuma variável de
// ambiente setada depois em tempo de teste o afetaria).
func (c *Client) downloadAssetTo(ctx context.Context, destDir string, assetID uint, filename, expectedSHA256 string) (DownloadResult, error) {
	token, baseURL, err := c.authSnapshot()
	if err != nil {
		return DownloadResult{}, err
	}

	url := fmt.Sprintf("%s/api/marketplace/assets/%d/download", baseURL, assetID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("montando requisição: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("não foi possível conectar a %s: %w", baseURL, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// segue abaixo
	case http.StatusUnauthorized:
		c.Logout()
		return DownloadResult{}, ErrNotLoggedIn
	case http.StatusForbidden:
		return DownloadResult{}, errors.New("você não tem acesso a este app")
	case http.StatusNotFound:
		return DownloadResult{}, errors.New("asset não encontrado (pode ter sido removido do catálogo)")
	default:
		return DownloadResult{}, fmt.Errorf("servidor respondeu %d", resp.StatusCode)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return DownloadResult{}, fmt.Errorf("criando pasta de downloads: %w", err)
	}
	destPath := uniqueDestPath(destDir, filename)

	out, err := os.Create(destPath)
	if err != nil {
		return DownloadResult{}, fmt.Errorf("criando arquivo de destino: %w", err)
	}

	hasher := sha256.New()
	written, copyErr := io.Copy(io.MultiWriter(out, hasher), resp.Body)
	closeErr := out.Close()
	if copyErr != nil {
		os.Remove(destPath)
		return DownloadResult{}, fmt.Errorf("gravando download: %w", copyErr)
	}
	if closeErr != nil {
		os.Remove(destPath)
		return DownloadResult{}, fmt.Errorf("fechando arquivo: %w", closeErr)
	}

	sum := hex.EncodeToString(hasher.Sum(nil))
	if expectedSHA256 != "" && !strings.EqualFold(sum, expectedSHA256) {
		os.Remove(destPath)
		return DownloadResult{}, fmt.Errorf("checksum não confere (esperado %s, obtido %s) — download descartado", expectedSHA256, sum)
	}

	return DownloadResult{Path: destPath, SizeBytes: written}, nil
}

// DownloadsDir resolve a pasta de Downloads do usuário. xdg.UserDirs já
// cobre Linux (via xdg-user-dirs) e Windows (known folder nativo) — ver
// pesquisa em paths_windows.go do próprio pacote. O fallback pra home só
// deve disparar num ambiente exótico sem nenhum dos dois configurado.
// Exportada para que vpnservice.go possa oferecer "abrir pasta" (Fase 12,
// ROADMAP.md) sem duplicar a lógica de resolução.
func DownloadsDir() string {
	if xdg.UserDirs.Download != "" {
		return xdg.UserDirs.Download
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return "."
}

// uniqueDestPath evita sobrescrever um download anterior do mesmo
// instalador (ex.: baixar a mesma versão duas vezes, ou dois apps com
// nomes de arquivo coincidentes) — mesmo padrão de "arquivo (1).ext" que
// navegadores usam. filename vem do servidor (AppAsset.Filename); passa
// por filepath.Base de novo aqui como defesa em profundidade (o servidor
// já sanitiza no upload, mas nunca custa checar de novo antes de montar
// um caminho de disco local a partir de dado que veio da rede).
func uniqueDestPath(dir, filename string) string {
	filename = filepath.Base(filename)
	if filename == "" || filename == "." || filename == string(filepath.Separator) {
		filename = "xvpn-marketplace-download"
	}
	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)

	candidate := filepath.Join(dir, filename)
	for i := 1; ; i++ {
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
		candidate = filepath.Join(dir, fmt.Sprintf("%s (%d)%s", base, i, ext))
	}
}

func (c *Client) authSnapshot() (token, baseURL string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token == "" {
		return "", "", ErrNotLoggedIn
	}
	return c.token, c.baseURL, nil
}

type apiError struct {
	Error string `json:"error"`
}

// doJSON é compartilhado por Login/ListApps — mesmo padrão de
// internal/apiclient.doJSON, com a adição do header Authorization opcional
// e da tradução de 401 para ErrNotLoggedIn (o chamador decide se isso
// deve limpar a sessão local). Essa tradução só vale quando havia token na
// requisição: no próprio POST /api/auth/login um 401 significa "usuário ou
// senha inválidos", e traduzi-lo para "sessão expirada" mandaria o usuário
// atrás de um problema que não existe.
func doJSON(ctx context.Context, hc *http.Client, method, baseURL, path, token string, body, out any) error {
	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("codificando requisição: %w", err)
		}
		reqBody = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, reqBody)
	if err != nil {
		return fmt.Errorf("montando requisição: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := hc.Do(req)
	if err != nil {
		return fmt.Errorf("não foi possível conectar a %s: %w", baseURL, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("lendo resposta do servidor: %w", err)
	}

	if resp.StatusCode == http.StatusUnauthorized && token != "" {
		return ErrNotLoggedIn
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
