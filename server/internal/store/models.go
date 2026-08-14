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

// Platform identifica o sistema operacional de destino de um AppAsset do
// marketplace (Fase 11 — ver PLAN.md §6.8). Linux/Windows/Android como
// plataformas de asset, não como lojas oficiais integradas.
type Platform string

const (
	PlatformLinux   Platform = "linux"
	PlatformWindows Platform = "windows"
	PlatformAndroid Platform = "android"
)

// Valid reporta se p é uma das três plataformas suportadas pelo
// marketplace.
func (p Platform) Valid() bool {
	switch p {
	case PlatformLinux, PlatformWindows, PlatformAndroid:
		return true
	default:
		return false
	}
}

// AppVisibility controla quem enxerga um App no catálogo do marketplace —
// ver PLAN.md §6.8 ("ACL: app global vs. lista de user IDs").
type AppVisibility string

const (
	// AppVisibilityGlobal libera o app para qualquer usuário autenticado
	// com papel member ou acima (ou seja, todos — member é o piso).
	AppVisibilityGlobal AppVisibility = "global"
	// AppVisibilityRestricted só libera o app para usuários com uma
	// linha correspondente em AppAccess. admin/super_admin sempre
	// enxergam e baixam mesmo sem AppAccess (ver PLAN.md §6.7, coluna
	// Marketplace: "Admin + download").
	AppVisibilityRestricted AppVisibility = "restricted"
)

// Valid reporta se v é um dos dois modos de visibilidade reconhecidos.
func (v AppVisibility) Valid() bool {
	return v == AppVisibilityGlobal || v == AppVisibilityRestricted
}

const (
	ChannelStable = "stable"
	ChannelBeta   = "beta"
)

// ValidChannel reporta se c é um canal de distribuição reconhecido — ver
// PLAN.md §6.8.
func ValidChannel(c string) bool {
	return c == ChannelStable || c == ChannelBeta
}

// App é um programa distribuído pelo catálogo interno do marketplace (Fase
// 11 — ver PLAN.md §6.8). Não confundir com o próprio cliente XVPN (esse
// continua distribuído via GitHub Releases, ver /download): App é sempre
// "outro" software (Linux/Windows/Android) que o admin decide disponibilizar
// aos usuários da VPN.
type App struct {
	ID          uint   `gorm:"primaryKey"`
	Name        string `gorm:"not null"`
	Description string
	IconURL     string
	// Visibility nunca fica vazio: AutoMigrate cria a coluna com default
	// "global" (mais permissivo dentro do que já é uma rede privada
	// autenticada) — admin ajusta para "restricted" explicitamente.
	Visibility AppVisibility `gorm:"not null;default:global"`
	CreatedAt  time.Time
	UpdatedAt  time.Time

	Versions []AppVersion `gorm:"foreignKey:AppID"`
	Access   []AppAccess  `gorm:"foreignKey:AppID"`
}

// AppVersion é uma versão publicada de um App — pode ter múltiplos
// AppAsset (um por plataforma/arquitetura). Ver PLAN.md §6.8.
type AppVersion struct {
	ID        uint   `gorm:"primaryKey"`
	AppID     uint   `gorm:"not null;index"`
	Version   string `gorm:"not null"`
	Channel   string `gorm:"not null;default:stable"`
	Changelog string
	CreatedAt time.Time

	App    App        `gorm:"foreignKey:AppID"`
	Assets []AppAsset `gorm:"foreignKey:AppVersionID"`
}

// AppAsset é um arquivo binário concreto (um .deb, um .exe, um .apk...)
// associado a uma AppVersion. O conteúdo real fica em disco, endereçado por
// conteúdo (ver internal/marketplace/storage.go) — StoragePath é sempre
// derivado pelo próprio servidor a partir do SHA-256 calculado no upload,
// nunca um caminho vindo do cliente (evita path traversal).
type AppAsset struct {
	ID           uint     `gorm:"primaryKey"`
	AppVersionID uint     `gorm:"not null;index"`
	Platform     Platform `gorm:"not null"`
	// Arch (ex.: "amd64", "arm64") é texto livre — o conjunto de
	// arquiteturas relevantes varia bastante entre Linux/Windows/Android
	// e não vale a pena travar num enum agora.
	Arch string `gorm:"not null;default:amd64"`
	// Filename é o nome original do arquivo enviado, devolvido no
	// download via Content-Disposition — nunca usado para montar o
	// caminho físico em disco.
	Filename      string `gorm:"not null"`
	SHA256        string `gorm:"not null;index"`
	SizeBytes     int64  `gorm:"not null"`
	StoragePath   string `gorm:"not null"`
	DownloadCount int64  `gorm:"not null;default:0"`
	CreatedAt     time.Time

	AppVersion AppVersion `gorm:"foreignKey:AppVersionID"`
}

// AppAccess concede acesso explícito a um App com Visibility ==
// "restricted" para um User específico — lista de IDs simples, suficiente
// para 1-15 usuários sem inventar um sistema de grupos (ver PLAN.md §6.8).
type AppAccess struct {
	ID     uint `gorm:"primaryKey"`
	AppID  uint `gorm:"not null;uniqueIndex:idx_app_access_app_user"`
	UserID uint `gorm:"not null;uniqueIndex:idx_app_access_app_user"`

	App  App  `gorm:"foreignKey:AppID"`
	User User `gorm:"foreignKey:UserID"`
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
