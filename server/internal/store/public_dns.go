package store

import "time"

// CloudflareAccount é uma API Cloudflare (Fase 39). Token só no VPS.
type CloudflareAccount struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"not null"`
	Email     string `gorm:"uniqueIndex;not null"`
	Token     string `gorm:"not null" json:"-"`
	AccountCF string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PublicZone é um domínio do stack (autoridade pública = Cloudflare).
type PublicZone struct {
	ID           uint     `gorm:"primaryKey"`
	AccountID    uint     `gorm:"not null;index"`
	Name         string   `gorm:"uniqueIndex;not null"`
	CloudflareID string   `gorm:"index"`
	NameServers  []string `gorm:"serializer:json"`
	Status       string   `gorm:"not null;default:pending"`
	Intranet     bool     `gorm:"not null;default:true"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// PublicRecord espelha um RR público e, opcionalmente, a visão interna.
type PublicRecord struct {
	ID           uint   `gorm:"primaryKey"`
	ZoneID       uint   `gorm:"not null;uniqueIndex:idx_public_rr"`
	CloudflareID string `gorm:"index"`
	Type         string `gorm:"not null;uniqueIndex:idx_public_rr"`
	Name         string `gorm:"not null;uniqueIndex:idx_public_rr"`
	Content      string `gorm:"not null"`
	TTL          int    `gorm:"not null;default:1"`
	Proxied      bool   `gorm:"not null;default:false"`
	IntranetIPv4 string
	Comment      string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
