package store

import (
	"fmt"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Store encapsula a conexão GORM e expõe operações de alto nível usadas
// pelos handlers HTTP, para não misturar SQL/GORM diretamente com lógica de
// negócio (ver go-backend.mdc).
type Store struct {
	DB *gorm.DB
}

// Open conecta ao SQLite em path e roda as migrações automáticas.
func Open(path string) (*Store, error) {
	db, err := gorm.Open(sqlite.Open(path), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, fmt.Errorf("abrindo banco %q: %w", path, err)
	}

	// Fase 10 (RBAC): detecta *antes* do AutoMigrate se a coluna `role`
	// ainda não existe, para saber se este boot é a migração inicial de um
	// banco pré-Fase-10 — nesse caso é preciso promover quem já existia
	// (ver backfillInitialRoles). Numa instalação nova ou já migrada, a
	// coluna já existe e o backfill vira no-op (sem usuários para migrar).
	needsRoleBackfill := !db.Migrator().HasColumn(&User{}, "role")

	if err := db.AutoMigrate(
		&User{}, &Device{}, &InviteToken{}, &AuditLog{}, &WaitlistEntry{},
		&App{}, &AppVersion{}, &AppAsset{}, &AppAccess{},
		&PanelSettings{},
		&SocialProfile{}, &Follow{}, &SocialGroup{}, &SocialGroupMember{},
		&DirectThread{}, &DirectThreadMember{}, &Message{},
	); err != nil {
		return nil, fmt.Errorf("migrando schema: %w", err)
	}

	if needsRoleBackfill {
		if err := backfillInitialRoles(db); err != nil {
			return nil, fmt.Errorf("migrando papéis (Fase 10): %w", err)
		}
	}

	return &Store{DB: db}, nil
}

// backfillInitialRoles roda uma única vez, no boot em que a coluna `role` é
// adicionada a um banco pré-Fase-10. O AutoMigrate já preencheu toda linha
// existente com o default da coluna ("member", o mais restritivo); aqui
// promovemos quem já existia antes do RBAC: o usuário mais antigo (sempre o
// bootstrap original, criado quando a tabela estava vazia) vira
// super_admin, os demais viram admin — ver ROADMAP.md Fase 10 e
// PLAN.md §6.7. Em uma instalação nova a tabela está vazia neste ponto
// (bootstrapAdmin roda depois de Open), então o loop não faz nada.
func backfillInitialRoles(db *gorm.DB) error {
	var users []User
	if err := db.Order("id ASC").Find(&users).Error; err != nil {
		return err
	}
	for i, u := range users {
		role := RoleAdmin
		if i == 0 {
			role = RoleSuperAdmin
		}
		if err := db.Model(&User{}).Where("id = ?", u.ID).Update("role", role).Error; err != nil {
			return fmt.Errorf("promovendo usuário %q (id=%d) para %q: %w", u.Username, u.ID, role, err)
		}
	}
	return nil
}

// LogAudit grava uma entrada de auditoria. Erros de auditoria são logados
// pelo chamador, mas nunca devem interromper a operação principal.
func (s *Store) LogAudit(actor, action, detail string) error {
	return s.DB.Create(&AuditLog{
		Actor:     actor,
		Action:    action,
		Detail:    detail,
		CreatedAt: time.Now(),
	}).Error
}

// CountUsers retorna quantos usuários existem — usado no bootstrap do
// primeiro admin.
func (s *Store) CountUsers() (int64, error) {
	var count int64
	err := s.DB.Model(&User{}).Count(&count).Error
	return count, err
}
