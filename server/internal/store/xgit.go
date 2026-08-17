package store

import (
	"errors"

	"gorm.io/gorm"
)

// SeedXgitApp garante o app de sistema no catálogo (ACL em Marketplace).
// Restricted + vpn: o waffle só mostra XGIT com AppAccess ou ProjectMember.
func SeedXgitApp(db *gorm.DB) error {
	var app App
	err := db.Where("slug = ?", "xgit").First(&app).Error
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
		Slug:        "xgit",
		Name:        "XGIT Forge",
		Description: "Git da intranet ihuull — xgit.corp.ihuull.com. ACL no catálogo; repos por ProjectMember.",
		Visibility:  AppVisibilityRestricted,
		Network:     AppNetworkVPN,
		Kind:        AppKindWeb,
		Source:      AppSourceBuild,
		SourcePath:  "xgit",
	}).Error
}
