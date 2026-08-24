package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type forgeSSHKeyJSON struct {
	ID          uint    `json:"id"`
	Title       string  `json:"title"`
	Fingerprint string  `json:"fingerprint"`
	DeviceID    *uint   `json:"device_id,omitempty"`
	LastUsedAt  *string `json:"last_used_at,omitempty"`
	CreatedAt   string  `json:"created_at"`
}

func toForgeSSHKeyJSON(k store.ForgeSSHKey) forgeSSHKeyJSON {
	out := forgeSSHKeyJSON{
		ID:          k.ID,
		Title:       k.Title,
		Fingerprint: k.Fingerprint,
		DeviceID:    k.DeviceID,
		CreatedAt:   k.CreatedAt.UTC().Format(time.RFC3339),
	}
	if k.LastUsedAt != nil {
		s := k.LastUsedAt.UTC().Format(time.RFC3339)
		out.LastUsedAt = &s
	}
	return out
}

type createForgeSSHKeyRequest struct {
	Title     string `json:"title"`
	PublicKey string `json:"public_key"`
}

// handleListMyForgeSSHKeys lista as chaves SSH do forge do usuário logado.
// GET /api/me/forge-ssh-keys
func (a *App) handleListMyForgeSSHKeys(c *gin.Context) {
	uid := callerUserID(c)
	var keys []store.ForgeSSHKey
	if err := a.Store.DB.Where("user_id = ?", uid).Order("id DESC").Find(&keys).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	out := make([]forgeSSHKeyJSON, 0, len(keys))
	for _, k := range keys {
		out = append(out, toForgeSSHKeyJSON(k))
	}
	c.JSON(http.StatusOK, gin.H{"keys": out})
}

// handleCreateMyForgeSSHKey adiciona uma chave SSH manual ao forge.
// POST /api/me/forge-ssh-keys
func (a *App) handleCreateMyForgeSSHKey(c *gin.Context) {
	var req createForgeSSHKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	title := strings.TrimSpace(req.Title)
	if title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "título é obrigatório"})
		return
	}
	if len(title) > 255 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "título muito longo"})
		return
	}
	key, err := normalizeSingleSSHPublicKey(req.PublicKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	uid := callerUserID(c)
	var count int64
	if err := a.Store.DB.Model(&store.ForgeSSHKey{}).Where("user_id = ?", uid).Count(&count).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if count >= maxForgeSSHKeysPerUser {
		c.JSON(http.StatusBadRequest, gin.H{"error": "limite de chaves SSH atingido"})
		return
	}

	var user store.User
	if err := a.Store.DB.First(&user, uid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "usuário não encontrado"})
		return
	}

	created, changed, err := a.upsertForgeSSHKey(c.Request.Context(), user, key, title, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if changed {
		_ = a.Store.LogAudit(user.Username, "forge.ssh_key.create", "fingerprint="+created.Fingerprint)
	}
	c.JSON(http.StatusOK, toForgeSSHKeyJSON(created))
}

// handleDeleteMyForgeSSHKey remove uma chave SSH do forge do usuário.
// DELETE /api/me/forge-ssh-keys/:id
func (a *App) handleDeleteMyForgeSSHKey(c *gin.Context) {
	uid := callerUserID(c)
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var key store.ForgeSSHKey
	if err := a.Store.DB.Where("id = ? AND user_id = ?", id, uid).First(&key).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "chave não encontrada"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if err := a.Store.DB.Delete(&key).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if err := a.applyForgeAuthorizedKeys(c.Request.Context()); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "chave removida, mas falha ao aplicar no servidor: " + provisionerErrMsg(err)})
		return
	}
	var user store.User
	_ = a.Store.DB.First(&user, uid).Error
	_ = a.Store.LogAudit(user.Username, "forge.ssh_key.delete", "fingerprint="+key.Fingerprint)
	c.Status(http.StatusNoContent)
}

