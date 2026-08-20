package store

import "time"

// ForgePackageKind é o formato do artefato no registry do XGIT (Fase 45.1).
type ForgePackageKind string

const (
	ForgePackageKindGeneric ForgePackageKind = "generic"
	ForgePackageKindNPM     ForgePackageKind = "npm"
)

func (k ForgePackageKind) Valid() bool {
	return k == ForgePackageKindGeneric || k == ForgePackageKindNPM
}

// ForgePackage é um nome publicado num projeto (npm ou genérico).
type ForgePackage struct {
	ID        uint             `gorm:"primaryKey"`
	ProjectID uint             `gorm:"uniqueIndex:idx_forge_pkg;not null"`
	Kind      ForgePackageKind `gorm:"uniqueIndex:idx_forge_pkg;not null"`
	Name      string           `gorm:"uniqueIndex:idx_forge_pkg;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time

	Project Project `gorm:"foreignKey:ProjectID"`
}

// ForgePackageVersion é uma versão imutável (blob SHA-256).
type ForgePackageVersion struct {
	ID            uint   `gorm:"primaryKey"`
	PackageID     uint   `gorm:"uniqueIndex:idx_forge_pkg_ver;not null"`
	Version       string `gorm:"uniqueIndex:idx_forge_pkg_ver;not null"`
	Filename      string `gorm:"not null"`
	SHA256        string `gorm:"not null"`
	Integrity     string
	Shasum        string
	Size          int64 `gorm:"not null"`
	StoragePath   string
	Description   string
	PublishedByID uint
	DownloadCount int64 `gorm:"not null;default:0"`
	CreatedAt     time.Time

	Package     ForgePackage `gorm:"foreignKey:PackageID"`
	PublishedBy User         `gorm:"foreignKey:PublishedByID"`
}
