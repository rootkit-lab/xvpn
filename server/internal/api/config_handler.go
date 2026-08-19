package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type configResponse struct {
	WireGuardInterface     string `json:"wireguard_interface"`
	WireGuardAddress       string `json:"wireguard_address"`
	WireGuardAllowedSubnet string `json:"wireguard_allowed_subnet"`
	WireGuardListenPort    int    `json:"wireguard_listen_port"`
	WireGuardEndpoint      string `json:"wireguard_endpoint"`
	ServerPublicKey        string `json:"server_public_key"`
	InviteTokenTTLMinutes  int    `json:"invite_token_ttl_minutes"`
	JWTTokenTTLMinutes     int    `json:"jwt_token_ttl_minutes"`
}

// handleGetConfig expõe apenas as configurações de rede não sensíveis do
// servidor (nunca segredos como JWTSecret ou a chave privada WireGuard) —
// usado pela tela "Configurações" do painel para exibição somente-leitura.
// Edição de firewall/DNS/rede via painel fica fora do escopo da Fase 3 (ver
// ROADMAP.md) e exigiria desenho próprio de validação/segurança antes de
// ser exposta por API.
// GET /api/config
func (a *App) handleGetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, a.configResponse())
}
