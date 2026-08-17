package store

import "time"

const (
	ServerRoleControl = "control"
	ServerRoleMesh    = "mesh"
	ServerRoleRunner  = "runner"
)

// MeshServer é um VPS da malha (Fase 38 — PLAN.md §6.16).
// A chave privada WireGuard nunca fica aqui — só o peer (Device) após enroll.
type MeshServer struct {
	ID              uint   `gorm:"primaryKey"`
	BitLaunchID     string `gorm:"uniqueIndex"`
	Name            string `gorm:"not null"`
	Hostname        string `gorm:"uniqueIndex;not null"`
	Role            string `gorm:"not null;default:mesh"`
	IPv4            string
	WgIP            string
	Region          string
	Size            string
	Image           string
	Status          string   `gorm:"not null;default:unknown"`
	Labels          []string `gorm:"serializer:json"`
	GroupID         *uint
	CreatedByUserID uint
	DeviceID        *uint
	EnrollToken     string
	EnrollExpiresAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ServerGroup agrupa MeshServers para ACL (ServerAccess).
type ServerGroup struct {
	ID          uint   `gorm:"primaryKey"`
	Name        string `gorm:"uniqueIndex;not null"`
	Description string
	CreatedAt   time.Time
}

// ServerAccess liga um User a um servidor ou grupo.
type ServerAccess struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    uint   `gorm:"uniqueIndex:idx_server_access;not null"`
	ServerID  *uint  `gorm:"uniqueIndex:idx_server_access"`
	GroupID   *uint  `gorm:"uniqueIndex:idx_server_access"`
	Role      string `gorm:"not null;default:viewer"`
	CreatedAt time.Time
}
