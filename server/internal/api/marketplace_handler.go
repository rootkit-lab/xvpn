package api

import (
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
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
	Slug        string                       `json:"slug"`
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	IconURL     string                       `json:"icon_url,omitempty"`
	Visibility  string                       `json:"visibility"`
	Network     string                       `json:"network"`
	Source      string                       `json:"source,omitempty"`
	SourcePath  string                       `json:"source_path,omitempty"`
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
		Slug:        app.Slug,
		Name:        app.Name,
		Description: app.Description,
		IconURL:     app.IconURL,
		Visibility:  string(app.Visibility),
		Network:     string(appNetworkOrDefault(app.Network)),
		Source:      app.Source,
		SourcePath:  app.SourcePath,
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

// appNetworkOrDefault trata linhas antigas (coluna vazia) como public —
// o default do AutoMigrate só vale para INSERT novo.
func appNetworkOrDefault(n store.AppNetwork) store.AppNetwork {
	if n == "" {
		return store.AppNetworkPublic
	}
	return n
}

// requestFromVPN reporta se a origem está na sub-rede WireGuard.
// RemoteIP() cobre o listener do túnel (10.66.66.1:8080). ClientIP()
// cobre o peer que chega ao Nginx (público ou *.corp) já pelo wg0
// (X-Forwarded-For = $remote_addr). Não aceita header forjado:
// trustedProxies é só loopback, então um XFF 10.66.66.x vindo da
// internet pública não vira ClientIP (ver clientip_test.go).
//
// Host (*.corp.ihuull.com) NÃO é prova de intranet: o mesmo processo
// Gin é alcançado pelos vhosts públicos, e um Host que não casa
// server_name cai no default_server ainda proxied para :8080.
func (a *App) requestFromVPN(c *gin.Context) bool {
	if a.Config == nil {
		return false
	}
	_, subnet, err := net.ParseCIDR(a.Config.WireGuardAllowedSubnet)
	if err != nil || subnet == nil {
		return false
	}
	if ip := net.ParseIP(c.RemoteIP()); ip != nil && subnet.Contains(ip) {
		return true
	}
	if ip := net.ParseIP(c.ClientIP()); ip != nil && subnet.Contains(ip) {
		return true
	}
	return false
}

// canSeeAppNetwork: visibility = quem (ACL); network = onde.
// network:public sempre passa. network:vpn só com origem na sub-rede
// WireGuard (PLAN.md §6.13). A loja pública (marketplace.ihuull.com)
// sem túnel não lista nem baixa app vpn — Host corp sozinho não basta.
func (a *App) canSeeAppNetwork(c *gin.Context, network store.AppNetwork) bool {
	switch appNetworkOrDefault(network) {
	case store.AppNetworkPublic:
		return true
	case store.AppNetworkVPN:
		return a.requestFromVPN(c)
	default:
		return false
	}
}

// handleListMarketplaceApps lista o catálogo do marketplace (Fase 11 — ver
// PLAN.md §6.8). admin/super_admin veem todos os apps (inclusive
// restritos, com a lista de quem tem acesso); os demais papéis só veem
// apps globais ou restritos aos quais já têm AppAccess. Apps
// network:vpn somem da loja pública sem túnel (PLAN.md §6.13).
// GET /api/marketplace/apps
func (a *App) handleListMarketplaceApps(c *gin.Context) {
	var apps []store.App
	err := a.Store.DB.
		Where("archived_at IS NULL").
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
		if !a.canSeeAppNetwork(c, app.Network) {
			continue
		}
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

// marketplaceStatsTopAssetsLimit é quantos assets aparecem no ranking do
// dashboard (Fase 12 — ROADMAP.md, "estatísticas de download no dashboard
// admin") — suficiente pra dar uma visão geral do que mais baixa sem virar
// uma segunda listagem completa (essa já existe em GET /marketplace/apps).
const marketplaceStatsTopAssetsLimit = 10

type marketplaceAssetStat struct {
	AssetID       uint   `json:"asset_id"`
	AppID         uint   `json:"app_id"`
	AppName       string `json:"app_name"`
	Version       string `json:"version"`
	Platform      string `json:"platform"`
	Arch          string `json:"arch"`
	Filename      string `json:"filename"`
	DownloadCount int64  `json:"download_count"`
}

type marketplaceStatsResponse struct {
	TotalApps      int64 `json:"total_apps"`
	TotalVersions  int64 `json:"total_versions"`
	TotalAssets    int64 `json:"total_assets"`
	TotalDownloads int64 `json:"total_downloads"`
	// TotalStorageBytes soma só blobs distintos (por storage_path) — dois
	// AppAsset com o mesmo conteúdo (dedupe do internal/marketplace)
	// contam uma vez só, senão o número não bateria com o espaço
	// realmente ocupado em disco.
	TotalStorageBytes int64                  `json:"total_storage_bytes"`
	TopAssets         []marketplaceAssetStat `json:"top_assets"`
}

// handleMarketplaceStats agrega métricas do catálogo inteiro pro dashboard
// admin (Fase 12 — ROADMAP.md). Fica no grupo viewerUp (mesmo nível de
// leitura do resto do dashboard/audit, ver server.go e PLAN.md §6.7), não
// authed nem adminOnly: é uma visão gerencial, não algo que member precise
// pra navegar o catálogo, nem uma ação de escrita.
// GET /api/marketplace/stats
func (a *App) handleMarketplaceStats(c *gin.Context) {
	var resp marketplaceStatsResponse

	if err := a.Store.DB.Model(&store.App{}).Where("archived_at IS NULL").Count(&resp.TotalApps).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if err := a.Store.DB.Model(&store.AppVersion{}).Count(&resp.TotalVersions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if err := a.Store.DB.Model(&store.AppAsset{}).Count(&resp.TotalAssets).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if err := a.Store.DB.Model(&store.AppAsset{}).
		Select("COALESCE(SUM(download_count), 0)").
		Scan(&resp.TotalDownloads).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	if err := a.Store.DB.Raw(`
		SELECT COALESCE(SUM(size_bytes), 0) FROM (
			SELECT size_bytes FROM app_assets GROUP BY storage_path
		)
	`).Scan(&resp.TotalStorageBytes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}

	var topAssets []marketplaceAssetStat
	err := a.Store.DB.Table("app_assets").
		Select(`app_assets.id AS asset_id, apps.id AS app_id, apps.name AS app_name,
			app_versions.version AS version, app_assets.platform AS platform,
			app_assets.arch AS arch, app_assets.filename AS filename,
			app_assets.download_count AS download_count`).
		Joins("JOIN app_versions ON app_versions.id = app_assets.app_version_id").
		Joins("JOIN apps ON apps.id = app_versions.app_id").
		Where("app_assets.download_count > 0 AND apps.archived_at IS NULL").
		Order("app_assets.download_count DESC").
		Limit(marketplaceStatsTopAssetsLimit).
		Find(&topAssets).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	resp.TopAssets = topAssets
	if resp.TopAssets == nil {
		resp.TopAssets = []marketplaceAssetStat{}
	}

	c.JSON(http.StatusOK, resp)
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
	if app.ArchivedAt != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "asset não encontrado"})
		return
	}
	if !a.canSeeAppNetwork(c, app.Network) {
		c.JSON(http.StatusForbidden, gin.H{"error": "este app só está disponível na VPN"})
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
