package api

import (
	"net/http"
	"strconv"
	"strings"
	"time"

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
	// Até a Fase 13 o handler bloqueava aqui quando SFTP era ligado sem
	// chave colada. Isso deixou de ser um erro na Fase 14: as chaves dos
	// dispositivos entram no authorized_keys sozinhas (ver
	// renderAuthorizedKeys), então ligar o toggle antes de a pessoa abrir
	// o XVPN é um fluxo válido — o acesso passa a valer no instante em que
	// a chave dela chegar, sem uma segunda rodada de conversa.
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
		c.JSON(http.StatusForbidden, gin.H{
			"error": "seu papel atual não pode gerenciar acesso a arquivos deste usuário — se você acabou de ser promovido, atualize a página ou faça login de novo",
		})
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
	// enableSFTP escreve o authorized_keys como a união das chaves dos
	// dispositivos do usuário com a chave manual que o admin enviou nesta
	// requisição — nunca só a manual, que apagaria o acesso de todos os
	// dispositivos auto-registrados.
	enableSFTP := func() error {
		content, err := a.renderAuthorizedKeys(target.ID, req.SSHPublicKey)
		if err != nil {
			return err
		}
		return a.UserProvisioner.EnableSFTP(ctx, username, content)
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
			// Zera só a chave MANUAL. As chaves dos dispositivos
			// (Device.SSHPublicKey) sobrevivem de propósito: desligar o
			// toggle é revogar o acesso, não desfazer o registro de quem
			// já se apresentou. Assim, religar o toggle volta a valer na
			// hora, sem pedir nada ao usuário — que é justamente o ponto
			// da Fase 14. Quem quiser revogar a chave de um dispositivo
			// específico revoga o dispositivo (ver revokeDevice).
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
		// Mesma decisão do caminho 1: só a chave manual sai (ver
		// comentário lá).
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
		if err := enableSFTP(); err != nil {
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
	// SFTP já on, mas a chave manual mudou: reescreve o authorized_keys
	// para o arquivo bater com o que o admin colou. Diferente da Fase 13,
	// esvaziar o campo também é uma mudança a aplicar — significa "remova
	// a chave manual, mantenha as dos dispositivos", e sem reescrever o
	// arquivo a chave removida continuaria autorizada.
	if req.SFTPEnabled && target.SFTPEnabled && req.SSHPublicKey != target.SSHPublicKey {
		if err := enableSFTP(); err != nil {
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

// deviceSSHKeyResponse descreve uma chave auto-registrada por um
// dispositivo, para o painel listar em modo leitura. Devolve o
// fingerprint, não a chave: é o que identifica a chave numa tela sem
// transformá-la num campo de texto gigante que o admin pode achar que
// deve editar (o textarea do diálogo continua sendo só para as chaves
// manuais).
type deviceSSHKeyResponse struct {
	DeviceID    uint       `json:"device_id"`
	DeviceName  string     `json:"device_name"`
	Fingerprint string     `json:"fingerprint"`
	UpdatedAt   *time.Time `json:"updated_at,omitempty"`
}

// handleListUserSSHKeys lista as chaves que os dispositivos daquele
// usuário registraram sozinhos (Fase 14 — ver handleRegisterDeviceSSHKey).
// Só leitura: não existe caminho de API para o admin editar a chave de um
// dispositivo, porque quem a informa é a máquina dona dela. Para revogar,
// revoga-se o dispositivo.
//
// GET /api/users/:id/ssh-keys  (adminOnly — ver server.go)
func (a *App) handleListUserSSHKeys(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var target store.User
	if err := a.Store.DB.First(&target, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}
	if !callerRole(c).CanManage(target.Role) {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "seu papel atual não pode ver o acesso a arquivos deste usuário — se você acabou de ser promovido, atualize a página ou faça login de novo",
		})
		return
	}

	var devices []store.Device
	if err := a.Store.DB.
		Where("user_id = ? AND ssh_public_key <> ''", target.ID).
		Order("id").Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	keys := make([]deviceSSHKeyResponse, 0, len(devices))
	for _, d := range devices {
		keys = append(keys, deviceSSHKeyResponse{
			DeviceID:    d.ID,
			DeviceName:  d.Name,
			Fingerprint: sshKeyFingerprint(d.SSHPublicKey),
			UpdatedAt:   d.SSHKeyUpdatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"device_keys": keys})
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
