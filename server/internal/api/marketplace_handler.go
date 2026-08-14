package api

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/marketplace"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

// defaultAssetArch é usado quando o admin não informa uma arquitetura no
// upload — a grande maioria dos instaladores atuais é amd64.
const defaultAssetArch = "amd64"

type marketplaceAssetResponse struct {
	ID            uint      `json:"id"`
	VersionID     uint      `json:"version_id"`
	Platform      string    `json:"platform"`
	Arch          string    `json:"arch"`
	Filename      string    `json:"filename"`
	SHA256        string    `json:"sha256"`
	SizeBytes     int64     `json:"size_bytes"`
	DownloadCount int64     `json:"download_count"`
	CreatedAt     time.Time `json:"created_at"`
}

type marketplaceVersionResponse struct {
	ID        uint                       `json:"id"`
	AppID     uint                       `json:"app_id"`
	Version   string                     `json:"version"`
	Channel   string                     `json:"channel"`
	Changelog string                     `json:"changelog"`
	CreatedAt time.Time                  `json:"created_at"`
	Assets    []marketplaceAssetResponse `json:"assets"`
}

type marketplaceAppResponse struct {
	ID          uint                         `json:"id"`
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	IconURL     string                       `json:"icon_url,omitempty"`
	Visibility  string                       `json:"visibility"`
	CreatedAt   time.Time                    `json:"created_at"`
	Versions    []marketplaceVersionResponse `json:"versions"`
	// AccessUserIDs só é preenchido para quem administra o marketplace
	// (admin+) — nunca exposto a viewer/member, para não vazar a um
	// usuário comum quem mais tem acesso a um app restrito (ver
	// handleListMarketplaceApps).
	AccessUserIDs []uint `json:"access_user_ids,omitempty"`
}

func toMarketplaceAssetResponse(a store.AppAsset) marketplaceAssetResponse {
	return marketplaceAssetResponse{
		ID:            a.ID,
		VersionID:     a.AppVersionID,
		Platform:      string(a.Platform),
		Arch:          a.Arch,
		Filename:      a.Filename,
		SHA256:        a.SHA256,
		SizeBytes:     a.SizeBytes,
		DownloadCount: a.DownloadCount,
		CreatedAt:     a.CreatedAt,
	}
}

func toMarketplaceVersionResponse(v store.AppVersion) marketplaceVersionResponse {
	assets := make([]marketplaceAssetResponse, 0, len(v.Assets))
	for _, asset := range v.Assets {
		assets = append(assets, toMarketplaceAssetResponse(asset))
	}
	return marketplaceVersionResponse{
		ID:        v.ID,
		AppID:     v.AppID,
		Version:   v.Version,
		Channel:   v.Channel,
		Changelog: v.Changelog,
		CreatedAt: v.CreatedAt,
		Assets:    assets,
	}
}

func toMarketplaceAppResponse(app store.App) marketplaceAppResponse {
	versions := make([]marketplaceVersionResponse, 0, len(app.Versions))
	for _, v := range app.Versions {
		versions = append(versions, toMarketplaceVersionResponse(v))
	}
	return marketplaceAppResponse{
		ID:          app.ID,
		Name:        app.Name,
		Description: app.Description,
		IconURL:     app.IconURL,
		Visibility:  string(app.Visibility),
		CreatedAt:   app.CreatedAt,
		Versions:    versions,
	}
}

// isAdminRole reproduz a checagem de PLAN.md §6.7 ("admin/super_admin:
// Admin + download") — usada aqui para decidir quem enxerga o catálogo
// inteiro (ignorando ACL) e quem baixa qualquer asset independente de
// AppAccess.
func isMarketplaceAdmin(role store.Role) bool {
	return role.Rank() >= store.RoleAdmin.Rank()
}

