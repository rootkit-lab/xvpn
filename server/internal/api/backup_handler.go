package api

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/rootkit-lab/xvpn/server/internal/backup"
	"github.com/rootkit-lab/xvpn/server/internal/store"
)

type backupEngine interface {
	Backup(ctx context.Context, dest backup.Dest, inc backup.Include, staging string, dryRun bool) (backup.Result, error)
}

type backupSettingsJSON struct {
	RetentionDays      int  `json:"retention_days"`
	IncludeMongo       bool `json:"include_mongo"`
	IncludeMarketplace bool `json:"include_marketplace"`
	IncludeGit         bool `json:"include_git"`
	IncludeSocial      bool `json:"include_social"`
}

type backupDestJSON struct {
	ID        uint   `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Endpoint  string `json:"endpoint"`
	Path      string `json:"path"`
	Enabled   bool   `json:"enabled"`
	HasSecret bool   `json:"has_secret"`
	Offsite   bool   `json:"offsite"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type backupJobJSON struct {
	ID            uint   `json:"id"`
	DestinationID uint   `json:"destination_id"`
	Destination   string `json:"destination"`
	DryRun        bool   `json:"dry_run"`
	Status        string `json:"status"`
	SnapshotID    string `json:"snapshot_id,omitempty"`
	Bytes         int64  `json:"bytes"`
	Error         string `json:"error,omitempty"`
	StartedAt     string `json:"started_at,omitempty"`
	FinishedAt    string `json:"finished_at,omitempty"`
	CreatedAt     string `json:"created_at"`
}

type upsertBackupDestRequest struct {
	Name     string           `json:"name"`
	Kind     store.BackupKind `json:"kind"`
	Endpoint string           `json:"endpoint"`
	Path     string           `json:"path"`
	Enabled  *bool            `json:"enabled"`
	Secret   *backup.Secret   `json:"secret"`
}

type patchBackupSettingsRequest struct {
	RetentionDays      *int  `json:"retention_days"`
	IncludeMongo       *bool `json:"include_mongo"`
	IncludeMarketplace *bool `json:"include_marketplace"`
	IncludeGit         *bool `json:"include_git"`
	IncludeSocial      *bool `json:"include_social"`
}

type runBackupRequest struct {
	DryRun bool `json:"dry_run"`
}

func (a *App) backupEngine() backupEngine {
	if a.Backup != nil {
		return a.Backup
	}
	return &backup.Runner{}
}

func (a *App) backupStaging() string {
	if a.Config != nil && a.Config.BackupDir != "" {
		return a.Config.BackupDir
	}
	return "/opt/xvpn/data/backups"
}

func destJSON(d store.BackupDestination) backupDestJSON {
	return backupDestJSON{
		ID:        d.ID,
		Name:      d.Name,
		Kind:      string(d.Kind),
		Endpoint:  d.Endpoint,
		Path:      d.Path,
		Enabled:   d.Enabled,
		HasSecret: strings.TrimSpace(d.Secret) != "",
		Offsite:   d.Kind.Offsite(),
		CreatedAt: d.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt: d.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func jobJSON(j store.BackupJob) backupJobJSON {
	out := backupJobJSON{
		ID:            j.ID,
		DestinationID: j.DestinationID,
		DryRun:        j.DryRun,
		Status:        string(j.Status),
		SnapshotID:    j.SnapshotID,
		Bytes:         j.Bytes,
		Error:         j.Error,
		CreatedAt:     j.CreatedAt.UTC().Format(time.RFC3339),
	}
	if j.Destination != nil {
		out.Destination = j.Destination.Name
	}
	if j.StartedAt != nil {
		out.StartedAt = j.StartedAt.UTC().Format(time.RFC3339)
	}
	if j.FinishedAt != nil {
		out.FinishedAt = j.FinishedAt.UTC().Format(time.RFC3339)
	}
	return out
}

func (a *App) loadBackupSettings() (store.BackupSettings, error) {
	if err := store.SeedBackupSettings(a.Store.DB); err != nil {
		return store.BackupSettings{}, err
	}
	var row store.BackupSettings
	err := a.Store.DB.First(&row, 1).Error
	return row, err
}

func (a *App) handleGetBackupSettings(c *gin.Context) {
	row, err := a.loadBackupSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	c.JSON(http.StatusOK, backupSettingsJSON{
		RetentionDays:      row.RetentionDays,
		IncludeMongo:       row.IncludeMongo,
		IncludeMarketplace: row.IncludeMarketplace,
		IncludeGit:         row.IncludeGit,
		IncludeSocial:      row.IncludeSocial,
	})
}

func (a *App) handlePatchBackupSettings(c *gin.Context) {
	row, err := a.loadBackupSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	var req patchBackupSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	if req.RetentionDays != nil {
		if *req.RetentionDays < 1 || *req.RetentionDays > 3650 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "retenção inválida"})
			return
		}
		row.RetentionDays = *req.RetentionDays
	}
	if req.IncludeMongo != nil {
		row.IncludeMongo = *req.IncludeMongo
	}
	if req.IncludeMarketplace != nil {
		row.IncludeMarketplace = *req.IncludeMarketplace
	}
	if req.IncludeGit != nil {
		row.IncludeGit = *req.IncludeGit
	}
	if req.IncludeSocial != nil {
		row.IncludeSocial = *req.IncludeSocial
	}
	if err := a.Store.DB.Save(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "backup.settings", "")
	a.handleGetBackupSettings(c)
}

