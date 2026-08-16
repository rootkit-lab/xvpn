package api

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/rootkit-lab/xvpn/server/internal/auth"
	"github.com/rootkit-lab/xvpn/server/internal/marketplace"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

// marketplaceSyncRequest é o corpo de POST /api/marketplace/sync — a
// lista COMPLETA de manifestos do diretório apps/ (full sync, PLAN.md
// §6.10.3). Deltas não bastam: um app removido do Git sobreviveria no
// catálogo por omissão.
type marketplaceSyncRequest struct {
	CommitSHA string                    `json:"commit_sha"`
	Apps      []marketplaceSyncAppInput `json:"apps"`
}

type marketplaceSyncAppInput struct {
	Slug        string                      `json:"slug"`
	Name        string                      `json:"name"`
	Description string                      `json:"description"`
	IconURL     string                      `json:"icon_url"`
	Visibility  store.AppVisibility         `json:"visibility"`
	Network     store.AppNetwork            `json:"network"`
	Source      string                      `json:"source"`
	SourcePath  string                      `json:"source_path"`
	Version     string                      `json:"version"`
	Channel     string                      `json:"channel"`
	Changelog   string                      `json:"changelog"`
	Assets      []marketplaceSyncAssetInput `json:"assets"`
}

type marketplaceSyncAssetInput struct {
	Platform string `json:"platform"`
	Arch     string `json:"arch"`
	URL      string `json:"url"`
	SHA256   string `json:"sha256"`
	Filename string `json:"filename"`
}

type marketplaceSyncSkipped struct {
	Slug   string `json:"slug"`
	Reason string `json:"reason"`
}

type marketplaceSyncResponse struct {
	Created   []string                 `json:"created"`
	Updated   []string                 `json:"updated"`
	Unchanged []string                 `json:"unchanged"`
	Archived  []string                 `json:"archived"`
	Skipped   []marketplaceSyncSkipped `json:"skipped"`
}

const marketplaceSyncActorCI = "ci"

// requireMarketplacePublishAuth aceita Bearer do XVPN_PUBLISH_TOKEN
// (comparação em tempo constante) OU JWT de super_admin. Registrado só
// quando PublishToken não é vazio (ver NewRouter).
func (a *App) requireMarketplacePublishAuth() gin.HandlerFunc {
	token := a.Config.PublishToken
	return func(c *gin.Context) {
		hdr := c.GetHeader("Authorization")
		if !strings.HasPrefix(hdr, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "não autenticado"})
			return
		}
		raw := strings.TrimSpace(strings.TrimPrefix(hdr, "Bearer "))
		if raw == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "não autenticado"})
			return
		}

		if subtle.ConstantTimeCompare([]byte(raw), []byte(token)) == 1 {
			c.Set(auth.ContextUsernameKey, marketplaceSyncActorCI)
			c.Set(auth.ContextRoleKey, store.RoleSuperAdmin)
			c.Next()
			return
		}

		claims, err := a.Tokens.Parse(raw)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "não autenticado"})
			return
		}
		if claims.Role != store.RoleSuperAdmin {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "apenas super_admin ou token de publicação"})
			return
		}
		c.Set(auth.ContextUserIDKey, claims.UserID)
		c.Set(auth.ContextUsernameKey, claims.Username)
		c.Set(auth.ContextRoleKey, claims.Role)
		c.Next()
	}
}

