package store

import "time"

// ProjectRole é o papel de um membro no forge (PLAN.md §6.15).
type ProjectRole string

const (
	ProjectRoleGuest      ProjectRole = "guest"
	ProjectRoleReporter   ProjectRole = "reporter"
	ProjectRoleDeveloper  ProjectRole = "developer"
	ProjectRoleMaintainer ProjectRole = "maintainer"
	ProjectRoleOwner      ProjectRole = "owner"
)

var projectRoleRank = map[ProjectRole]int{
	ProjectRoleGuest:      0,
	ProjectRoleReporter:   1,
	ProjectRoleDeveloper:  2,
	ProjectRoleMaintainer: 3,
	ProjectRoleOwner:      4,
}

func (r ProjectRole) Valid() bool {
	_, ok := projectRoleRank[r]
	return ok
}

func (r ProjectRole) Rank() int {
	if n, ok := projectRoleRank[r]; ok {
		return n
	}
	return -1
}

// Project é um slug do forge (Fase 37). Pode espelhar um App do
// marketplace ou existir só como metadado. Issues/activity vivem no
// SocialGroup apontado por SocialGroupID — sem segundo social.
type Project struct {
	ID            uint   `gorm:"primaryKey"`
	Slug          string `gorm:"uniqueIndex;not null"`
	Name          string `gorm:"not null"`
	Description   string
	AppID         *uint
	SocialGroupID uint          `gorm:"not null;index"`
	FilesEnabled  bool          `gorm:"not null;default:false"`
	Visibility    AppVisibility `gorm:"not null;default:global"`
	Network       AppNetwork    `gorm:"not null;default:vpn"`
	Runners       []string      `gorm:"serializer:json"`
	ArchivedAt    *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time

	App   *App        `gorm:"foreignKey:AppID"`
	Group SocialGroup `gorm:"foreignKey:SocialGroupID"`
}

// ProjectMember liga um User a um Project com um papel do forge.
type ProjectMember struct {
	ID        uint        `gorm:"primaryKey"`
	ProjectID uint        `gorm:"uniqueIndex:idx_project_member;not null"`
	UserID    uint        `gorm:"uniqueIndex:idx_project_member;not null"`
	Role      ProjectRole `gorm:"not null;default:guest"`
	CreatedAt time.Time

	User User `gorm:"foreignKey:UserID"`
}

// ValidProjectSlug aceita [a-z0-9][a-z0-9-]{0,18}[a-z0-9] (2–20), sem
// hífen nas pontas — a mesma chave de App.Slug no catálogo.
func ValidProjectSlug(s string) bool {
	if len(s) < 2 || len(s) > 20 {
		return false
	}
	if s[0] == '-' || s[len(s)-1] == '-' {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			continue
		}
		return false
	}
	return true
}
