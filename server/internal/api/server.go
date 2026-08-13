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
	"github.com/rootkit-lab/xvpn/server/internal/store"
	"github.com/rootkit-lab/xvpn/server/internal/wireguard"
)

// APIVersion é o contrato de compatibilidade cliente↔servidor descrito em
// PLAN.md §13.3: um inteiro incremental, bump sempre que um endpoint mudar
// de forma incompatível com clientes existentes.
const APIVersion = 1

// StartedAt é preenchido no boot do processo, usado por GET /api/status
// para relatar uptime.
var StartedAt = time.Now()

// App agrega as dependências compartilhadas pelos handlers.
type App struct {
	Store  *store.Store
	WG     wireguard.PeerManager
	Tokens *auth.TokenManager
	Config *config.Config

	// ServerPublicKey é derivada da chave privada lida na inicialização
	// (nunca a chave privada em si) e devolvida aos clientes no enrollment.
	ServerPublicKey string

	// enrollMu serializa operações de enrollment (alocação de IP +
	// registro de peer), evitando duas requisições simultâneas alocarem o
	// mesmo IP livre.
	enrollMu sync.Mutex
}

// NewRouter monta todas as rotas da API sobre um App já inicializado.
func NewRouter(app *App) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(requestLogger())

	apiGroup := r.Group("/api")
	{
		apiGroup.POST("/auth/login", app.handleLogin)
		apiGroup.POST("/devices/enroll", app.handleDeviceEnroll)
		apiGroup.GET("/status", app.handleStatus)

		authed := apiGroup.Group("")
		authed.Use(auth.RequireAuth(app.Tokens))
		{
			authed.GET("/users", app.handleListUsers)
			authed.POST("/users", app.handleCreateUser)
			authed.DELETE("/users/:id", app.handleDeleteUser)
			authed.POST("/users/:id/invite", app.handleCreateInvite)

			authed.GET("/devices", app.handleListDevices)
			authed.DELETE("/devices/:id", app.handleDeleteDevice)

			authed.GET("/audit", app.handleListAudit)
			authed.GET("/config", app.handleGetConfig)
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
