package store

import (
	"time"

	"gorm.io/gorm"
)

// DefaultOrgSlug é a organização seed do XGIT (PLAN.md §6.15). Não é
// produto e não existe hostname xcorp.corp.
const DefaultOrgSlug = "xcorp"

// OrgRole é o papel de um membro na organização (não no repo).
type OrgRole string

const (
	OrgRoleOwner  OrgRole = "owner"
	OrgRoleAdmin  OrgRole = "admin"
	OrgRoleMember OrgRole = "member"
)

func (r OrgRole) Valid() bool {
	switch r {
	case OrgRoleOwner, OrgRoleAdmin, OrgRoleMember:
		return true
	}
	return false
}

// ForgeOrganization é o owner GitHub-like do path <org>/<slug>.
// Não é SocialGroup (XGROUP) nem Project (repo).
type ForgeOrganization struct {
	ID          uint   `gorm:"primaryKey"`
	Slug        string `gorm:"uniqueIndex;not null"`
	Name        string `gorm:"not null"`
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (ForgeOrganization) TableName() string { return "forge_organizations" }

// OrgMember liga um User a uma ForgeOrganization.
type OrgMember struct {
	ID             uint    `gorm:"primaryKey"`
	OrganizationID uint    `gorm:"uniqueIndex:idx_org_member;not null"`
	UserID         uint    `gorm:"uniqueIndex:idx_org_member;not null"`
	Role           OrgRole `gorm:"not null;default:member"`
	CreatedAt      time.Time

	User User `gorm:"foreignKey:UserID"`
}

func (OrgMember) TableName() string { return "org_members" }

// OrgTeam agrupa repos dentro da org (exemplos → packages / workflows).
type OrgTeam struct {
	ID             uint   `gorm:"primaryKey"`
	OrganizationID uint   `gorm:"uniqueIndex:idx_org_team;not null"`
	Slug           string `gorm:"uniqueIndex:idx_org_team;not null"`
	Name           string `gorm:"not null"`
	ParentID       *uint
	CreatedAt      time.Time
}

func (OrgTeam) TableName() string { return "org_teams" }

// OrgTeamMember é a inscrição no time.
type OrgTeamMember struct {
	ID        uint `gorm:"primaryKey"`
	TeamID    uint `gorm:"uniqueIndex:idx_org_team_member;not null"`
	UserID    uint `gorm:"uniqueIndex:idx_org_team_member;not null"`
	CreatedAt time.Time
}

func (OrgTeamMember) TableName() string { return "org_team_members" }

// ValidOrgSlug é a mesma chave de ValidProjectSlug (2–20).
func ValidOrgSlug(s string) bool {
	return ValidProjectSlug(s)
}

// ReservedOrgSlug são rotas da home em xgit.corp (não podem ser org).
func ReservedOrgSlug(s string) bool {
	switch s {
	case "repositories", "packages", "stars", "overview", "settings", "login",
		"admin", "issues", "pulls", "edit", "projects", "milestones", "labels",
		"xgit", "new", "orgs", "people", "teams", "security", "wiki", "agents",
		"actions", "marketplace":
		return true
	default:
		return false
	}
}

// migrateLegacyProjectOrgColumns adiciona organization_id com DEFAULT 0
// antes do AutoMigrate. SQLite recusa ADD COLUMN NOT NULL sem default em
// tabela já populada — o crash do deploy pós-#167.
func migrateLegacyProjectOrgColumns(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&Project{}) {
		return nil
	}
	if !db.Migrator().HasColumn(&Project{}, "organization_id") {
		if err := db.Exec("ALTER TABLE projects ADD COLUMN organization_id integer NOT NULL DEFAULT 0").Error; err != nil {
			return err
		}
	}
	if !db.Migrator().HasColumn(&Project{}, "team_id") {
		if err := db.Exec("ALTER TABLE projects ADD COLUMN team_id integer").Error; err != nil {
			return err
		}
	}
	return nil
}

// migrateDevicesNetworkID adiciona network_id com DEFAULT 0 antes do
// AutoMigrate. SQLite recusa ADD COLUMN NOT NULL sem default em tabela
// já populada — crash do deploy pós-#179.
func migrateDevicesNetworkID(db *gorm.DB) error {
	if db == nil || !db.Migrator().HasTable(&Device{}) {
		return nil
	}
	if db.Migrator().HasColumn(&Device{}, "network_id") {
		return nil
	}
	return db.Exec("ALTER TABLE devices ADD COLUMN network_id integer NOT NULL DEFAULT 0").Error
}

// SeedXcorp cria a org principal, times e o owner (primeiro super_admin)
// ou admin). Não inscreve o resto da VPN — herança de repo é
// ProjectMember / time (Fase 58.2), não OrgMember em massa.
func SeedXcorp(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	org := ForgeOrganization{
		Slug:        DefaultOrgSlug,
		Name:        "xcorp",
		Description: "Organização principal da intranet ihuull. Não é produto.",
	}
	if err := db.Where("slug = ?", DefaultOrgSlug).FirstOrCreate(&org).Error; err != nil {
		return err
	}
	exemplos := OrgTeam{OrganizationID: org.ID, Slug: "exemplos", Name: "exemplos"}
	if err := db.Where("organization_id = ? AND slug = ?", org.ID, "exemplos").FirstOrCreate(&exemplos).Error; err != nil {
		return err
	}
	packages := OrgTeam{OrganizationID: org.ID, Slug: "packages", Name: "packages", ParentID: &exemplos.ID}
	if err := db.Where("organization_id = ? AND slug = ?", org.ID, "packages").FirstOrCreate(&packages).Error; err != nil {
		return err
	}
	workflows := OrgTeam{OrganizationID: org.ID, Slug: "workflows", Name: "workflows", ParentID: &exemplos.ID}
	if err := db.Where("organization_id = ? AND slug = ?", org.ID, "workflows").FirstOrCreate(&workflows).Error; err != nil {
		return err
	}

	var owner User
	err := db.Where("role = ?", RoleSuperAdmin).Order("id").First(&owner).Error
	if err != nil {
		err = db.Where("role = ?", RoleAdmin).Order("id").First(&owner).Error
	}
	if err != nil {
		return nil
	}
	row := OrgMember{OrganizationID: org.ID, UserID: owner.ID, Role: OrgRoleOwner}
	if err := db.Where("organization_id = ? AND user_id = ?", org.ID, owner.ID).FirstOrCreate(&row).Error; err != nil {
		return err
	}
	tm := OrgTeamMember{TeamID: exemplos.ID, UserID: owner.ID}
	return db.Where("team_id = ? AND user_id = ?", exemplos.ID, owner.ID).FirstOrCreate(&tm).Error
}
