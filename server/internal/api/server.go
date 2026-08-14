// Package api contém os handlers HTTP do control-plane (painel + endpoints
// de enrollment) e a montagem das rotas Gin. Lógica de negócio (validação,
// alocação de IP, chamadas ao wgctrl) fica nos outros pacotes internal/ —
// os handlers aqui só traduzem HTTP <-> chamadas de domínio (ver
// go-backend.mdc).
package api

import (
	"log/slog"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/config"
	"github.com/rootkit-lab/xvpn/server/internal/marketplace"
	"github.com/rootkit-lab/xvpn/server/internal/store"
	"github.com/rootkit-lab/xvpn/server/internal/wireguard"
)

// APIVersion é o contrato de compatibilidade cliente↔servidor descrito em
// PLAN.md §13.3: um inteiro incremental, bump sempre que um endpoint mudar
// de forma incompatível com clientes existentes.
const APIVersion = 1

// Limites dos endpoints públicos sensíveis (sem essa proteção, alguém
// podia tentar senhas/códigos de convite em loop apertado — ver
// ROADMAP.md Fase 9). Generosos o bastante para um usuário legítimo que
// erra a senha algumas vezes ou tem uma conexão instável durante o
// enrollment, curtos o suficiente para tornar força bruta pouco atrativa.
const (
	loginRateLimitMax    = 10
	loginRateLimitWindow = 5 * time.Minute

	enrollRateLimitMax    = 20
	enrollRateLimitWindow = 10 * time.Minute
)

// StartedAt é preenchido no boot do processo, usado por GET /api/status
// para relatar uptime.
var StartedAt = time.Now()

// App agrega as dependências compartilhadas pelos handlers.
type App struct {
	Store  *store.Store
	WG     wireguard.PeerManager
	Tokens *auth.TokenManager
	Config *config.Config

	// Marketplace grava/lê os blobs de asset do catálogo de software
	// (Fase 11 — ver PLAN.md §6.8). Nunca nil em produção (inicializado
	// em cmd/xvpn-server/main.go junto com o restante do App).
	Marketplace *marketplace.Store

	// UserProvisioner chama o binário privilegiado xvpn-user-provision
	// (Fase 13 — ver PLAN.md §6.9) para criar contas Unix e habilitar/
	// desabilitar SFTP/Samba por usuário. Nil quando a Fase 13 não
	// está configurada neste servidor (ex.: binário não instalado) —
	// o handler devolve 503 nesse caso. Em produção é inicializado em
	// cmd/xvpn-server/main.go junto com o restante do App.
	UserProvisioner UserProvisioner

	// ServerPublicKey é derivada da chave privada lida na inicialização
	// (nunca a chave privada em si) e devolvida aos clientes no enrollment.
	ServerPublicKey string

	// enrollMu serializa operações de enrollment (alocação de IP +
	// registro de peer), evitando duas requisições simultâneas alocarem o
	// mesmo IP livre.
	enrollMu sync.Mutex

	// waitlistLimiter protege o único endpoint de escrita público (sem
	// autenticação) da API, POST /api/waitlist — ver ratelimit.go.
	waitlistLimiter *ipRateLimiter

	// loginLimiter/enrollLimiter protegem os outros dois endpoints sem
	// JWT (login ainda não tem sessão; enroll usa só o código de
	// convite) contra tentativas em loop — ver ratelimit.go.
	loginLimiter  *ipRateLimiter
	enrollLimiter *ipRateLimiter

	// statusCache* memoizam a última resposta de GET /api/status por
	// statusCacheTTL — endpoint público e chamado em polling pelo painel
	// e pelo cliente desktop, sem isso cada requisição batia direto no
	// wgctrl/kernel (ver status_handler.go e ROADMAP.md Fase 9).
	statusCacheMu   sync.Mutex
	statusCacheAt   time.Time
	statusCacheResp statusResponse
}

