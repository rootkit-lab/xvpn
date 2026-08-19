package api

import (
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

// contextTunnelDeviceKey guarda no gin.Context o Device já resolvido pelo
// middleware de origem, para o handler não repetir a consulta.
const contextTunnelDeviceKey = "xvpn_tunnel_device"

// RequireTunnelOrigin é o middleware das rotas que autenticam o
// dispositivo pelo IP de origem dentro do túnel, sem JWT (Fase 14 —
// PLAN.md §6.9). Ele faz duas coisas, nesta ordem: exige que a origem
// esteja na sub-rede da VPN e resolve o Device correspondente àquele IP.
//
// O primitivo é c.RemoteIP(), NUNCA c.ClientIP(). ClientIP() consulta
// X-Forwarded-For/X-Real-IP quando o peer TCP é um proxy confiável, e o
// Nginx é exatamente isso (ver trustedProxies em server.go) — uma
// requisição da internet pública com "X-Forwarded-For: 10.66.66.2"
// passaria por daqui. RemoteIP() lê só Request.RemoteAddr, o peer TCP
// real, que nenhum header altera.
//
// É essa exigência que dispensa uma segunda árvore de rotas para o
// listener do túnel: como o Nginx conecta de 127.0.0.1, que não está em
// 10.66.66.0/24, tudo que chega pelo painel público já é rejeitado aqui
// por construção. Um segundo gin.Engine só acrescentaria a possibilidade
// de registrar uma rota na árvore errada — falha silenciosa.
func (a *App) RequireTunnelOrigin() gin.HandlerFunc {
	var subnet *net.IPNet
	if _, parsed, err := net.ParseCIDR(a.Config.WireGuardAllowedSubnet); err == nil {
		subnet = parsed
	} else {
		// Config quebrada: falha fechada em vez de aberta. Não abortamos o
		// boot porque o resto da API (painel, enrollment) continua
		// utilizável, mas estas rotas ficam indisponíveis e o motivo fica
		// no journal.
		slog.Error("sub-rede da VPN inválida — rotas de identidade por túnel desabilitadas",
			"subnet", a.Config.WireGuardAllowedSubnet, "err", err)
	}

	return func(c *gin.Context) {
		if subnet == nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError,
				gin.H{"error": "sub-rede da VPN mal configurada no servidor"})
			return
		}
		ip := net.ParseIP(c.RemoteIP())
		if ip == nil || !subnet.Contains(ip) {
			// Mensagem deliberadamente genérica: quem chegou aqui de fora
			// do túnel não precisa saber que existe um caminho interno.
			c.AbortWithStatusJSON(http.StatusForbidden,
				gin.H{"error": "esta rota só responde a dispositivos conectados à VPN"})
			return
		}

		device, err := a.deviceByTunnelIP(ip)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				c.AbortWithStatusJSON(http.StatusNotFound,
					gin.H{"error": "nenhum dispositivo registrado para este IP da VPN"})
				return
			}
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
			return
		}
		c.Set(contextTunnelDeviceKey, device)
		c.Next()
	}
}

// deviceByTunnelIP acha o Device cujo AllowedIP corresponde a ip. O campo
// é gravado por wireguard.AllocateIP sempre no formato "<ip>/32", mas a
// consulta aceita as duas formas para não depender disso.
func (a *App) deviceByTunnelIP(ip net.IP) (store.Device, error) {
	var device store.Device
	err := a.Store.DB.
		Where("allowed_ip = ? OR allowed_ip = ?", ip.String(), ip.String()+"/32").
		First(&device).Error
	return device, err
}

// tunnelDevice recupera o Device que o middleware resolveu.
func tunnelDevice(c *gin.Context) (store.Device, bool) {
	value, ok := c.Get(contextTunnelDeviceKey)
	if !ok {
		return store.Device{}, false
	}
	device, ok := value.(store.Device)
	return device, ok
}

// tunnelIdentityResponse é o que o cliente desktop precisa saber sobre a
// pessoa por trás do dispositivo: o username, para montar o nome do share
// pessoal (`home-<username>`), e os dois toggles, para desabilitar o botão
// com a razão visível em vez de deixar o usuário cair num erro de mount.
type tunnelIdentityResponse struct {
	Username      string        `json:"username"`
	SFTPEnabled   bool          `json:"sftp_enabled"`
	SambaEnabled  bool          `json:"samba_enabled"`
	IntranetHosts []dnsHostJSON `json:"intranet_hosts"`
}

