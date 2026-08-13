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

	if err := db.AutoMigrate(&User{}, &Device{}, &InviteToken{}, &AuditLog{}, &WaitlistEntry{}); err != nil {
		return nil, fmt.Errorf("migrando schema: %w", err)
	}

	return &Store{DB: db}, nil
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