// upsertForgeSSHKey grava ou atualiza a chave do usuário no forge.
// Idempotente pelo par (tipo, blob) da chave pública.
func (a *App) upsertForgeSSHKey(ctx context.Context, user store.User, publicKey, title string, deviceID *uint) (store.ForgeSSHKey, bool, error) {
	fingerprint := sshKeyFingerprint(publicKey)
	var existing []store.ForgeSSHKey
	if err := a.Store.DB.Where("user_id = ?", user.ID).Find(&existing).Error; err != nil {
		return store.ForgeSSHKey{}, false, err
	}
	for _, k := range existing {
		if sameSSHKey(k.PublicKey, publicKey) {
			updates := map[string]any{}
			if strings.TrimSpace(k.Title) != strings.TrimSpace(title) && strings.TrimSpace(title) != "" {
				updates["title"] = title
			}
			if deviceID != nil && (k.DeviceID == nil || *k.DeviceID != *deviceID) {
				updates["device_id"] = deviceID
			}
			if len(updates) > 0 {
				if err := a.Store.DB.Model(&store.ForgeSSHKey{}).Where("id = ?", k.ID).Updates(updates).Error; err != nil {
					return store.ForgeSSHKey{}, false, err
				}
				_ = a.Store.DB.First(&k, k.ID)
			}
			return k, false, nil
		}
	}

	var count int64
	if err := a.Store.DB.Model(&store.ForgeSSHKey{}).Where("user_id = ?", user.ID).Count(&count).Error; err != nil {
		return store.ForgeSSHKey{}, false, err
	}
	if count >= maxForgeSSHKeysPerUser {
		return store.ForgeSSHKey{}, false, fmt.Errorf("limite de chaves SSH atingido")
	}

	created := store.ForgeSSHKey{
		UserID:      user.ID,
		Title:       title,
		PublicKey:   publicKey,
		Fingerprint: fingerprint,
		DeviceID:    deviceID,
	}
	if err := a.Store.DB.Create(&created).Error; err != nil {
		return store.ForgeSSHKey{}, false, err
	}
	if err := a.applyForgeAuthorizedKeys(ctx); err != nil {
		return created, true, fmt.Errorf("chave registrada, mas falha ao aplicar no servidor: %w", err)
	}
	return created, true, nil
}

// reconcileForgeSSHKeysFromDevices espelha chaves de dispositivos já no DB.
func (a *App) reconcileForgeSSHKeysFromDevices(ctx context.Context) error {
	var devices []store.Device
	if err := a.Store.DB.Where("ssh_public_key <> ''").Order("id").Find(&devices).Error; err != nil {
		return err
	}
	for _, d := range devices {
		var owner store.User
		if err := a.Store.DB.First(&owner, d.UserID).Error; err != nil {
			continue
		}
		title := d.Name
		if title == "" {
			title = "xvpn-device"
		}
		deviceID := d.ID
		_, _, _ = a.upsertForgeSSHKey(ctx, owner, d.SSHPublicKey, title, &deviceID)
	}
	return nil
}

// registerForgeSSHKeyFromDevice é chamado após POST /api/me/ssh-key para
// espelhar a mesma chave no git@xgit. Falha silenciosa no cliente — o SFTP
// já foi tratado; o forge é conveniência adicional.
func (a *App) registerForgeSSHKeyFromDevice(c *gin.Context, device store.Device, owner store.User, publicKey string) bool {
	title := strings.TrimSpace(device.Name)
	if title == "" {
		title = "xvpn-device"
	}
	deviceID := device.ID
	_, changed, err := a.upsertForgeSSHKey(c.Request.Context(), owner, publicKey, title, &deviceID)
	if err != nil {
		return false
	}
	if changed {
		_ = a.Store.LogAudit(owner.Username, "forge.ssh_key.autoregister",
			"device_id="+strconv.FormatUint(uint64(device.ID), 10)+" fingerprint="+sshKeyFingerprint(publicKey))
	}
	return true
}
