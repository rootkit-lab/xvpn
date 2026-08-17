package store

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

// CodeSpace é um worktree do forge (Fase 49). Não é VM nem shell.
type CodeSpace struct {
	ID        uint   `gorm:"primaryKey"`
	PublicID  string `gorm:"uniqueIndex;not null"`
	UserID    uint   `gorm:"index;not null"`
	ProjectID uint   `gorm:"index;not null"`
	Branch    string `gorm:"not null"`
	RelPath   string `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time

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
		Description: "IDE Monaco na intranet — xcodespaces.corp.ihuull.com. Worktree do forge; sem VM/shell.",
		Visibility:  AppVisibilityRestricted,
		Network:     AppNetworkVPN,
		Kind:        AppKindWeb,
		Source:      AppSourceBuild,
		SourcePath:  "xcodespaces",
	}).Error
}
