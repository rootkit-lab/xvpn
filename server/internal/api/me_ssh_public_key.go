package api

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type updateMySSHPublicKeyRequest struct {
	SSHPublicKey string `json:"ssh_public_key"`
}

// handleUpdateMySSHPublicKey é o autosserviço da chave SSH *manual* no
// portal (Fase 15): o próprio usuário atualiza User.SSHPublicKey sem
// passar pelo admin. As chaves dos dispositivos continuam automáticas
// via POST /api/me/ssh-key (túnel). Se SFTP estiver ligado, reaplica
// authorized_keys; se não, só grava no DB (mesmo contrato de
// applyAuthorizedKeys).
//
// PUT /api/me/ssh-public-key  (authed)
func (a *App) handleUpdateMySSHPublicKey(c *gin.Context) {
	var req updateMySSHPublicKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	req.SSHPublicKey = strings.TrimSpace(req.SSHPublicKey)
	if !validSSHPublicKey(req.SSHPublicKey) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chave pública SSH inválida"})
		return
	}

	uid := callerUserID(c)
	var user store.User
	if err := a.Store.DB.First(&user, uid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}

	if sameSSHKey(user.SSHPublicKey, req.SSHPublicKey) {
		c.JSON(http.StatusOK, toUserResponse(user))
		return
	}

	user.SSHPublicKey = req.SSHPublicKey
	if err := a.Store.DB.Model(&user).Update("ssh_public_key", user.SSHPublicKey).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	if err := a.applyAuthorizedKeys(c.Request.Context(), user); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "chave salva, mas falhou ao aplicar no SFTP: " + err.Error()})
		return
	}

	_ = a.Store.LogAudit(user.Username, "me.ssh_public_key", sshKeyFingerprint(user.SSHPublicKey))
	c.JSON(http.StatusOK, toUserResponse(user))
}