// handleListMarketplaceApps lista o catálogo do marketplace (Fase 11 — ver
// PLAN.md §6.8). admin/super_admin veem todos os apps (inclusive
// restritos, com a lista de quem tem acesso); os demais papéis só veem
// apps globais ou restritos aos quais já têm AppAccess.
// GET /api/marketplace/apps
func (a *App) handleListMarketplaceApps(c *gin.Context) {
	var apps []store.App
	err := a.Store.DB.
		Preload("Versions", func(db *gorm.DB) *gorm.DB { return db.Order("id desc") }).
		Preload("Versions.Assets", func(db *gorm.DB) *gorm.DB { return db.Order("platform, arch") }).
		Order("name").Find(&apps).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	var allAccess []store.AppAccess
	if err := a.Store.DB.Order("app_id, user_id").Find(&allAccess).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	accessByApp := make(map[uint][]uint, len(apps))
	for _, ar := range allAccess {
		accessByApp[ar.AppID] = append(accessByApp[ar.AppID], ar.UserID)
	}

	role := callerRole(c)
	userID := callerUserID(c)
	admin := isMarketplaceAdmin(role)

	resp := make([]marketplaceAppResponse, 0, len(apps))
	for _, app := range apps {
		if !admin && app.Visibility == store.AppVisibilityRestricted && !containsUint(accessByApp[app.ID], userID) {
			continue
		}
		item := toMarketplaceAppResponse(app)
		if admin {
			item.AccessUserIDs = accessByApp[app.ID]
		}
		resp = append(resp, item)
	}
	c.JSON(http.StatusOK, resp)
}

type createMarketplaceAppRequest struct {
	Name        string              `json:"name" binding:"required"`
	Description string              `json:"description"`
	IconURL     string              `json:"icon_url"`
	Visibility  store.AppVisibility `json:"visibility"`
}

// handleCreateMarketplaceApp cria uma entrada nova no catálogo (o "publish"
// inicial de um programa — ver ROADMAP.md Fase 11, item de audit log).
// POST /api/marketplace/apps
func (a *App) handleCreateMarketplaceApp(c *gin.Context) {
	var req createMarketplaceAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name é obrigatório"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name é obrigatório"})
		return
	}
	visibility := req.Visibility
	if visibility == "" {
		visibility = store.AppVisibilityGlobal
	}
	if !visibility.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "visibility inválido (use global ou restricted)"})
		return
	}

	app := store.App{
		Name:        name,
		Description: strings.TrimSpace(req.Description),
		IconURL:     strings.TrimSpace(req.IconURL),
		Visibility:  visibility,
	}
	if err := a.Store.DB.Create(&app).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "marketplace.app_create",
		fmt.Sprintf("app_id=%d name=%s visibility=%s", app.ID, app.Name, app.Visibility))

	c.JSON(http.StatusCreated, toMarketplaceAppResponse(app))
}

type updateMarketplaceAppRequest struct {
	Name        *string              `json:"name"`
	Description *string              `json:"description"`
	IconURL     *string              `json:"icon_url"`
	Visibility  *store.AppVisibility `json:"visibility"`
}

// handleUpdateMarketplaceApp edita metadados de um app existente — nunca
// mexe em versões/assets (ver handle*MarketplaceVersion/Asset).
// PATCH /api/marketplace/apps/:id
func (a *App) handleUpdateMarketplaceApp(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var app store.App
	if err := a.Store.DB.First(&app, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "app não encontrado"})
		return
	}

	var req updateMarketplaceAppRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}

	updates := map[string]any{}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name não pode ficar vazio"})
			return
		}
		updates["name"] = name
	}
	if req.Description != nil {
		updates["description"] = strings.TrimSpace(*req.Description)
	}
	if req.IconURL != nil {
		updates["icon_url"] = strings.TrimSpace(*req.IconURL)
	}
	if req.Visibility != nil {
		if !req.Visibility.Valid() {
			c.JSON(http.StatusBadRequest, gin.H{"error": "visibility inválido (use global ou restricted)"})
			return
		}
		updates["visibility"] = *req.Visibility
	}
	if len(updates) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nada para atualizar"})
		return
	}

	if err := a.Store.DB.Model(&store.App{}).Where("id = ?", app.ID).Updates(updates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "marketplace.app_update", "app_id="+c.Param("id"))

	if err := a.Store.DB.First(&app, id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.JSON(http.StatusOK, toMarketplaceAppResponse(app))
}

