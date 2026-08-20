// Package config carrega a configuração do xvpn-server a partir de variáveis
// de ambiente, com valores padrão sensatos para desenvolvimento local.
package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
)

// Config agrega todas as opções de runtime do servidor.
type Config struct {
	// HTTPAddr é o endereço/porta em que a API escuta. Fica atrás do Nginx
	// (ver PLAN.md §5) e nunca deve ser exposto diretamente na internet.
	HTTPAddr string

	// TunnelHTTPAddr é um listener ADICIONAL, servindo o mesmo router, na
	// interface wg0 (produção: "10.66.66.1:8080" — mesma porta, outra
	// interface; ver PLAN.md §5). Existe por roteamento, não por segurança:
	// o cliente instala uma rota /32 para o IP público do VPS antes de
	// trocar a rota padrão (senão o próprio handshake WireGuard entraria em
	// loop), e como xvpn.ihuull.com resolve para esse mesmo IP, o
	// HTTPS do painel nunca trafega dentro do túnel. Sem este listener, um
	// peer não teria como falar com a API tendo um 10.66.66.x como IP de
	// origem, e as rotas de identidade por túnel (GET /api/me,
	// POST /api/me/ssh-key) seriam inalcançáveis.
	//
	// Nunca faça bind em 0.0.0.0 aqui — seria expor a API na interface
	// pública por fora do Nginx. Vazio desabilita o listener (é o default
	// em dev/teste, onde não existe wg0).
	TunnelHTTPAddr string

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

	// SocialMediaDir guarda blobs de anexo/áudio/stories do chat (Fase 21).
	// Em produção: /opt/xvpn/data/social (dentro de ReadWritePaths).
	SocialMediaDir string

	// PackagesDir guarda blobs do registry do XGIT (Fase 45.1 — npm/generic).
	// Em produção: /opt/xvpn/data/packages (dentro de ReadWritePaths).
	// Sem hostname novo: a API vive em xgit.corp.
	PackagesDir string

	// MongoURI, se definido, torna o Mongo a fonte da verdade (Fase 28).
	// Bind só 127.0.0.1:27017 em produção. Vazio = SQLite (testes/CI).
	MongoURI string

	// XbotToken autentica POST /api/hooks/chat/broadcast (Fase 27).
	// Comparado em tempo constante. Vazio = rota de hook não é registrada.
	XbotToken string

	// PublishToken autentica POST /api/marketplace/sync (Fase 16 —
	// PLAN.md §6.10.3). Comparado em tempo constante. Vazio = a rota de
	// sync nem é registrada (servidor que não publica não expõe a
	// superfície). Em produção fica em /opt/xvpn/xvpn-server.env.
	PublishToken string

	// UserProvisionBinaryPath é o caminho absoluto do binário
	// privilegiado xvpn-user-provision (Fase 13 — ver PLAN.md §6.9),
	// invocado pelo xvpn-server via sudoers.d restrito (NOPASSWD, sem
	// wildcard de argumento) para criar contas Unix e habilitar
	// SFTP/Samba por usuário. Em produção fica em /opt/xvpn/bin (ver
	// deploy da Fase 13); o default aponta pra lá. Em testes/dev o
	// caminho não precisa existir — o cliente injetável (ver
	// internal/userprovision) nunca chama o binário de verdade nos
	// testes.
	UserProvisionBinaryPath string

	// DriverSharedDir é o share [shared] do Samba (/srv/xvpn/shared).
	// XDriver nativo lê/grava daqui — sem FileBrowser.
	DriverSharedDir string

	// DriverHomeRoot é o prefixo das pastas pessoais (/home). O path
	// real é <root>/<username>/files.
	DriverHomeRoot string

	// DriverProjectsDir é a raiz dos shares de projeto no XDRIVER
	// (Fase 37 — /opt/xvpn/data/projects/<slug>). Sem FileBrowser e
	// sem Samba [project-*] nesta fase; só o Drive web em xdriver.corp.
	DriverProjectsDir string

	// GitDir é a raiz dos bare repos do forge (Fase 40 —
	// /opt/xvpn/data/git/<org>/<slug>.git). Smart HTTP só em xgit.corp.
	GitDir string

	// CodespacesDir é a raiz dos worktrees do XCODESPACES (Fase 49 —
	// /opt/xvpn/data/codespaces/<user>/<slug>/<id>/). Fora do bare.
	CodespacesDir string

	// BackupDir é o staging dos jobs off-site (Fase 44 — restic cache,
	// rclone.conf temporário). Credenciais ficam no Mongo, não aqui.
	BackupDir string

	// BitLaunchToken (Fase 38) só no VPS, chmod 600. Se o banco estiver
	// vazio, semeia a primeira BitLaunchAccount. O caminho normal é
	// Compute → Configurações (várias contas). Vazio e sem contas =
	// só import do node local; create/rebuild devolvem 503.
	BitLaunchToken string

	// CloudflareToken (Fase 39) só no VPS. Semeia a primeira
	// CloudflareAccount se o banco estiver vazio. Caminho normal:
	// DNS → Configurações.
	CloudflareToken string
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
		TunnelHTTPAddr:          os.Getenv("XVPN_TUNNEL_HTTP_ADDR"),
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
		SocialMediaDir:          getEnv("XVPN_SOCIAL_MEDIA_DIR", "social-media-data"),
		PackagesDir:             getEnv("XVPN_PACKAGES_DIR", "/opt/xvpn/data/packages"),
		PublishToken:            os.Getenv("XVPN_PUBLISH_TOKEN"),
		MongoURI:                os.Getenv("XVPN_MONGO_URI"),
		XbotToken:               os.Getenv("XVPN_XBOT_TOKEN"),
		UserProvisionBinaryPath: getEnv("XVPN_USER_PROVISION_BIN", "/opt/xvpn/bin/xvpn-user-provision"),
		DriverSharedDir:         getEnv("XVPN_DRIVER_SHARED_DIR", "/srv/xvpn/shared"),
		DriverHomeRoot:          getEnv("XVPN_DRIVER_HOME_ROOT", "/home"),
		DriverProjectsDir:       getEnv("XVPN_DRIVER_PROJECTS_DIR", "/opt/xvpn/data/projects"),
		GitDir:                  getEnv("XVPN_GIT_DIR", "/opt/xvpn/data/git"),
		CodespacesDir:           getEnv("XVPN_CODESPACES_DIR", "/opt/xvpn/data/codespaces"),
		BackupDir:               getEnv("XVPN_BACKUP_DIR", "/opt/xvpn/data/backups"),
		BitLaunchToken:          os.Getenv("XVPN_BITLAUNCH_TOKEN"),
		CloudflareToken:         os.Getenv("XVPN_CLOUDFLARE_TOKEN"),
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
	// A sub-rede é a base da autenticação por IP de origem das rotas de
	// identidade por túnel (Fase 14): se ela não parseia, aquelas rotas
	// falham fechadas e o motivo fica escondido num log. Melhor recusar o
	// boot aqui, junto do resto da validação de configuração.
	if _, _, err := net.ParseCIDR(cfg.WireGuardAllowedSubnet); err != nil {
		return nil, fmt.Errorf("XVPN_WG_SUBNET inválido (%q): %w", cfg.WireGuardAllowedSubnet, err)
	}
	if err := validateTunnelHTTPAddr(cfg.TunnelHTTPAddr); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validateTunnelHTTPAddr recusa um bind que exponha o listener do túnel
// fora da wg0. É o invariante 2 do AGENTS.md aplicado no código, não só na
// documentação: o listener existe para atender peers em 10.66.66.x, e
// "0.0.0.0"/"" como host o colocaria também na interface pública, por fora
// do Nginx e do TLS.
func validateTunnelHTTPAddr(addr string) error {
	if addr == "" {
		return nil // listener desabilitado (default em dev/teste)
	}
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("XVPN_TUNNEL_HTTP_ADDR inválido (%q, esperado host:porta): %w", addr, err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("XVPN_TUNNEL_HTTP_ADDR deve ter um IP explícito, não um nome (%q)", addr)
	}
	if ip.IsUnspecified() {
		return fmt.Errorf("XVPN_TUNNEL_HTTP_ADDR não pode ser um endereço curinga (%q) — o listener do túnel só deve escutar na wg0", addr)
	}
	return nil
}
