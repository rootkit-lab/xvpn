package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

// handleSetFileAccess aplica o estado de acesso a arquivos (SFTP +
// Samba + chave pública SSH) desejado pelo admin para um usuário.
// Calcula o diff contra o estado atual e chama só o provisionador pro
// que mudou — idempotente.
//
// Consistência DB↔sistema (mesma preocupação da Fase 9 com WireGuard):
// cada chamada do provisionador que tem sucesso atualiza o campo
// correspondente no DB imediatamente, então o DB sempre reflete o que
// o sistema de fato tem. Se uma chamada falha no meio, o DB já gravou
// o que sucedeu e o handler devolve 500; o reconcile no boot (ver
// cmd/xvpn-server/main.go) converge o resto. Não há "estado fantasma"
// onde o DB diz "ligado" mas o sistema não tem.
//
// PUT /api/users/:id/file-access  (adminOnly — ver server.go)
func (a *App) handleSetFileAccess(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var req fileAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	req.SSHPublicKey = strings.TrimSpace(req.SSHPublicKey)
	if !validSSHPublicKey(req.SSHPublicKey) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chave pública SSH malformada"})
		return
	}
	if req.SFTPEnabled && req.SSHPublicKey == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "chave pública SSH é obrigatória para habilitar SFTP"})
		return
	}
	if a.UserProvisioner == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "provisionamento de contas Unix não configurado neste servidor"})
		return
	}

	var target store.User
	if err := a.Store.DB.First(&target, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	if !callerRole(c).CanManage(target.Role) {
		c.JSON(http.StatusForbidden, gin.H{"error": "seu papel não pode gerenciar acesso a arquivos deste usuário"})
		return
	}

	ctx := c.Request.Context()
	username := target.Username

	// Caminho 1: ambos desligados no destino → remove tudo de uma vez.
	// Só chama se algo estava ligado antes (evita Disable num usuário
	// que nunca teve acesso — seria no-op mas recarregaria serviços à toa).
	if !req.SFTPEnabled && !req.SambaEnabled {
		if target.SFTPEnabled || target.SambaEnabled {
			if err := a.UserProvisioner.Disable(ctx, username); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": provisionerErrMsg(err)})
				return
			}
			target.SFTPEnabled = false
			target.SambaEnabled = false
			target.SSHPublicKey = ""
		}
		a.persistFileAccess(c, &target)
		return
	}

	// Caminho 2: pelo menos um ligado — aplica diffs individuais.
	// Ordem: desligar antes de ligar (evita recarregar sshd duas vezes
	// seguidas; e desligar Samba não afeta SFTP, vice-versa).
	if !req.SFTPEnabled && target.SFTPEnabled {
		if err := a.UserProvisioner.DisableSFTP(ctx, username); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": provisionerErrMsg(err)})
			return
		}
		target.SFTPEnabled = false
		target.SSHPublicKey = ""
	}
	if !req.SambaEnabled && target.SambaEnabled {
		if err := a.UserProvisioner.DisableSamba(ctx, username); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": provisionerErrMsg(err)})
			return
		}
		target.SambaEnabled = false
	}
	if req.SFTPEnabled && !target.SFTPEnabled {
		if err := a.UserProvisioner.EnableSFTP(ctx, username, req.SSHPublicKey); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": provisionerErrMsg(err)})
			return
		}
		target.SFTPEnabled = true
		target.SSHPublicKey = req.SSHPublicKey
	}
	if req.SambaEnabled && !target.SambaEnabled {
		if err := a.UserProvisioner.EnableSamba(ctx, username); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": provisionerErrMsg(err)})
			return
		}
		target.SambaEnabled = true
	}
	// SFTP já on, mas chave mudou: re-aplica EnableSFTP pra reescrever
	// o authorized_keys (garante que o arquivo bate com o que o admin colou).
	if req.SFTPEnabled && target.SFTPEnabled && req.SSHPublicKey != target.SSHPublicKey && req.SSHPublicKey != "" {
		if err := a.UserProvisioner.EnableSFTP(ctx, username, req.SSHPublicKey); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": provisionerErrMsg(err)})
			return
		}
		target.SSHPublicKey = req.SSHPublicKey
	}

	a.persistFileAccess(c, &target)
}

// persistFileAccess grava o estado final no DB e devolve a resposta.
// O audit log registra a transição (ligou/desligou SFTP/Samba) — não
// a chave em si (chave pública é pública, mas não precisa ficar no log).
func (a *App) persistFileAccess(c *gin.Context, target *store.User) {
	if err := a.Store.DB.Model(&store.User{}).Where("id = ?", target.ID).Updates(map[string]any{
		"sftp_enabled":   target.SFTPEnabled,
		"samba_enabled":  target.SambaEnabled,
		"ssh_public_key": target.SSHPublicKey,
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	actor, _ := c.Get(auth.ContextUsernameKey)
	actions := []string{"sftp=off", "samba=off"}
	if target.SFTPEnabled {
		actions[0] = "sftp=on"
	}
	if target.SambaEnabled {
		actions[1] = "samba=on"
	}
	_ = a.Store.LogAudit(actorString(actor), "user.file_access", "user_id="+strconv.FormatUint(uint64(target.ID), 10)+" "+strings.Join(actions, " "))
	c.JSON(http.StatusOK, fileAccessResponse{
		SFTPEnabled:  target.SFTPEnabled,
		SambaEnabled: target.SambaEnabled,
		SSHPublicKey: target.SSHPublicKey,
	})
}
