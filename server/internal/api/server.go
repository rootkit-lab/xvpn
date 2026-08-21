// Package api contém os handlers HTTP do control-plane (painel + endpoints
// de enrollment) e a montagem das rotas Gin. Lógica de negócio (validação,
// alocação de IP, chamadas ao wgctrl) fica nos outros pacotes internal/ —
// os handlers aqui só traduzem HTTP <-> chamadas de domínio (ver
// go-backend.mdc).
package api

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
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
//
// 2 (Fase 14): o enrollment passou a devolver `username` e a API ganhou as
// rotas de identidade por túnel (GET /api/me, POST /api/me/ssh-key). Um
// cliente da versão 1 continua funcionando contra um servidor 2, mas o
// contrário não: o cliente 2 conta com o username para abrir o share
// pessoal.
const APIVersion = 2

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

	wsConnRateLimitMax    = 30
	wsConnRateLimitWindow = 1 * time.Minute
	msgRateLimitMax       = 60
	msgRateLimitWindow    = 1 * time.Minute

	// POST /api/me/ssh-key só responde dentro do túnel e é idempotente
	// (mesma chave = no-op), mas trocar de chave a cada requisição faz o
	// servidor reescrever o authorized_keys e recarregar o sshd. O limite
	// existe para isso: um peer comprometido não deve conseguir manter o
	// sshd em recarga contínua. Folgado o suficiente para o uso normal, que
	// é uma chamada por conexão de túnel.
	sshKeyRateLimitMax    = 12
	sshKeyRateLimitWindow = 10 * time.Minute
)

// StartedAt é preenchido no boot do processo, usado por GET /api/status
// para relatar uptime.
var StartedAt = time.Now()

// trustedProxies lista as únicas origens em que o Gin pode confiar para
// derivar c.ClientIP() a partir de X-Forwarded-For/X-Real-IP. Só o
// loopback: o xvpn-server escuta em 127.0.0.1:8080 (config.HTTPAddr) e o
// único proxy à frente dele é o Nginx local (deploy/nginx/xvpn.conf).
var trustedProxies = []string{"127.0.0.1/32", "::1/128"}

