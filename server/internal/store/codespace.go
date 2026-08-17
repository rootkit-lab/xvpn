package store

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

const (
	CodespaceKindQuick  = "quick"
	CodespaceKindRemote = "remote"
	CodespaceStopped    = "stopped"
	CodespaceStarting   = "starting"
	CodespaceRunning    = "running"
	CodespaceError      = "error"
)

// CodeSpace é um workspace do XCODESPACES.
// Kind=quick: worktree + Monaco (Fase 49). Kind=remote: clone + container (Fase 50).
type CodeSpace struct {
	ID           uint   `gorm:"primaryKey"`
	PublicID     string `gorm:"uniqueIndex;not null"`
	UserID       uint   `gorm:"index;not null"`
	ProjectID    uint   `gorm:"index;not null"`
	Branch       string `gorm:"not null"`
	RelPath      string `gorm:"not null"`
	Kind         string `gorm:"not null;default:quick"`
	Status       string `gorm:"not null;default:stopped"`
	HostPort     int
	Image        string
	GitTokenHash string
	LastActiveAt *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time

	User    User    `gorm:"foreignKey:UserID"`
	Project Project `gorm:"foreignKey:ProjectID"`
}

// SeedXcodespacesApp garante o app de sistema no catálogo.
func SeedXcodespacesApp(db *gorm.DB) error {
	var app App
	err := db.Where("slug = ?", "xcodespaces").First(&app).Error
	if err == nil {
		if app.ArchivedAt == nil {
			return nil
		}
		app.ArchivedAt = nil
		return db.Save(&app).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return db.Create(&App{
		Slug:        "xcodespaces",
		Name:        "XCODESPACES IDE",
		Description: "IDE na intranet — editor rápido (Monaco) e codespace remoto (VS Code + Docker).",
		Visibility:  AppVisibilityRestricted,
		Network:     AppNetworkVPN,
		Kind:        AppKindWeb,
		Source:      AppSourceBuild,
		SourcePath:  "xcodespaces",
	}).Error
}