func (a *App) handleListBackupDestinations(c *gin.Context) {
	var rows []store.BackupDestination
	if err := a.Store.DB.Order("id asc").Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]backupDestJSON, 0, len(rows))
	for _, d := range rows {
		items = append(items, destJSON(d))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) handleCreateBackupDestination(c *gin.Context) {
	var req upsertBackupDestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" || !req.Kind.Valid() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "nome ou tipo inválido"})
		return
	}
	if req.Kind == store.BackupKindLocal {
		c.JSON(http.StatusBadRequest, gin.H{"error": "destino local só para testes"})
		return
	}
	row := store.BackupDestination{
		Name:     name,
		Kind:     req.Kind,
		Endpoint: strings.TrimSpace(req.Endpoint),
		Path:     strings.TrimSpace(req.Path),
		Enabled:  true,
	}
	if req.Enabled != nil {
		row.Enabled = *req.Enabled
	}
	if req.Secret != nil {
		row.Secret = req.Secret.Encode()
	}
	if err := a.validateBackupDest(row); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.Store.DB.Create(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "backup.dest.create", row.Name)
	c.JSON(http.StatusCreated, destJSON(row))
}

func (a *App) handlePatchBackupDestination(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var row store.BackupDestination
	if err := a.Store.DB.First(&row, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "destino não encontrado"})
		return
	}
	var req upsertBackupDestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "corpo inválido"})
		return
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		row.Name = name
	}
	if req.Kind != "" {
		if !req.Kind.Valid() || req.Kind == store.BackupKindLocal {
			c.JSON(http.StatusBadRequest, gin.H{"error": "tipo inválido"})
			return
		}
		row.Kind = req.Kind
	}
	if req.Endpoint != "" || c.Request.ContentLength > 0 {
		row.Endpoint = strings.TrimSpace(req.Endpoint)
	}
	if req.Path != "" {
		row.Path = strings.TrimSpace(req.Path)
	}
	if req.Enabled != nil {
		row.Enabled = *req.Enabled
	}
	if req.Secret != nil {
		row.Secret = req.Secret.Encode()
	}
	if err := a.validateBackupDest(row); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := a.Store.DB.Save(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "backup.dest.patch", row.Name)
	c.JSON(http.StatusOK, destJSON(row))
}

func (a *App) handleDeleteBackupDestination(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var row store.BackupDestination
	if err := a.Store.DB.First(&row, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "destino não encontrado"})
		return
	}
	if err := a.Store.DB.Delete(&row).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	_ = a.Store.LogAudit(callerUsername(c), "backup.dest.delete", row.Name)
	c.Status(http.StatusNoContent)
}

