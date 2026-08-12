// Package store contém os modelos GORM e a inicialização do banco SQLite.
package store

import "time"

// User é um administrador do painel web. Não confundir com "peer"/"device":
// um usuário pode ter múltiplos dispositivos (Fase 3 permitirá isso), mas na
// Fase 2 o fluxo de convite já suporta N dispositivos por usuário.
type User struct {
	ID           uint   `gorm:"primaryKey"`
	Username     string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time

	Devices      []Device      `gorm:"foreignKey:UserID"`
	InviteTokens []InviteToken `gorm:"foreignKey:UserID"`
}

// InviteToken é um código de curta duração gerado pelo painel para permitir
// que um dispositivo se registre (enrollment) sem precisar da senha do
// admin. Uso único: UsedAt é preenchido assim que consumido.
type InviteToken struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    uint   `gorm:"not null;index"`
	Token     string `gorm:"uniqueIndex;not null"`
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time

	User User `gorm:"foreignKey:UserID"`
}

// Device é um peer WireGuard registrado (o que `wg show` chama de "peer").
// A chave privada correspondente NUNCA é conhecida pelo servidor — só a
// pública, recebida no enrollment (ver AGENTS.md, invariante de segurança).
type Device struct {
	ID        uint   `gorm:"primaryKey"`
	UserID    uint   `gorm:"not null;index"`
	Name      string `gorm:"not null"`
	PublicKey string `gorm:"uniqueIndex;not null"`
	// AllowedIP é o /32 alocado a este peer dentro da sub-rede WireGuard,
	// ex.: "10.66.66.5/32". Único por design (uma IP, um device).
	AllowedIP string `gorm:"uniqueIndex;not null"`
	CreatedAt time.Time

	User User `gorm:"foreignKey:UserID"`
}

// AuditLog registra ações administrativas relevantes (quem fez o quê,
// quando). Nunca deve conter segredos — ver go-backend.mdc.
type AuditLog struct {
	ID        uint   `gorm:"primaryKey"`
	Actor     string `gorm:"not null"`
	Action    string `gorm:"not null"`
	Detail    string
	CreatedAt time.Time
}
