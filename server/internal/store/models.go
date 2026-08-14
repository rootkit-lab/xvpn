// Package store contém os modelos GORM e a inicialização do banco SQLite.
package store

import "time"

// Role é o papel de um User no painel (Fase 10 — ver PLAN.md §6.7). Controla
// tanto o acesso ao painel administrativo quanto o que a VPN/marketplace
// permitem para aquele usuário.
type Role string

const (
	RoleSuperAdmin Role = "super_admin"
	RoleAdmin      Role = "admin"
	RoleViewer     Role = "viewer"
	RoleMember     Role = "member"
)

// roleRank ordena papéis para checagens de "quem pode gerenciar quem"
// (ex.: um admin não deve conseguir editar/apagar um super_admin). Não
// representa uma escala de "confiança" absoluta, só a relação de gestão
// dentro do painel.
var roleRank = map[Role]int{
	RoleSuperAdmin: 3,
	RoleAdmin:      2,
	RoleViewer:     1,
	RoleMember:     0,
}

// Valid reporta se r é um dos quatro papéis reconhecidos.
func (r Role) Valid() bool {
	_, ok := roleRank[r]
	return ok
}

// Rank retorna a posição de r na hierarquia de gestão (maior = mais
// privilegiado). Papéis desconhecidos rankeiam abaixo de tudo (-1), nunca
// acima — never o contrário, para nunca conceder gestão por engano.
func (r Role) Rank() int {
	if rank, ok := roleRank[r]; ok {
		return rank
	}
	return -1
}

// CanManage reporta se um ator com o papel r pode gerenciar (editar papel,
// resetar senha, excluir) um alvo com o papel target — ver PLAN.md §6.7.
// Um papel só gerencia papéis no mesmo nível ou abaixo; nunca acima.
func (r Role) CanManage(target Role) bool {
	return target.Rank() <= r.Rank()
}

// AdminRoles são os papéis com acesso de escrita ao painel administrativo
// (users/devices/waitlist/marketplace) — ver tabela em PLAN.md §6.7.
var AdminRoles = []Role{RoleSuperAdmin, RoleAdmin}

// ViewerUpRoles são os papéis com acesso de leitura ao painel administrativo
// (inclui os de escrita, que também podem ler).
var ViewerUpRoles = []Role{RoleSuperAdmin, RoleAdmin, RoleViewer}

// User é uma conta do painel web. Não confundir com "peer"/"device": um
// usuário pode ter múltiplos dispositivos, mas na Fase 2 o fluxo de convite
// já suporta N dispositivos por usuário. Role (Fase 10) define o que essa
// conta pode fazer no painel — ver PLAN.md §6.7.
type User struct {
	ID           uint   `gorm:"primaryKey"`
	Username     string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null"`
	// Role nunca fica vazio em produção: AutoMigrate cria a coluna com
	// default "member" (mais restritivo) e Open() faz backfill explícito
	// logo em seguida para as linhas pré-existentes — ver store.go.
	Role      Role `gorm:"not null;default:member"`
	CreatedAt time.Time
	UpdatedAt time.Time

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

// WaitlistEntry é um cadastro de interesse feito na landing pública ("/",
// sem autenticação) — ver PLAN.md pela decisão de design. O status
// ("approved"/"rejected") só sinaliza triagem; o provisionamento de acesso
// real (criar o User + InviteToken) é um passo explícito à parte — via
// handleProvisionWaitlist (orquestra os dois numa chamada) ou manualmente
// pela tela Usuários. Não existe um segundo caminho de credencial: os dois
// fluxos acabam criando exatamente um User + InviteToken.
type WaitlistEntry struct {
	ID      uint   `gorm:"primaryKey"`
	Name    string `gorm:"not null"`
	Email   string `gorm:"index;not null"`
	Message string
	// Status: "pending" (padrão) | "approved" | "rejected".
	Status     string `gorm:"not null;default:pending"`
	CreatedAt  time.Time
	ReviewedAt *time.Time
}