// handleDeleteMarketplaceApp remove um app e tudo que pertence a ele
// (versões, assets, ACL) — os blobs físicos em disco só são removidos se
// nenhum outro AppAsset (de outro app) ainda referenciar o mesmo conteúdo
// (ver removeOrphanBlobs, dedupe do internal/marketplace).
// DELETE /api/marketplace/apps/:id
func (a *App) handleDeleteMarketplaceApp(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var app store.App
	if err := a.Store.DB.First(&app, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "app não encontrado"})
		return
	}

	var versions []store.AppVersion
	if err := a.Store.DB.Where("app_id = ?", app.ID).Find(&versions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	versionIDs := make([]uint, 0, len(versions))
	for _, v := range versions {
		versionIDs = append(versionIDs, v.ID)
	}

	var assets []store.AppAsset
	if len(versionIDs) > 0 {
		if err := a.Store.DB.Where("app_version_id IN ?", versionIDs).Find(&assets).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
			return
		}
	}

	err = a.Store.DB.Transaction(func(tx *gorm.DB) error {
		if len(versionIDs) > 0 {
			if err := tx.Where("app_version_id IN ?", versionIDs).Delete(&store.AppAsset{}).Error; err != nil {
				return err
			}
			if err := tx.Where("app_id = ?", app.ID).Delete(&store.AppVersion{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("app_id = ?", app.ID).Delete(&store.AppAccess{}).Error; err != nil {
			return err
		}
		return tx.Delete(&store.App{}, app.ID).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	a.removeOrphanBlobs(assets)

	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "marketplace.app_delete",
		fmt.Sprintf("app_id=%d name=%s", app.ID, app.Name))

	c.Status(http.StatusNoContent)
}

type setMarketplaceAppAccessRequest struct {
	UserIDs []uint `json:"user_ids"`
}

// handleSetMarketplaceAppAccess substitui de uma vez a lista de usuários
// com acesso a um app restrito (ver PLAN.md §6.8, "ACL: lista de user
// IDs") — mais simples para a UI que um endpoint de adicionar/remover um
// por um, e suficiente na escala de 1-15 usuários do projeto.
// PUT /api/marketplace/apps/:id/access
func (a *App) handleSetMarketplaceAppAccess(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var app store.App
	if err := a.Store.DB.First(&app, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "app não encontrado"})
		return
	}

	var req setMarketplaceAppAccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}

	userIDs := dedupUint(req.UserIDs)
	if len(userIDs) > 0 {
		var count int64
		if err := a.Store.DB.Model(&store.User{}).Where("id IN ?", userIDs).Count(&count).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
			return
		}
		if int(count) != len(userIDs) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "um ou mais user_id não existem"})
			return
		}
	}

	err = a.Store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("app_id = ?", app.ID).Delete(&store.AppAccess{}).Error; err != nil {
			return err
		}
		for _, uid := range userIDs {
			if err := tx.Create(&store.AppAccess{AppID: app.ID, UserID: uid}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "marketplace.app_access_update",
		fmt.Sprintf("app_id=%d user_ids=%s", app.ID, joinUints(userIDs)))

	c.JSON(http.StatusOK, gin.H{"user_ids": userIDs})
}

type createMarketplaceVersionRequest struct {
	Version   string `json:"version" binding:"required"`
	Channel   string `json:"channel"`
	Changelog string `json:"changelog"`
}

// handleCreateMarketplaceVersion publica uma nova versão de um app — o
// upload dos assets (arquivos de verdade) é um passo à parte, ver
// handleUploadMarketplaceAsset.
// POST /api/marketplace/apps/:id/versions
func (a *App) handleCreateMarketplaceVersion(c *gin.Context) {
	appID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var app store.App
	if err := a.Store.DB.First(&app, appID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "app não encontrado"})
		return
	}

	var req createMarketplaceVersionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "version é obrigatório"})
		return
	}
	version := strings.TrimSpace(req.Version)
	if version == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "version é obrigatório"})
		return
	}
	channel := strings.TrimSpace(req.Channel)
	if channel == "" {
		channel = store.ChannelStable
	}
	if !store.ValidChannel(channel) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "channel inválido (use stable ou beta)"})
		return
	}

	v := store.AppVersion{
		AppID:     app.ID,
		Version:   version,
		Channel:   channel,
		Changelog: strings.TrimSpace(req.Changelog),
	}
	if err := a.Store.DB.Create(&v).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "marketplace.version_create",
		fmt.Sprintf("app_id=%d version_id=%d version=%s channel=%s", app.ID, v.ID, v.Version, v.Channel))

	c.JSON(http.StatusCreated, toMarketplaceVersionResponse(v))
}

