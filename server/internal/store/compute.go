package store

import (
	"strings"
	"time"
)

const (
	ServerRoleControl  = "control"
	ServerRoleMesh     = "mesh"
	ServerRoleRunner   = "runner"
	ServerRoleExternal = "external"
)

// Hosts BitLaunch com app própria — inventário só (Fase 38.2).
const (
	ExternalHostCriptoProd = "server-cripto-prod"
	ExternalIPv4Cripto     = "65.38.120.203"
)

func IsExternalHost(name, hostname, ipv4 string) bool {
	if strings.TrimSpace(ipv4) == ExternalIPv4Cripto {
		return true
	}
	blob := strings.ToLower(name + " " + hostname)
	return strings.Contains(blob, ExternalHostCriptoProd)
}

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
	AccountID       *uint
	Notes           string `gorm:"type:text"`
	EnrollToken     string
	EnrollExpiresAt *time.Time
	RunnerTokenHash string `json:"-"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// BitLaunchAccount é uma API/e-mail BitLaunch (Fase 38.1).
// O token fica só no banco do VPS — nunca no Git nem nas listagens.
type BitLaunchAccount struct {
	ID        uint   `gorm:"primaryKey"`
	Name      string `gorm:"not null"`
	Email     string `gorm:"uniqueIndex;not null"`
	Token     string `gorm:"not null" json:"-"`
	CreatedAt time.Time
	UpdatedAt time.Time
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