// handleMarketplaceSync aplica o full sync do diretório apps/ no catálogo.
// POST /api/marketplace/sync
func (a *App) handleMarketplaceSync(c *gin.Context) {
	var req marketplaceSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	if req.Apps == nil {
		req.Apps = []marketplaceSyncAppInput{}
	}

	resp := marketplaceSyncResponse{
		Created:   []string{},
		Updated:   []string{},
		Unchanged: []string{},
		Archived:  []string{},
		Skipped:   []marketplaceSyncSkipped{},
	}
	seen := make(map[string]struct{}, len(req.Apps))

	for _, in := range req.Apps {
		slug := strings.TrimSpace(in.Slug)
		if slug == "" {
			resp.Skipped = append(resp.Skipped, marketplaceSyncSkipped{Slug: "", Reason: "slug é obrigatório"})
			continue
		}
		if _, dup := seen[slug]; dup {
			resp.Skipped = append(resp.Skipped, marketplaceSyncSkipped{Slug: slug, Reason: "slug duplicado no payload"})
			continue
		}
		seen[slug] = struct{}{}

		status, err := a.syncOneMarketplaceApp(c.Request.Context(), in)
		if err != nil {
			resp.Skipped = append(resp.Skipped, marketplaceSyncSkipped{Slug: slug, Reason: err.Error()})
			continue
		}
		switch status {
		case "created":
			resp.Created = append(resp.Created, slug)
		case "updated":
			resp.Updated = append(resp.Updated, slug)
		default:
			resp.Unchanged = append(resp.Unchanged, slug)
		}
	}

	archived, err := a.archiveMissingMarketplaceApps(seen)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	resp.Archived = archived

	actor, _ := c.Get(auth.ContextUsernameKey)
	detail := fmt.Sprintf("created=%d updated=%d unchanged=%d archived=%d skipped=%d commit=%s",
		len(resp.Created), len(resp.Updated), len(resp.Unchanged), len(resp.Archived), len(resp.Skipped),
		strings.TrimSpace(req.CommitSHA))
	_ = a.Store.LogAudit(actorString(actor), "marketplace.sync", detail)

	c.JSON(http.StatusOK, resp)
}

func (a *App) syncOneMarketplaceApp(ctx context.Context, in marketplaceSyncAppInput) (string, error) {
	slug := strings.TrimSpace(in.Slug)
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return "", errors.New("name é obrigatório")
	}
	source := strings.TrimSpace(in.Source)
	if source != store.AppSourceBuild && source != store.AppSourceExternal {
		return "", errors.New("source inválido (use build ou external)")
	}
	sourcePath := strings.TrimSpace(in.SourcePath)
	if sourcePath == "" {
		sourcePath = "apps/" + slug
	}
	visibility := in.Visibility
	if visibility == "" {
		visibility = store.AppVisibilityGlobal
	}
	if !visibility.Valid() {
		return "", errors.New("visibility inválido")
	}
	network := in.Network
	if network == "" {
		network = store.AppNetworkPublic
	}
	if !network.Valid() {
		return "", errors.New("network inválido (use public ou vpn)")
	}
	version := strings.TrimSpace(in.Version)
	if version == "" {
		return "", errors.New("version é obrigatório")
	}
	channel := strings.TrimSpace(in.Channel)
	if channel == "" {
		channel = store.ChannelStable
	}
	if !store.ValidChannel(channel) {
		return "", errors.New("channel inválido")
	}

	for i, asset := range in.Assets {
		if !store.Platform(strings.TrimSpace(asset.Platform)).Valid() {
			return "", fmt.Errorf("assets[%d]: platform inválida", i)
		}
		if strings.TrimSpace(asset.URL) == "" || strings.TrimSpace(asset.SHA256) == "" {
			return "", fmt.Errorf("assets[%d]: url e sha256 são obrigatórios", i)
		}
	}

	created := false
	changed := false

	var app store.App
	err := a.Store.DB.Where("slug = ?", slug).First(&app).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		app = store.App{
			Slug:        slug,
			Name:        name,
			Description: strings.TrimSpace(in.Description),
			IconURL:     strings.TrimSpace(in.IconURL),
			Visibility:  visibility,
			Network:     network,
			Source:      source,
			SourcePath:  sourcePath,
		}
		if err := a.Store.DB.Create(&app).Error; err != nil {
			return "", err
		}
		created = true
		changed = true
	} else if err != nil {
		return "", err
	} else {
		if app.ArchivedAt != nil {
			app.ArchivedAt = nil
			changed = true
		}
		desc := strings.TrimSpace(in.Description)
		icon := strings.TrimSpace(in.IconURL)
		if app.Name != name || app.Description != desc || app.IconURL != icon ||
			app.Visibility != visibility || app.Network != network ||
			app.Source != source || app.SourcePath != sourcePath {
			app.Name = name
			app.Description = desc
			app.IconURL = icon
			app.Visibility = visibility
			app.Network = network
			app.Source = source
			app.SourcePath = sourcePath
			changed = true
		}
		if changed {
			if err := a.Store.DB.Save(&app).Error; err != nil {
				return "", err
			}
		}
	}

	var ver store.AppVersion
	err = a.Store.DB.Where("app_id = ? AND version = ?", app.ID, version).First(&ver).Error
	changelog := strings.TrimSpace(in.Changelog)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		ver = store.AppVersion{
			AppID:     app.ID,
			Version:   version,
			Channel:   channel,
			Changelog: changelog,
		}
		if err := a.Store.DB.Create(&ver).Error; err != nil {
			return "", err
		}
		changed = true
	} else if err != nil {
		return "", err
	} else if ver.Channel != channel || ver.Changelog != changelog {
		ver.Channel = channel
		ver.Changelog = changelog
		if err := a.Store.DB.Save(&ver).Error; err != nil {
			return "", err
		}
		changed = true
	}

	for _, assetIn := range in.Assets {
		assetChanged, err := a.syncOneMarketplaceAsset(ctx, ver.ID, assetIn)
		if err != nil {
			return "", err
		}
		if assetChanged {
			changed = true
		}
	}

	if created {
		return "created", nil
	}
	if changed {
		return "updated", nil
	}
	return "unchanged", nil
}

