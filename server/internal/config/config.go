// Package config carrega a configuração do xvpn-server a partir de variáveis
// de ambiente, com valores padrão sensatos para desenvolvimento local.
package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config agrega todas as opções de runtime do servidor.
type Config struct {
	// HTTPAddr é o endereço/porta em que a API escuta. Fica atrás do Nginx
	// (ver PLAN.md §5) e nunca deve ser exposto diretamente na internet.
	HTTPAddr string

	// DBPath é o caminho do arquivo SQLite.
	DBPath string

	// JWTSecret assina os tokens de autenticação do painel. Obrigatório em
	// produção — não há valor padrão de propósito, para nunca rodar em
	// produção com um segredo previsível.
	JWTSecret string

	// JWTTokenTTLMinutes é a validade do token de sessão do painel.
	JWTTokenTTLMinutes int

	// InviteTokenTTLMinutes é a validade do código de convite de enrollment.
	InviteTokenTTLMinutes int

	// WireGuardInterface é o nome da interface (ex.: "wg0").
	WireGuardInterface string

	// WireGuardPrivateKeyPath aponta para a chave privada do servidor,
	// gerada manualmente na Fase 1 (ver ROADMAP.md) — o servidor nunca gera
	// nem sobrescreve essa chave sozinho, só lê.
	WireGuardPrivateKeyPath string

	// WireGuardListenPort é a porta UDP pública do WireGuard.
	WireGuardListenPort int

	// WireGuardAddress é o IP/máscara da própria interface (ex.: "10.66.66.1/24").
	WireGuardAddress string

	// WireGuardAllowedSubnet é a sub-rede usada para alocar IPs de peers
	// (ex.: "10.66.66.0/24"). O primeiro host (.1) é sempre reservado ao
	// próprio servidor e nunca alocado a um peer.
	WireGuardAllowedSubnet string

	// WireGuardEndpoint é o "host:porta" público que os clientes devem usar
	// no campo Endpoint do seu próprio wg0.conf (ex.:
	// "206.189.224.72:51820"). Obrigatório para o fluxo de enrollment
	// funcionar — sem isso o servidor não sabe que endereço devolver ao
	// cliente.
	WireGuardEndpoint string

	// AdminBootstrapUsername/Password, se definidos, criam o primeiro admin
	// no primeiro boot (quando a tabela de usuários está vazia). Se não
	// definidos, uma senha aleatória é gerada e logada uma única vez.
	AdminBootstrapUsername string
	AdminBootstrapPassword string

	// MarketplaceDataDir é o diretório raiz onde os blobs de asset do
	// marketplace (Fase 11 — ver PLAN.md §6.8) são gravados, endereçados
	// por conteúdo (ver internal/marketplace). Em produção fica dentro de
	// /opt/xvpn/data (único caminho com ReadWritePaths no systemd, ver
	// PLAN.md §5), nunca fora dele.
	MarketplaceDataDir string
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	raw := os.Getenv(key)
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s inválido (%q): %w", key, raw, err)
	}
	return v, nil
}

// Load lê a configuração do ambiente. Retorna erro se algo obrigatório
// estiver ausente ou malformado — falha alto e cedo em vez de subir com
// configuração inconsistente.
func Load() (*Config, error) {
	cfg := &Config{
		HTTPAddr:                getEnv("XVPN_HTTP_ADDR", "127.0.0.1:8080"),
		DBPath:                  getEnv("XVPN_DB_PATH", "xvpn.db"),
		JWTSecret:               os.Getenv("XVPN_JWT_SECRET"),
		WireGuardInterface:      getEnv("XVPN_WG_INTERFACE", "wg0"),
		WireGuardPrivateKeyPath: getEnv("XVPN_WG_PRIVATE_KEY_PATH", "/etc/wireguard/server.key"),
		WireGuardAddress:        getEnv("XVPN_WG_ADDRESS", "10.66.66.1/24"),
		WireGuardAllowedSubnet:  getEnv("XVPN_WG_SUBNET", "10.66.66.0/24"),
		WireGuardEndpoint:       os.Getenv("XVPN_WG_ENDPOINT"),
		AdminBootstrapUsername:  os.Getenv("XVPN_ADMIN_USERNAME"),
		AdminBootstrapPassword:  os.Getenv("XVPN_ADMIN_PASSWORD"),
		MarketplaceDataDir:      getEnv("XVPN_MARKETPLACE_DIR", "marketplace-data"),
	}

	var err error
	if cfg.JWTTokenTTLMinutes, err = getEnvInt("XVPN_JWT_TTL_MINUTES", 12*60); err != nil {
		return nil, err
	}
	if cfg.InviteTokenTTLMinutes, err = getEnvInt("XVPN_INVITE_TTL_MINUTES", 15); err != nil {
		return nil, err
	}
	if cfg.WireGuardListenPort, err = getEnvInt("XVPN_WG_LISTEN_PORT", 51820); err != nil {
		return nil, err
	}

	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("XVPN_JWT_SECRET é obrigatório (gere com: openssl rand -hex 32)")
	}
	if len(cfg.JWTSecret) < 32 {
		return nil, fmt.Errorf("XVPN_JWT_SECRET deve ter pelo menos 32 caracteres")
	}
	if cfg.WireGuardEndpoint == "" {
		return nil, fmt.Errorf("XVPN_WG_ENDPOINT é obrigatório (ex.: 206.189.224.72:51820)")
	}

	return cfg, nil
}
