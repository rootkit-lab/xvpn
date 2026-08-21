package store

import "time"

const (
	SecKindDeps   = "deps"
	SecKindCode   = "code"
	SecKindSecret = "secret"

	SecStatusOpen      = "open"
	SecStatusDismissed = "dismissed"
)

// SecAlert é um finding de Security and quality (Fase 62). Sem SaaS.
type SecAlert struct {
	ID        uint   `gorm:"primaryKey"`
	ProjectID uint   `gorm:"index;not null"`
	Kind      string `gorm:"not null;index"`
	Severity  string `gorm:"not null"`
	Title     string `gorm:"not null"`
	Tool      string `gorm:"not null"`
	Status    string `gorm:"not null;default:open;index"`
	Raw       string `gorm:"type:text"`
	JobNumber *uint
	CreatedAt time.Time
	UpdatedAt time.Time
}