func (a *App) handleListBackupJobs(c *gin.Context) {
	var rows []store.BackupJob
	q := a.Store.DB.Preload("Destination").Order("id desc").Limit(40)
	if err := q.Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	items := make([]backupJobJSON, 0, len(rows))
	for _, j := range rows {
		items = append(items, jobJSON(j))
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (a *App) handleRunBackup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id inválido"})
		return
	}
	var dest store.BackupDestination
	if err := a.Store.DB.First(&dest, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "destino não encontrado"})
		return
	}
	if !dest.Enabled {
		c.JSON(http.StatusConflict, gin.H{"error": "destino desabilitado"})
		return
	}
	var req runBackupRequest
	_ = c.ShouldBindJSON(&req)
	settings, err := a.loadBackupSettings()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	job := store.BackupJob{
		DestinationID: dest.ID,
		DryRun:        req.DryRun,
		Status:        store.BackupJobRunning,
	}
	now := time.Now()
	job.StartedAt = &now
	if err := a.Store.DB.Create(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "erro interno"})
		return
	}
	inc := backup.Include{
		Mongo:          settings.IncludeMongo,
		Marketplace:    settings.IncludeMarketplace,
		Git:            settings.IncludeGit,
		Social:         settings.IncludeSocial,
		MongoURI:       a.mongoURI(),
		MarketplaceDir: a.marketplaceDir(),
		GitDir:         a.gitDir(),
		SocialDir:      a.socialDir(),
	}
	runDest := backup.Dest{
		Kind:     string(dest.Kind),
		Endpoint: dest.Endpoint,
		Path:     dest.Path,
		Secret:   backup.ParseSecret(dest.Secret),
	}
	if dest.Kind == store.BackupKindXDriver {
		// [shared] é guest ok — nunca copiar mongodump (hashes, secrets).
		inc.Mongo = false
		inc.MongoURI = ""
		runDest.Path = a.xdriverBackupPath(dest)
	}
	res, runErr := a.backupEngine().Backup(c.Request.Context(), runDest, inc, a.backupStaging(), req.DryRun)
	done := time.Now()
	job.FinishedAt = &done
	job.SnapshotID = res.SnapshotID
	job.Bytes = res.Bytes
	if runErr != nil {
		job.Status = store.BackupJobError
		job.Error = runErr.Error()
	} else {
		job.Status = store.BackupJobOK
	}
	_ = a.Store.DB.Save(&job)
	job.Destination = &dest
	_ = a.Store.LogAudit(callerUsername(c), "backup.run", dest.Name)
	if runErr != nil {
		c.JSON(http.StatusBadGateway, jobJSON(job))
		return
	}
	c.JSON(http.StatusOK, jobJSON(job))
}

func (a *App) validateBackupDest(d store.BackupDestination) error {
	if d.Kind == store.BackupKindXDriver {
		if a.xdriverBackupPath(d) == "" {
			return errBackupXDriverPath
		}
	}
	return nil
}

var errBackupXDriverPath = errString("path XDRIVER inválido")

type errString string

func (e errString) Error() string { return string(e) }

func (a *App) xdriverBackupPath(d store.BackupDestination) string {
	root := ""
	if a.Config != nil {
		root = a.Config.DriverSharedDir
	}
	if root == "" {
		root = "/srv/xvpn/shared"
	}
	base := filepath.Join(root, "xvpn-backups")
	name := sanitizePathPart(d.Name)
	if d.Path != "" {
		name = sanitizePathPart(d.Path)
	}
	full := filepath.Join(base, name)
	cleanBase, err := filepath.Abs(base)
	if err != nil {
		return ""
	}
	cleanFull, err := filepath.Abs(full)
	if err != nil {
		return ""
	}
	if cleanFull != cleanBase && !strings.HasPrefix(cleanFull, cleanBase+string(os.PathSeparator)) {
		return ""
	}
	return cleanFull
}

func (a *App) mongoURI() string {
	if a.Config != nil {
		return a.Config.MongoURI
	}
	return os.Getenv("XVPN_MONGO_URI")
}

func (a *App) marketplaceDir() string {
	if a.Config != nil && a.Config.MarketplaceDataDir != "" {
		return a.Config.MarketplaceDataDir
	}
	return ""
}

func (a *App) socialDir() string {
	if a.Config != nil && a.Config.SocialMediaDir != "" {
		return a.Config.SocialMediaDir
	}
	return ""
}