// handleDeleteMarketplaceVersion remove uma versão e seus assets — mesma
// lógica de limpeza de blob órfão da remoção de app.
// DELETE /api/marketplace/versions/:id
func (a *App) handleDeleteMarketplaceVersion(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var version store.AppVersion
	if err := a.Store.DB.First(&version, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "versão não encontrada"})
		return
	}

	var assets []store.AppAsset
	if err := a.Store.DB.Where("app_version_id = ?", version.ID).Find(&assets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	err = a.Store.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("app_version_id = ?", version.ID).Delete(&store.AppAsset{}).Error; err != nil {
			return err
		}
		return tx.Delete(&store.AppVersion{}, version.ID).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	a.removeOrphanBlobs(assets)

	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "marketplace.version_delete",
		fmt.Sprintf("app_id=%d version_id=%d version=%s", version.AppID, version.ID, version.Version))

	c.Status(http.StatusNoContent)
}

// handleUploadMarketplaceAsset recebe um arquivo (multipart/form-data,
// campos "platform", "arch" opcional e "file") para uma versão existente,
// grava o blob content-addressed (internal/marketplace) e registra o
// AppAsset com o SHA-256/tamanho calculados no próprio upload — nunca
// confiando em hash informado pelo cliente.
// POST /api/marketplace/versions/:id/assets
func (a *App) handleUploadMarketplaceAsset(c *gin.Context) {
	versionID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var version store.AppVersion
	if err := a.Store.DB.First(&version, versionID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "versão não encontrada"})
		return
	}

	// Limite duro no corpo da requisição inteira, além do limite já
	// aplicado dentro de marketplace.Store.Put — defesa em profundidade
	// contra um upload gigante consumir disco/tempo de parsing do
	// multipart antes mesmo de chegarmos ao Put (margem extra para o
	// overhead de framing do multipart e dos outros campos do form).
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, marketplace.MaxAssetSize+2<<20)

	platform := store.Platform(strings.TrimSpace(c.PostForm("platform")))
	if !platform.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platform inválida (use linux, windows ou android)"})
		return
	}
	arch := strings.TrimSpace(c.PostForm("arch"))
	if arch == "" {
		arch = defaultAssetArch
	}
	if len(arch) > 32 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "arch inválida"})
		return
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		if strings.Contains(err.Error(), "too large") {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "arquivo excede o tamanho máximo permitido"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "arquivo (\"file\") é obrigatório"})
		return
	}

	src, err := fileHeader.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	defer src.Close()

	result, err := a.Marketplace.Put(src)
	if err != nil {
		if errors.Is(err, marketplace.ErrAssetTooLarge) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "arquivo excede o tamanho máximo permitido"})
			return
		}
		slog.Error("falha ao gravar asset do marketplace", "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	filename := filepath.Base(fileHeader.Filename)
	if len(filename) > 255 {
		filename = filename[:255]
	}

	asset := store.AppAsset{
		AppVersionID: version.ID,
		Platform:     platform,
		Arch:         arch,
		Filename:     filename,
		SHA256:       result.SHA256,
		SizeBytes:    result.Size,
		StoragePath:  result.RelPath,
	}
	if err := a.Store.DB.Create(&asset).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "marketplace.asset_upload",
		fmt.Sprintf("app_id=%d version_id=%d asset_id=%d platform=%s arch=%s filename=%s sha256=%s size_bytes=%d",
			version.AppID, version.ID, asset.ID, asset.Platform, asset.Arch, asset.Filename, asset.SHA256, asset.SizeBytes))

	c.JSON(http.StatusCreated, toMarketplaceAssetResponse(asset))
}

