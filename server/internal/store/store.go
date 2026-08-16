package store

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Store encapsula a conexão GORM (SQLite ou cache em memória quando o
// Mongo é a fonte da verdade — Fase 28) e expõe operações de alto nível.
type Store struct {
	DB    *gorm.DB
	mongo *mongoSync
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
