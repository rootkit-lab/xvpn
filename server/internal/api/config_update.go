package api

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

const (
	minInviteTTLMinutes = 1
	maxInviteTTLMinutes = 7 * 24 * 60 // 7 dias
	minJWTTTLMinutes    = 5
	maxJWTTTLMinutes    = 7 * 24 * 60
)

type updateConfigRequest struct {
	InviteTokenTTLMinutes *int `json:"invite_token_ttl_minutes"`
	JWTTokenTTLMinutes    *int `json:"jwt_token_ttl_minutes"`
}

// handleUpdateConfig edita TTLs de convite/sessão (Fase 15). Parâmetros
// WireGuard ficam de fora de propósito — mudá-los ao vivo (sub-rede,
// porta, endpoint) exigiria redesenhar peers/firewall e reinício
// coordenado; continuam só via env + restart (ver settings-page).
//
// PATCH /api/config  (adminOnly)
func (a *App) handleUpdateConfig(c *gin.Context) {
	var req updateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	if req.InviteTokenTTLMinutes == nil && req.JWTTokenTTLMinutes == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "informe ao menos um campo editável"})
		return
	}

	settings, err := a.loadOrInitPanelSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	if req.InviteTokenTTLMinutes != nil {
		v := *req.InviteTokenTTLMinutes
		if v < minInviteTTLMinutes || v > maxInviteTTLMinutes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invite_token_ttl_minutes fora do intervalo permitido"})
			return
		}
		settings.InviteTokenTTLMinutes = v
		a.Config.InviteTokenTTLMinutes = v
	}
	if req.JWTTokenTTLMinutes != nil {
		v := *req.JWTTokenTTLMinutes
		if v < minJWTTTLMinutes || v > maxJWTTTLMinutes {
			c.JSON(http.StatusBadRequest, gin.H{"error": "jwt_token_ttl_minutes fora do intervalo permitido"})
			return
		}
		settings.JWTTokenTTLMinutes = v
		a.Config.JWTTokenTTLMinutes = v
		a.Tokens.SetTTL(time.Duration(v) * time.Minute)
	}

	if err := a.Store.DB.Save(&settings).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno ao persistir"})
		return
	}

	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "config.update", "")
	c.JSON(http.StatusOK, a.configResponse())
}

func (a *App) configResponse() configResponse {
	return configResponse{
		WireGuardInterface:     a.Config.WireGuardInterface,
		WireGuardAddress:       a.Config.WireGuardAddress,
		WireGuardAllowedSubnet: a.Config.WireGuardAllowedSubnet,
		WireGuardListenPort:    a.Config.WireGuardListenPort,
		WireGuardEndpoint:      a.Config.WireGuardEndpoint,
		ServerPublicKey:        a.ServerPublicKey,
		InviteTokenTTLMinutes:  a.Config.InviteTokenTTLMinutes,
		JWTTokenTTLMinutes:     a.Config.JWTTokenTTLMinutes,
	}
}

func (a *App) loadOrInitPanelSettings() (store.PanelSettings, error) {
	var s store.PanelSettings
	err := a.Store.DB.First(&s, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		s = store.PanelSettings{ID: 1}
		if err := a.Store.DB.Create(&s).Error; err != nil {
			return s, fmt.Errorf("criando panel_settings: %w", err)
		}
		return s, nil
	}
	if err != nil {
		return s, fmt.Errorf("lendo panel_settings: %w", err)
	}
	return s, nil
}

// ApplyPanelSettingsOverrides carrega TTLs persistidos no DB por cima do
// env (boot). Chamado uma vez em cmd/xvpn-server após montar o App.
func (a *App) ApplyPanelSettingsOverrides() error {
	var s store.PanelSettings
	err := a.Store.DB.First(&s, 1).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if s.InviteTokenTTLMinutes >= minInviteTTLMinutes {
		a.Config.InviteTokenTTLMinutes = s.InviteTokenTTLMinutes
	}
	if s.JWTTokenTTLMinutes >= minJWTTTLMinutes {
		a.Config.JWTTokenTTLMinutes = s.JWTTokenTTLMinutes
		a.Tokens.SetTTL(time.Duration(s.JWTTokenTTLMinutes) * time.Minute)
	}
	return nil
}
