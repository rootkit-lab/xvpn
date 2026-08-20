package store

import (
	"path/filepath"
	"testing"
	"time"
)

// TestOpen_BackfillsRolesForPreExistingDatabase simula o boot de um banco
// criado antes da Fase 10 (RBAC): a tabela `users` já existe, mas sem a
// coluna `role`. Open() precisa detectar isso, migrar a coluna e promover
// quem já existia — o usuário mais antigo vira super_admin, os demais
// admin (ver store.go e PLAN.md §6.7) — sem isso, todo mundo acordaria
// "member" (o default da coluna) e ninguém conseguiria mais administrar o
// próprio painel.
func TestOpen_BackfillsRolesForPreExistingDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "pre-fase10.db")

	pre, err := Open(dbPath)
	if err != nil {
		t.Fatalf("erro na primeira abertura (simulando schema pré-Fase-10): %v", err)
	}
	// Recria a tabela sem a coluna role, para simular fielmente um banco
	// legado — Open() decide se faz backfill checando a existência dessa
	// coluna, então o teste perde o sentido se ela já estiver presente.
	if err := pre.DB.Exec("DROP TABLE users").Error; err != nil {
		t.Fatalf("erro derrubando tabela users: %v", err)
	}
	if err := pre.DB.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		created_at DATETIME,
		updated_at DATETIME
	)`).Error; err != nil {
		t.Fatalf("erro criando tabela legada: %v", err)
	}
	now := time.Now()
	if err := pre.DB.Exec("INSERT INTO users (username, password_hash, created_at, updated_at) VALUES (?, 'hash1', ?, ?)", "primeiro", now, now).Error; err != nil {
		t.Fatalf("erro inserindo primeiro usuário legado: %v", err)
	}
	if err := pre.DB.Exec("INSERT INTO users (username, password_hash, created_at, updated_at) VALUES (?, 'hash2', ?, ?)", "segundo", now.Add(time.Second), now.Add(time.Second)).Error; err != nil {
		t.Fatalf("erro inserindo segundo usuário legado: %v", err)
	}
	sqlDB, err := pre.DB.DB()
	if err != nil {
		t.Fatalf("erro obtendo *sql.DB: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("erro fechando conexão de setup: %v", err)
	}

	migrated, err := Open(dbPath)
	if err != nil {
		t.Fatalf("erro reabrindo banco para migrar (Fase 10): %v", err)
	}

	var users []User
	if err := migrated.DB.Order("id ASC").Find(&users).Error; err != nil {
		t.Fatalf("erro lendo usuários pós-migração: %v", err)
	}
	if len(users) != 2 {
		t.Fatalf("esperava 2 usuários, obtido %d", len(users))
	}
	if users[0].Username != "primeiro" || users[0].Role != RoleSuperAdmin {
		t.Errorf("esperava o usuário mais antigo (%q) como super_admin, obtido role=%q", users[0].Username, users[0].Role)
	}
	if users[1].Username != "segundo" || users[1].Role != RoleAdmin {
		t.Errorf("esperava o segundo usuário (%q) como admin, obtido role=%q", users[1].Username, users[1].Role)
	}
}

// TestOpen_DoesNotReRunBackfillOnAlreadyMigratedDatabase garante que um
// segundo boot (coluna `role` já existente) nunca reaplica o backfill —
// senão um admin que rebaixou deliberadamente alguém via painel veria o
// papel "voltar" sozinho a cada restart do servidor.
func TestOpen_DoesNotReRunBackfillOnAlreadyMigratedDatabase(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "ja-migrado.db")

	first, err := Open(dbPath)
	if err != nil {
		t.Fatalf("erro na primeira abertura: %v", err)
	}
	user := User{Username: "viewer-custom", PasswordHash: "hash", Role: RoleViewer}
	if err := first.DB.Create(&user).Error; err != nil {
		t.Fatalf("erro criando usuário de teste: %v", err)
	}
	sqlDB, err := first.DB.DB()
	if err != nil {
		t.Fatalf("erro obtendo *sql.DB: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("erro fechando conexão de setup: %v", err)
	}

	second, err := Open(dbPath)
	if err != nil {
		t.Fatalf("erro reabrindo banco já migrado: %v", err)
	}
	var reloaded User
	if err := second.DB.First(&reloaded, user.ID).Error; err != nil {
		t.Fatalf("erro relendo usuário: %v", err)
	}
	if reloaded.Role != RoleViewer {
		t.Fatalf("esperava que o papel customizado (viewer) sobrevivesse a um reboot, obtido %q", reloaded.Role)
	}
}

// TestOpen_AddsOrganizationIDToLegacyProjects reproduz o SQLite de
// produção pré-#167: projects já existe com linhas, sem organization_id.
// AutoMigrate sozinho faz ADD COLUMN NOT NULL sem default e aborta o boot.
func TestOpen_AddsOrganizationIDToLegacyProjects(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "pre-org.db")

	pre, err := Open(dbPath)
	if err != nil {
		t.Fatalf("erro na primeira abertura: %v", err)
	}
	now := time.Now()
	if err := pre.DB.Exec(`INSERT INTO projects (slug, name, social_group_id, files_enabled, visibility, network, created_at, updated_at)
		VALUES (?, ?, 1, 0, 'global', 'vpn', ?, ?)`, "hello-js", "hello-js", now, now).Error; err != nil {
		t.Fatalf("erro inserindo projeto legado: %v", err)
	}
	if err := pre.DB.Exec(`CREATE TABLE projects_legacy AS SELECT id, slug, name, description, app_id, social_group_id, files_enabled, visibility, network, runners, archived_at, created_at, updated_at FROM projects`).Error; err != nil {
		t.Fatalf("erro clonando projects sem org: %v", err)
	}
	if err := pre.DB.Exec("DROP TABLE projects").Error; err != nil {
		t.Fatalf("erro derrubando projects: %v", err)
	}
	if err := pre.DB.Exec("ALTER TABLE projects_legacy RENAME TO projects").Error; err != nil {
		t.Fatalf("erro renomeando projects legado: %v", err)
	}
	sqlDB, err := pre.DB.DB()
	if err != nil {
		t.Fatalf("erro obtendo *sql.DB: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("erro fechando conexão de setup: %v", err)
	}

	migrated, err := Open(dbPath)
	if err != nil {
		t.Fatalf("erro reabrindo banco legado (organization_id): %v", err)
	}
	if !migrated.DB.Migrator().HasColumn(&Project{}, "organization_id") {
		t.Fatal("esperava coluna organization_id após Open()")
	}
	var row Project
	if err := migrated.DB.Where("slug = ?", "hello-js").First(&row).Error; err != nil {
		t.Fatalf("erro lendo projeto legado: %v", err)
	}
	if row.OrganizationID != 0 {
		t.Fatalf("esperava organization_id=0 no legado (cutover no seed), obtido %d", row.OrganizationID)
	}
}