// NewRouter monta todas as rotas da API sobre um App já inicializado.
func NewRouter(app *App) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger())

	if app.waitlistLimiter == nil {
		// 5 tentativas por IP a cada 10 minutos — generoso pra um
		// visitante legítimo (que só envia o formulário uma vez), curto
		// o suficiente para tornar spam/scraping automatizado pouco
		// atrativo nesse endpoint público.
		app.waitlistLimiter = newIPRateLimiter(5, 10*time.Minute)
	}
	if app.loginLimiter == nil {
		app.loginLimiter = newIPRateLimiter(loginRateLimitMax, loginRateLimitWindow)
	}
	if app.enrollLimiter == nil {
		app.enrollLimiter = newIPRateLimiter(enrollRateLimitMax, enrollRateLimitWindow)
	}

	apiGroup := r.Group("/api")
	{
		apiGroup.POST("/auth/login", rateLimit(app.loginLimiter), app.handleLogin)
		apiGroup.POST("/devices/enroll", rateLimit(app.enrollLimiter), app.handleDeviceEnroll)
		apiGroup.GET("/status", app.handleStatus)
		// Único endpoint de escrita da API sem autenticação — ver
		// waitlist_handler.go e AGENTS.md (qualquer superfície pública
		// nova precisa de justificativa explícita).
		apiGroup.POST("/waitlist", rateLimit(app.waitlistLimiter), app.handleJoinWaitlist)

		// authed: qualquer papel autenticado (inclusive member) — só
		// identidade própria, sem telas de admin (ver PLAN.md §6.7,
		// tabela de papéis: "member: sem telas de admin, portal
		// mínimo").
		authed := apiGroup.Group("")
		authed.Use(auth.RequireAuth(app.Tokens))
		{
			authed.GET("/auth/me", app.handleMe)

			authed.GET("/me/devices", app.handleListMyDevices)
			authed.DELETE("/me/devices/:id", app.handleDeleteMyDevice)

			// Marketplace (Fase 11, PLAN.md §6.8): catálogo e download são
			// liberados a qualquer papel autenticado (inclusive member) —
			// a ACL de fato (global vs. restrito) é aplicada dentro do
			// handler, não no roteamento, porque depende do app/asset e
			// não só do papel do chamador (ver PLAN.md §6.7, coluna
			// Marketplace: viewer/member "download se ACL permitir").
			authed.GET("/marketplace/apps", app.handleListMarketplaceApps)
			authed.GET("/marketplace/assets/:id/download", app.handleDownloadMarketplaceAsset)
		}

		// viewerUp: leitura das telas de admin (dashboard, listas,
		// audit) — inclui viewer, admin e super_admin.
		viewerUp := apiGroup.Group("")
		viewerUp.Use(auth.RequireAuth(app.Tokens), auth.RequireRole(store.ViewerUpRoles...))
		{
			viewerUp.GET("/users", app.handleListUsers)
			viewerUp.GET("/devices", app.handleListDevices)
			viewerUp.GET("/waitlist", app.handleListWaitlist)
			viewerUp.GET("/audit", app.handleListAudit)
			viewerUp.GET("/config", app.handleGetConfig)
		}

		// adminOnly: escrita nas telas de admin — admin e super_admin.
		// viewer fica de fora mesmo aqui (ex.: "não cria convites", ver
		// PLAN.md §6.7); a distinção admin-vs-super_admin dentro desse
		// grupo (ex.: só super_admin promove outro super_admin) é
		// aplicada dentro de cada handler via store.Role.CanManage, não
		// aqui no roteamento.
		adminOnly := apiGroup.Group("")
		adminOnly.Use(auth.RequireAuth(app.Tokens), auth.RequireRole(store.AdminRoles...))
		{
			adminOnly.POST("/users", app.handleCreateUser)
			adminOnly.PATCH("/users/:id", app.handleUpdateUser)
			adminOnly.DELETE("/users/:id", app.handleDeleteUser)
			adminOnly.POST("/users/:id/invite", app.handleCreateInvite)
			adminOnly.POST("/users/:id/reset-password", app.handleResetPassword)
			// Acesso a arquivos (Fase 13, PLAN.md §6.9): toggle SFTP/Samba
			// + chave pública SSH por usuário. adminOnly — ação de escrita
			// que provisiona conta Unix na VPS.
			adminOnly.PUT("/users/:id/file-access", app.handleSetFileAccess)

			adminOnly.DELETE("/devices/:id", app.handleDeleteDevice)

			adminOnly.POST("/waitlist/:id/approve", app.handleApproveWaitlist)
			adminOnly.POST("/waitlist/:id/reject", app.handleRejectWaitlist)
			adminOnly.POST("/waitlist/:id/provision", app.handleProvisionWaitlist)

			// Marketplace: gestão do catálogo (criar/editar/remover
			// apps/versões, subir asset, ajustar ACL) — ver PLAN.md §6.7,
			// "admin: Admin + download" no marketplace.
			adminOnly.POST("/marketplace/apps", app.handleCreateMarketplaceApp)
			adminOnly.PATCH("/marketplace/apps/:id", app.handleUpdateMarketplaceApp)
			adminOnly.DELETE("/marketplace/apps/:id", app.handleDeleteMarketplaceApp)
			adminOnly.PUT("/marketplace/apps/:id/access", app.handleSetMarketplaceAppAccess)
			adminOnly.POST("/marketplace/apps/:id/versions", app.handleCreateMarketplaceVersion)
			adminOnly.DELETE("/marketplace/versions/:id", app.handleDeleteMarketplaceVersion)
			adminOnly.POST("/marketplace/versions/:id/assets", app.handleUploadMarketplaceAsset)
			adminOnly.DELETE("/marketplace/assets/:id", app.handleDeleteMarketplaceAsset)
		}
	}

	registerWebUI(r)

	return r
}

// requestLogger emite uma linha slog por request HTTP, sem headers/corpo
// (podem conter tokens/senhas) — ver go-backend.mdc.
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("http",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	}
}
