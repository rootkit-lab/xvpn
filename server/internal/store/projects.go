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

// ProtectedBranch impede push direto (Fase 40). Merge via MR é Fase 41.
type ProtectedBranch struct {
	ID          uint        `gorm:"primaryKey"`
	ProjectID   uint        `gorm:"uniqueIndex:idx_project_protected;not null"`
	Pattern     string      `gorm:"uniqueIndex:idx_project_protected;not null"`
	MinPushRole ProjectRole `gorm:"not null;default:maintainer"`
	CreatedAt   time.Time
}

// DefaultProtectedBranches são as regras criadas com o projeto.
var DefaultProtectedBranches = []ProtectedBranch{
	{Pattern: "main", MinPushRole: ProjectRoleMaintainer},
	{Pattern: "master", MinPushRole: ProjectRoleMaintainer},
}

// MergeRequestStatus é o ciclo de vida do MR (Fase 41).
type MergeRequestStatus string

const (
	MROpen   MergeRequestStatus = "open"
	MRMerged MergeRequestStatus = "merged"
	MRClosed MergeRequestStatus = "closed"
)

func (s MergeRequestStatus) Valid() bool {
	return s == MROpen || s == MRMerged || s == MRClosed
}

// MergeRequest é o caminho de merge em branch protegida (PLAN.md §6.15).
// ThreadID é um DirectThread Kind=mr; SocialPostID é a issue no XGROUP.
type MergeRequest struct {
	ID           uint               `gorm:"primaryKey"`
	ProjectID    uint               `gorm:"uniqueIndex:idx_project_mr;not null"`
	Number       uint               `gorm:"uniqueIndex:idx_project_mr;not null"`
	Title        string             `gorm:"not null"`
	Description  string             `gorm:"type:text"`
	SourceBranch string             `gorm:"not null"`
	TargetBranch string             `gorm:"not null"`
	AuthorID     uint               `gorm:"not null;index"`
	Status       MergeRequestStatus `gorm:"not null;default:open;index"`
	ThreadID     uint               `gorm:"not null;index"`
	SocialPostID *uint
	MergedAt     *time.Time
	MergedByID   *uint
	CreatedAt    time.Time
	UpdatedAt    time.Time

	Author User `gorm:"foreignKey:AuthorID"`
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