// handleDeleteMarketplaceAsset remove um único asset (uma plataforma/
// arquitetura específica) sem apagar a versão inteira.
// DELETE /api/marketplace/assets/:id
func (a *App) handleDeleteMarketplaceAsset(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var asset store.AppAsset
	if err := a.Store.DB.First(&asset, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "asset não encontrado"})
		return
	}

	if err := a.Store.DB.Delete(&store.AppAsset{}, asset.ID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	a.removeOrphanBlobs([]store.AppAsset{asset})

	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "marketplace.asset_delete",
		fmt.Sprintf("asset_id=%d version_id=%d filename=%s", asset.ID, asset.AppVersionID, asset.Filename))

	c.Status(http.StatusNoContent)
}

// handleDownloadMarketplaceAsset serve o blob de um asset a um usuário
// autenticado — nunca anônimo (rota fica no grupo "authed" de server.go),
// ver PLAN.md §6.8. Aplica a mesma regra de ACL de handleListMarketplaceApps
// (admin+ sempre baixa; os demais só se o app for global ou tiverem
// AppAccess) e conta o download para métricas simples no painel.
// GET /api/marketplace/assets/:id/download
func (a *App) handleDownloadMarketplaceAsset(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var asset store.AppAsset
	if err := a.Store.DB.First(&asset, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "asset não encontrado"})
		return
	}
	var version store.AppVersion
	if err := a.Store.DB.First(&version, asset.AppVersionID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	var app store.App
	if err := a.Store.DB.First(&app, version.AppID).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	role := callerRole(c)
	userID := callerUserID(c)
	allowed := isMarketplaceAdmin(role) || app.Visibility == store.AppVisibilityGlobal
	if !allowed {
		var count int64
		if err := a.Store.DB.Model(&store.AppAccess{}).Where("app_id = ? AND user_id = ?", app.ID, userID).Count(&count).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
			return
		}
		allowed = count > 0
	}
	if !allowed {
		c.JSON(http.StatusForbidden, gin.H{"error": "você não tem acesso a este app"})
		return
	}

	absPath, err := a.Marketplace.AbsPath(asset.StoragePath)
	if err != nil {
		slog.Error("caminho de asset inválido", "asset_id", asset.ID, "err", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	if err := a.Store.DB.Model(&store.AppAsset{}).Where("id = ?", asset.ID).
		UpdateColumn("download_count", gorm.Expr("download_count + 1")).Error; err != nil {
		slog.Error("falha ao incrementar contador de download", "asset_id", asset.ID, "err", err)
	}

	actor, _ := c.Get(auth.ContextUsernameKey)
	_ = a.Store.LogAudit(actorString(actor), "marketplace.asset_download",
		fmt.Sprintf("app_id=%d version_id=%d asset_id=%d filename=%s", app.ID, version.ID, asset.ID, asset.Filename))

	c.FileAttachment(absPath, asset.Filename)
}

// removeOrphanBlobs apaga o blob físico de cada asset em removed, a não
// ser que outro AppAsset ainda vivo (de qualquer app/versão) aponte para o
// mesmo StoragePath — preserva o dedupe automático do internal/marketplace
// mesmo quando um dos "donos" do blob é excluído. Chamado sempre depois da
// transação de banco já ter commitado a exclusão dos assets em removed, ou
// a contagem abaixo os enxergaria como ainda existentes.
func (a *App) removeOrphanBlobs(removed []store.AppAsset) {
	for _, asset := range removed {
		var count int64
		if err := a.Store.DB.Model(&store.AppAsset{}).Where("storage_path = ?", asset.StoragePath).Count(&count).Error; err != nil {
			slog.Error("falha ao checar referências de blob antes de remover", "asset_id", asset.ID, "err", err)
			continue
		}
		if count > 0 {
			continue
		}
		if err := a.Marketplace.Remove(asset.StoragePath); err != nil {
			slog.Error("falha ao remover blob órfão", "asset_id", asset.ID, "storage_path", asset.StoragePath, "err", err)
		}
	}
}

func containsUint(list []uint, v uint) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func dedupUint(list []uint) []uint {
	seen := make(map[uint]bool, len(list))
	out := make([]uint, 0, len(list))
	for _, v := range list {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

func joinUints(list []uint) string {
	parts := make([]string, len(list))
	for i, v := range list {
		parts[i] = strconv.FormatUint(uint64(v), 10)
	}
	return strings.Join(parts, ",")
}
