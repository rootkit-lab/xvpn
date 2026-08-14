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
// Consistência DB↔sistema (Fase 13, PLAN.md §6.9): cada chamada do
// provisionador que tem sucesso é seguida imediatamente de uma
// gravação no DB do campo correspondente (saveFileAccessFields). Assim,
// se uma chamada falhar no meio, o DB já reflete tudo o que sucedeu —
// nunca fica "OS ligado mas DB desligado". O reconcile no boot (ver
// reconcile.go) converge o resto a partir do DB.
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

	// save persiste o estado atual do target no DB. Chamado depois de
	// CADA chamada de provisionador que sucede — é o ponto que garante
	// a consistência DB↔sistema (ver comentário do handler).
	save := func() {
		if err := a.saveFileAccessFields(&target); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno ao persistir"})
			return
		}
	}
	provisionErr := func(err error) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": provisionerErrMsg(err)})
	}

	// Caminho 1: ambos desligados no destino → remove tudo de uma vez.
	// Só chama Disable se algo estava ligado antes (evita Disable num
	// usuário que nunca teve acesso — seria no-op mas recarregaria
	// serviços à toa).
	if !req.SFTPEnabled && !req.SambaEnabled {
		if target.SFTPEnabled || target.SambaEnabled {
			if err := a.UserProvisioner.Disable(ctx, username); err != nil {
				provisionErr(err)
				return
			}
			target.SFTPEnabled = false
			target.SambaEnabled = false
			target.SSHPublicKey = ""
			save()
			if c.Writer.Written() {
				return
			}
		}
		a.respondFileAccess(c, &target)
		return
	}

	// Caminho 2: pelo menos um ligado — aplica diffs individuais.
	// Ordem: desligar antes de ligar (evita recarregar sshd duas vezes
	// seguidas; e desligar Samba não afeta SFTP, vice-versa). Cada
	// passo persiste sozinho.
	if !req.SFTPEnabled && target.SFTPEnabled {
		if err := a.UserProvisioner.DisableSFTP(ctx, username); err != nil {
			provisionErr(err)
			return
		}
		target.SFTPEnabled = false
		target.SSHPublicKey = ""
		save()
		if c.Writer.Written() {
			return
		}
	}
	if !req.SambaEnabled && target.SambaEnabled {
		if err := a.UserProvisioner.DisableSamba(ctx, username); err != nil {
			provisionErr(err)
			return
		}
		target.SambaEnabled = false
		save()
		if c.Writer.Written() {
			return
		}
	}
	if req.SFTPEnabled && !target.SFTPEnabled {
		if err := a.UserProvisioner.EnableSFTP(ctx, username, req.SSHPublicKey); err != nil {
			provisionErr(err)
			return
		}
		target.SFTPEnabled = true
		target.SSHPublicKey = req.SSHPublicKey
		save()
		if c.Writer.Written() {
			return
		}
	}
	if req.SambaEnabled && !target.SambaEnabled {
		if err := a.UserProvisioner.EnableSamba(ctx, username); err != nil {
			provisionErr(err)
			return
		}
		target.SambaEnabled = true
		save()
		if c.Writer.Written() {
			return
		}
	}
	// SFTP já on, mas chave mudou: re-aplica EnableSFTP pra reescrever
	// o authorized_keys (garante que o arquivo bate com o que o admin colou).
	if req.SFTPEnabled && target.SFTPEnabled && req.SSHPublicKey != target.SSHPublicKey && req.SSHPublicKey != "" {
		if err := a.UserProvisioner.EnableSFTP(ctx, username, req.SSHPublicKey); err != nil {
			provisionErr(err)
			return
		}
		target.SSHPublicKey = req.SSHPublicKey
		save()
		if c.Writer.Written() {
			return
		}
	}

	a.respondFileAccess(c, &target)
}

// saveFileAccessFields grava os três campos de acesso a arquivos do
// usuário no DB. Usado pelo handler após cada chamada de provisionador
// bem-sucedida (consistência DB↔sistema) e pelo reconcile.
func (a *App) saveFileAccessFields(target *store.User) error {
	return a.Store.DB.Model(&store.User{}).Where("id = ?", target.ID).Updates(map[string]any{
		"sftp_enabled":   target.SFTPEnabled,
		"samba_enabled":  target.SambaEnabled,
		"ssh_public_key": target.SSHPublicKey,
	}).Error
}

// respondFileAccess grava o audit log e devolve o estado atual. O DB
// já foi persistido passo a passo; aqui só registramos a transição
// final (sftp=on/off samba=on/off) e respondemos.
func (a *App) respondFileAccess(c *gin.Context, target *store.User) {
	actor, _ := c.Get(auth.ContextUsernameKey)
	actions := []string{"sftp=off", "samba=off"}
	if target.SFTPEnabled {
		actions[0] = "sftp=on"
	}
	if target.SambaEnabled {
		actions[1] = "samba=on"
	}
	_ = a.Store.LogAudit(actorString(actor), "user.file_access",
		"user_id="+strconv.FormatUint(uint64(target.ID), 10)+" "+strings.Join(actions, " "))
	c.JSON(http.StatusOK, fileAccessResponse{
		SFTPEnabled:  target.SFTPEnabled,
		SambaEnabled: target.SambaEnabled,
		SSHPublicKey: target.SSHPublicKey,
	})
}