// handleTunnelIdentity responde quem é o dono do dispositivo que fez a
// requisição, resolvido pelo IP de origem dentro do túnel. Sem JWT: o IP
// 10.66.66.x é ligado ao peer pelo allowed-ips do próprio WireGuard, então
// não é falsificável de dentro da VPN — mesma premissa já aceita para o
// Samba guest (ver SECURITY.md).
//
// O nome handleMe não estava disponível: já é o handler de
// GET /api/auth/me, que é outra coisa (identidade do usuário logado no
// painel, via JWT).
//
// GET /api/me  (RequireTunnelOrigin — ver server.go)
func (a *App) handleTunnelIdentity(c *gin.Context) {
	device, ok := tunnelDevice(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	var owner store.User
	if err := a.Store.DB.First(&owner, device.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "dono do dispositivo não encontrado"})
		return
	}
	c.JSON(http.StatusOK, tunnelIdentityResponse{
		Username:      owner.Username,
		SFTPEnabled:   owner.SFTPEnabled,
		SambaEnabled:  owner.SambaEnabled,
		IntranetHosts: a.enabledIntranetHosts(),
	})
}

// registerSSHKeyRequest é o corpo de POST /api/me/ssh-key: exatamente uma
// chave pública, a deste dispositivo.
type registerSSHKeyRequest struct {
	PublicKey string `json:"public_key"`
}

// registerSSHKeyResponse confirma o registro sem devolver a chave. O
// Fingerprint permite o cliente exibir/logar qual chave está valendo;
// SFTPEnabled informa se o acesso já vale de fato (o admin pode não ter
// ligado o toggle ainda, e isso não é erro); Changed distingue um
// registro novo de um no-op.
type registerSSHKeyResponse struct {
	Fingerprint string `json:"fingerprint"`
	SFTPEnabled bool   `json:"sftp_enabled"`
	Changed     bool   `json:"changed"`
}

// handleRegisterDeviceSSHKey grava a chave pública SSH que o dispositivo
// gerou sozinho e re-renderiza o authorized_keys do dono (Fase 14.2 —
// PLAN.md §6.9). Idempotente: a mesma chave de novo não chama o
// provisionador nem gera entrada de auditoria.
//
// Ordem deliberadamente invertida em relação a handleSetFileAccess (que
// chama o provisionador antes de gravar no banco): aqui o banco vem
// primeiro. O estado perigoso é "chave viva no sistema que o banco não
// conhece" — assim o banco é sempre um superconjunto do que está
// autorizado no sistema, e o reconcile no boot converge o resto.
//
// POST /api/me/ssh-key  (RequireTunnelOrigin — ver server.go)
func (a *App) handleRegisterDeviceSSHKey(c *gin.Context) {
	device, ok := tunnelDevice(c)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	var req registerSSHKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	key, err := normalizeSingleSSHPublicKey(req.PublicKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var owner store.User
	if err := a.Store.DB.First(&owner, device.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "dono do dispositivo não encontrado"})
		return
	}

	fingerprint := sshKeyFingerprint(key)
	if sameSSHKey(device.SSHPublicKey, key) {
		c.JSON(http.StatusOK, registerSSHKeyResponse{
			Fingerprint: fingerprint,
			SFTPEnabled: owner.SFTPEnabled,
			Changed:     false,
		})
		return
	}

	now := time.Now()
	if err := a.Store.DB.Model(&store.Device{}).Where("id = ?", device.ID).Updates(map[string]any{
		"ssh_public_key":     key,
		"ssh_key_updated_at": now,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	device.SSHPublicKey = key
	device.SSHKeyUpdatedAt = &now

	// applyAuthorizedKeys é no-op se o dono está com SFTP desligado — a
	// chave fica guardada e passa a valer quando o admin ligar o toggle,
	// sem precisar de uma segunda rodada de conversa com o usuário.
	if err := a.applyAuthorizedKeys(c.Request.Context(), owner); err != nil {
		// O banco já tem a chave; o sistema converge no próximo
		// registro ou no reconcile do boot. Reportamos o erro para o
		// cliente poder tentar de novo, mas sem desfazer a gravação.
		c.JSON(http.StatusInternalServerError,
			gin.H{"error": "chave registrada, mas falha ao aplicar no servidor: " + provisionerErrMsg(err)})
		return
	}

	// Auditoria com device_id e fingerprint — nunca a chave inteira (ver
	// go-backend.mdc).
	_ = a.Store.LogAudit("device:"+strconv.FormatUint(uint64(device.ID), 10), "sshkey.autoregister",
		"user_id="+strconv.FormatUint(uint64(owner.ID), 10)+" fingerprint="+fingerprint)

	c.JSON(http.StatusOK, registerSSHKeyResponse{
		Fingerprint: fingerprint,
		SFTPEnabled: owner.SFTPEnabled,
		Changed:     true,
	})
}