// App agrega as dependências compartilhadas pelos handlers.
type App struct {
	Store  *store.Store
	WG     wireguard.PeerManager
	Tokens *auth.TokenManager
	// Handoff guarda tickets opacos de SSO (uso único, 60s).
	Handoff *auth.TicketStore
	Config  *config.Config

	// Marketplace grava/lê os blobs de asset do catálogo de software
	// (Fase 11 — ver PLAN.md §6.8). Nunca nil em produção (inicializado
	// em cmd/xvpn-server/main.go junto com o restante do App).
	Marketplace *marketplace.Store

	// SocialMedia guarda anexos/áudio/stories do chat (Fase 21). Mesmo
	// store content-addressed do marketplace, raiz distinta.
	SocialMedia *marketplace.Store

	// Packages guarda tarballs/artefatos do registry XGIT (Fase 45.1).
	// Mesmo store SHA-256 do marketplace, raiz XVPN_PACKAGES_DIR.
	Packages *marketplace.Store

	// BitLaunch (Fase 38). Só os testes injetam o fake. Em produção o
	// cliente sai de BitLaunchAccount (Compute → Configurações).
	BitLaunch BitLaunchAPI

	// Cloudflare (Fase 39). Só os testes injetam o fake. Em produção o
	// cliente sai de CloudflareAccount (DNS → Configurações).
	Cloudflare CloudflareAPI

	// fetchAsset baixa um asset por URL durante o sync (Fase 16). Nil =
	// marketplace.FetchAndPut. Os testes injetam um fake sem rede.
	fetchAsset func(context.Context, *marketplace.Store, string, string) (marketplace.PutResult, string, error)

	// UserProvisioner chama o binário privilegiado xvpn-user-provision
	// (Fase 13 — ver PLAN.md §6.9) para criar contas Unix e habilitar/
	// desabilitar SFTP/Samba por usuário. Nil quando a Fase 13 não
	// está configurada neste servidor (ex.: binário não instalado) —
	// o handler devolve 503 nesse caso. Em produção é inicializado em
	// cmd/xvpn-server/main.go junto com o restante do App.
	UserProvisioner UserProvisioner

	// Backup (Fase 44) roda restic/rclone. Nil = backup.Runner no PATH.
	Backup backupEngine

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

	// sshKeyLimiter protege POST /api/me/ssh-key, que também não usa JWT
	// (autentica pelo IP de origem dentro do túnel) e chama o
	// provisionador — ver sshKeyRateLimitMax.
	sshKeyLimiter *ipRateLimiter

	// llmLimiter limita chat/generate commit por usuário.
	llmLimiter *ipRateLimiter
	// llmHTTP é injetado nos testes (httptest). Nil = http.Client padrão.
	llmHTTP *http.Client

	// Hub entrega eventos do social (Fase 19.3) em memória. Um node.
	Hub *Hub

	// wsLimiter limita tentativas de upgrade por IP; msgLimiter limita
	// POST de mensagem por usuário (chave "u:<id>").
	wsLimiter  *ipRateLimiter
	msgLimiter *ipRateLimiter

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

	// Sem isto o Gin trata qualquer origem como proxy confiável
	// (gin.New() nasce com trustedProxies = 0.0.0.0/0 + ::/0) e deriva
	// c.ClientIP() do X-Forwarded-For — um header que o cliente escolhe.
	// Como ClientIP() é a chave do rate limit dos três endpoints sem
	// autenticação (login, enroll, waitlist), qualquer um zerava as três
	// proteções trocando o header a cada requisição.
	//
	// Confiar só no loopback corrige nas duas pontas: o Nginx anexa o
	// $remote_addr real no fim do X-Forwarded-For
	// ($proxy_add_x_forwarded_for) e o Gin percorre a cadeia da direita
	// para a esquerda, parando no primeiro IP não-confiável — o IP real,
	// seja lá o que o cliente tenha forjado à esquerda dele. Requisições
	// que não chegam pelo loopback nem consultam o header.
	//
	// A confiança é declarada explicitamente aqui, em vez de herdada do
	// padrão, também por causa da Fase 14 (GET /api/me e POST
	// /api/me/ssh-key autenticam o dispositivo pelo IP de origem dentro
	// de 10.66.66.0/24, com listener próprio em 10.66.66.1:8080):
	// acrescentar qualquer faixa não-loopback nesta lista — ou registrar
	// essas rotas neste router público — volta a tornar o IP de origem
	// forjável e derruba a autenticação delas junto.
	if err := r.SetTrustedProxies(trustedProxies); err != nil {
		// Lista constante e validada em teste: só falha se alguém editá-la
		// com um CIDR inválido. Abortar o boot é melhor que subir com a
		// lista vazia, porque aí o ClientIP de todo mundo vira 127.0.0.1 e
		// o rate limit passa a derrubar usuários legítimos em vez de
		// proteger.
		panic(fmt.Sprintf("lista de proxies confiáveis inválida: %v", err))
	}

	r.Use(gin.Recovery())
	r.Use(requestLogger())
	r.Use(app.maybeServePages())
	r.Use(app.maybeCodespaceProxy())

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
	if app.sshKeyLimiter == nil {
		app.sshKeyLimiter = newIPRateLimiter(sshKeyRateLimitMax, sshKeyRateLimitWindow)
	}
	if app.llmLimiter == nil {
		app.llmLimiter = newIPRateLimiter(20, time.Minute)
	}
	if app.Hub == nil {
		app.Hub = newHub()
	}
	if app.wsLimiter == nil {
		app.wsLimiter = newIPRateLimiter(wsConnRateLimitMax, wsConnRateLimitWindow)
	}
	if app.msgLimiter == nil {
		app.msgLimiter = newIPRateLimiter(msgRateLimitMax, msgRateLimitWindow)
	}
	if app.Handoff == nil {
		app.Handoff = auth.NewTicketStore()
	}

	apiGroup := r.Group("/api")
	{
		apiGroup.POST("/auth/login", rateLimit(app.loginLimiter), app.handleLogin)
		apiGroup.POST("/auth/session", rateLimit(app.loginLimiter), app.handleEstablishSession)
		apiGroup.GET("/auth/handoff-continue", rateLimit(app.loginLimiter), app.handleHandoffContinue)
		apiGroup.GET("/auth/redeem", rateLimit(app.loginLimiter), app.handleRedeemHandoff)
		apiGroup.POST("/auth/logout", app.handleLogout)
		apiGroup.POST("/devices/enroll", rateLimit(app.enrollLimiter), app.handleDeviceEnroll)
		apiGroup.POST("/servers/enroll", rateLimit(app.enrollLimiter), app.handleMeshServerEnroll)
		apiGroup.GET("/status", app.handleStatus)
		// Escritas públicas: login, session (handoff SSO), enroll, waitlist.
		// Qualquer superfície pública nova precisa de justificativa (AGENTS.md).
		apiGroup.POST("/waitlist", rateLimit(app.waitlistLimiter), app.handleJoinWaitlist)

		// WebSocket social (Fase 19.3): upgrade sem JWT no handshake —
		// o token vai no primeiro frame, nunca na query (access log).
		apiGroup.GET("/ws", rateLimit(app.wsLimiter), app.handleSocialWS)

		// Identidade por túnel (Fase 14, PLAN.md §6.9): as duas únicas
		// rotas que autenticam por IP de origem em vez de JWT. Ficam neste
		// mesmo router de propósito — RequireTunnelOrigin exige
		// RemoteIP() dentro de 10.66.66.0/24, e o Nginx conecta de
		// 127.0.0.1, então tudo que vem pelo painel público é rejeitado
		// por construção, sem precisar de uma segunda árvore de rotas.
		// Em produção elas só são alcançáveis pelo listener adicional em
		// 10.66.66.1:8080 (XVPN_TUNNEL_HTTP_ADDR, ver PLAN.md §5), porque
		// o HTTPS do painel não trafega dentro do túnel.
		tunnel := apiGroup.Group("", app.RequireTunnelOrigin())
		{
			tunnel.GET("/me", app.handleTunnelIdentity)
			tunnel.POST("/me/ssh-key", rateLimit(app.sshKeyLimiter), app.handleRegisterDeviceSSHKey)
		}

		// CI runner (Fase 42): só wg0. Token do MeshServer role=runner.
		// O agent fala com 10.66.66.1:8080 — Nginx público (127.0.0.1) cai.
		ci := apiGroup.Group("/ci", app.RequireVPN())
		{
			ci.GET("/jobs/next", app.handleCiClaim)
			ci.POST("/jobs/:id/log", app.handleCiLog)
			ci.POST("/jobs/:id/finish", app.handleCiFinish)
			ci.POST("/jobs/:id/artifact", app.handleCiArtifact)
		}

		// Agent de serviços (Fase 43): só wg0. Token do MeshServer mesh/runner.
		svcAgent := apiGroup.Group("/svc", app.RequireVPN())
		{
			svcAgent.GET("/desired", app.handleSvcDesired)
			svcAgent.POST("/:id/status", app.handleSvcAgentStatus)
		}

		// Assistente do codespace: JWE (painel) ou token Git do
		// container (extensão Node — sem cookie do browser).
		llm := apiGroup.Group("")
		llm.Use(app.requireCodespaceLLMHost(), app.requireLLMCaller(), app.refreshCallerFromDB())
		{
			llm.GET("/xcodespaces/llm/models", app.handleLLMModels)
			llm.POST("/xcodespaces/llm/chat", rateLimit(app.llmLimiter), app.handleLLMChat)
			llm.POST("/xcodespaces/llm/commit-message", rateLimit(app.llmLimiter), app.handleLLMCommitMessage)
		}

		// authed: qualquer papel autenticado (inclusive member) — só
		// identidade própria, sem telas de admin (ver PLAN.md §6.7,
		// tabela de papéis: "member: sem telas de admin, portal
		// mínimo").
		authed := apiGroup.Group("")
		authed.Use(auth.RequireAuth(app.Tokens), app.refreshCallerFromDB(), auth.RejectPackagesScopedToken())
		{
			authed.GET("/auth/me", app.handleMe)

			authed.GET("/me/devices", app.handleListMyDevices)
			authed.DELETE("/me/devices/:id", app.handleDeleteMyDevice)
			authed.PUT("/me/ssh-public-key", app.handleUpdateMySSHPublicKey)
			// Troca de senha do próprio usuário (Fase 18). Rate limit
			// reusa o do login: a senha atual é o mesmo segredo que o
			// POST /auth/login protege contra força bruta.
			authed.PATCH("/me/password", rateLimit(app.loginLimiter), app.handleChangeMyPassword)
			authed.GET("/me/dns-suffixes", app.handleMeDNSSuffixes)

			// Marketplace (Fase 11, PLAN.md §6.8): catálogo e download são
			// liberados a qualquer papel autenticado (inclusive member) —
			// a ACL de fato (global vs. restrito) é aplicada dentro do
			// handler, não no roteamento, porque depende do app/asset e
			// não só do papel do chamador (ver PLAN.md §6.7, coluna
			// Marketplace: viewer/member "download se ACL permitir").
			authed.GET("/marketplace/apps", app.handleListMarketplaceApps)
			authed.GET("/marketplace/assets/:id/download", app.handleDownloadMarketplaceAsset)

			authed.GET("/social/people", app.handleSocialPeople)
			authed.GET("/social/profile", app.handleSocialMeGet)
			authed.PATCH("/social/profile", app.handleSocialMePatch)
			authed.GET("/social/u/:username", app.handleSocialProfileGet)
			authed.POST("/social/follow/:username", app.handleSocialFollow)
			authed.DELETE("/social/follow/:username", app.handleSocialUnfollow)
			authed.GET("/social/groups", app.handleSocialListGroups)
			authed.POST("/social/groups", app.handleSocialCreateGroup)
			authed.POST("/social/groups/:id/invite", app.handleSocialInviteGroup)
			authed.GET("/social/threads", app.handleSocialListThreads)
			authed.POST("/social/threads", app.handleSocialOpenThread)
			authed.GET("/social/threads/:kind/:id/messages", app.handleSocialListMessages)
			authed.POST("/social/threads/:kind/:id/messages", app.handleSocialPostMessage)
			authed.POST("/social/attachments", app.handleSocialUploadAttachment)
			authed.GET("/social/attachments/:id", app.handleSocialDownloadAttachment)
			authed.GET("/social/stories", app.handleSocialListStories)
			authed.POST("/social/stories", app.handleSocialCreateStory)
			authed.POST("/social/stories/:id/view", app.handleSocialViewStory)
			authed.POST("/social/acks", app.handleSocialAck)
			authed.GET("/social/feed", app.handleSocialFeed)
			authed.POST("/social/posts", app.handleSocialCreatePost)
			authed.GET("/social/u/:username/posts", app.handleSocialUserPosts)
			authed.DELETE("/social/posts/:id", app.handleSocialDeletePost)
			authed.POST("/social/posts/:id/star", app.handleSocialStarPost)
			authed.GET("/social/posts/:id/comments", app.handleSocialListComments)
			authed.POST("/social/posts/:id/comments", app.handleSocialCreateComment)
			authed.POST("/social/posts/:id/repost", app.handleSocialRepost)

			// Forge (Fase 37): listagem e detalhe para o picker do feed
			// e o XDRIVER. Member só vê projetos em que participa.
			authed.GET("/orgs/:org", app.handleGetOrg)
			authed.GET("/orgs/:org/teams/:team/members", app.handleListTeamMembers)
			authed.POST("/orgs/:org/teams/:team/members", app.handleAddTeamMember)
			authed.DELETE("/orgs/:org/teams/:team/members/:userID", app.handleRemoveTeamMember)
			authed.GET("/projects", app.handleListProjects)
			authed.POST("/xgit/repos", app.handleCreateProjectAuthed)
			authed.GET("/projects/:org/:slug", app.handleGetProject)
			authed.GET("/projects/:org/:slug/git", app.handleGetProjectGit)
			authed.GET("/projects/:org/:slug/codespaces/envs", app.handleGetProjectCodespaceEnvs)
			authed.PUT("/projects/:org/:slug/codespaces/envs", app.handlePutProjectCodespaceEnvs)
			authed.GET("/projects/:org/:slug/tree", app.handleListTree)
			authed.GET("/projects/:org/:slug/blob", app.handleGetBlob)
			authed.GET("/projects/:org/:slug/commits", app.handleListCommits)
			authed.GET("/xgit/settings", app.handleGetForgeSettings)
			authed.GET("/xgit/overview", app.handleXgitOverview)
			authed.GET("/xgit/stars", app.handleXgitStars)
			registry := authed.Group("")
			registry.Use(app.RequireRegistryHost())
			{
				registry.GET("/registry/token", app.handleRegistryToken)
				registry.GET("/registry/auth", app.handleRegistryAuth)
			}
			packages := authed.Group("")
			packages.Use(app.RequirePackagesHost())
			{
				packages.GET("/xgit/packages", app.handleListForgePackages)
				packages.GET("/projects/:org/:slug/packages", app.handleListProjectPackages)
				packages.POST("/projects/:org/:slug/packages", app.handleUploadProjectPackage)
				packages.GET("/projects/:org/:slug/packages/:id/download", app.handleDownloadPackageVersion)
				packages.PUT("/packages/:org/:slug/npm/*pkg", app.handleNpmPublish)
				packages.GET("/packages/:org/:slug/npm/*pkg", app.handleNpmPackument)
				packages.POST("/packages/:org/:slug/pypi", app.handlePypiUpload)
				packages.GET("/packages/:org/:slug/pypi/simple", app.handlePypiSimpleIndex)
				packages.GET("/packages/:org/:slug/pypi/simple/", app.handlePypiSimpleIndex)
				packages.GET("/packages/:org/:slug/pypi/simple/:name", app.handlePypiSimplePackage)
				packages.GET("/packages/:org/:slug/pypi/simple/:name/", app.handlePypiSimplePackage)
				packages.PUT("/packages/:org/:slug/maven/*filepath", app.handleMavenPut)
				packages.GET("/packages/:org/:slug/maven/*filepath", app.handleMavenGet)
				packages.GET("/packages/:org/:slug/nuget/index.json", app.handleNugetIndex)
				packages.PUT("/packages/:org/:slug/nuget", app.handleNugetPush)
				packages.POST("/packages/:org/:slug/nuget", app.handleNugetPush)
				packages.GET("/packages/:org/:slug/nuget/flat/:name/index.json", app.handleNugetVersions)
				packages.GET("/packages/:org/:slug/nuget/flat/:name/:version/:file", app.handleNugetDownload)
				packages.POST("/packages/:org/:slug/rubygems", app.handleRubygemsPush)
				packages.POST("/packages/:org/:slug/rubygems/api/v1/gems", app.handleRubygemsPush)
				packages.GET("/packages/:org/:slug/rubygems/gems/:filename", app.handleRubygemsDownload)
			}
			authed.POST("/projects/:org/:slug/star", app.handleToggleProjectStar)
			authed.GET("/xcodespaces", app.handleListCodespaces)
			authed.POST("/xcodespaces", app.handleCreateCodespace)
			authed.GET("/xcodespaces/:id", app.handleGetCodespace)
			authed.POST("/xcodespaces/:id/start", app.handleStartCodespace)
			authed.POST("/xcodespaces/:id/stop", app.handleStopCodespace)
			authed.PATCH("/xcodespaces/:id/demo", app.handlePatchCodespaceDemo)
			authed.DELETE("/xcodespaces/:id", app.handleDeleteCodespace)
			authed.GET("/xcodespaces/:id/tree", app.handleCodespaceTree)
			authed.GET("/xcodespaces/:id/blob", app.handleCodespaceBlob)
			authed.PUT("/xcodespaces/:id/contents", app.handleCodespaceWrite)
			authed.POST("/xcodespaces/:id/commit", app.handleCodespaceCommit)
			authed.GET("/projects/:org/:slug/branches", app.handleListProjectBranches)
			authed.GET("/projects/:org/:slug/issues", app.handleListIssues)
			authed.POST("/projects/:org/:slug/issues", app.handleCreateIssue)
			authed.GET("/projects/:org/:slug/issues/:n", app.handleGetIssue)
			authed.PATCH("/projects/:org/:slug/issues/:n", app.handlePatchIssue)
			authed.GET("/projects/:org/:slug/labels", app.handleListIssueLabels)
			authed.GET("/projects/:org/:slug/milestones", app.handleListMilestones)
			authed.POST("/projects/:org/:slug/milestones", app.handleCreateMilestone)
			authed.PATCH("/projects/:org/:slug/milestones/:n", app.handlePatchMilestone)
			authed.GET("/projects/:org/:slug/work-projects", app.handleListWorkProjects)
			authed.POST("/projects/:org/:slug/work-projects", app.handleCreateWorkProject)
			authed.GET("/projects/:org/:slug/work-projects/:n", app.handleGetWorkProject)
			authed.PATCH("/projects/:org/:slug/work-projects/:n", app.handlePatchWorkProject)
			authed.GET("/projects/:org/:slug/work-projects/:n/items", app.handleListWorkItems)
			authed.POST("/projects/:org/:slug/work-projects/:n/items", app.handleCreateWorkItem)
			authed.PATCH("/projects/:org/:slug/work-projects/:n/items/:id", app.handlePatchWorkItem)
			authed.DELETE("/projects/:org/:slug/work-projects/:n/items/:id", app.handleDeleteWorkItem)
			authed.GET("/projects/:org/:slug/merge-requests", app.handleListMergeRequests)
			authed.POST("/projects/:org/:slug/merge-requests", app.handleCreateMergeRequest)
			authed.GET("/projects/:org/:slug/merge-requests/:iid", app.handleGetMergeRequest)
			authed.PATCH("/projects/:org/:slug/merge-requests/:iid", app.handlePatchMergeRequest)
			authed.POST("/projects/:org/:slug/merge-requests/:iid/merge", app.handleMergeMergeRequest)
			authed.POST("/projects/:org/:slug/merge-requests/:iid/close", app.handleCloseMergeRequest)
			authed.GET("/projects/:org/:slug/merge-requests/:iid/commits", app.handleListMRCommits)
			authed.GET("/projects/:org/:slug/merge-requests/:iid/diff", app.handleGetMRDiff)
			authed.GET("/projects/:org/:slug/merge-requests/:iid/reviews", app.handleListMRReviews)
			authed.POST("/projects/:org/:slug/merge-requests/:iid/reviews", app.handleCreateMRReview)
			authed.PUT("/projects/:org/:slug/contents", app.handlePutContents)
			authed.GET("/projects/:org/:slug/archive", app.handleGetArchive)
			authed.GET("/ci/workflow-templates", app.handleListWorkflowTemplates)
			authed.POST("/projects/:org/:slug/workflows", app.handleApplyWorkflowTemplate)
			authed.GET("/projects/:org/:slug/jobs", app.handleListCiJobs)
			authed.GET("/projects/:org/:slug/jobs/:n", app.handleGetCiJob)
			authed.GET("/projects/:org/:slug/jobs/:n/log", app.handleGetCiJobLog)
			authed.GET("/projects/:org/:slug/jobs/:n/artifact", app.handleGetCiJobArtifact)
			authed.POST("/projects/:org/:slug/jobs/:n/cancel", app.handleCancelCiJob)
			authed.POST("/projects/:org/:slug/jobs/:n/approve", app.handleApproveCiJob)
			authed.POST("/projects/:org/:slug/jobs/:n/rerun", app.handleRerunCiJob)
			authed.GET("/projects/:org/:slug/runners", app.handleListProjectRunners)
			authed.GET("/projects/:org/:slug/services", app.handleListProjectServices)
			authed.GET("/projects/:org/:slug/wiki", app.handleListWiki)
			authed.GET("/projects/:org/:slug/wiki/:page", app.handleGetWikiPage)
			authed.PUT("/projects/:org/:slug/wiki/:page", app.handlePutWikiPage)
			authed.GET("/projects/:org/:slug/pages", app.handleGetPagesStatus)
			authed.POST("/projects/:org/:slug/pages", app.handlePublishPages)
			authed.GET("/projects/:org/:slug/security", app.handleGetProjectSecurity)
			authed.POST("/projects/:org/:slug/security/report", app.handleCreateSecurityReport)
			authed.GET("/projects/:org/:slug/agents", app.handleListProjectAgents)

			driver := authed.Group("/driver")
			driver.Use(app.RequireDriverHost())
			{
				driver.GET("/ls", app.handleDriverList)
				driver.POST("/mkdir", app.handleDriverMkdir)
				driver.POST("/upload", app.handleDriverUpload)
				driver.GET("/download", app.handleDriverDownload)
				driver.PUT("/write", app.handleDriverWrite)
				driver.POST("/extract", app.handleDriverExtract)
				driver.DELETE("/rm", app.handleDriverDelete)
			}
		}

		// viewerUp: leitura das telas de admin (dashboard, listas,
		// audit) — inclui viewer, admin e super_admin.
		viewerUp := apiGroup.Group("")
		viewerUp.Use(auth.RequireAuth(app.Tokens), app.refreshCallerFromDB(), auth.RejectPackagesScopedToken(), auth.RequireRole(store.ViewerUpRoles...))
		{
			viewerUp.GET("/users", app.handleListUsers)
			viewerUp.GET("/users/:id", app.handleGetUser)
			viewerUp.GET("/devices", app.handleListDevices)
			viewerUp.GET("/waitlist", app.handleListWaitlist)
			viewerUp.GET("/audit", app.handleListAudit)
			viewerUp.GET("/config", app.handleGetConfig)
			viewerUp.GET("/config/xcodespaces", app.handleGetCodespaceSettings)
			viewerUp.GET("/dns", app.handleGetDNS)
			viewerUp.GET("/dns/recursor", app.handleDNSRecursor)
			viewerUp.GET("/dns/public/settings", app.handleGetPublicDNSSettings)
			viewerUp.GET("/dns/public/zones", app.handleListPublicZones)
			viewerUp.GET("/dns/public/zones/:id", app.handleGetPublicZone)
			viewerUp.GET("/dns/public/zones/:id/records", app.handleListPublicRecords)
			// Estatísticas agregadas do marketplace (Fase 12): mesmo nível
			// de leitura do resto do dashboard — não authed (não é algo
			// que member precise pra navegar o catálogo) nem adminOnly
			// (não escreve nada).
			viewerUp.GET("/marketplace/stats", app.handleMarketplaceStats)
			viewerUp.GET("/servers", app.handleListMeshServers)
			viewerUp.GET("/servers/:id", app.handleGetMeshServer)
			viewerUp.GET("/server-groups", app.handleListServerGroups)
			viewerUp.GET("/compute/settings", app.handleGetComputeSettings)
			viewerUp.GET("/services", app.handleListServices)
			viewerUp.GET("/services/:slug", app.handleGetService)
			viewerUp.GET("/backups/settings", app.handleGetBackupSettings)
			viewerUp.GET("/backups/destinations", app.handleListBackupDestinations)
			viewerUp.GET("/backups/jobs", app.handleListBackupJobs)
		}

		// adminOnly: escrita nas telas de admin — admin e super_admin.
		// viewer fica de fora mesmo aqui (ex.: "não cria convites", ver
		// PLAN.md §6.7); a distinção admin-vs-super_admin dentro desse
		// grupo (ex.: só super_admin promove outro super_admin) é
		// aplicada dentro de cada handler via store.Role.CanManage, não
		// aqui no roteamento.
		adminOnly := apiGroup.Group("")
		adminOnly.Use(auth.RequireAuth(app.Tokens), app.refreshCallerFromDB(), auth.RejectPackagesScopedToken(), auth.RequireRole(store.AdminRoles...))
		{
			// IAM: create/invite/patch/reset — não é produto. Todo admin+
			// continua gerenciando contas (CanManage + CoversAccount).
			// DELETE revoga peers/SFTP: exige core, senão a loja
			// contorna RequireProduct(core) em /devices.
			adminOnly.POST("/users", app.handleCreateUser)
			adminOnly.PATCH("/users/:id", app.handleUpdateUser)
			adminOnly.POST("/users/:id/invite", app.handleCreateInvite)
			adminOnly.POST("/users/:id/reset-password", app.handleResetPassword)

			// Core VPN (Fase 33): peers, waitlist, TTLs, apagar usuário.
			// Admin sem products:["core"] não revoga WireGuard.
			coreWrite := adminOnly.Group("")
			coreWrite.Use(auth.RequireProduct(store.ProductCore))
			{
				coreWrite.DELETE("/users/:id", app.handleDeleteUser)
				coreWrite.DELETE("/devices/:id", app.handleDeleteDevice)
				coreWrite.POST("/waitlist/:id/approve", app.handleApproveWaitlist)
				coreWrite.POST("/waitlist/:id/reject", app.handleRejectWaitlist)
				coreWrite.POST("/waitlist/:id/provision", app.handleProvisionWaitlist)
				coreWrite.PATCH("/config", app.handleUpdateConfig)
				coreWrite.PATCH("/config/xcodespaces", app.handlePatchCodespaceSettings)
				coreWrite.POST("/config/xcodespaces/test", rateLimit(app.llmLimiter), app.handleTestCodespaceLLM)
				coreWrite.PATCH("/backups/settings", app.handlePatchBackupSettings)
				coreWrite.POST("/backups/destinations", app.handleCreateBackupDestination)
				coreWrite.PATCH("/backups/destinations/:id", app.handlePatchBackupDestination)
				coreWrite.DELETE("/backups/destinations/:id", app.handleDeleteBackupDestination)
				coreWrite.POST("/backups/destinations/:id/run", app.handleRunBackup)
			}

			// DNS intranet (Fase 35) + público do stack (Fase 39).
			dnsWrite := adminOnly.Group("")
			dnsWrite.Use(auth.RequireProduct(store.ProductDNS))
			{
				dnsWrite.PATCH("/dns", app.handleUpdateDNSSettings)
				dnsWrite.POST("/dns/records", app.handleCreateDNSRecord)
				dnsWrite.PATCH("/dns/records/:id", app.handleUpdateDNSRecord)
				dnsWrite.DELETE("/dns/records/:id", app.handleDeleteDNSRecord)
				dnsWrite.POST("/dns/apply", app.handleApplyDNS)
				dnsWrite.POST("/dns/public/settings/accounts", app.handleCreateCloudflareAccount)
				dnsWrite.DELETE("/dns/public/settings/accounts/:id", app.handleDeleteCloudflareAccount)
				dnsWrite.POST("/dns/public/zones", app.handleCreatePublicZone)
				dnsWrite.POST("/dns/public/zones/import", app.handleImportPublicZones)
				dnsWrite.POST("/dns/public/zones/:id/records", app.handleCreatePublicRecord)
				dnsWrite.DELETE("/dns/public/zones/:id/records/:rid", app.handleDeletePublicRecord)
			}

			// XDriver: SFTP/Samba/cota. Fora do escopo xdriver → 403.
			driverWrite := adminOnly.Group("")
			driverWrite.Use(auth.RequireProduct(store.ProductXDriver))
			{
				driverWrite.PUT("/users/:id/file-access", app.handleSetFileAccess)
				driverWrite.GET("/users/:id/ssh-keys", app.handleListUserSSHKeys)
			}

			// Marketplace: só a ACL operacional no painel (Fase 16).
			marketWrite := adminOnly.Group("")
			marketWrite.Use(auth.RequireProduct(store.ProductMarketplace))
			{
				marketWrite.PUT("/marketplace/apps/:id/access", app.handleSetMarketplaceAppAccess)
			}

			// Forge (Fase 37+40): projeto, membros, bare repo e protected branches.
			forgeWrite := adminOnly.Group("")
			forgeWrite.Use(auth.RequireProduct(store.ProductForge))
			{
				forgeWrite.POST("/projects", app.handleCreateProject)
				forgeWrite.PATCH("/projects/:org/:slug", app.handleUpdateProject)
				forgeWrite.PATCH("/xgit/settings", app.handleUpdateForgeSettings)
				forgeWrite.PUT("/projects/:org/:slug/members", app.handleSetProjectMembers)
				forgeWrite.POST("/projects/:org/:slug/git", app.handleInitProjectGit)
				forgeWrite.PUT("/projects/:org/:slug/protected-branches", app.handleSetProtectedBranches)
			}

			// Compute (Fase 38): malha BitLaunch. Token só no VPS.
			computeWrite := adminOnly.Group("")
			computeWrite.Use(auth.RequireProduct(store.ProductCompute))
			{
				computeWrite.POST("/servers/import", app.handleImportMeshServers)
				computeWrite.POST("/servers", app.handleCreateMeshServer)
				computeWrite.PATCH("/servers/:id", app.handleUpdateMeshServer)
				computeWrite.DELETE("/servers/:id", app.handleDestroyMeshServer)
				computeWrite.POST("/servers/:id/rebuild", app.handleRebuildMeshServer)
				computeWrite.POST("/servers/:id/runner-token", app.handleIssueRunnerToken)
				computeWrite.PUT("/servers/:id/access", app.handleSetServerAccess)
				computeWrite.POST("/server-groups", app.handleCreateServerGroup)
				computeWrite.PUT("/server-groups/:id/access", app.handleSetGroupAccess)
				computeWrite.POST("/compute/settings/accounts", app.handleCreateBitLaunchAccount)
				computeWrite.PATCH("/compute/settings/accounts/:id", app.handleUpdateBitLaunchAccount)
				computeWrite.DELETE("/compute/settings/accounts/:id", app.handleDeleteBitLaunchAccount)
				computeWrite.POST("/compute/settings/accounts/:id/topup", app.handleCreateBitLaunchTopUp)
				computeWrite.POST("/servers/:id/agent-token", app.handleIssueAgentToken)
			}

			// Serviços gerenciados (Fase 43 — PLAN.md §6.18).
			managedWrite := adminOnly.Group("")
			managedWrite.Use(auth.RequireProduct(store.ProductManaged))
			{
				managedWrite.POST("/services", app.handleCreateService)
				managedWrite.POST("/services/:slug/apply", app.handleApplyService)
				managedWrite.POST("/services/:slug/stop", app.handleStopService)
				managedWrite.POST("/services/:slug/rotate", app.handleRotateServiceSecret)
				managedWrite.DELETE("/services/:slug", app.handleDeleteService)
			}
		}

		// Sync do catálogo a partir de apps/*/marketplace.yaml (Fase 16).
		// Só registra a rota se XVPN_PUBLISH_TOKEN estiver definido —
		// servidor sem publicação não expõe a superfície.
		if app.Config != nil && app.Config.PublishToken != "" {
			publish := apiGroup.Group("")
			publish.Use(app.requireMarketplacePublishAuth())
			publish.POST("/marketplace/sync", app.handleMarketplaceSync)
		}

		if app.Config != nil && app.Config.XbotToken != "" {
			apiGroup.POST("/hooks/chat/broadcast", app.handleHooksChatBroadcast)
		}
	}

	// Smart HTTP (Fase 40): só xgit.corp. Fora da VPN o Nginx recusa.
	git := r.Group("")
	git.Use(app.RequireGitHost())
	{
		git.GET("/:org/:slug/info/refs", app.handleGitSmartHTTP)
		git.POST("/:org/:slug/git-upload-pack", app.handleGitSmartHTTP)
		git.POST("/:org/:slug/git-receive-pack", app.handleGitSmartHTTP)
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