func (a *App) syncOneMarketplaceAsset(ctx context.Context, versionID uint, in marketplaceSyncAssetInput) (bool, error) {
	platform := store.Platform(strings.TrimSpace(in.Platform))
	arch := strings.TrimSpace(in.Arch)
	if arch == "" {
		arch = defaultAssetArch
	}
	sha := strings.ToLower(strings.TrimSpace(in.SHA256))
	assetURL := strings.TrimSpace(in.URL)

	var existing store.AppAsset
	err := a.Store.DB.Where("app_version_id = ? AND platform = ? AND arch = ?", versionID, platform, arch).
		First(&existing).Error
	if err == nil && existing.SHA256 == sha {
		return false, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	isNew := errors.Is(err, gorm.ErrRecordNotFound)

	fetcher := a.assetFetcher()
	result, suggestedName, err := fetcher(ctx, a.Marketplace, assetURL, sha)
	if err != nil {
		return false, err
	}

	filename := strings.TrimSpace(in.Filename)
	if filename == "" {
		filename = suggestedName
	}
	filename = path.Base(filename)
	if len(filename) > 255 {
		filename = filename[:255]
	}

	if isNew {
		asset := store.AppAsset{
			AppVersionID: versionID,
			Platform:     platform,
			Arch:         arch,
			Filename:     filename,
			SHA256:       result.SHA256,
			SizeBytes:    result.Size,
			StoragePath:  result.RelPath,
		}
		if err := a.Store.DB.Create(&asset).Error; err != nil {
			a.removeOrphanBlobs([]store.AppAsset{{StoragePath: result.RelPath}})
			return false, err
		}
		return true, nil
	}

	oldPath := existing.StoragePath
	existing.Filename = filename
	existing.SHA256 = result.SHA256
	existing.SizeBytes = result.Size
	existing.StoragePath = result.RelPath
	if err := a.Store.DB.Save(&existing).Error; err != nil {
		a.removeOrphanBlobs([]store.AppAsset{{StoragePath: result.RelPath}})
		return false, err
	}
	if oldPath != "" && oldPath != result.RelPath {
		a.removeOrphanBlobs([]store.AppAsset{{StoragePath: oldPath}})
	}
	return true, nil
}

func (a *App) archiveMissingMarketplaceApps(keep map[string]struct{}) ([]string, error) {
	var apps []store.App
	if err := a.Store.DB.Where("archived_at IS NULL").Find(&apps).Error; err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	archived := []string{}
	for _, app := range apps {
		if app.Slug == "" {
			continue
		}
		if _, ok := keep[app.Slug]; ok {
			continue
		}
		app.ArchivedAt = &now
		if err := a.Store.DB.Save(&app).Error; err != nil {
			return nil, err
		}
		archived = append(archived, app.Slug)
	}
	return archived, nil
}

func (a *App) assetFetcher() func(context.Context, *marketplace.Store, string, string) (marketplace.PutResult, string, error) {
	if a.fetchAsset != nil {
		return a.fetchAsset
	}
	return marketplace.FetchAndPut
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
