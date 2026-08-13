package api

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"gorm.io/gorm"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/store"
	"github.com/rootkit-lab/xvpn/server/internal/wireguard"
)

type enrollRequest struct {
	InviteToken string `json:"invite_token" binding:"required"`
	PublicKey   string `json:"public_key" binding:"required"`
	DeviceName  string `json:"device_name" binding:"required"`
}

type enrollResponse struct {
	AssignedIP          string `json:"assigned_ip"`
	ServerPublicKey     string `json:"server_public_key"`
	Endpoint            string `json:"endpoint"`
	AllowedIPs          string `json:"allowed_ips"`
	PersistentKeepalive int    `json:"persistent_keepalive"`
	APIVersion          int    `json:"api_version"`
}

// handleDeviceEnroll é o único endpoint de escrita que não exige JWT — o
// cliente ainda não tem sessão, só um código de convite de curto prazo. A
// chave pública recebida aqui nunca inclui a privada correspondente, que
// nunca sai do dispositivo (ver AGENTS.md, invariante de segurança).
// POST /api/devices/enroll
func (a *App) handleDeviceEnroll(c *gin.Context) {
	var req enrollRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invite_token, public_key e device_name são obrigatórios"})
		return
	}

	if _, err := wgtypes.ParseKey(req.PublicKey); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "public_key inválida"})
		return
	}

	a.enrollMu.Lock()
	defer a.enrollMu.Unlock()

	var invite store.InviteToken
	err := a.Store.DB.Where("token = ?", req.InviteToken).First(&invite).Error
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "código de convite inválido"})
		return
	}
	if invite.UsedAt != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "código de convite já utilizado"})
		return
	}
	if time.Now().After(invite.ExpiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "código de convite expirado"})
		return
	}

	var existing store.Device
	err = a.Store.DB.Where("public_key = ?", req.PublicKey).First(&existing).Error
	if err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "esta chave pública já está registrada em outro dispositivo"})
		return
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	var devices []store.Device
	if err := a.Store.DB.Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	usedIPs := make([]string, 0, len(devices))
	for _, d := range devices {
		usedIPs = append(usedIPs, d.AllowedIP)
	}

	assignedIP, err := wireguard.AllocateIP(a.Config.WireGuardAllowedSubnet, usedIPs)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "nenhum IP disponível na sub-rede da VPN"})
		return
	}

	device := store.Device{
		UserID:    invite.UserID,
		Name:      req.DeviceName,
		PublicKey: req.PublicKey,
		AllowedIP: assignedIP,
	}

	err = a.Store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&device).Error; err != nil {
			return err
		}
		now := time.Now()
		invite.UsedAt = &now
		return tx.Save(&invite).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	if err := a.WG.AddPeer(wireguard.PeerSpec{PublicKey: device.PublicKey, AllowedIP: device.AllowedIP}); err != nil {
		// Reverte tanto o dispositivo quanto o consumo do convite — sem
		// isso, o convite ficava "queimado" (used_at preenchido) mesmo
		// com o enrollment tendo falhado, impedindo o cliente de tentar
		// de novo com o mesmo código (ver ROADMAP.md Fase 9).
		if rbErr := a.Store.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.Delete(&device).Error; err != nil {
				return err
			}
			return tx.Model(&store.InviteToken{}).Where("id = ?", invite.ID).Update("used_at", nil).Error
		}); rbErr != nil {
			slog.Error("falha ao reverter enrollment após erro de WireGuard", "device_id", device.ID, "err", rbErr)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao registrar peer na interface WireGuard"})
		return
	}

	_ = a.Store.LogAudit("enrollment", "device.enroll", "device_id="+strconv.FormatUint(uint64(device.ID), 10))

	c.JSON(http.StatusCreated, enrollResponse{
		AssignedIP:          device.AllowedIP,
		ServerPublicKey:     a.ServerPublicKey,
		Endpoint:            a.Config.WireGuardEndpoint,
		AllowedIPs:          "0.0.0.0/0, ::/0",
		PersistentKeepalive: 25,
		APIVersion:          APIVersion,
	})
}

type deviceResponse struct {
	ID            uint       `json:"id"`
	UserID        uint       `json:"user_id"`
	Name          string     `json:"name"`
	PublicKey     string     `json:"public_key"`
	AllowedIP     string     `json:"allowed_ip"`
	CreatedAt     time.Time  `json:"created_at"`
	LastHandshake *time.Time `json:"last_handshake,omitempty"`
	ReceiveBytes  int64      `json:"receive_bytes"`
	TransmitBytes int64      `json:"transmit_bytes"`
	Endpoint      string     `json:"endpoint,omitempty"`
}

// handleListDevices lista os dispositivos registrados, combinando os dados
// persistidos (nome, dono) com o estado ao vivo lido do kernel (handshake,
// tráfego, endpoint atual).
// GET /api/devices
func (a *App) handleListDevices(c *gin.Context) {
	var devices []store.Device
	if err := a.Store.DB.Order("id").Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	live, err := a.WG.ListPeers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro consultando estado da interface WireGuard"})
		return
	}
	liveByKey := make(map[string]wireguard.PeerStatus, len(live))
	for _, p := range live {
		liveByKey[p.PublicKey] = p
	}

	resp := make([]deviceResponse, 0, len(devices))
	for _, d := range devices {
		dr := deviceResponse{
			ID:        d.ID,
			UserID:    d.UserID,
			Name:      d.Name,
			PublicKey: d.PublicKey,
			AllowedIP: d.AllowedIP,
			CreatedAt: d.CreatedAt,
		}
		if p, ok := liveByKey[d.PublicKey]; ok {
			dr.LastHandshake = p.LastHandshake
			dr.ReceiveBytes = p.ReceiveBytes
			dr.TransmitBytes = p.TransmitBytes
			dr.Endpoint = p.Endpoint
		}
		resp = append(resp, dr)
	}
	c.JSON(http.StatusOK, resp)
}

// handleDeleteDevice revoga um dispositivo imediatamente: remove o peer da
// interface WireGuard (o handshake para de funcionar já na próxima
// tentativa) e apaga o registro do banco.
// DELETE /api/devices/:id
func (a *App) handleDeleteDevice(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}

	var device store.Device
	if err := a.Store.DB.First(&device, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "dispositivo não encontrado"})
		return
	}

	if err := a.WG.RemovePeer(device.PublicKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "falha ao revogar peer na interface WireGuard"})
		return
	}

	if err := a.Store.DB.Delete(&device).Error; err != nil {
		// Compensa: o peer já saiu do kernel, mas o registro continua no
		// banco — sem re-adicionar, um restart do servidor chamaria
		// ReconcilePeers só com o que sobrou no banco (que ainda inclui
		// este device) e o peer "ressuscitaria" sozinho, sem que o
		// admin tenha pedido isso (ver ROADMAP.md Fase 9).
		if addErr := a.WG.AddPeer(wireguard.PeerSpec{PublicKey: device.PublicKey, AllowedIP: device.AllowedIP}); addErr != nil {
			slog.Error("falha ao compensar remoção de peer após erro de banco", "device_id", device.ID, "err", addErr)
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "device.revoke", "device_id="+c.Param("id"))

	c.Status(http.StatusNoContent)
}
